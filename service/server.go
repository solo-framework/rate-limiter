package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"ratelimiter/internal/config"
	"ratelimiter/internal/logger"
)

type Server struct {
	listener        net.Listener
	ctx             context.Context
	cancelFn        context.CancelFunc
	logger          logger.ILogger
	config          *config.Config
	requestHandler  IRequestHandler
	tasksCh         chan net.Conn
	workersWg       sync.WaitGroup
	connWg          sync.WaitGroup
	shutdownStarted bool
	shutMutex       sync.Mutex
}

func NewServer(logger logger.ILogger, config *config.Config, requestHandler IRequestHandler) *Server {
	ctx, cancelFn := context.WithCancel(context.Background())

	return &Server{
		listener: nil,
		config:   config,
		// we shouldn't store context in struct (see https://go.dev/blog/context-and-structs)
		// but here we have very plain use case with context - just stopping service. No another actions.
		// So let's store.
		ctx:             ctx,
		cancelFn:        cancelFn,
		logger:          logger,
		tasksCh:         make(chan net.Conn, config.MaxConnections),
		workersWg:       sync.WaitGroup{},
		connWg:          sync.WaitGroup{},
		requestHandler:  requestHandler,
		shutdownStarted: false,
		shutMutex:       sync.Mutex{},
	}
}

func (s *Server) Start() error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", s.config.Port))
	if err != nil {
		return err
	}

	// replace listener with one that can limit the number of connections
	s.listener = NewLimitListener(listener, s.config.MaxConnections)

	s.startWorkers()
	go s.acceptConnections()
	return nil
}

func (s *Server) HealthCheck() bool {
	return true
}

func (s *Server) acceptConnections() {
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			conn, err := s.listener.Accept()
			if err != nil {
				if errors.Is(err, ErrTooManyConnections) {
					s.logger.Warn("too many connections")
					continue
				}

				// if listener (server) is stopped
				if errors.Is(err, ErrClosed) {
					return
				}

				s.logger.Error("server accept conn error", "error", err)
				continue
			}

			select {
			case s.tasksCh <- conn:
				// r.logger.Debug("conn is sent to channel")
			case <-s.ctx.Done():
				conn.Close()
			}
		}
	}
}

