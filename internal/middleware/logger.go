package middleware

import (
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

// Logger is a middleware that logs the details of each request.
func Logger(next http.Handler, log *logrus.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create a response writer to capture the status code
		lrw := &loggingResponseWriter{ResponseWriter: w}

		// Call the next handler
		next.ServeHTTP(lrw, r)

		log.WithFields(logrus.Fields{
			"method":      r.Method,
			"path":        r.URL.Path,
			"status":      lrw.statusCode,
			"duration":    time.Since(start),
			"remote_addr": r.RemoteAddr,
		}).Info("Request handled")
	})
}

// loggingResponseWriter is a wrapper around http.ResponseWriter to capture the status code.
type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

func (lrw *loggingResponseWriter) Write(b []byte) (int, error) {
	if lrw.statusCode == 0 {
		lrw.statusCode = http.StatusOK
	}
	return lrw.ResponseWriter.Write(b)
}
