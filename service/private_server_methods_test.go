package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"ratelimiter/internal/config"

	"github.com/stretchr/testify/require"
)

type noopLogger struct{}

func (l *noopLogger) With(args ...any) *slog.Logger   { return nil }
func (l *noopLogger) Info(msg string, fields ...any)  {}
func (l *noopLogger) Error(msg string, fields ...any) {}
func (l *noopLogger) Warn(msg string, fields ...any)  {}
func (l *noopLogger) Debug(msg string, fields ...any) {}

type countingLogger struct {
	errorCount int
}

func (l *countingLogger) With(args ...any) *slog.Logger  { return nil }
func (l *countingLogger) Info(msg string, fields ...any) {}
func (l *countingLogger) Error(msg string, fields ...any) {
	l.errorCount++
}
func (l *countingLogger) Warn(msg string, fields ...any)  {}
func (l *countingLogger) Debug(msg string, fields ...any) {}

type timeoutErr struct{}

func (timeoutErr) Error() string   { return "timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return true }

type fakeConn struct {
	readBuf    *bytes.Reader
	writeBuf   bytes.Buffer
	readErr    error
	writeErr   error
	readDLErr  error
	writeDLErr error
	closeErr   error
	closed     bool
	closedCh   chan struct{}
	readDL     time.Time
	writeDL    time.Time
	deadline   time.Time
	localAddr  net.Addr
	remoteAddr net.Addr
}

func (c *fakeConn) Read(p []byte) (int, error) {
	if c.readErr != nil {
		return 0, c.readErr
	}
	if c.readBuf == nil {
		return 0, io.EOF
	}
	return c.readBuf.Read(p)
}

func (c *fakeConn) Write(p []byte) (int, error) {
	if c.writeErr != nil {
		return 0, c.writeErr
	}
	return c.writeBuf.Write(p)
}

func (c *fakeConn) Close() error {
	c.closed = true
	if c.closedCh != nil {
		close(c.closedCh)
	}
	return c.closeErr
}

func (c *fakeConn) LocalAddr() net.Addr  { return c.localAddr }
func (c *fakeConn) RemoteAddr() net.Addr { return c.remoteAddr }

func (c *fakeConn) SetDeadline(t time.Time) error {
	c.deadline = t
	return nil
}

func (c *fakeConn) SetReadDeadline(t time.Time) error {
	c.readDL = t
	return c.readDLErr
}

func (c *fakeConn) SetWriteDeadline(t time.Time) error {
	c.writeDL = t
	return c.writeDLErr
}

type acceptResult struct {
	conn net.Conn
	err  error
}

type fakeListener struct {
	results []acceptResult
	index   int
	closed  bool
}

func (l *fakeListener) Accept() (net.Conn, error) {
	if l.index < len(l.results) {
		res := l.results[l.index]
		l.index++
		return res.conn, res.err
	}
	if l.closed {
		return nil, ErrClosed
	}
	return nil, ErrClosed
}

func (l *fakeListener) Close() error {
	l.closed = true
	return nil
}

func (l *fakeListener) Addr() net.Addr {
	return &net.IPAddr{}
}

type stubHandler struct {
	received []byte
	response []byte
	panicOn  bool
}

func (h *stubHandler) Handle(data []byte) []byte {
	if h.panicOn {
		panic("boom")
	}
	h.received = append([]byte(nil), data...)
	return h.response
}

func Test_handleConnection_success(t *testing.T) {
	handler := &stubHandler{response: []byte(RESPONSE_OK)}
	srv := &Server{
		logger:         &noopLogger{},
		config:         &config.Config{PacketSize: 64, ReadTimeout: 50 * time.Millisecond, WriteTimeout: 50 * time.Millisecond},
		requestHandler: handler,
	}

	conn := &fakeConn{readBuf: bytes.NewReader([]byte("hello"))}

	srv.handleConnection(conn)

	require.Equal(t, []byte("hello"), handler.received)
	require.Equal(t, RESPONSE_OK, conn.writeBuf.String())
	require.True(t, conn.closed)
}

func Test_handleConnection_readTimeout(t *testing.T) {
	handler := &stubHandler{panicOn: true}
	srv := &Server{
		logger:         &noopLogger{},
		config:         &config.Config{PacketSize: 64, ReadTimeout: 50 * time.Millisecond, WriteTimeout: 50 * time.Millisecond},
		requestHandler: handler,
	}

	conn := &fakeConn{readErr: timeoutErr{}}

	srv.handleConnection(conn)

	require.Nil(t, handler.received)
	require.Equal(t, RESPONSE_ERROR_READ_TIMEOUT, conn.writeBuf.String())
	require.True(t, conn.closed)
}

func Test_handleConnection_handlerPanic(t *testing.T) {
	handler := &stubHandler{panicOn: true}
	srv := &Server{
		logger:         &noopLogger{},
		config:         &config.Config{PacketSize: 64, ReadTimeout: 50 * time.Millisecond, WriteTimeout: 50 * time.Millisecond},
		requestHandler: handler,
	}

	conn := &fakeConn{readBuf: bytes.NewReader([]byte("hello"))}

	srv.handleConnection(conn)

	require.Equal(t, RESPONSE_ERROR_INTERNAL, conn.writeBuf.String())
	require.True(t, conn.closed)
}

func Test_handleReadError_internalErrorWritesResponse(t *testing.T) {
	srv := &Server{
		logger: &noopLogger{},
		config: &config.Config{WriteTimeout: 50 * time.Millisecond},
	}

	client, serverConn := net.Pipe()
	defer client.Close()
	defer serverConn.Close()

	respCh := make(chan []byte, 1)
	go func() {
		buf := make([]byte, 64)
		n, _ := client.Read(buf)
		respCh <- buf[:n]
	}()

	ok := srv.handleReadError(serverConn, errors.New("boom"), 10*time.Millisecond)
	require.True(t, ok)

	select {
	case resp := <-respCh:
		require.Equal(t, RESPONSE_ERROR_INTERNAL, string(resp))
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timeout waiting for response")
	}
}

func Test_handleReadError_readTimeoutWritesResponse(t *testing.T) {
	srv := &Server{
		logger: &noopLogger{},
		config: &config.Config{WriteTimeout: 50 * time.Millisecond},
	}

	client, serverConn := net.Pipe()
	defer client.Close()
	defer serverConn.Close()

	respCh := make(chan []byte, 1)
	go func() {
		buf := make([]byte, 64)
		n, _ := client.Read(buf)
		respCh <- buf[:n]
	}()

	ok := srv.handleReadError(serverConn, ErrReadTimeout, 10*time.Millisecond)
	require.True(t, ok)

	select {
	case resp := <-respCh:
		require.Equal(t, RESPONSE_ERROR_READ_TIMEOUT, string(resp))
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timeout waiting for response")
	}
}

func Test_handleReadError_noErrorNoResponse(t *testing.T) {
	srv := &Server{
		logger: &noopLogger{},
		config: &config.Config{WriteTimeout: 50 * time.Millisecond},
	}

	client, serverConn := net.Pipe()
	defer client.Close()
	defer serverConn.Close()

	ok := srv.handleReadError(serverConn, nil, 10*time.Millisecond)
	require.False(t, ok)

	_ = client.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
	buf := make([]byte, 64)
	n, err := client.Read(buf)
	require.Error(t, err)
	require.Equal(t, 0, n)
}

func Test_acceptConnections_enqueuesAndStopsOnClosed(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	conn := &fakeConn{}
	listener := &fakeListener{
		results: []acceptResult{
			{conn: conn, err: nil},
			{conn: nil, err: ErrTooManyConnections},
			{conn: nil, err: ErrClosed},
		},
	}

	srv := &Server{
		listener: listener,
		ctx:      ctx,
		cancelFn: cancel,
		logger:   &noopLogger{},
		tasksCh:  make(chan net.Conn, 1),
	}

	done := make(chan struct{})
	go func() {
		srv.acceptConnections()
		close(done)
	}()

	select {
	case got := <-srv.tasksCh:
		require.Equal(t, conn, got)
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timeout waiting for accepted connection")
	}

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timeout waiting for acceptConnections to stop")
	}
}

