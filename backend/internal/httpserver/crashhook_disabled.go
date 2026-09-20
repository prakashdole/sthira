//go:build !crashtest

package httpserver

import "net/http"

// registerCrashHook is a no-op in production builds. The crash fault-injection
// endpoint exists only when built with the `crashtest` tag.
func (s *Server) registerCrashHook(mux *http.ServeMux) {}
