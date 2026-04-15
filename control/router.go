package control

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/pprof"
	"ratelimiter/internal/config"
	"ratelimiter/internal/logger"
)

type IRouter interface {
	GetHandlers() map[string]http.HandlerFunc
}

type Router struct {
	handlers map[string]http.HandlerFunc
	logger   logger.ILogger
	config   config.Config
}

func NewRouter(methods ControlMethods, config config.Config, logger logger.ILogger) *Router {

	handlers := make(map[string]http.HandlerFunc)

	r := &Router{
		logger:   logger,
		handlers: handlers,
		config:   config,
	}

	// add routing entries here
	r.handlers["GET /stat"] = r.MainHandler(methods.GetStat, true)
	r.handlers["GET /info"] = r.MainHandler(methods.GetInfo, true)
	r.handlers["POST /group_add"] = r.MainHandler(methods.AddGroup, true)
	r.handlers["POST /group_delete"] = r.MainHandler(methods.DeleteGroup, true)
	r.handlers["POST /group_update"] = r.MainHandler(methods.UpdateGroup, true)
	r.handlers["POST /enable_prof"] = r.MainHandler(methods.EnableProfiling, true)
	r.handlers["POST /disable_prof"] = r.MainHandler(methods.DisableProfiling, true)
	// eof routing

	// profiling handlers. Do not change.
	r.handlers["GET /debug/pprof/"] = r.BasicAuthMiddleware(http.HandlerFunc(pprof.Index))
	r.handlers["GET /debug/pprof/cmdline"] = r.BasicAuthMiddleware(http.HandlerFunc(pprof.Cmdline))
	r.handlers["GET /debug/pprof/profile"] = r.BasicAuthMiddleware(http.HandlerFunc(pprof.Profile))
	r.handlers["GET /debug/pprof/symbol"] = r.BasicAuthMiddleware(http.HandlerFunc(pprof.Symbol))
	r.handlers["GET /debug/pprof/trace"] = r.BasicAuthMiddleware(http.HandlerFunc(pprof.Trace))

	return r
}

func (s *Router) GetHandlers() map[string]http.HandlerFunc {
	return s.handlers
}

func (s *Router) BasicAuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		user, pass, ok := r.BasicAuth()
		userMatch := (subtle.ConstantTimeCompare([]byte(user), []byte(s.config.BasicAuthUser)) == 1)
		passwordMatch := (subtle.ConstantTimeCompare([]byte(pass), []byte(s.config.BasicAuthPass)) == 1)

		if !ok || !userMatch || !passwordMatch {
			w.Header().Set("WWW-Authenticate", `Basic realm="Rate limiter profiling service"`)
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	}
}

func (s *Router) MainHandler(fn HttpFunc, checkAuth bool) http.HandlerFunc {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		log := GetLoggerFromContext(r.Context())

		// Checking secret token stays here for now. Can be moved to middleware later
		if checkAuth {

			token := r.Header.Get("X-Auth-Token")
			match := (subtle.ConstantTimeCompare([]byte(token), []byte(s.config.HttpSecret)) == 1)

			if !match {
				log.Warn("access denied: X-Auth-Token")
				http.Error(w, "access denied", http.StatusUnauthorized)
				return
			}
		}

		// execute handler
		res, err := fn(r)

		if err != nil {
			var me IKnownError

			if errors.As(err, &me) {
				if me.NeedLogging() {
					// log the real internal error not of user-facing text.
					// if internal error is nil, we log user-facing message
					var unwrapErr error
					if me.Unwrap() != nil {
						unwrapErr = me.Unwrap()
					} else {
						unwrapErr = me
					}

					log.Warn(me.Error(), "error", unwrapErr, "url", r.RequestURI)
				}

				// send error response
				err = s.SendError(w, err, me.GetStatus())

				// this check is probably redundant because sendError always gets an error, but keep it
				if errors.Is(err, ErrJsonDecode) {
					log.Error("SendError decode error", "error", err.Error(), "url", r.RequestURI)
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
					return
				}
				if errors.Is(err, ErrWriteResponse) {
					// client likely disconnected so nothing can be sent. Only logging is possible
					log.Error("SendError write error", "error", err.Error(), "url", r.RequestURI)
					return
				}

				return

			} else {
				// Some kind of unknown error, log now and decide later how to handle it
				log.Error("unhandled error", "error", err.Error(), "url", r.RequestURI)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
		}

		// send successful response
		log.Info("success", "out", res)
		err = s.SendResponse(w, res, http.StatusOK)

		// response may contain arbitrary data here and it may fail JSON encoding
		if errors.Is(err, ErrJsonDecode) {
			log.Error("SendResponse decode error", "error", err.Error(), "url", r.RequestURI)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		// client likely disconnected already so nothing can be sent. Only logging is possible
		if errors.Is(err, ErrWriteResponse) {
			log.Error("SendResponse write error", "error", err.Error(), "url", r.RequestURI)
			return
		}
	})
}

func (s *Router) SendResponse(w http.ResponseWriter, data any, code int) error {

	if data == nil {
		data = "{}"
	}

	res := responseStruct{Error: false, Data: data}

	out, err := json.Marshal(res)
	if err != nil {
		err = errors.Join(err, ErrJsonDecode)
		return fmt.Errorf("SendError json encode error: %w", err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, err = w.Write(out)

	if err != nil {
		err = errors.Join(err, ErrWriteResponse)
		return fmt.Errorf("SendResponse write error: %w", err)
	}
	return nil
}

func (s *Router) SendError(w http.ResponseWriter, data any, code int) error {

	if data == nil {
		data = "{}"
	}

	var tmp any = data
	if r, ok := data.(error); ok {
		tmp = r.Error()
	}

	resp := responseStruct{Error: true, Data: tmp}

	out, err := json.Marshal(resp)
	if err != nil {
		err = errors.Join(err, ErrJsonDecode)
		return fmt.Errorf("SendError json encode error: %w", err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, err = w.Write(out)

	if err != nil {
		err = errors.Join(err, ErrWriteResponse)
		return fmt.Errorf("SendError write error: %w", err)
	}
	return nil
}
