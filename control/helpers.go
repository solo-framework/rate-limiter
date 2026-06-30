package control

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/google/uuid"
	"ratelimiter/internal/logger"
)

type contextKey string

const (
	ctxKeyTrackID contextKey = "track_id"
	ctxKeyLogger  contextKey = "logger"
)

func GetTrackID(ctx context.Context) string {
	if id, ok := ctx.Value(ctxKeyTrackID).(string); ok {
		return id
	}
	return ""
}

func GenerateTrackId() (res string) {
	var err error
	res = ""

	fallback := func() string {
		ts := time.Now().UnixNano()
		return "ts-" + strconv.FormatInt(ts, 10)
	}

	defer func() {
		if r := recover(); r != nil {
			res = fallback()
		}

		if err != nil {
			res = fallback()
		}
	}()

	guid, err := uuid.NewV7()
	res = guid.String()
	return res
}

// GetLoggerFromContext return logger from context (set in middleware)
func GetLoggerFromContext(ctx context.Context) logger.ILogger {
	if logger, ok := ctx.Value(ctxKeyLogger).(*slog.Logger); ok {
		return logger
	}
	return slog.Default()
}
