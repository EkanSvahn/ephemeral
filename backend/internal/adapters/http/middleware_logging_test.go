package http

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestLoggerLogsSafeRouteLabel(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/secrets/{id}/consume", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := chain(mux, withRequestID, requestLogger(logger, nil))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/secrets/secret-id-123/consume", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)
	logLine := buf.String()

	if !strings.Contains(logLine, `"route":"POST /api/v1/secrets/{id}/consume"`) {
		t.Fatalf("expected templated route label, got %s", logLine)
	}
	if strings.Contains(logLine, "secret-id-123") {
		t.Fatalf("expected log to avoid raw secret id path, got %s", logLine)
	}
	if !strings.Contains(logLine, `"request_id":"`) {
		t.Fatalf("expected request_id in log, got %s", logLine)
	}
}

// Paths that match no route are chosen by the client. They can contain a
// secret id, so the log must not repeat them.
func TestRequestLoggerDoesNotLogUnmatchedPaths(t *testing.T) {
	cases := []struct {
		method string
		path   string
	}{
		{http.MethodDelete, "/api/v1/secrets/secret-id-123"},
		{http.MethodGet, "/api/v1/secrets/secret-id-123/extra"},
		{http.MethodPut, "/api/v1/secrets/secret-id-123/consume"},
		{http.MethodGet, "/secret-id-123"},
	}
	for _, c := range cases {
		t.Run(c.method+" "+c.path, func(t *testing.T) {
			var buf bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&buf, nil))
			handler := chain(http.NewServeMux(), withRequestID, requestLogger(logger, nil))

			req := httptest.NewRequest(c.method, c.path, nil)
			req.RemoteAddr = "127.0.0.1:1234"
			handler.ServeHTTP(httptest.NewRecorder(), req)

			logLine := buf.String()
			if strings.Contains(logLine, "secret-id-123") {
				t.Fatalf("expected log to leave out the raw path, got %s", logLine)
			}
			if !strings.Contains(logLine, `"route":"`+c.method+` unmatched"`) {
				t.Fatalf("expected unmatched route label, got %s", logLine)
			}
		})
	}
}

func TestRequestLoggerKeepsKnownStaticRoutes(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	handler := chain(http.NewServeMux(), withRequestID, requestLogger(logger, nil))

	req := httptest.NewRequest(http.MethodGet, "/api/ready", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if !strings.Contains(buf.String(), `"route":"GET /api/ready"`) {
		t.Fatalf("expected known route to be logged, got %s", buf.String())
	}
}

func TestRequestLoggerNormalisesUnusualMethods(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	handler := chain(http.NewServeMux(), withRequestID, requestLogger(logger, nil))

	req := httptest.NewRequest("PROPFINDSOMETHINGLONG", "/api/health", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	handler.ServeHTTP(httptest.NewRecorder(), req)

	logLine := buf.String()
	if strings.Contains(logLine, "PROPFINDSOMETHINGLONG") {
		t.Fatalf("expected unusual method to be replaced, got %s", logLine)
	}
	if !strings.Contains(logLine, `"method":"OTHER"`) {
		t.Fatalf("expected method OTHER, got %s", logLine)
	}
}

func TestRequestIDFromClientIsValidated(t *testing.T) {
	cases := []struct {
		name     string
		supplied string
		keep     bool
	}{
		{"plain id", "abc-123_DEF.4", true},
		{"contains a space", "abc 123", false},
		{"contains a quote", `abc"123`, false},
		{"too long", strings.Repeat("a", 65), false},
		{"longest allowed", strings.Repeat("a", 64), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			handler := withRequestID(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))
			req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
			req.Header.Set("X-Request-Id", c.supplied)
			res := httptest.NewRecorder()
			handler.ServeHTTP(res, req)

			got := res.Header().Get("X-Request-Id")
			if got == "" {
				t.Fatal("expected a request id in the response")
			}
			if c.keep && got != c.supplied {
				t.Fatalf("expected supplied id to be kept, got %q", got)
			}
			if !c.keep && got == c.supplied {
				t.Fatalf("expected supplied id to be replaced, got %q", got)
			}
		})
	}
}
