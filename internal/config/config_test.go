package config

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helper function to create a temporary TOML file
func createTempConfigFile(t *testing.T, content string) string {
	tmpFile, err := os.CreateTemp("", "config*.toml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tmpFile.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatal(err)
	}
	return tmpFile.Name()
}

func TestLoadConfig_Success(t *testing.T) {
	content := `
port = 9000
http_port = 8090
max_connections = 10000
worker_pool_size = 5
packet_size = 512
http_secret = "set as env variable"
basic_auth_user = "user"
basic_auth_pass = "password"

# Valid time units are "ns", "us" (or "µs"), "ms", "s", "m", "h".
read_timeout = "100m"
write_timeout = "100ms"
shutdown_timeout = "3s"
cleanup_interval = "1m"
ttl = "30s"

[[groups]]
name = "test_group"
rate = 200.0
burst = 1000
`
	path := createTempConfigFile(t, content)
	defer os.Remove(path) // nolint

	err := LoadConfig(path)
	// check no error
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// check base fields
	if AppConfig().Port != 9000 {
		t.Errorf("Expected port 9000, got %d", AppConfig().Port)
	}

	// check group was added via Add method
	groups := AppConfig().Groups.List()
	if len(groups) != 1 {
		t.Errorf("Expected 1 group, got %d", len(groups))
	}
	if groups[0].Name != "test_group" {
		t.Errorf("Expected group name 'test_group', got '%s'", groups[0].Name)
	}
}

func Test_LoadConfig_FileNotFound(t *testing.T) {
	err := LoadConfig("nonexistent.toml")
	if err == nil {
		t.Fatal("Expected error due to non-existent file, but got nil")
	}
}

func Test_LoadConfig_InvalidFormat(t *testing.T) {
	content := `
[[Groups]]
Name (typo here) "invalid name"
Rate = 1.0
Burst = 1
`
	path := createTempConfigFile(t, content)
	defer os.Remove(path)

	err := LoadConfig(path)
	if err == nil {
		t.Fatal("Expected error due to invalid format, but got nil")
	}
}

func TestLoadConfig_ValidationError(t *testing.T) {
	// group name with invalid character (space), breaks regexp
	content := `
port = 9000
http_port = 8090
max_connections = 10000
worker_pool_size = 5
packet_size = 512
http_secret = "set as env variable"
basic_auth_user = "user"
basic_auth_pass = "password"

# Valid time units are "ns", "us" (or "µs"), "ms", "s", "m", "h".
read_timeout = "100m"
write_timeout = "100ms"
shutdown_timeout = "3s"
cleanup_interval = "1m"
ttl = "30s"

[[groups]]
name = "test group"
rate = 200.0
burst = 1000
`
	path := createTempConfigFile(t, content)
	defer os.Remove(path)

	err := LoadConfig(path)
	fmt.Println(err)
	// expect validation error from group.go
	if err == nil {
		t.Fatal("Expected validation error due to invalid group name, but got nil")
	}
}

func TestLoadConfig_EnvOverride(t *testing.T) {
	// content := `Port = 8080`
	content := `
port = 9000
http_port = 8090
max_connections = 10000
worker_pool_size = 5
packet_size = 512
http_secret = "set as env variable"
basic_auth_user = "user"
basic_auth_pass = "password"

# Valid time units are "ns", "us" (or "µs"), "ms", "s", "m", "h".
read_timeout = "100m"
write_timeout = "100ms"
shutdown_timeout = "3s"
cleanup_interval = "1m"
ttl = "30s"

[[groups]]
name = "test_group"
rate = 200.0
burst = 1000
`

	path := createTempConfigFile(t, content)
	defer os.Remove(path)

	// set env var override
	os.Setenv("RATE_LIMITER_PORT", "7777")
	defer os.Unsetenv("RATE_LIMITER_PORT")

	err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}

	// ENV value should take priority over file value
	if AppConfig().Port != 7777 {
		t.Errorf("Expected port 7777 (from ENV), got %d", AppConfig().Port)
	}
}

