package http

import (
	"net/http"
	"path"
	"strings"
	"sync"
	"time"

	"ephemeral/backend/internal/metrics"
)

// cleanupInterval is how often Allow sweeps the bucket map.
const cleanupInterval = time.Minute

type tokenBucket struct {
	tokens     float64
	lastRefill time.Time
}

type Limiter struct {
	mu          sync.Mutex
	buckets     map[string]*tokenBucket
	ratePerS    float64
	burst       float64
	lastCleanup time.Time
}

func NewLimiter(rpm int, burst int) *Limiter {
	return &Limiter{
		buckets:  map[string]*tokenBucket{},
		ratePerS: float64(rpm) / 60.0,
		burst:    float64(burst),
	}
}

func (l *Limiter) Allow(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.cleanupRefilled(now)

	bucket, ok := l.buckets[key]
	if !ok {
		bucket = &tokenBucket{tokens: l.burst, lastRefill: now}
		l.buckets[key] = bucket
	}

	elapsed := now.Sub(bucket.lastRefill).Seconds()
	bucket.tokens += elapsed * l.ratePerS
	if bucket.tokens > l.burst {
		bucket.tokens = l.burst
	}
	bucket.lastRefill = now

	if bucket.tokens < 1.0 {
		return false
	}
	bucket.tokens -= 1.0
	return true
}

// cleanupRefilled removes every bucket that has refilled completely. Such a
// bucket behaves exactly like one that does not exist yet, so dropping it
// never gives a client more requests than it was entitled to. The sweep runs
// at most once per cleanupInterval, which keeps the map from growing without
// bound when requests arrive from many addresses.
func (l *Limiter) cleanupRefilled(now time.Time) {
	if !l.lastCleanup.IsZero() && now.Sub(l.lastCleanup) < cleanupInterval {
		return
	}
	l.lastCleanup = now
	for key, bucket := range l.buckets {
		refilled := bucket.tokens + now.Sub(bucket.lastRefill).Seconds()*l.ratePerS
		if refilled >= l.burst {
			delete(l.buckets, key)
		}
	}
}

func rateLimit(limiter *Limiter, counters *metrics.Counters, resolveClientIP func(*http.Request) string) func(http.Handler) http.Handler {
	if resolveClientIP == nil {
		resolveClientIP = remoteIP
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			routeKey, protected := classifyProtectedRoute(r.Method, r.URL.Path)
			if !protected {
				next.ServeHTTP(w, r)
				return
			}
			key := rateLimitClientKey(resolveClientIP(r)) + "|" + routeKey
			if !limiter.Allow(key, time.Now()) {
				if counters != nil {
					counters.IncRateLimited()
				}
				w.Header().Set("Retry-After", "60")
				writeError(w, http.StatusTooManyRequests, "rate_limited", "too many requests")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// classifyProtectedRoute returns the route template for the endpoints that
// read or write secrets. The template is the rate-limit key and the label in
// the access log, so a secret id never ends up in either.
func classifyProtectedRoute(method, rawPath string) (string, bool) {
	p := path.Clean(strings.TrimSpace(rawPath))
	if p == "." {
		p = "/"
	}
	if method == http.MethodPost && p == "/api/v1/secrets" {
		return "POST /api/v1/secrets", true
	}
	const secretsPrefix = "/api/v1/secrets/"
	if strings.HasPrefix(p, secretsPrefix) {
		rest := strings.TrimPrefix(p, secretsPrefix)
		if rest == "" || strings.Contains(rest, "/") {
			if method == http.MethodPost && strings.HasSuffix(p, "/consume") {
				parts := strings.Split(strings.TrimPrefix(p, secretsPrefix), "/")
				if len(parts) == 2 && parts[1] == "consume" && parts[0] != "" {
					return "POST /api/v1/secrets/{id}/consume", true
				}
			}
			return "", false
		}
		// The router serves HEAD through the GET handler, so both count
		// against the same limit.
		if method == http.MethodGet || method == http.MethodHead {
			return "GET /api/v1/secrets/{id}", true
		}
	}
	return "", false
}
