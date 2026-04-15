package logger

import (
	"log/slog"
	"os"
)

type ILogger interface {
	Info(msg string, fields ...any)
	Error(msg string, fields ...any)
	Warn(msg string, fields ...any)
	Debug(msg string, fields ...any)
	With(args ...any) *slog.Logger
}

func GetLogger(env string) *slog.Logger {

	var logger *slog.Logger

	if env == "dev" {
		opts := slog.HandlerOptions{
			AddSource: true,
			Level:     slog.LevelDebug,
		}

		handler := slog.NewTextHandler(os.Stdout, &opts)
		logger = slog.New(handler)

	} else {

		opts := slog.HandlerOptions{
			AddSource: false,
			Level:     slog.LevelInfo,
		}

		handler := slog.NewJSONHandler(os.Stdout, &opts)
		logger = slog.New(handler)
	}
	return logger
}
