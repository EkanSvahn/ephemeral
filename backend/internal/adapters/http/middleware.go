package http

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"ephemeral/backend/internal/security"
)

type contextKey string

const requestIDKey contextKey = "request_id"

// A request id supplied by the client is echoed in the response and written
// to the log, so only short ids from a harmless alphabet are accepted.
var requestIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

func withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rid := r.Header.Get("X-Request-Id")
		if !requestIDPattern.MatchString(rid) {
			rid = ""
		}
		if rid == "" {
			generated, err := security.NewURLSafeID(12)
			if err == nil {
				rid = generated
			}
		}
		if rid != "" {
			w.Header().Set("X-Request-Id", rid)
			r = r.WithContext(context.WithValue(r.Context(), requestIDKey, rid))
		}
		next.ServeHTTP(w, r)
	})
}

func requestLogger(logger *slog.Logger, resolveClientIP func(*http.Request) string) func(http.Handler) http.Handler {
	if resolveClientIP == nil {
		resolveClientIP = remoteIP
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			started := time.Now()
			recorder := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
			next.ServeHTTP(recorder, r)

			requestID, _ := r.Context().Value(requestIDKey).(string)
			method := methodLabel(r.Method)
			route := routeLabel(method, r.URL.Path)
			durationMs := time.Since(started).Milliseconds()

			logger.Info(
				"http_request",
				"request_id", requestID,
				"method", method,
				"route", route,
				"status", recorder.statusCode,
				"duration_ms", durationMs,
				"remote_ip", resolveClientIP(r),
				"rate_limited", recorder.statusCode == http.StatusTooManyRequests,
			)
		})
	}
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'; base-uri 'none';")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store, max-age=0")
			w.Header().Set("Pragma", "no-cache")
		}
		next.ServeHTTP(w, r)
	})
}

func cors(allowedOrigins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && isAllowedOrigin(origin, allowedOrigins) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type,X-Request-Id")
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func isAllowedOrigin(origin string, allowedOrigins []string) bool {
	for _, allowed := range allowedOrigins {
		if origin == allowed {
			return true
		}
	}
	return false
}

func withTimeout(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.TimeoutHandler(next, timeout, "request timeout")
	}
}

func withBodyLimit(limit int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, limit)
			next.ServeHTTP(w, r)
		})
	}
}

func chain(h http.Handler, middlewares ...func(http.Handler) http.Handler) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}
	return h
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		// The status line is already sent, so the client cannot be told.
		// The payload may be ciphertext and is never logged.
		slog.Warn("response_write_failed", "status", status, "error", err.Error())
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{
		"error":   code,
		"message": message,
	})
}

func cleanID(id string) string {
	return strings.TrimSpace(id)
}

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (s *statusRecorder) WriteHeader(statusCode int) {
	s.statusCode = statusCode
	s.ResponseWriter.WriteHeader(statusCode)
}

// methodLabel returns the request method for the access log. The method is
// chosen by the client, so anything outside the usual set becomes "OTHER".
func methodLabel(method string) string {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut,
		http.MethodPatch, http.MethodDelete, http.MethodOptions:
		return method
	}
	return "OTHER"
}

// routeLabel returns the route for the access log. Only known routes are
// written out. Any other path is chosen by the client and may contain a
// secret id, so it is logged as "unmatched".
func routeLabel(method, rawPath string) string {
	if route, ok := classifyProtectedRoute(method, rawPath); ok {
		return route
	}
	switch p := strings.TrimSpace(rawPath); p {
	case "/api/health", "/api/ready", "/api/status", "/api/metrics":
		return method + " " + p
	}
	return method + " unmatched"
}
