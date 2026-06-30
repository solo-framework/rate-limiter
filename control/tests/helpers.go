package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"ratelimiter/control"
	"ratelimiter/internal/logger"
)

func addTrackingMiddleware(logger logger.ILogger, mhandler http.HandlerFunc) http.Handler {
	return control.TrackingMiddleware(logger)(mhandler)
}

func getDummyLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func decodeJSON(t *testing.T, body []byte) map[string]any {
	t.Helper()

	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("failed to decode json: %v", err)
	}
	return out
}

func createDummyHttpRequest(requestData string) *http.Request {
	reader := bytes.NewReader([]byte(requestData))
	return httptest.NewRequest("", "/", reader)
}

func anyToString(t *testing.T, data any) string {
	t.Helper()
	out, err := json.Marshal(data)
	require.NoError(t, err, "can't decode object to JSON")
	return string(out)
}

// var _ control.IRouter = (*stubRouter)(nil)

type stubRouter struct{}

// GetHandlers implements [control.IRouter].
func (f *stubRouter) GetHandlers() map[string]http.HandlerFunc {
	res := make(map[string]http.HandlerFunc)
	res["/"] = func(w http.ResponseWriter, r *http.Request) {}
	return res
}

// type loggerWithRecord struct {
// 	record map[string]any
// }

// func getLoggerWithRecord() *loggerWithRecord {

// 	return &loggerWithRecord{
// 		record: make(map[string]any),
// 	}
// }

// func (s *loggerWithRecord) getRecord() map[string]any {
// 	return s.record
// }

// func (s *loggerWithRecord) parseLog(msg string, fields ...any) {

// 	s.record["message"] = msg
// 	for i := 0; i < len(fields); i = i + 2 {
// 		s.record[fmt.Sprintf("%v", fields[i])] = fields[i+1]
// 	}
// }

// func (s *loggerWithRecord) With(args ...any) *slog.Logger {
// 	panic("not impl")
// }

// func (s *loggerWithRecord) Info(msg string, fields ...any) {
// 	s.parseLog(msg, fields...)
// }
// func (s *loggerWithRecord) Error(msg string, fields ...any) {
// 	s.parseLog(msg, fields...)
// }
// func (s *loggerWithRecord) Warn(msg string, fields ...any) {

// 	s.parseLog(msg, fields...)
// }
// func (s *loggerWithRecord) Debug(msg string, fields ...any) {
// 	s.parseLog(msg, fields...)
// }

// -------

var _ http.ResponseWriter = (*writerWithError)(nil)

type writerWithError struct {
	header http.Header
	Сode   int
	Err    error
}

// Header implements [http.ResponseWriter].
func (w *writerWithError) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

// Write implements [http.ResponseWriter].
func (w *writerWithError) Write([]byte) (int, error) {
	// return 0, fmt.Errorf("some writer error")
	return 0, fmt.Errorf("some writer error: %w", w.Err)
}

// WriteHeader implements [http.ResponseWriter].
func (w *writerWithError) WriteHeader(statusCode int) {
	w.Сode = statusCode
}
