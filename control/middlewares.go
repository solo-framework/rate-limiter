package control

import (
	"context"
	"fmt"
	"net/http"
	"ratelimiter/internal/logger"
	"runtime/debug"
)

// MaxBodySizeMiddleware limits body of POST request for security reason
// limit is number of bytes
func MaxBodySizeMiddleware(limit int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			if r.Method == http.MethodPost ||
				r.Method == http.MethodPatch ||
				r.Method == http.MethodPut ||
				r.Method == http.MethodDelete {

				if r.Body != nil {
					r.Body = http.MaxBytesReader(w, r.Body, limit)
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func TrackingMiddleware(logger logger.ILogger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			trackId := GenerateTrackId()
			log := logger.With("track_id", trackId)

			getStack := func() []byte {
				var stack []byte
				defer func() {
					if sr := recover(); sr != nil {
						stack = fmt.Appendf(nil, "getStack capture panic: %v", sr)
					}
				}()
				stack = debug.Stack()
				return stack
			}

			defer func() {
				if rcv := recover(); rcv != nil {
					stack := string(getStack())
					log.Error("[PANIC]", "panic", rcv, "stack", stack, "url", r.RequestURI)
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				}
			}()

			w.Header().Add("X-Track-Id", trackId)

			if r.RequestURI != "/health" {
				log.Info("request", "method", r.Method, "url", r.RequestURI, "remote", r.RemoteAddr)
			}

			ctx := r.Context()
			ctx = context.WithValue(ctx, ctxKeyTrackID, trackId)
			ctx = context.WithValue(ctx, ctxKeyLogger, log)
			r = r.WithContext(ctx)

			next.ServeHTTP(w, r)
		})
	}
}