func Test_startWorkers_handlesConnectionFromTasks(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	handler := &stubHandler{response: []byte(RESPONSE_OK)}
	srv := &Server{
		logger:         &noopLogger{},
		config:         &config.Config{WorkerPoolSize: 1, PacketSize: 64, ReadTimeout: 50 * time.Millisecond, WriteTimeout: 50 * time.Millisecond},
		requestHandler: handler,
		tasksCh:        make(chan net.Conn, 1),
		ctx:            ctx,
		cancelFn:       cancel,
	}

	srv.startWorkers()

	conn := &fakeConn{
		readBuf:  bytes.NewReader([]byte("ping")),
		closedCh: make(chan struct{}),
	}
	srv.tasksCh <- conn

	select {
	case <-conn.closedCh:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timeout waiting for connection to close")
	}
	require.Equal(t, RESPONSE_OK, conn.writeBuf.String())

	cancel()
	srv.workersWg.Wait()
}

func Test_handleWriteError_noErrorReturnsFalse(t *testing.T) {
	srv := &Server{
		logger: &noopLogger{},
	}

	ok := srv.handleWriteError(nil)
	require.False(t, ok)
}

func Test_handleWriteError_timeoutReturnsTrueNoLog(t *testing.T) {
	logger := &countingLogger{}
	srv := &Server{
		logger: logger,
	}

	ok := srv.handleWriteError(ErrWriteTimeout)
	require.True(t, ok)
	require.Equal(t, 0, logger.errorCount)
}

