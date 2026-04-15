package tests

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"ratelimiter/control"
	"ratelimiter/internal/config"
	"ratelimiter/internal/logtest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRouter_GetHandlers(t *testing.T) {
	cm := createControlMethods(t)
	cfg := config.Config{
		HttpSecret:    "secret",
		BasicAuthUser: "user",
		BasicAuthPass: "pass",
	}
	router := control.NewRouter(*cm, cfg, getDummyLogger())
	handlers := router.GetHandlers()

	expectedRoutes := []string{
		"GET /stat",
		"GET /info",
		"POST /group_add",
		"POST /group_delete",
		"POST /group_update",
		"POST /enable_prof",
		"POST /disable_prof",
		"GET /debug/pprof/",
		"GET /debug/pprof/cmdline",
		"GET /debug/pprof/profile",
		"GET /debug/pprof/symbol",
		"GET /debug/pprof/trace",
	}

	require.Len(t, handlers, len(expectedRoutes))

	for _, route := range expectedRoutes {
		require.Contains(t, handlers, route)
	}
}

func TestRouter_Auth(t *testing.T) {
	cm := createControlMethods(t)
	cfg := config.Config{HttpSecret: "secret"}
	router := control.NewRouter(*cm, cfg, getDummyLogger())

	t.Run("request with good X-Auth-Token", func(t *testing.T) {

		req := httptest.NewRequest(http.MethodGet, "/stat", nil)
		req.Header.Add("X-Auth-Token", "secret")
		recorder := httptest.NewRecorder()
		logger, _ := logtest.NewTestLogger()

		handler := router.GetHandlers()["GET /stat"]
		call := addTrackingMiddleware(logger, handler)
		call.ServeHTTP(recorder, req)

		res := decodeJSON(t, recorder.Body.Bytes())

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, res["error"], false)
		require.NotEmpty(t, recorder.Header().Get("X-Track-Id"))
	})

	t.Run("request without X-Auth-Token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/stat", nil)
		recorder := httptest.NewRecorder()
		logger, logH := logtest.NewTestLogger()

		handler := router.GetHandlers()["GET /stat"]
		call := addTrackingMiddleware(logger, handler)
		call.ServeHTTP(recorder, req)

		require.Equal(t, http.StatusUnauthorized, recorder.Code)
		logtest.AssertLogContainsMessage(t, logH, "access denied")
		require.Equal(t, "access denied\n", recorder.Body.String())
		require.NotEmpty(t, recorder.Header().Get("X-Track-Id"))
	})

	t.Run("request with wrong X-Auth-Token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/stat", nil)
		rec := httptest.NewRecorder()
		logger, logH := logtest.NewTestLogger()

		req.Header.Add("X-Auth-Token", "wrong pass")

		handler := router.GetHandlers()["GET /stat"]
		call := addTrackingMiddleware(logger, handler)
		call.ServeHTTP(rec, req)

		require.Equal(t, http.StatusUnauthorized, rec.Code)
		require.Equal(t, "access denied\n", rec.Body.String())
		require.NotEmpty(t, rec.Header().Get("X-Track-Id"))
		logtest.AssertLogContainsMessage(t, logH, "access denied")
	})

}

func TestRouter_BasicAuth(t *testing.T) {

	cm := createControlMethods(t)
	cfg := config.Config{
		// HttpSecret:    "secret",
		BasicAuthUser: "user",
		BasicAuthPass: "password",
	}
	router := control.NewRouter(*cm, cfg, getDummyLogger())

	t.Run("requires basic auth", func(t *testing.T) {

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		called := false
		handler := router.BasicAuthMiddleware(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
			}),
		)

		handler(rec, req)

		require.Equal(t, http.StatusUnauthorized, rec.Code)
		require.False(t, called)
	})

	t.Run("requires basic auth OK", func(t *testing.T) {

		called := false
		handler := router.BasicAuthMiddleware(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
			}),
		)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		req.SetBasicAuth("user", "password")

		handler(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		require.True(t, called)
	})

	t.Run("requires basic auth WRONG PASS", func(t *testing.T) {

		called := false
		handler := router.BasicAuthMiddleware(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
			}),
		)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		req.SetBasicAuth("user", "wrong password")

		handler(rec, req)

		require.Equal(t, http.StatusUnauthorized, rec.Code)
		require.False(t, called)
	})
}

