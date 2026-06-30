package tests

import (
	"bytes"
	"io"
	"strings"
	"testing"
	_ "unsafe"

	"ratelimiter/control"
)

type trackingReadCloser struct {
	io.Reader
	closed bool
}

func (t *trackingReadCloser) Close() error {
	t.closed = true
	return nil
}

func TestDecodeRequest(t *testing.T) {
	type req struct {
		Name  string            `json:"name"`
		Rate  control.JsonFloat `json:"rate"`
		Burst control.JsonFloat `json:"burst"`
	}

	t.Run("valid with values as numbers", func(t *testing.T) {
		body := &trackingReadCloser{Reader: bytes.NewReader([]byte(`{"name":"gold","rate":5.5,"burst":7}`))}

		got, err := control.DecodeRequest[req](body)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if !body.closed {
			t.Fatalf("expected body to be closed")
		}
		if got.Name != "gold" || got.Rate != 5.5 || got.Burst != 7 {
			t.Fatalf("unexpected request: %#v", got)
		}
	})

	t.Run("valid with values as strings", func(t *testing.T) {
		body := &trackingReadCloser{Reader: bytes.NewReader([]byte(`{"name":"gold","rate":"5.50000000000000000000000","burst":"9223372036854775806"}`))}

		got, err := control.DecodeRequest[req](body)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if !body.closed {
			t.Fatalf("expected body to be closed")
		}
		if got.Name != "gold" || got.Rate != 5.5 || got.Burst != 9223372036854775806 {
			t.Fatalf("unexpected request: %#v", got)
		}
	})

	t.Run("invalid", func(t *testing.T) {
		body := &trackingReadCloser{Reader: strings.NewReader("{")}

		_, err := control.DecodeRequest[req](body)
		if err == nil {
			t.Fatalf("expected error")
		}
		if !body.closed {
			t.Fatalf("expected body to be closed")
		}
	})
}

// package tests

// import (
// 	"bytes"
// 	"encoding/json"
// 	"io"
// 	"log/slog"
// 	"net/http"
// 	"net/http/httptest"
// 	"reflect"
// 	"testing"
// 	"unsafe"

// 	"ratelimiter/ratelimiter"
// 	"ratelimiter/ratelimiter/control"
// )

// type responseDTO struct {
// 	Error bool `json:"error"`
// 	Data  any  `json:"data"`
// }

// func handlerFromServer(t *testing.T, s *control.HTTPServer) http.Handler {
// 	t.Helper()

// 	v := reflect.ValueOf(s).Elem().FieldByName("srv")
// 	if !v.IsValid() {
// 		t.Fatal("expected HTTPServer to have srv field")
// 	}
// 	if v.IsNil() {
// 		t.Fatal("expected HTTPServer.srv to be initialized")
// 	}

// 	srv := reflect.NewAt(v.Type(), unsafe.Pointer(v.UnsafeAddr())).Elem().Interface().(*http.Server)
// 	if srv.Handler == nil {
// 		t.Fatal("expected HTTPServer.srv.Handler to be initialized")
// 	}
// 	return srv.Handler
// }

// func newTestServer(t *testing.T) (*control.HTTPServer, *service.Manager, http.Handler) {
// 	t.Helper()

// 	groups := service.NewGroupList()
// 	if err := groups.Add("base", 1, 1); err != nil {
// 		t.Fatalf("add base group: %v", err)
// 	}

// 	manager, err := service.NewManager(*groups, 60, 10, service.NewTimeProvider())
// 	if err != nil {
// 		t.Fatalf("new manager: %v", err)
// 	}

// 	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
// 	cfg := service.Config{HttpPort: 0, HttpSecret: ""}
// 	server := control.NewHTTPServer(cfg, manager, nil, logger)
// 	return server, manager, handlerFromServer(t, server)
// }

// func decodeResponse(t *testing.T, body []byte) responseDTO {
// 	t.Helper()
// 	var resp responseDTO
// 	if err := json.Unmarshal(body, &resp); err != nil {
// 		t.Fatalf("unmarshal response: %v", err)
// 	}
// 	return resp
// }

