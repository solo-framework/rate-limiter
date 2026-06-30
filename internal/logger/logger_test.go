package logger

import (
	"bytes"
	"io"
	"os"
	"ratelimiter/internal/config"
	"time"

	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	oldStdout := os.Stdout
	defer func() {
		os.Stdout = oldStdout
	}()

	r, w, err := os.Pipe()
	if err != nil {
		require.NoError(t, err, "failed to create pipe")
	}

	os.Stdout = w

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("failed to close writer: %v", err)
	}

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("failed to read output: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("failed to close reader: %v", err)
	}
	return buf.String()
}

func TestGetLogger_DevTextHandlerDebugAndSource(t *testing.T) {

	output := captureStdout(t, func() {
		ReinitLogger()
		InitLogger("dev")
		logger := GetLogger()
		logger.Info("hello")
		logger.Debug("dbg")
		CloseLogger()
	})

	if !strings.Contains(output, "msg=hello") {
		t.Fatalf("expected info message in output, got: '%s'", output)
	}
	if !strings.Contains(output, "msg=dbg") {
		t.Fatalf("expected debug message in output, got: %s", output)
	}
	if !strings.Contains(output, "source=") {
		t.Fatalf("expected source info in output, got: %s", output)
	}
	if strings.HasPrefix(strings.TrimSpace(output), "{") {
		t.Fatalf("expected text output, got json: %s", output)
	}
}

func TestGetLogger_ProdJSONInfoLevelNoSource(t *testing.T) {

	output := captureStdout(t, func() {
		ReinitLogger()
		InitLogger("prod")
		logger := GetLogger()
		logger.Info("hello")
		logger.Debug("dbg")
		CloseLogger()
	})

	trimmed := strings.TrimSpace(output)
	if !strings.HasPrefix(trimmed, "{") {
		t.Fatalf("expected json output, got: %s", output)
	}
	if !strings.Contains(output, `"msg":"hello"`) {
		t.Fatalf("expected info message in output, got: %s", output)
	}
	if strings.Contains(output, `"dbg"`) {
		t.Fatalf("did not expect debug message in output, got: %s", output)
	}
	if strings.Contains(output, `"source"`) {
		t.Fatalf("did not expect source info in output, got: %s", output)
	}
}

func TestLogger_SecretReplace(t *testing.T) {

	output := captureStdout(t, func() {
		ReinitLogger()
		InitLogger("dev")
		logger := GetLogger()
		logger.Debug("hello", "config", getConfig())
		CloseLogger()
	})

	require.Contains(t, output, "HttpSecret: [censored]")
	require.Contains(t, output, "BasicAuthUser: [censored]")
	require.Contains(t, output, "BasicAuthPass: [censored]")

}

func getConfig() *config.Config {

	groups := config.NewGroupList()
	_ = groups.Add("default", 10.0, 1_000)

	return &config.Config{
		Port:            0, // random port
		MaxConnections:  9,
		WorkerPoolSize:  3,
		ReadTimeout:     1000 * time.Millisecond,
		WriteTimeout:    3000 * time.Millisecond,
		ShutdownTimeout: 2000 * time.Millisecond,
		PacketSize:      512,
		BasicAuthPass:   "password",
		Groups:          *groups,
	}
}
