package httpserver

import (
	"net/http"
	"net/http/pprof"
	"strings"
)

// registerPprofRoutes attaches guarded net/http/pprof endpoints to the mux
// if cfg.EnablePprof is true and a non-empty PprofToken is configured.
func (s *Server) registerPprofRoutes(mux *http.ServeMux) {
	if !s.cfg.EnablePprof || s.cfg.PprofToken == "" {
		return
	}

	authGuard := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			token := r.Header.Get("X-Pprof-Token")
			if token == "" && strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
				token = strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			}
			if token == "" || token != s.cfg.PprofToken {
				http.Error(w, "Unauthorized pprof access", http.StatusUnauthorized)
				return
			}
			next(w, r)
		}
	}

	mux.HandleFunc("/debug/pprof/", authGuard(pprof.Index))
	mux.HandleFunc("/debug/pprof/cmdline", authGuard(pprof.Cmdline))
	mux.HandleFunc("/debug/pprof/profile", authGuard(pprof.Profile))
	mux.HandleFunc("/debug/pprof/symbol", authGuard(pprof.Symbol))
	mux.HandleFunc("/debug/pprof/trace", authGuard(pprof.Trace))
	mux.HandleFunc("/debug/pprof/heap", authGuard(pprof.Handler("heap").ServeHTTP))
	mux.HandleFunc("/debug/pprof/goroutine", authGuard(pprof.Handler("goroutine").ServeHTTP))
	mux.HandleFunc("/debug/pprof/allocs", authGuard(pprof.Handler("allocs").ServeHTTP))
	mux.HandleFunc("/debug/pprof/block", authGuard(pprof.Handler("block").ServeHTTP))
	mux.HandleFunc("/debug/pprof/mutex", authGuard(pprof.Handler("mutex").ServeHTTP))
	mux.HandleFunc("/debug/pprof/threadcreate", authGuard(pprof.Handler("threadcreate").ServeHTTP))
}
