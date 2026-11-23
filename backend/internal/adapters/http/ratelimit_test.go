package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ephemeral/backend/internal/metrics"
)

func TestLimiterAllowRefill(t *testing.T) {
	limiter := NewLimiter(60, 2) // 1 token/sec, burst 2
	key := "127.0.0.1|POST /api/v1/secrets"
	now := time.Unix(0, 0)

	if !limiter.Allow(key, now) {
		t.Fatal("expected first request to pass")
	}
	if !limiter.Allow(key, now) {
		t.Fatal("expected second request to pass")
	}
	if limiter.Allow(key, now) {
		t.Fatal("expected third request to be rate limited")
	}
	if !limiter.Allow(key, now.Add(1*time.Second)) {
		t.Fatal("expected token refill after one second")
	}
}

func TestRateLimitMiddlewareProtectedRoute(t *testing.T) {
	limiter := NewLimiter(1, 1)
	counters := metrics.NewCounters()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/secrets", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	handler := rateLimit(limiter, counters, nil)(mux)

	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/secrets", nil)
	req1.RemoteAddr = "127.0.0.1:1234"
	res1 := httptest.NewRecorder()
	handler.ServeHTTP(res1, req1)
	if res1.Code != http.StatusCreated {
		t.Fatalf("expected first request to pass, got %d", res1.Code)
	}

	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/secrets", nil)
	req2.RemoteAddr = "127.0.0.1:1234"
	res2 := httptest.NewRecorder()
	handler.ServeHTTP(res2, req2)
	if res2.Code != http.StatusTooManyRequests {
		t.Fatalf("expected second request to be rate limited, got %d", res2.Code)
	}
	snapshot := counters.Snapshot()
	if snapshot.RateLimited != 1 {
		t.Fatalf("expected one rate-limited count, got %d", snapshot.RateLimited)
	}
}

func TestRateLimitMiddlewareIsolatesPerIP(t *testing.T) {
	limiter := NewLimiter(1, 1)
	counters := metrics.NewCounters()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/secrets", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
	handler := rateLimit(limiter, counters, nil)(mux)

	doPost := func(remoteAddr string) int {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/secrets", nil)
		req.RemoteAddr = remoteAddr
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		return res.Code
	}

	if got := doPost("10.0.0.1:1234"); got != http.StatusCreated {
		t.Fatalf("client A first request: got %d, want 201", got)
	}
	if got := doPost("10.0.0.1:1234"); got != http.StatusTooManyRequests {
		t.Fatalf("client A second request: got %d, want 429", got)
	}
	if got := doPost("10.0.0.2:1234"); got != http.StatusCreated {
		t.Fatalf("client B must have a fresh bucket: got %d, want 201", got)
	}
	if got := doPost("10.0.0.2:1234"); got != http.StatusTooManyRequests {
		t.Fatalf("client B second request: got %d, want 429", got)
	}
}

