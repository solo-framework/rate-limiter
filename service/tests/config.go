package tests

import (
	"time"

	"ratelimiter/internal/config"
)

func getConfig() *config.Config {
	var config *config.Config = &config.Config{
		Port:            0, // random port
		MaxConnections:  9,
		WorkerPoolSize:  3,
		ReadTimeout:     1000 * time.Millisecond,
		WriteTimeout:    3000 * time.Millisecond,
		ShutdownTimeout: 2000 * time.Millisecond,
		PacketSize:      512,
	}
	return config
}
