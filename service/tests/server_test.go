package tests

import (
	"fmt"
	"net"
	"ratelimiter/internal/config"
	"ratelimiter/service"

	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Server_Start_Stop(t *testing.T) {

	config := &config.Config{
		Port:            0, // random port
		MaxConnections:  9,
		WorkerPoolSize:  3,
		ReadTimeout:     1000,
		WriteTimeout:    3000,
		ShutdownTimeout: 2,
		PacketSize:      512,
	}

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", config.Port))
	require.NoError(t, err)
	require.NotNil(t, listener)

	server := service.NewServer(
		newLogger(),
		config,
		newDummyHandler(),
	)

	err = server.Start()
	require.NoError(t, err)
	require.True(t, server.HealthCheck())

	err = server.Shutdown()
	require.True(t, server.HealthCheck())

	require.NoError(t, err)
}

func Test_Server_request_OK(t *testing.T) {

	config := getConfig()
	config.Port = getRandomPort(t)

	server := service.NewServer(
		newLogger(),
		config,
		newDummyHandler(),
	)

	err := server.Start()
	require.NoError(t, err, "some error %+v", err)

	addr := &net.TCPAddr{Port: int(config.Port)}
	client := newClient(addr)
	defer client.Close()

	// tcp request
	err = client.Write([]byte("HELLO"))
	require.NoError(t, err)

	// read response
	resp, err := client.Read()

	require.NoError(t, err)
	assert.Equal(t, "HELLO", string(resp))

	// stop server
	_ = server.Shutdown()
}

func Test_handleConnection_panic(t *testing.T) {

	config := getConfig()
	config.Port = getRandomPort(t)

	server := service.NewServer(
		newLogger(),
		config,
		newPanicHandler(),
	)

	err := server.Start()
	require.NoError(t, err)

	addr := &net.TCPAddr{Port: int(config.Port)}
	client := newClient(addr)
	defer client.Close()

	// tcp request
	err = client.Write([]byte("HELLO"))
	require.NoError(t, err)

	// read response
	resp, err := client.Read()
	require.NoError(t, err)

	assert.Equal(t, service.RESPONSE_ERROR_INTERNAL, string(resp))

	// stop server
	_ = server.Shutdown()
}

func Test_read_timeout(t *testing.T) {

	config := getConfig()
	config.Port = getRandomPort(t)
	config.ReadTimeout = 1 * time.Millisecond

	server := service.NewServer(
		newLogger(),
		config,
		newDummyHandler(),
	)

	_ = server.Start()
	defer func() { _ = server.Shutdown() }()

	addr := &net.TCPAddr{Port: int(config.Port)}
	client := newClient(addr)

	defer client.Close()

	// tcp request
	client.BadWrite([]byte("HELLO"))
	// read response
	resp, err := client.Read()
	require.NoError(t, err)

	assert.Equal(t, service.RESPONSE_ERROR_READ_TIMEOUT, string(resp))

	// stop server
	_ = server.Shutdown()
}

func Test_request_too_large_returns_invalid_data_without_reset(t *testing.T) {
	config := getConfig()
	config.Port = getRandomPort(t)
	config.PacketSize = 8

	server := service.NewServer(
		newLogger(),
		config,
		newDummyHandler(),
	)

	err := server.Start()
	require.NoError(t, err)
	defer func() { _ = server.Shutdown() }()

	addr := &net.TCPAddr{Port: int(config.Port)}
	client := newClient(addr)
	defer client.Close()

	err = client.Write([]byte("default:client-id-too-long"))
	require.NoError(t, err)

	resp, err := client.Read()
	require.NoError(t, err)
	assert.Equal(t, service.RESPONSE_ERROR_INVALID_DATA, string(resp))
}

func Test_TooManyConnections(t *testing.T) {

	config := getConfig()
	config.Port = getRandomPort(t)
	config.MaxConnections = 5
	config.WorkerPoolSize = 5

	server := service.NewServer(
		newLogger(),
		// listener,
		config,
		// add handling delay
		// so connection queue on server gets filled
		NewHandlerWithDelay(200),
	)

	_ = server.Start()
	defer func() { _ = server.Shutdown() }()

	// run 10 more requests than server can accept
	n := config.MaxConnections + 10
	ch := make(chan []byte, n)
	wg := sync.WaitGroup{}

	addr := &net.TCPAddr{Port: int(config.Port)}

	for i := range n {

		wg.Go(func() {

			client := newClient(addr)
			defer client.Close()

			// tcp request
			query := fmt.Sprintf("default:user_id_%d", i)

			err := client.Write([]byte(query))
			require.NoError(t, err)

			// // read response
			r, err := client.Read()
			require.NoError(t, err)
			ch <- r
		})
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	busyCount := 0

	for d := range ch {
		if string(d) == service.RESPONSE_ERROR_TOO_MANY_CONNECTIONS {
			busyCount++
		}
	}

	// should receive n-config.MaxConnections "busy" responses, others are "ok"
	assert.Equal(t, n-config.MaxConnections, busyCount)
}

func Test_shutdown_timeout(t *testing.T) {

	config := getConfig()
	config.Port = getRandomPort(t)

	config.ShutdownTimeout = 10 // nanoseconds!! too little
	config.MaxConnections = 100
	config.WorkerPoolSize = 5

	server := service.NewServer(
		newLogger(),
		// listener,
		config,
		NewHandlerWithDelay(500),
	)

	err := server.Start()
	require.NoError(t, err)

	addr := &net.TCPAddr{Port: int(config.Port)}
	// start many requests
	n := 20
	for i := range n {

		go func() {

			client := newClient(addr)
			defer client.Close()

			// tcp request
			query := fmt.Sprintf("default:user_id_%d", i)

			err := client.Write([]byte(query))
			require.NoError(t, err)

			// // read response
			_, _ = client.Read()

		}()
	}

	// wait until clients connect
	time.Sleep(50 * time.Millisecond)

	// and stop server immediately
	err = server.Shutdown()
	assert.ErrorIs(t, err, service.ErrShutdown)

	err = server.Shutdown()
	assert.NoError(t, err)
}

func Test_shutdown_graceful(t *testing.T) {

	config := getConfig()
	config.Port = getRandomPort(t)

	config.ShutdownTimeout = 10 * time.Millisecond
	config.MaxConnections = 100
	config.WorkerPoolSize = 5
	config.ShutdownTimeout = 2000 * time.Millisecond

	server := service.NewServer(
		newLogger(),
		config,
		NewHandlerWithDelay(50),
	)
	addr := &net.TCPAddr{Port: int(config.Port)}

	err := server.Start()
	require.NoError(t, err)

	// start many requests
	n := 20
	for i := range n {

		go func() {

			client := newClient(addr)
			defer client.Close()

			// tcp request
			query := fmt.Sprintf("default:user_id_%d", i)

			err := client.Write([]byte(query))
			require.NoError(t, err)

			// // read response
			_, _ = client.Read()

		}()
	}

	// wait until clients connect
	time.Sleep(50 * time.Millisecond)

	// and stop server immediately
	// no error expected because shutdown timeout is long and workers finish in time
	err = server.Shutdown()
	assert.NoError(t, err)
}