func (s *Server) startWorkers() {
	for i := 0; i < s.config.WorkerPoolSize; i++ {
		s.workersWg.Add(1)
		go func(i int) {
			defer s.workersWg.Done()

			s.logger.Debug("worker started", "id", i)
			for {
				select {
				case conn := <-s.tasksCh:
					s.handleConnection(conn)
				case <-s.ctx.Done():
					s.logger.Debug("worker stopped", "id", i)
					return // goroutine exit
				}
			}
		}(i)
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	s.connWg.Add(1)
	defer s.connWg.Done()

	defer func() {
		if reco := recover(); reco != nil {

			// something bad happened,
			s.logger.Error("panic in handleConnection", "error", reco)

			// tell the client that a server error occurred
			err := s.writeResponse(conn, []byte(RESPONSE_ERROR_INTERNAL), s.config.WriteTimeout)
			if err != nil {
				s.logger.Error("handleConnection defer write response error", "error", err)
			}
		}
		if err := conn.Close(); err != nil {
			s.logger.Error("handleConnection defer conn close error", "error", err)
		}
	}()

	// read data from the connection
	data, err := s.readFromConnection(conn, s.config.PacketSize, s.config.ReadTimeout)
	if bad := s.handleReadError(conn, err, s.config.WriteTimeout); bad {
		return
	}

	// process request
	response := s.requestHandler.Handle(data)

	// send response to client
	err = s.writeResponse(conn, []byte(response), s.config.WriteTimeout)
	if s.handleWriteError(err) {
		return
	}
}

func (s *Server) handleWriteError(err error) bool {
	if err != nil {
		// client did not read data; nothing else to do
		if errors.Is(err, ErrWriteTimeout) {
			return true
		}

		// another error besides timeout: log and exit,
		// because we can no longer send anything to the client
		s.logger.Error("write to connection error", "error", err)
		return true
	}
	return false
}

func (s *Server) handleReadError(conn net.Conn, err error, writeTimeout time.Duration) bool {
	if err != nil {

		// incoming package was too large, so we must read all client data and then close a connection
		// to avoid sending RST error to client:
		// 1. we got a large package
		// 2. send RESPONSE_ERROR_INVALID_DATA
		// 3. read all data from connection (clean connection)
		// 4. close the connection
		if errors.Is(err, ErrPacketTooLarge) {
			we := s.writeResponse(conn, []byte(RESPONSE_ERROR_INVALID_DATA), writeTimeout)
			if we != nil {
				s.logger.Error("handleReadError.writeResponse invalid data error", "error", err)
			}

			s.cleanConnection(conn, s.config.ReadTimeout)
			return true
		}

		// if reading timed out (likely client didn't close write side after send),
		// send a timeout error to the client
		if errors.Is(err, ErrReadTimeout) {
			we := s.writeResponse(conn, []byte(RESPONSE_ERROR_READ_TIMEOUT), writeTimeout)
			if we != nil {
				s.logger.Error("handleReadError.writeResponse timeout error", "error", err)
			}
			return true
		}

		// some other read error happened; we failed to read from connection,
		// send ERROR_INTERNAL to the client
		s.logger.Error("handleReadError", "error", err)
		we := s.writeResponse(conn, []byte(RESPONSE_ERROR_INTERNAL), s.config.WriteTimeout)
		if we != nil {
			s.logger.Error("handleReadError.writeResponse internal error", "error", err)
		}
		return true
	}

	return false
}

func (s *Server) writeResponse(conn net.Conn, data []byte, writeTimeout time.Duration) error {
	err := conn.SetWriteDeadline(time.Now().Add(writeTimeout))
	if err != nil {
		return fmt.Errorf("writeResponse set write deadline: %w", err)
	}

	_, err = conn.Write(data)
	if err != nil {
		// This code is unlikely to be achievable, because Write actually writes to the kernel buffer
		// rather than directly to the client, and the write operation will almost always be successful.
		// To catch this timeout the server must write a LOT of data and the client must not read anything at all.
		// It is impossible in our case.

		var i interface{ Timeout() bool }
		if errors.As(err, &i) && i.Timeout() {
			return ErrWriteTimeout
		}
		return fmt.Errorf("writeResponse write: %w", err)
	}
	return nil
}

// readFromConnection reads data from connection and returns it
// if error occurs, it returns nil and error
// if read timeout occurs, it returns nil and ErrReadTimeout

func (s *Server) readFromConnection(conn net.Conn, size int, readTimeout time.Duration) ([]byte, error) {
	err := conn.SetReadDeadline(time.Now().Add(readTimeout))
	if err != nil {
		return nil, fmt.Errorf("readFromConnection set read deadline: %w", err)
	}

	// we don't know was package bigger then package_size, so read +1
	limitReader := io.LimitReader(conn, int64(size+1))
	data, err := io.ReadAll(limitReader)
	if err != nil {
		var i interface{ Timeout() bool }

		if errors.As(err, &i) && i.Timeout() {
			return nil, ErrReadTimeout
		}
		return nil, fmt.Errorf("readFromConnection read: %w", err)
	}
	// if received data is too long raise ErrPacketTooLarge
	if len(data) > size {
		return nil, ErrPacketTooLarge
	}
	return data, nil
}

// / cleanConnection
func (s *Server) cleanConnection(conn net.Conn, readTimeout time.Duration) {
	err := conn.SetReadDeadline(time.Now().Add(readTimeout))
	if err != nil {
		s.logger.Error("cleanConnection set read deadline error", "error", err)
		return
	}

	_, err = io.Copy(io.Discard, conn)
	if err == nil || errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF) {
		return
	}

	var i interface{ Timeout() bool }
	if errors.As(err, &i) && i.Timeout() {
		return
	}

	s.logger.Error("cleanConnection read error", "error", err)
}

func (s *Server) Shutdown() error {
	s.shutMutex.Lock()
	if s.shutdownStarted {
		s.logger.Warn("server is already shutting down")
		return nil
	}
	s.shutdownStarted = true
	s.shutMutex.Unlock()

	// cancel context to notify all goroutines about shutdown
	s.cancelFn()

	err := s.listener.Close()
	if err != nil {
		s.logger.Error("listener close error", "error", err)
	}

	done := make(chan struct{})
	go func() {
		s.workersWg.Wait()
		s.connWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// all connections are closed gracefully
		return nil
	case <-time.After(time.Duration(s.config.ShutdownTimeout) * time.Millisecond):
		// all connections are closed by timeout
		return ErrShutdown
	}
}
