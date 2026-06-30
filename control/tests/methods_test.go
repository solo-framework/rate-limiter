package tests

import (
	"errors"
	"testing"

	"ratelimiter/control"
	"ratelimiter/internal/config"
	interrs "ratelimiter/internal/errors"
	"ratelimiter/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createControlMethods(t *testing.T) *control.ControlMethods {
	t.Helper()

	groups := config.NewGroupList()
	_ = groups.Add("default", 10, 10)
	mgr, err := service.NewManager(*groups, 10, 10, service.NewTimeProvider())
	if err != nil {
		require.NoError(t, err, "failed to create manager: %v")
	}
	return control.NewControlMethods(mgr)
}

func Test_ControlMethods_AddGroup(t *testing.T) {
	cm := createControlMethods(t)

	t.Run("OK", func(t *testing.T) {
		req := createDummyHttpRequest(`{"name":"gold","rate":5.5,"burst":"7"}`)
		res, err := cm.AddGroup(req)

		require.NoError(t, err)
		require.NotNil(t, res)
		require.Equal(t, true, res)
	})

	t.Run("incorrect json", func(t *testing.T) {
		req := createDummyHttpRequest(`{"name":"gold","rate":5.5 incorrect!!!!!}`)
		res, err := cm.AddGroup(req)

		var outErr *control.ValidationError
		assert.ErrorAs(t, err, &outErr)
		assert.Nil(t, res)
	})

	t.Run("incorrect params", func(t *testing.T) {
		req := createDummyHttpRequest(`{"name":"gold","rate":5.5,"burst":-70}`)
		res, err := cm.AddGroup(req)

		// check response error is wrapped as ValidationError
		var outErr *control.ValidationError
		assert.ErrorAs(t, err, &outErr)

		// check inner error comes from validator
		assert.ErrorIs(t, err, interrs.ErrInvalidData)
		assert.Nil(t, res)
	})

	t.Run("group exists error", func(t *testing.T) {
		req := createDummyHttpRequest(`{"name":"default","rate":5.5,"burst":70}`)
		res, err := cm.AddGroup(req)

		// check that inner error is validation error
		assert.ErrorIs(t, err, service.ErrGroupExists)

		// check response error is wrapped as LogicError
		var outErr *control.LogicError
		assert.ErrorAs(t, err, &outErr)

		assert.Nil(t, res)
	})

	t.Run("unknown error", func(t *testing.T) {
		origInvalidDataErr := interrs.ErrInvalidData
		// replace service.ErrInvalidData with nil to test unknown error branch in IF condition
		interrs.ErrInvalidData = nil
		defer func() {
			interrs.ErrInvalidData = origInvalidDataErr
		}()

		req := createDummyHttpRequest(`{"name":"gold","rate":5.5,"burst":-70}`)
		res, err := cm.AddGroup(req)

		require.Error(t, err)
		assert.Nil(t, res)

		var validationErr *control.ValidationError
		assert.False(t, errors.As(err, &validationErr))

		var logicErr *control.LogicError
		assert.False(t, errors.As(err, &logicErr))

		var serviceErr *interrs.Error
		assert.True(t, errors.As(err, &serviceErr))
	})
}

func Test_ControlMethods_GetStat(t *testing.T) {
	cm := createControlMethods(t)
	req := createDummyHttpRequest("")

	res, err := cm.GetStat(req)

	require.NoError(t, err)

	out := anyToString(t, res)
	// assert.Equal(t, `[{"name":"default","count":0,"rate":10,"burst":10}]`, out)
	assert.Equal(t, `{"default":0}`, out)
}

func Test_ControlMethods_GetInfo(t *testing.T) {
	cm := createControlMethods(t)
	req := createDummyHttpRequest("")

	res, err := cm.GetInfo(req)

	require.NoError(t, err)

	out := anyToString(t, res)
	assert.Equal(t, `[{"name":"default","count":0,"rate":10,"burst":10}]`, out)
	// assert.Equal(t, `{"default":0}`, out)
}

func Test_ControlMethods_DeleteGroup(t *testing.T) {
	cm := createControlMethods(t)

	t.Run("group not exists", func(t *testing.T) {
		req := createDummyHttpRequest(`{"name": "non_existing_blablablablalba"}`)
		res, err := cm.DeleteGroup(req)

		assert.NoError(t, err)
		assert.Equal(t, true, res)
	})

	t.Run("del default", func(t *testing.T) {
		req := createDummyHttpRequest(`{"name": "default"}`)
		res, err := cm.DeleteGroup(req)

		assert.NoError(t, err)
		assert.Equal(t, true, res)

		// now there are no groups left
		res, err = cm.GetStat(createDummyHttpRequest(""))
		assert.NoError(t, err)
		assert.Equal(t, "{}", anyToString(t, res))
	})

	t.Run("incorrect json", func(t *testing.T) {
		req := createDummyHttpRequest(`{"name`)
		res, err := cm.DeleteGroup(req)

		var outErr *control.ValidationError
		assert.ErrorAs(t, err, &outErr)
		assert.Nil(t, res)
	})
}