func TestLoadConfig_Durations(t *testing.T) {
	content := `
port = 9000
http_port = 8090
max_connections = 10000
worker_pool_size = 5
packet_size = 512
http_secret = "set as env variable"
basic_auth_user = "user"
basic_auth_pass = "password"

# Valid time units are "ns", "us" (or "µs"), "ms", "s", "m", "h".
read_timeout = "100s"
write_timeout = "100ms"
shutdown_timeout = "3m20s"
cleanup_interval = "10h"
ttl = "24h"

[[groups]]
name = "test_group"
rate = 200.0
burst = 1000
`
	path := createTempConfigFile(t, content)
	defer os.Remove(path)

	err := LoadConfig(path)
	require.NoError(t, err)

	assert.Equal(t, time.Millisecond*100, AppConfig().WriteTimeout)
	assert.Equal(t, time.Second*100, AppConfig().ReadTimeout)
	assert.Equal(t, time.Minute*3+time.Second*20, AppConfig().ShutdownTimeout)
	assert.Equal(t, time.Hour*10, AppConfig().CleanupInterval)
	assert.Equal(t, time.Hour*24, AppConfig().TTL)
}

func TestLoadConfig_Defaults(t *testing.T) {
	content := `


[[groups]]
name = "test_group"
rate = 200.0
burst = 1000
`
	path := createTempConfigFile(t, content)
	defer os.Remove(path)

	err := LoadConfig(path)
	require.NoError(t, err)

	assert.Equal(t, uint(49105), AppConfig().Port)
	assert.Equal(t, uint(8090), AppConfig().HttpPort)
	assert.Equal(t, 1000, AppConfig().MaxConnections)
	assert.Equal(t, 100, AppConfig().WorkerPoolSize)
	assert.Equal(t, 512, AppConfig().PacketSize)

	assert.Equal(t, "secret", AppConfig().HttpSecret)
	assert.Equal(t, "user", AppConfig().BasicAuthUser)
	assert.Equal(t, "password", AppConfig().BasicAuthPass)

	assert.Equal(t, time.Millisecond*100, AppConfig().WriteTimeout)
	assert.Equal(t, time.Millisecond*100, AppConfig().ReadTimeout)
	assert.Equal(t, time.Second*5, AppConfig().ShutdownTimeout)
	assert.Equal(t, time.Hour*12, AppConfig().CleanupInterval)
	assert.Equal(t, time.Hour*48, AppConfig().TTL)
}

func TestLoadConfig_ValidationErrorMessage_SingleField(t *testing.T) {
	content := `
port = 70000
http_port = 8090
max_connections = 10000
worker_pool_size = 5
packet_size = 512
http_secret = "secret"
basic_auth_user = "user"
basic_auth_pass = "password"

read_timeout = "100ms"
write_timeout = "100ms"
shutdown_timeout = "3s"
cleanup_interval = "1m"
ttl = "30s"

[[groups]]
name = "test_group"
rate = 200.0
burst = 1000
`

	path := createTempConfigFile(t, content)
	defer os.Remove(path)

	err := LoadConfig(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config validation error:")
	assert.Contains(t, err.Error(), "Port has wrong value `70000` [must be port")
}

func TestLoadConfig_ValidationErrorMessage_MultipleFields(t *testing.T) {
	content := `
port = 9000
http_port = 8090
max_connections = 0
worker_pool_size = 5
packet_size = 512
http_secret = "secret"
basic_auth_user = "user"
basic_auth_pass = "123"

read_timeout = "100ms"
write_timeout = "100ms"
shutdown_timeout = "3s"
cleanup_interval = "1m"
ttl = "30s"

[[groups]]
name = "test_group"
rate = 200.0
burst = 1000
`

	path := createTempConfigFile(t, content)
	defer os.Remove(path)

	err := LoadConfig(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config validation error:")
	assert.Contains(t, err.Error(), "MaxConnections has wrong value `0` [must be required ]")
	assert.Contains(t, err.Error(), "BasicAuthPass has wrong value `123` [must be min 6]")
}
