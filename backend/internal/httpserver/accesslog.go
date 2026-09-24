package httpserver

import (
	"log/slog"
	"net/http"
	"time"
)

// statusTrackingResponseWriter wraps an http.ResponseWriter to capture
// the response status code and bytes written for access logging.
type statusTrackingResponseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int64
	wroteHeader  bool
}

func newStatusTrackingResponseWriter(w http.ResponseWriter) *statusTrackingResponseWriter {
	return &statusTrackingResponseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
	}
}

func (rw *statusTrackingResponseWriter) WriteHeader(code int) {
	if !rw.wroteHeader {
		rw.statusCode = code
		rw.wroteHeader = true
		rw.ResponseWriter.WriteHeader(code)
	}
}

func (rw *statusTrackingResponseWriter) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}
	n, err := rw.ResponseWriter.Write(b)
	rw.bytesWritten += int64(n)
	return n, err
}

// logAccess emits a structured access log line adhering to strict privacy invariants.
// It logs ONLY low-cardinality operational metadata (request_id, method, path, status, duration).
// It strictly NEVER logs bearer tokens, credentials, GPS coordinates, session IDs, raw audio, or transcripts.
func (s *Server) logAccess(reqID string, method, path string, status int, duration time.Duration, bytesWritten int64) {
	if s.logger == nil {
		return
	}

	attrs := []any{
		slog.String("request_id", reqID),
		slog.String("method", method),
		slog.String("path", path),
		slog.Int("status", status),
		slog.Int64("duration_ms", duration.Milliseconds()),
		slog.Int64("bytes", bytesWritten),
	}

	if status >= 500 {
		s.logger.Error("http request error", attrs...)
	} else if status >= 400 {
		s.logger.Warn("http request client error", attrs...)
	} else {
		s.logger.Info("http request completed", attrs...)
	}
}