func TestRateLimitMiddlewareIsolatesPerRoute(t *testing.T) {
	limiter := NewLimiter(1, 1)
	counters := metrics.NewCounters()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/secrets", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
	mux.HandleFunc("GET /api/v1/secrets/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := rateLimit(limiter, counters, nil)(mux)

	send := func(method, path string) int {
		req := httptest.NewRequest(method, path, nil)
		req.RemoteAddr = "10.0.0.1:1234"
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		return res.Code
	}

	if got := send(http.MethodPost, "/api/v1/secrets"); got != http.StatusCreated {
		t.Fatalf("first POST: got %d, want 201", got)
	}
	if got := send(http.MethodPost, "/api/v1/secrets"); got != http.StatusTooManyRequests {
		t.Fatalf("second POST: got %d, want 429", got)
	}
	if got := send(http.MethodGet, "/api/v1/secrets/abc"); got != http.StatusOK {
		t.Fatalf("GET on different route must have its own bucket: got %d, want 200", got)
	}
}

func TestLimiterCleansUpRefilledBuckets(t *testing.T) {
	limiter := NewLimiter(60, 2) // 1 token/sec, burst 2
	now := time.Unix(1000, 0)

	limiter.mu.Lock()
	// Used long ago: has refilled completely and can be forgotten.
	limiter.buckets["refilled"] = &tokenBucket{tokens: 0, lastRefill: now.Add(-10 * time.Second)}
	// Emptied one second ago: still owes a token and must be remembered.
	limiter.buckets["draining"] = &tokenBucket{tokens: 0, lastRefill: now.Add(-1 * time.Second)}
	limiter.mu.Unlock()

	_ = limiter.Allow("other", now)

	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	if _, ok := limiter.buckets["refilled"]; ok {
		t.Fatal("expected fully refilled bucket to be removed")
	}
	if _, ok := limiter.buckets["draining"]; !ok {
		t.Fatal("expected bucket that has not refilled to remain")
	}
}

func TestLimiterCleanupRunsAtMostOncePerInterval(t *testing.T) {
	limiter := NewLimiter(60, 2)
	now := time.Unix(1000, 0)

	_ = limiter.Allow("first", now) // Runs the first sweep.

	limiter.mu.Lock()
	limiter.buckets["refilled"] = &tokenBucket{tokens: 2, lastRefill: now}
	limiter.mu.Unlock()

	_ = limiter.Allow("first", now.Add(cleanupInterval-time.Second))
	limiter.mu.Lock()
	_, stillThere := limiter.buckets["refilled"]
	limiter.mu.Unlock()
	if !stillThere {
		t.Fatal("sweep ran before the interval had passed")
	}

	_ = limiter.Allow("first", now.Add(cleanupInterval))
	limiter.mu.Lock()
	_, stillThere = limiter.buckets["refilled"]
	limiter.mu.Unlock()
	if stillThere {
		t.Fatal("expected sweep once the interval had passed")
	}
}

// Dropping a bucket must never hand a client extra requests: a client that
// has used up its burst stays limited across a sweep.
func TestLimiterCleanupDoesNotResetAnExhaustedClient(t *testing.T) {
	limiter := NewLimiter(1, 3) // 1 token/minute, burst 3
	key := "203.0.113.5|POST /api/v1/secrets"
	now := time.Unix(1000, 0)

	for i := 0; i < 3; i++ {
		if !limiter.Allow(key, now) {
			t.Fatalf("request %d within burst should pass", i+1)
		}
	}
	if limiter.Allow(key, now) {
		t.Fatal("burst exhausted: expected rate limit")
	}

	// 90 seconds later a sweep runs. One token has come back, not three.
	later := now.Add(90 * time.Second)
	if !limiter.Allow(key, later) {
		t.Fatal("expected the one refilled token to be usable")
	}
	if limiter.Allow(key, later) {
		t.Fatal("sweep must not have reset the bucket to a full burst")
	}
}

func TestRateLimitMiddlewareCountsHeadAsGet(t *testing.T) {
	limiter := NewLimiter(1, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/secrets/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := rateLimit(limiter, metrics.NewCounters(), nil)(mux)

	send := func(method string) int {
		req := httptest.NewRequest(method, "/api/v1/secrets/abc", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		return res.Code
	}

	if got := send(http.MethodHead); got != http.StatusOK {
		t.Fatalf("first HEAD: got %d, want 200 (the router serves HEAD through GET)", got)
	}
	if got := send(http.MethodHead); got != http.StatusTooManyRequests {
		t.Fatalf("second HEAD: got %d, want 429", got)
	}
	if got := send(http.MethodGet); got != http.StatusTooManyRequests {
		t.Fatalf("GET after HEAD must share the limit: got %d, want 429", got)
	}
}

func TestRateLimitMiddlewareCountsIPv6PerSubnet(t *testing.T) {
	limiter := NewLimiter(1, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/secrets", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
	handler := rateLimit(limiter, metrics.NewCounters(), nil)(mux)

	doPost := func(remoteAddr string) int {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/secrets", nil)
		req.RemoteAddr = remoteAddr
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		return res.Code
	}

	if got := doPost("[2001:db8:1:1::1]:1234"); got != http.StatusCreated {
		t.Fatalf("first request: got %d, want 201", got)
	}
	if got := doPost("[2001:db8:1:1:ffff:ffff:ffff:ffff]:1234"); got != http.StatusTooManyRequests {
		t.Fatalf("another address in the same /64 must share the limit: got %d, want 429", got)
	}
	if got := doPost("[2001:db8:1:2::1]:1234"); got != http.StatusCreated {
		t.Fatalf("a different /64 must have its own limit: got %d, want 201", got)
	}
}

func TestRateLimitClientKey(t *testing.T) {
	cases := map[string]string{
		"203.0.113.50":         "203.0.113.50",
		"::ffff:203.0.113.50":  "::ffff:203.0.113.50",
		"2001:db8:1:1::1":      "2001:db8:1:1::/64",
		"2001:db8:1:1:a:b:c:d": "2001:db8:1:1::/64",
		"unknown":              "unknown",
	}
	for in, want := range cases {
		if got := rateLimitClientKey(in); got != want {
			t.Errorf("rateLimitClientKey(%q) = %q, want %q", in, got, want)
		}
	}
}