// func findGroup(t *testing.T, info any, name string) (float64, int, bool) {
// 	t.Helper()

// 	data, err := json.Marshal(info)
// 	if err != nil {
// 		t.Fatalf("marshal info: %v", err)
// 	}
// 	var groups []struct {
// 		Name  string  `json:"name"`
// 		Rate  float64 `json:"rate"`
// 		Burst int     `json:"burst"`
// 	}
// 	if err := json.Unmarshal(data, &groups); err != nil {
// 		t.Fatalf("unmarshal info: %v", err)
// 	}

// 	for _, g := range groups {
// 		if g.Name == name {
// 			return g.Rate, g.Burst, true
// 		}
// 	}
// 	return 0, 0, false
// }

// func TestDTOAddGroupRequest(t *testing.T) {
// 	_, manager, handler := newTestServer(t)

// 	body := []byte(`{"name":"gold","rate":5.5,"burst":7}`)
// 	req := httptest.NewRequest(http.MethodPost, "/group_add", bytes.NewReader(body))
// 	rec := httptest.NewRecorder()

// 	handler.ServeHTTP(rec, req)

// 	if rec.Code != http.StatusCreated {
// 		t.Fatalf("expected status %d, got %d", http.StatusCreated, rec.Code)
// 	}

// 	resp := decodeResponse(t, rec.Body.Bytes())
// 	if resp.Error {
// 		t.Fatalf("expected error false")
// 	}
// 	if resp.Data != "ok" {
// 		t.Fatalf("expected data 'ok', got %#v", resp.Data)
// 	}

// 	rate, burst, ok := findGroup(t, manager.GetInfo(), "gold")
// 	if !ok {
// 		t.Fatalf("expected group to be added")
// 	}
// 	if rate != 5.5 || burst != 7 {
// 		t.Fatalf("expected rate 5.5 and burst 7, got rate=%v burst=%d", rate, burst)
// 	}
// }

// func TestDTOUpdateGroupRequest(t *testing.T) {
// 	_, manager, handler := newTestServer(t)

// 	body := []byte(`{"name":"base","rate":12.25,"burst":9}`)
// 	req := httptest.NewRequest(http.MethodPost, "/group_update", bytes.NewReader(body))
// 	rec := httptest.NewRecorder()

// 	handler.ServeHTTP(rec, req)

// 	if rec.Code != http.StatusOK {
// 		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
// 	}

// 	resp := decodeResponse(t, rec.Body.Bytes())
// 	if resp.Error {
// 		t.Fatalf("expected error false")
// 	}
// 	if resp.Data != "ok" {
// 		t.Fatalf("expected data 'ok', got %#v", resp.Data)
// 	}

// 	rate, burst, ok := findGroup(t, manager.GetInfo(), "base")
// 	if !ok {
// 		t.Fatalf("expected group to exist")
// 	}
// 	if rate != 12.25 || burst != 9 {
// 		t.Fatalf("expected rate 12.25 and burst 9, got rate=%v burst=%d", rate, burst)
// 	}
// }

// func TestDTODeleteGroupRequest(t *testing.T) {
// 	_, manager, handler := newTestServer(t)

// 	body := []byte(`{"name":"base"}`)
// 	req := httptest.NewRequest(http.MethodPost, "/group_del", bytes.NewReader(body))
// 	rec := httptest.NewRecorder()

// 	handler.ServeHTTP(rec, req)

// 	if rec.Code != http.StatusOK {
// 		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
// 	}

// 	resp := decodeResponse(t, rec.Body.Bytes())
// 	if resp.Error {
// 		t.Fatalf("expected error false")
// 	}
// 	if resp.Data != "ok" {
// 		t.Fatalf("expected data 'ok', got %#v", resp.Data)
// 	}

// 	_, _, ok := findGroup(t, manager.GetInfo(), "base")
// 	if ok {
// 		t.Fatalf("expected group to be deleted")
// 	}
// }