func TestRouter_MainHandler(t *testing.T) {

	cm := createControlMethods(t)
	cfg := config.Config{HttpSecret: "secret"}
	router := control.NewRouter(*cm, cfg, getDummyLogger())

	t.Run("handle panic", func(t *testing.T) {

		logger, logH := logtest.NewTestLogger()

		called := false
		main := router.MainHandler(func(r *http.Request) (any, error) {
			b := 0
			_ = 1 / b // panic!

			called = true
			return "ok", nil
		}, true)

		handler := addTrackingMiddleware(logger, main)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Auth-Token", "secret")

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		require.Equal(t, http.StatusInternalServerError, rec.Code)
		assert.False(t, called)
		require.NotEmpty(t, rec.Header().Get("X-Track-Id"))

		logtest.AssertLogContainsMessage(t, logH, "[PANIC]")
		logtest.AssertLogContainsAttr(t, logH, "panic", "integer divide by zero")

	})

	t.Run("requires auth token", func(t *testing.T) {
		logger, logH := logtest.NewTestLogger()

		called := false
		main := router.MainHandler(func(r *http.Request) (any, error) {
			called = true
			return "ok", nil
		}, true)
		handler := addTrackingMiddleware(logger, main)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		require.Equal(t, http.StatusUnauthorized, rec.Code)
		require.Equal(t, "access denied\n", rec.Body.String())
		require.NotEmpty(t, rec.Header().Get("X-Track-Id"))
		require.False(t, called)
		logtest.AssertLogContainsMessage(t, logH, "access denied")
	})

	t.Run("returns success response", func(t *testing.T) {

		logger, _ := logtest.NewTestLogger()

		main := router.MainHandler(func(r *http.Request) (any, error) {
			return map[string]any{"value": "ok"}, nil
		}, true)

		handler := addTrackingMiddleware(logger, main)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Auth-Token", "secret")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, "application/json", rec.Header().Get("Content-Type"))
		require.NotEmpty(t, rec.Header().Get("X-Track-Id"))

		out := decodeJSON(t, rec.Body.Bytes())
		require.Equal(t, false, out["error"])
		require.Equal(t, "ok", out["data"].(map[string]any)["value"])

	})

	t.Run("returns managed error response", func(t *testing.T) {

		handler := router.MainHandler(func(r *http.Request) (any, error) {
			return nil, control.NewValidationError("bad input", errors.New("inner"))
		}, false)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		handler(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
		require.Equal(t, "application/json", rec.Header().Get("Content-Type"))
		// require.NotEmpty(t, rec.Header().Get("X-Track-Id"))

		out := decodeJSON(t, rec.Body.Bytes())
		require.Equal(t, true, out["error"])
		require.Equal(t, "bad input", out["data"])
	})

	t.Run("returns managed error without wrapped error", func(t *testing.T) {

		logger, logH := logtest.NewTestLogger()
		router = control.NewRouter(*cm, cfg, logger)

		main := router.MainHandler(func(r *http.Request) (any, error) {
			return nil, control.NewLogicError("some message for user", nil, http.StatusConflict)
		}, false)

		handler := addTrackingMiddleware(logger, main)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		require.Equal(t, http.StatusConflict, rec.Code)
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
		assert.NotEmpty(t, rec.Header().Get("X-Track-Id"))

		out := decodeJSON(t, rec.Body.Bytes())
		assert.Equal(t, true, out["error"])
		assert.Equal(t, "some message for user", out["data"])

		assert.Equal(t, 2, logH.Count())
		logtest.AssertLogContainsMessage(t, logH, "some message for user")
		logtest.AssertLogContainsAttr(t, logH, "error", "some message for user")
	})

	t.Run("returns managed error with wrapped error", func(t *testing.T) {

		// logger := getLoggerWithRecord()
		logger, logH := logtest.NewTestLogger()
		router = control.NewRouter(*cm, cfg, logger)

		main := router.MainHandler(func(r *http.Request) (any, error) {
			return nil, control.NewLogicError("some message for user", io.ErrClosedPipe, http.StatusConflict)
		}, false)

		handler := addTrackingMiddleware(logger, main)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		require.Equal(t, http.StatusConflict, rec.Code)
		require.Equal(t, "application/json", rec.Header().Get("Content-Type"))
		require.NotEmpty(t, rec.Header().Get("X-Track-Id"))

		out := decodeJSON(t, rec.Body.Bytes())
		assert.Equal(t, true, out["error"])
		assert.Equal(t, "some message for user", out["data"])

		assert.Equal(t, 2, logH.Count())

		logtest.AssertLogContainsMessage(t, logH, "some message for user")
		logtest.AssertLogContainsAttr(t, logH, "error", io.ErrClosedPipe.Error())
	})

	t.Run("returns unhandled error", func(t *testing.T) {

		logger, logH := logtest.NewTestLogger()
		router = control.NewRouter(*cm, cfg, logger)

		main := router.MainHandler(func(r *http.Request) (any, error) {
			return nil, fmt.Errorf("error from vendor lib: %w", io.ErrShortBuffer)
		}, false)

		handler := addTrackingMiddleware(logger, main)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		// check response
		require.Equal(t, http.StatusInternalServerError, rec.Code)
		assert.Equal(t, "text/plain; charset=utf-8", rec.Header().Get("Content-Type")) // plain text, no json
		assert.NotEmpty(t, rec.Header().Get("X-Track-Id"))
		out := rec.Body.String()
		assert.Equal(t, "Internal Server Error\n", out)

		// check logs
		logtest.AssertLogContainsAttr(t, logH, "error", "error from vendor lib: short buffer")
		logtest.AssertLogContainsMessage(t, logH, "unhandled error")
	})

	t.Run("SendResponse json decode error", func(t *testing.T) {

		logger, logH := logtest.NewTestLogger()
		router = control.NewRouter(*cm, cfg, logger)

		main := router.MainHandler(func(r *http.Request) (any, error) {
			return func() {}, nil // this causes a JSON conversion error
		}, false)

		handler := addTrackingMiddleware(logger, main)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		logtest.AssertLogContainsMessage(t, logH, "SendResponse decode error")
		logtest.AssertLogContainsAttr(t, logH, "error", "json: unsupported type: func()")

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
		assert.NotEmpty(t, rec.Header().Get("X-Track-Id"))
		assert.Equal(t, "Internal Server Error\n", rec.Body.String())
		assert.Equal(t, "text/plain; charset=utf-8", rec.Header().Get("Content-Type")) // plain text, no json
	})

	t.Run("SendError json decode error", func(t *testing.T) {

		// log := getLoggerWithRecord()
		log, logH := logtest.NewTestLogger()
		router = control.NewRouter(*cm, cfg, log)

		main := router.MainHandler(func(r *http.Request) (any, error) {
			return nil, control.NewValidationError("bad input", io.ErrUnexpectedEOF)
		}, false)

		handler := addTrackingMiddleware(log, main)

		// test Router  'json decode error' branch in a non-trivial way
		// err = s.SendError(w, err, me.GetStatus())
		// if errors.Is(err, ErrJsonDecode) {
		// err here cannot trigger ErrJsonDecode because error is stringified.
		// use another way: return write error from w.Write(out)
		// with fake writer whose Write returns ErrJsonDecode
		// so we hit the needed code branch

		rec := &writerWithError{Err: control.ErrJsonDecode}
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		handler.ServeHTTP(rec, req)

		require.Equal(t, http.StatusInternalServerError, rec.Сode)
		assert.NotEmpty(t, rec.Header().Get("X-Track-Id"))

		logtest.AssertLogContainsMessage(t, logH, "SendError decode error")
		logtest.AssertLogContainsAttr(t, logH, "error", "json decode error")
		logtest.AssertLogContainsAttr(t, logH, "track_id", rec.Header().Get("X-Track-Id"))
	})

	t.Run("sendError write error", func(t *testing.T) {

		logger, logH := logtest.NewTestLogger()
		router = control.NewRouter(*cm, cfg, logger)

		main := router.MainHandler(func(r *http.Request) (any, error) {
			return nil, control.NewValidationError("bad input", io.ErrUnexpectedEOF)
		}, false)
		handler := addTrackingMiddleware(logger, main)

		rec := &writerWithError{Err: control.ErrWriteResponse}
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		handler.ServeHTTP(rec, req)

		logtest.AssertLogContainsMessage(t, logH, "SendError write error")
		logtest.AssertLogContainsAttr(t, logH, "error", "write response error")
		logtest.AssertLogContainsAttr(t, logH, "track_id", rec.Header().Get("X-Track-Id"))

	})

	t.Run("sendResponse write error", func(t *testing.T) {

		logger, logH := logtest.NewTestLogger()
		router = control.NewRouter(*cm, cfg, logger)

		main := router.MainHandler(func(r *http.Request) (any, error) {
			return "ok", nil
		}, false)
		handler := addTrackingMiddleware(logger, main)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := &writerWithError{Err: io.ErrClosedPipe}
		handler.ServeHTTP(rec, req)

		assert.NotEmpty(t, rec.Header().Get("X-Track-Id"))

		logtest.AssertLogContainsMessage(t, logH, "SendResponse write error")
		logtest.AssertLogContainsAttr(t, logH, "error", "some writer error")
		logtest.AssertLogContainsAttr(t, logH, "track_id", rec.Header().Get("X-Track-Id"))
		logtest.AssertLogContainsAttr(t, logH, "error", "write response error")

	})

}

func TestRouter_SendError(t *testing.T) {
	cm := createControlMethods(t)
	cfg := config.Config{HttpSecret: "secret"}
	logger := getDummyLogger()
	router := control.NewRouter(*cm, cfg, logger)

	t.Run("json decode error", func(t *testing.T) {
		rec := httptest.NewRecorder()
		err := router.SendError(rec, func() {}, http.StatusOK)

		require.ErrorIs(t, err, control.ErrJsonDecode)
		require.NotErrorIs(t, err, control.ErrWriteResponse)
		assert.Contains(t, err.Error(), "json: unsupported type: func()")
	})

	t.Run("http writer error", func(t *testing.T) {
		wrt := &writerWithError{Err: io.ErrClosedPipe}
		err := router.SendError(wrt, "data", http.StatusOK)

		require.ErrorIs(t, err, control.ErrWriteResponse)
		require.NotErrorIs(t, err, control.ErrJsonDecode)
		assert.Contains(t, err.Error(), "some writer error")
	})

	t.Run("nil", func(t *testing.T) {
		rec := httptest.NewRecorder()
		err := router.SendError(rec, nil, 999)
		require.NoError(t, err)
		out := decodeJSON(t, rec.Body.Bytes())

		require.Equal(t, 999, rec.Code)
		require.Equal(t, "application/json", rec.Header().Get("Content-Type"))
		assert.Equal(t, "{}", out["data"])
		assert.Equal(t, true, out["error"])
	})
}

func TestRouter_SendResponse(t *testing.T) {

	cm := createControlMethods(t)
	cfg := config.Config{HttpSecret: "secret"}
	router := control.NewRouter(*cm, cfg, getDummyLogger())

	t.Run("OK", func(t *testing.T) {
		rec := httptest.NewRecorder()
		err := router.SendResponse(rec, "some data", http.StatusOK)
		require.NoError(t, err)

		out := decodeJSON(t, rec.Body.Bytes())
		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, "application/json", rec.Header().Get("Content-Type"))
		assert.Equal(t, "some data", out["data"])
		assert.Equal(t, false, out["error"])
	})

	t.Run("nil", func(t *testing.T) {
		rec := httptest.NewRecorder()
		err := router.SendResponse(rec, nil, http.StatusOK)
		require.NoError(t, err)
		out := decodeJSON(t, rec.Body.Bytes())

		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, "application/json", rec.Header().Get("Content-Type"))
		assert.Equal(t, "{}", out["data"])
		assert.Equal(t, false, out["error"])
	})

	t.Run("JSON encode error", func(t *testing.T) {

		rec := httptest.NewRecorder()
		err := router.SendResponse(rec, func() {}, http.StatusOK)

		require.ErrorIs(t, err, control.ErrJsonDecode)
		require.NotErrorIs(t, err, control.ErrWriteResponse)
		assert.Contains(t, err.Error(), "json: unsupported type: func()")
	})

	t.Run("http writer error", func(t *testing.T) {
		wrt := &writerWithError{Err: io.ErrClosedPipe}
		err := router.SendResponse(wrt, "data", http.StatusOK)

		require.ErrorIs(t, err, control.ErrWriteResponse)
		require.NotErrorIs(t, err, control.ErrJsonDecode)
		assert.Contains(t, err.Error(), "some writer error")
	})

}

func TestRouter_GenerateTrackId(t *testing.T) {
	trackID1 := control.GenerateTrackId()
	trackID2 := control.GenerateTrackId()

	require.NotEmpty(t, trackID1)
	require.NotEmpty(t, trackID2)
	require.NotEqual(t, trackID1, trackID2)
}
