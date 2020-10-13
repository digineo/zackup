package graylog

import (
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

type muxLogger struct {
	logger *logrus.Entry
}

// NewMuxLogger returns a logging middleware.
func NewMuxLogger(logger *logrus.Entry) func(http.Handler) http.Handler {
	m := &muxLogger{logger}
	return m.middleware
}

func realIP(req *http.Request) string {
	ra := req.RemoteAddr
	if ip := req.Header.Get("X-Forwarded-For"); ip != "" {
		ra = strings.Split(ip, ", ")[0]
	} else if ip := req.Header.Get("X-Real-IP"); ip != "" {
		ra = ip
	} else {
		ra, _, _ = net.SplitHostPort(ra)
	}
	return ra
}

// statusCapture wraps a ResponseWriter to capture the status code.
type statusCapture struct {
	http.ResponseWriter
	statusCode int
}

func (lw *statusCapture) WriteHeader(code int) {
	lw.statusCode = code
	lw.ResponseWriter.WriteHeader(code)
}

func (lw *statusCapture) Write(b []byte) (int, error) {
	return lw.ResponseWriter.Write(b)
}

// Middleware implement mux middleware interface.
func (m *muxLogger) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f := logrus.Fields{}
		start := time.Now()

		if reqID := r.Header.Get("X-Request-Id"); reqID != "" {
			f["request-id"] = reqID
		}

		if remote := realIP(r); remote != "" {
			f["remote"] = remote
		}

		lw := &statusCapture{w, http.StatusOK}
		next.ServeHTTP(lw, r)

		f["status"] = lw.statusCode
		f["dur"] = time.Since(start).String()
		m.logger.WithFields(f).Infof("%s %s", r.Method, r.RequestURI)
	})
}
