package tests

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"ratelimiter/control"
	"ratelimiter/internal/config"
)

func TestControlServer_Health(t *testing.T) {
	logger := getDummyLogger()
	server := control.NewControlServer(config.Config{}, logger, &stubRouter{})

	request := httptest.NewRequest("GET", "/health", nil)
	recorder := httptest.NewRecorder()

	// call the request handler
	server.GetHttpServer().Handler.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code, "need 200 OK")
	require.Equal(t, "ok", recorder.Body.String())
}

func TestControlServer_Start(t *testing.T) {
	t.Run("returns error when listen fails", func(t *testing.T) {
		// get a random port
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		defer ln.Close()

		port := uint(ln.Addr().(*net.TCPAddr).Port)
		logger := getDummyLogger()
		// and try to start server on that port
		server := control.NewControlServer(config.Config{HttpPort: port}, logger, &stubRouter{})

		// expect start error
		err = server.Start()
		require.Error(t, err)
	})

	t.Run("shutdown", func(t *testing.T) {
		// we can't get actual port what gets server when it is configured with port 0.
		// So pick a free random port, then reuse it in server
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		port := uint(ln.Addr().(*net.TCPAddr).Port)
		require.NoError(t, ln.Close())

		logger := getDummyLogger()
		server := control.NewControlServer(config.Config{HttpPort: port}, logger, &stubRouter{})

		errCh := make(chan error, 1)
		go func() {
			errCh <- server.Start()
		}()

		url := fmt.Sprintf("http://127.0.0.1:%d/health", port)
		deadline := time.Now().Add(1 * time.Second)
		started := false

		doneCh := make(chan struct{})

		go func() {
			for time.Now().Before(deadline) {
				resp, reqErr := http.Get(url)
				if reqErr == nil {
					_ = resp.Body.Close()
					if resp.StatusCode == http.StatusOK {
						started = true
						doneCh <- struct{}{}
						break
					}
				}
				time.Sleep(20 * time.Millisecond)
			}
		}()

		select {
		case <-doneCh:
			require.True(t, started, "server did not start in time")
		case startErr := <-errCh:
			require.NoError(t, startErr)
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Start not started in time")
		}

		require.NoError(t, server.Shutdown())

		_, err = http.Get(url)
		require.Error(t, err, "server should be unavailable after shutdown")
	})
}

func TestMaxBodySizeMiddleware(t *testing.T) {
	t.Run("limit methods body size", func(t *testing.T) {
		var gotErr error
		var gotBody string

		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(r.Body)
			gotErr = err
			gotBody = string(body)
			w.WriteHeader(http.StatusOK)
		})

		handler := control.MaxBodySizeMiddleware(3)(next)
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("abcd"))
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var maxErr *http.MaxBytesError
		require.Error(t, gotErr)
		require.True(t, errors.As(gotErr, &maxErr))
		require.Equal(t, "abc", gotBody)
	})

	t.Run("does not limit non mutating methods body size", func(t *testing.T) {
		var gotErr error
		var gotBody string

		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(r.Body)
			gotErr = err
			gotBody = string(body)
			w.WriteHeader(http.StatusOK)
		})

		handler := control.MaxBodySizeMiddleware(3)(next)
		req := httptest.NewRequest(http.MethodGet, "/", strings.NewReader("abcd"))
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		require.NoError(t, gotErr)
		require.Equal(t, "abcd", gotBody)
	})
}
