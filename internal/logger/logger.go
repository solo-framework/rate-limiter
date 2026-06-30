package logger

import (
	"log/slog"
	"os"
	"sync"
)

type ILogger interface {
	Info(msg string, fields ...any)
	Error(msg string, fields ...any)
	Warn(msg string, fields ...any)
	Debug(msg string, fields ...any)
	With(args ...any) *slog.Logger
}

var globalLogger *slog.Logger

// var mu sync.Mutex
var initOnce sync.Once = sync.Once{}

// ReinitLogger for unit test only
func ReinitLogger() {
	initOnce = sync.Once{}
}

// InitLogger creates new logger or
func InitLogger(env string, args ...any) {

	initOnce.Do(func() {

		var handler slog.Handler

		if env == "dev" {
			opts := slog.HandlerOptions{
				AddSource: true,
				Level:     slog.LevelDebug,
			}

			handler = slog.NewTextHandler(os.Stdout, &opts)

		} else {

			opts := slog.HandlerOptions{
				AddSource: false,
				Level:     slog.LevelInfo,
			}

			handler = slog.NewJSONHandler(os.Stdout, &opts)
		}

		if len(args) > 0 {
			globalLogger = slog.New(handler).With(args...)
		} else {
			globalLogger = slog.New(handler)
		}
	})

}

func CloseLogger() {
	globalLogger = nil
}

// GetLogger  return if exists
func GetLogger() *slog.Logger {

	if globalLogger == nil {
		panic("globalLogger is not initialized, call InitLogger()")
	}
	return globalLogger
}

func DummyLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}
