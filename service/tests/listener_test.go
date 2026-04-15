package tests

import (
	"errors"
	"io"
	"net"
	"ratelimiter/service"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type errConn struct {
	closeErr error
}

func (c errConn) Read(_ []byte) (int, error)  { return 0, io.EOF }
func (c errConn) Write(_ []byte) (int, error) { return 0, io.ErrClosedPipe }
func (c errConn) Close() error                { return c.closeErr }
func (c errConn) LocalAddr() net.Addr         { return nil }
func (c errConn) RemoteAddr() net.Addr        { return nil }
func (c errConn) SetDeadline(_ time.Time) error {
	return nil
}
func (c errConn) SetReadDeadline(_ time.Time) error {
	return nil
}
func (c errConn) SetWriteDeadline(_ time.Time) error {
	return nil
}

func Test_LimitedConnection_Close_Error(t *testing.T) {
	s := make(chan struct{}, 1)
	s <- struct{}{}

	closeErr := errors.New("close boom")

	lc := service.NewLimitedConnection(s, errConn{closeErr: closeErr})

	err := lc.Close()
	require.Error(t, err)
	assert.ErrorIs(t, err, closeErr)
	assert.Contains(t, err.Error(), "close limited connection error")
	assert.Len(t, s, 0)
}

func Test_LimitListener_Accept_ReturnsErrorForNonTCPConn(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	lst := &stubListener{conn: serverConn}
	limit := service.NewLimitListener(lst, 0)

	conn, err := limit.Accept()
	require.Nil(t, conn)
	require.Error(t, err)
	require.Contains(t, err.Error(), "connection doesn't support CloseWrite()")
}

type stubListener struct {
	conn net.Conn
}

func (l *stubListener) Accept() (net.Conn, error) {
	return l.conn, nil
}

func (l *stubListener) Close() error {
	return nil
}

func (l *stubListener) Addr() net.Addr {
	return &net.IPAddr{}
}