func Test_ControlMethods_UpdateGroup(t *testing.T) {
	cm := createControlMethods(t)

	t.Run("group not found", func(t *testing.T) {
		req := createDummyHttpRequest(`{"name":"non_existing","rate":5.5,"burst":70}`)
		res, err := cm.UpdateGroup(req)

		assert.ErrorIs(t, err, service.ErrGroupNotFound)
		assert.Nil(t, res)
		// assert.Equal(t, true, res)
	})

	t.Run("incorrect json", func(t *testing.T) {
		req := createDummyHttpRequest(`{"name":"gold","rate":5.5 incorrect!!!!!}`)
		res, err := cm.UpdateGroup(req)

		var outErr *control.ValidationError
		assert.ErrorAs(t, err, &outErr)
		assert.Nil(t, res)
	})

	t.Run("OK", func(t *testing.T) {
		req := createDummyHttpRequest(`{"name":"default","rate":10000,"burst":10000}`)
		res, err := cm.UpdateGroup(req)

		assert.NoError(t, err)
		assert.Equal(t, true, res)

		res, err = cm.GetInfo(createDummyHttpRequest(""))
		assert.NoError(t, err)
		assert.Equal(t, `[{"name":"default","count":0,"rate":10000,"burst":10000}]`, anyToString(t, res))
	})

	t.Run("incorrect params", func(t *testing.T) {
		req := createDummyHttpRequest(`{"name":"gold","rate":5.5,"burst":-70}`)
		res, err := cm.UpdateGroup(req)

		// check response error is wrapped as ValidationError
		var outErr *control.ValidationError
		assert.ErrorAs(t, err, &outErr)

		// check inner error comes from validator
		assert.ErrorIs(t, err, interrs.ErrInvalidData)
		assert.Nil(t, res)
	})

	t.Run("unknown error", func(t *testing.T) {
		origInvalidDataErr := interrs.ErrInvalidData
		// replace service.ErrInvalidData with nil to test unknown error branch in IF condition
		interrs.ErrInvalidData = nil
		defer func() {
			interrs.ErrInvalidData = origInvalidDataErr
		}()

		req := createDummyHttpRequest(`{"name":"gold","rate":5.5,"burst":-70}`)
		res, err := cm.UpdateGroup(req)

		require.Error(t, err)
		assert.Nil(t, res)

		var validationErr *control.ValidationError
		assert.False(t, errors.As(err, &validationErr))

		var logicErr *control.LogicError
		assert.False(t, errors.As(err, &logicErr))

		var serviceErr *interrs.Error
		assert.True(t, errors.As(err, &serviceErr))
	})
}

// import (
// 	"io"
// 	"log/slog"
// 	"net/http"
// 	"net/http/httptest"
// 	"ratelimiter/ratelimiter"
// 	"ratelimiter/ratelimiter/control"
// 	"reflect"
// 	"testing"
// 	"unsafe"

// 	"github.com/stretchr/testify/require"
// )

// func healthFn() bool {
// 	return true
// }

// func getDummyLogger() *slog.Logger {
// 	return slog.New(slog.NewTextHandler(io.Discard, nil))
// }

// // func getServer() *control.HTTPServer {
// // 	return control.NewHTTPServer(
// // 		service.Config{HttpPort: 0},
// // 		service.NewManager(),
// // 		healthFn,
// // 		getDummyLogger(),
// // 	)
// // }

// // dirty hack to access private server fields (without starting it)
// // or expose the HTTP instance publicly?
// func handlerFromServer(t *testing.T, s *control.ControlServer) http.Handler {
// 	t.Helper()

// 	v := reflect.ValueOf(s).Elem().FieldByName("srv")
// 	require.True(t, v.IsValid(), "expected HTTPServer to have srv field")
// 	require.NotNil(t, v.IsNil(), "expected HTTPServer.srv to be initialized")

// 	srv := reflect.NewAt(v.Type(), unsafe.Pointer(v.UnsafeAddr())).Elem().Interface().(*http.Server)
// 	require.NotNil(t, srv.Handler, "expected HTTPServer.srv.Handler to be initialized")

// 	// if srv.Handler == nil {
// 	// 	t.Fatal("expected HTTPServer.srv.Handler to be initialized")
// 	// }
// 	return srv.Handler
// }

// func newTestServer(t *testing.T) (*control.ControlServer, *service.Manager, http.Handler) {
// 	t.Helper()

// 	groups := service.NewGroupList()
// 	if err := groups.Add("default", 1, 1); err != nil {
// 		// t.Fatalf("add base group: %v", err)
// 		require.NoError(t, err, "add default group: %v", err)
// 	}

// 	manager, err := service.NewManager(*groups, 60, 10, service.NewTimeProvider())
// 	if err != nil {
// 		require.NoError(t, err, "new manager: %v", err)
// 	}

// 	logger := getDummyLogger()
// 	cfg := service.Config{HttpPort: 0, HttpSecret: ""}

// 	server := control.NewControlServer(cfg, manager, healthFn, logger)
// 	return server, manager, handlerFromServer(t, server)
// }

// func Test_http_handler_health_test(t *testing.T) {

// 	request := httptest.NewRequest("GET", "/health", nil)
// 	recorder := httptest.NewRecorder()

// 	_, _, handler := newTestServer(t)
// 	handler.ServeHTTP(recorder, request)

// 	require.Equal(t, http.StatusOK, recorder.Code, "need 200 OK")
// 	require.Equal(t, "ok", recorder.Body.String())
// }

// func Test_http_handler_health__________test(t *testing.T) {

// 	request := httptest.NewRequest("GET", "/health", nil)
// 	recorder := httptest.NewRecorder()

// 	s, _, _ := newTestServer(t)
// 	// handler.ServeHTTP(recorder, request)
// 	s.GetHttpServer().Handler.ServeHTTP(recorder, request)

// 	require.Equal(t, http.StatusOK, recorder.Code, "need 200 OK")
// 	require.Equal(t, "ok", recorder.Body.String())
// }