func Test_handleWriteError_otherErrorReturnsTrueLogs(t *testing.T) {
	logger := &countingLogger{}
	srv := &Server{
		logger: logger,
	}

	ok := srv.handleWriteError(errors.New("boom"))
	require.True(t, ok)
	require.Equal(t, 1, logger.errorCount)
}

func Test_writeResponse_successWritesAndSetsDeadline(t *testing.T) {
	srv := &Server{}
	conn := &fakeConn{}

	start := time.Now()
	err := srv.writeResponse(conn, []byte("ok"), 50*time.Millisecond)
	require.NoError(t, err)
	require.Equal(t, "ok", conn.writeBuf.String())
	require.True(t, conn.writeDL.After(start))
}

func Test_writeResponse_setWriteDeadlineError(t *testing.T) {
	srv := &Server{}
	conn := &fakeConn{writeDLErr: errors.New("deadline")}

	err := srv.writeResponse(conn, []byte("ok"), 50*time.Millisecond)
	require.Error(t, err)
	require.Contains(t, err.Error(), "writeResponse set write deadline")
	require.Equal(t, "", conn.writeBuf.String())
}

func Test_writeResponse_writeTimeout(t *testing.T) {
	srv := &Server{}
	conn := &fakeConn{writeErr: timeoutErr{}}

	err := srv.writeResponse(conn, []byte("ok"), 50*time.Millisecond)
	require.ErrorIs(t, err, ErrWriteTimeout)
}

func Test_writeResponse_writeOtherError(t *testing.T) {
	srv := &Server{}
	conn := &fakeConn{writeErr: errors.New("boom")}

	err := srv.writeResponse(conn, []byte("ok"), 50*time.Millisecond)
	require.Error(t, err)
	require.Contains(t, err.Error(), "writeResponse write")
}

func Test_readFromConnection_successLimited(t *testing.T) {
	srv := &Server{}
	conn := &fakeConn{readBuf: bytes.NewReader([]byte("hello"))}

	start := time.Now()
	data, err := srv.readFromConnection(conn, 5, 50*time.Millisecond)
	require.NoError(t, err)
	require.Equal(t, []byte("hello"), data)
	require.True(t, conn.readDL.After(start))
}

func Test_readFromConnection_setReadDeadlineError(t *testing.T) {
	srv := &Server{}
	conn := &fakeConn{readDLErr: errors.New("deadline")}

	data, err := srv.readFromConnection(conn, 5, 50*time.Millisecond)
	require.Error(t, err)
	require.Contains(t, err.Error(), "readFromConnection set read deadline")
	require.Nil(t, data)
}

func Test_readFromConnection_readTimeout(t *testing.T) {
	srv := &Server{}
	conn := &fakeConn{readErr: timeoutErr{}}

	data, err := srv.readFromConnection(conn, 5, 50*time.Millisecond)
	require.ErrorIs(t, err, ErrReadTimeout)
	require.Nil(t, data)
}

func Test_readFromConnection_readOtherError(t *testing.T) {
	srv := &Server{}
	conn := &fakeConn{readErr: errors.New("boom")}

	data, err := srv.readFromConnection(conn, 5, 50*time.Millisecond)
	require.Error(t, err)
	require.Contains(t, err.Error(), "readFromConnection read")
	require.Nil(t, data)
}

func Test_readFromConnection_packetTooLarge(t *testing.T) {
	srv := &Server{logger: &noopLogger{}}
	conn := &fakeConn{readBuf: bytes.NewReader([]byte("hello world"))}

	data, err := srv.readFromConnection(conn, 5, 50*time.Millisecond)
	require.ErrorIs(t, err, ErrPacketTooLarge)
	require.Nil(t, data)
}
