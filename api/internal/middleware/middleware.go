package middleware

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/sirupsen/logrus"
)

// Logger returns a middleware that logs HTTP requests
func Logger(logger *logrus.Logger) func(next http.Handler) http.Handler {
	return middleware.RequestLogger(&logFormatter{logger: logger})
}

// logFormatter implements middleware.LogFormatter
type logFormatter struct {
	logger *logrus.Logger
}

// NewLogEntry creates a new log entry for the request
func (l *logFormatter) NewLogEntry(r *http.Request) middleware.LogEntry {
	entry := &logEntry{
		logger: l.logger.WithFields(logrus.Fields{
			"method":     r.Method,
			"path":       r.URL.Path,
			"remote_ip":  r.RemoteAddr,
			"user_agent": r.UserAgent(),
		}),
	}

	entry.logger.Info("Request started")
	return entry
}

// logEntry implements middleware.LogEntry
type logEntry struct {
	logger *logrus.Entry
}

// Write logs the response
func (l *logEntry) Write(status, bytes int, header http.Header, elapsed time.Duration, extra interface{}) {
	l.logger.WithFields(logrus.Fields{
		"status":  status,
		"bytes":   bytes,
		"elapsed": elapsed,
	}).Info("Request completed")
}

// Panic logs panics
func (l *logEntry) Panic(v interface{}, stack []byte) {
	l.logger.WithFields(logrus.Fields{
		"panic": v,
		"stack": string(stack),
	}).Error("Request panicked")
}

// CORS returns a CORS middleware with appropriate settings
func CORS() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, X-CSRF-Token")
			w.Header().Set("Access-Control-Allow-Credentials", "true")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// JSON ensures requests have correct content type for JSON endpoints
func JSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip content type check for GET, HEAD, OPTIONS
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		contentType := r.Header.Get("Content-Type")
		if contentType == "" || contentType != "application/json" {
			http.Error(w, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Recovery recovers from panics and logs them
func Recovery(logger *logrus.Logger) func(next http.Handler) http.Handler {
	return middleware.Recoverer
}
