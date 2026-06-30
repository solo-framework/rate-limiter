package tests

import (
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"ratelimiter/service"
)

func getRandomPort(t *testing.T) uint {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := uint(ln.Addr().(*net.TCPAddr).Port)
	require.NoError(t, ln.Close())
	return port
}

type client struct {
	conn *net.TCPConn
}

func newClient(addr net.Addr) *client {
	conn, err := net.Dial("tcp", addr.String())
	if err != nil {
		panic(err)
	}

	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		panic("can't convert to *net.TCPConn")
	}

	return &client{
		conn: tcpConn,
	}
}

func (c *client) BadWrite(data []byte) {
	// client writes incorrectly: does not close socket for writing after sending data
	// server cannot fully read the request and should fail by read timeout
	_, err := c.conn.Write(data)
	if err != nil {
		panic(err)
	}
}

func (c *client) Write(data []byte) error {
	var err error

	// close socket for writing so server receives EOF
	// and stops waiting for more data from connection
	defer func() {
		err = c.conn.CloseWrite()
	}()

	_, err = c.conn.Write(data)
	return err
}

func (c *client) Read() ([]byte, error) {
	data, err := io.ReadAll(c.conn)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (c *client) Close() error {
	return c.conn.Close()
}

// --------------------------

type handlerWithDelay struct {
	delayMs int
}

func NewHandlerWithDelay(delayMs int) *handlerWithDelay {
	return &handlerWithDelay{
		delayMs: delayMs,
	}
}

func (h *handlerWithDelay) Handle(data []byte) []byte {
	time.Sleep(time.Millisecond * time.Duration(h.delayMs))
	return data
}

// --------------------------

type dummyHandler struct{}

func newDummyHandler() *dummyHandler {
	return &dummyHandler{}
}

func (h dummyHandler) Handle(data []byte) []byte {
	return data
}

type panicHandler struct{}

func newPanicHandler() *panicHandler {
	return &panicHandler{}
}

func (h panicHandler) Handle(data []byte) []byte {
	panic("some panic happend")
}

// --------------------------

type dummyLogger struct{}

// With implements [ratelimiter.ILogger].
func (l *dummyLogger) With(args ...any) *slog.Logger {
	panic("unimplemented")
}

func newLogger() *dummyLogger {
	return &dummyLogger{}
}

func (l dummyLogger) Info(msg string, fields ...any) {
	// println(msg)
}

func (l dummyLogger) Error(msg string, fields ...any) {
	// println(msg)
}

func (l dummyLogger) Warn(msg string, fields ...any) {
	// println(msg)
}

func (l dummyLogger) Debug(msg string, fields ...any) {
	// println("DEBUG LOG FROM TEST:", msg)
}

// // --------------------------

func getTimeProvider() *service.TimeProvider {
	return &service.TimeProvider{}
}

// with fixed value
type fixedTimeProvider struct {
	now int64
}

func newFixedTimeProvider(val int64) *fixedTimeProvider {
	return &fixedTimeProvider{
		now: val,
	}
}

func (f *fixedTimeProvider) Now() int64 {
	return f.now
}

type incrementTimeProvider struct {
	count int64
}

func newIncrementTimeProvider() *incrementTimeProvider {
	return &incrementTimeProvider{count: 0}
}

func (f *incrementTimeProvider) Now() int64 {
	f.count++
	return f.count
}
