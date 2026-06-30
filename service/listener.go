package service

import (
	"errors"
	"fmt"
	"net"
	"sync"
)

type LimitedConnection struct {
	net.Conn
	sem chan struct{}
}

func NewLimitedConnection(sem chan struct{}, conn net.Conn) LimitedConnection {
	return LimitedConnection{
		Conn: conn,
		sem:  sem,
	}
}

func (c LimitedConnection) Close() error {
	<-c.sem // release slot on connection close
	err := c.Conn.Close()
	if err != nil {
		return fmt.Errorf("close limited connection error: %w", err)
	}
	return nil
}

type LimitListener struct {
	net.Listener
	maxConnections int
	sem            chan struct{}
	isClosed       bool
	mu             sync.Mutex
}

func NewLimitListener(listener net.Listener, maxConnections int) *LimitListener {
	return &LimitListener{
		Listener:       listener,
		maxConnections: maxConnections,
		sem:            make(chan struct{}, maxConnections),
		isClosed:       false,
		mu:             sync.Mutex{},
	}
}

func (l *LimitListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {

		if errors.Is(err, net.ErrClosed) {
			return nil, ErrClosed
		}

		if l.IsClosed() {
			return nil, ErrClosed
		}
		return nil, fmt.Errorf("limit connection accept error: %w", err)
	}

	// _ = conn.(*net.TCPConn).SetKeepAlive(true)

	select {
	case l.sem <- struct{}{}:
		// slot acquired successfully
		// return LimitedConnection{sem: l.sem, Conn: conn}, nil
		return NewLimitedConnection(l.sem, conn), nil
	default:

		tcpConn, ok := conn.(*net.TCPConn)
		if !ok {
			return nil, fmt.Errorf("connection doesn't support CloseWrite()")
		}

		// if the channel is already full, send a response that
		// too many connections are open and close the connection
		_, wrErr := tcpConn.Write([]byte(RESPONSE_ERROR_TOO_MANY_CONNECTIONS))

		// close the connection for writing;  client can still read
		closeErr := tcpConn.CloseWrite()
		return nil, errors.Join(ErrTooManyConnections, wrErr, closeErr)
	}
}

func (l *LimitListener) IsClosed() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.isClosed
}

func (l *LimitListener) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.isClosed = true
	return l.Listener.Close()
}
