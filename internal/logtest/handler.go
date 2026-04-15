package logtest

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"testing"
)

// LogRecord is a log record
type LogRecord struct {
	// Time    time.Time
	Level   slog.Level
	Message string
	Source  *slog.Source
	Attrs   map[string]any
}

// NewTestLogger creates a new test logger
func NewTestLogger() (*slog.Logger, *TestHandler) {
	handler := NewTestHandler()
	logger := slog.New(handler)
	return logger, handler
}

// TestHandler - custom handler for testing
type TestHandler struct {
	state *testHandlerState
	attrs []slog.Attr
}

type testHandlerState struct {
	records []LogRecord
	mu      sync.Mutex
}

// NewTestHandler creates a new test handler
func NewTestHandler() *TestHandler {
	return &TestHandler{
		state: &testHandlerState{
			records: make([]LogRecord, 0),
		},
		attrs: make([]slog.Attr, 0),
	}
}

// Enabled checks if logging is enabled
func (s *TestHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return true //log all in tests
}

// Handle handles a log record
func (s *TestHandler) Handle(ctx context.Context, record slog.Record) error {
	s.state.mu.Lock()
	defer s.state.mu.Unlock()

	attrs := make(map[string]any)
	for _, a := range s.attrs {
		attrs[a.Key] = a.Value.Any()
	}
	record.Attrs(func(a slog.Attr) bool {
		attrs[a.Key] = a.Value.Any()
		return true
	})

	s.state.records = append(s.state.records, LogRecord{
		// Time:    record.Time,
		Level:   record.Level,
		Message: record.Message,
		Source:  record.Source(),
		Attrs:   attrs,
	})

	return nil
}

// WithAttrs
func (s *TestHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	merged := make([]slog.Attr, len(s.attrs), len(s.attrs)+len(attrs))
	copy(merged, s.attrs)
	merged = append(merged, attrs...)

	return &TestHandler{
		state: s.state,
		attrs: merged,
	}
}

// WithGroup creates a new group
func (s *TestHandler) WithGroup(name string) slog.Handler {
	return s
}

// Records returns all log records
func (s *TestHandler) Records() []LogRecord {
	s.state.mu.Lock()
	defer s.state.mu.Unlock()

	records := make([]LogRecord, len(s.state.records))
	copy(records, s.state.records)
	return records
}

// Reset  clears all log records
func (s *TestHandler) Reset() {
	s.state.mu.Lock()
	defer s.state.mu.Unlock()
	s.state.records = s.state.records[:0]
}

// Count returns the number of log records
func (s *TestHandler) Count() int {
	s.state.mu.Lock()
	defer s.state.mu.Unlock()
	return len(s.state.records)
}

// AssertLogCount checks log count
func AssertLogCount(t *testing.T, handler *TestHandler, expected int) {
	t.Helper()
	if count := handler.Count(); count != expected {
		t.Errorf("expected %d log records, got %d", expected, count)
	}
}

// AssertLogContains checks is message exists
func AssertLogContainsMessage(t *testing.T, handler *TestHandler, expectedMessage string) {
	t.Helper()
	records := handler.Records()
	for _, r := range records {
		if strings.Contains(r.Message, expectedMessage) {
			return
		}
	}
	t.Errorf("log record with message '%s' not found", expectedMessage)
}

// AssertLogContainsAttr checks is attr exists
func AssertLogContainsAttr(t *testing.T, handler *TestHandler, key string, expectedValue any) {
	t.Helper()
	records := handler.Records()

	for _, r := range records {
		if v, ok := r.Attrs[key]; ok {
			if strings.Contains(fmt.Sprint(v), fmt.Sprint(expectedValue)) {
				return
			}
		}
	}
	t.Errorf("log record with attr '%s=%v' not found", key, expectedValue)
}

// AssertLogLevel cheks log level
func AssertLogLevel(t *testing.T, handler *TestHandler, level slog.Level) {
	t.Helper()
	records := handler.Records()
	if len(records) == 0 {
		t.Fatal("no logs")
	}
	lastRecord := records[len(records)-1]
	if lastRecord.Level != level {
		t.Errorf("expected log level %v, got %v", level, lastRecord.Level)
	}
}

// GetLastRecord returns last record
func GetLastRecord(handler *TestHandler) *LogRecord {
	records := handler.Records()
	if len(records) == 0 {
		return nil
	}
	return &records[len(records)-1]
}
