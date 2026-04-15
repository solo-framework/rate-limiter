package control

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"ratelimiter/internal/config"
	"ratelimiter/internal/logger"
	"time"
)

type HttpFunc func(*http.Request) (any, error)

type ControlServer struct {
	httpSrv *http.Server
	logger  logger.ILogger
	config  config.Config
}

func NewControlServer(config config.Config, logger logger.ILogger, router IRouter) *ControlServer {

	s := &ControlServer{
		logger: logger,
		config: config,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.handleHealth)

	handlers := router.GetHandlers()
	for path, fn := range handlers {
		mux.HandleFunc(path, fn)
	}

	h := TrackingMiddleware(logger)(mux)
	h = MaxBodySizeMiddleware(config.HttpMaxBodySize)(h)
	// h := MaxBodySizeMiddleware(config.HttpMaxBodySize)(mux)

	// mux1 := TrackingMiddleware(logger)(
	// 	MaxBodySizeMiddleware(config.HttpMaxBodySize)(mux),
	// )

	s.httpSrv = &http.Server{
		Addr:    ":" + fmt.Sprint(config.HttpPort),
		Handler: h,

		ReadHeaderTimeout: time.Second * 3,
		ReadTimeout:       time.Second * 3,
		WriteTimeout:      time.Second * 3,
		IdleTimeout:       time.Second * 60,
		MaxHeaderBytes:    1 << 10,
	}

	return s
}

func (s *ControlServer) GetHttpServer() *http.Server {
	return s.httpSrv
}

func (s *ControlServer) handleHealth(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (s *ControlServer) Start() error {

	err := s.httpSrv.ListenAndServe()
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (s *ControlServer) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.httpSrv.Shutdown(ctx)
}
