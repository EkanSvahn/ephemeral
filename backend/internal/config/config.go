package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	minBodyBytes = 4 * 1024
	maxBodyBytes = 1024 * 1024 * 1024
)

type Config struct {
	ListenAddr           string
	RedisURL             string
	MaxBodyBytes         int64
	MaxMetaBytes         int
	MaxCipherBytes       int
	MaxFileBytes         int64
	AllowedFileMIMEs     []string
	MaxTTLSeconds        int64
	MaxViews             int64
	IDLengthBytes        int
	RequestTimeout       time.Duration
	AllowedOrigins       []string
	TrustedProxyCID      string
	RateLimitRPM         int
	RateLimitBurst       int
	MetricsEnabled       bool
	RequirePassword      bool
	StrictRedisEphemeral bool
}

func Load() (Config, error) {
	cfg := Config{
		ListenAddr:      getenv("LISTEN_ADDR", ":8080"),
		RedisURL:        getenv("REDIS_URL", "redis://redis:6379/0"),
		TrustedProxyCID: strings.TrimSpace(getenv("TRUSTED_PROXY_CIDR", "")),
	}
	allowedOriginsRaw := getenv("ALLOWED_ORIGINS", "")
	if allowedOriginsRaw == "" {
		// Backward compatibility with previous single-origin config key.
		allowedOriginsRaw = getenv("ALLOWED_ORIGIN", "http://localhost:5173,http://127.0.0.1:5173")
	}
	cfg.AllowedOrigins = parseAllowedOrigins(allowedOriginsRaw)

	var err error
	cfg.MaxBodyBytes, err = getenvInt64("MAX_BODY_BYTES", 90*1024*1024)
	if err != nil {
		return Config{}, fmt.Errorf("MAX_BODY_BYTES: %w", err)
	}
	// The upper bound is far above what the largest allowed file needs, and
	// keeps the value inside the range of int on every platform.
	if cfg.MaxBodyBytes < minBodyBytes || cfg.MaxBodyBytes > maxBodyBytes {
		return Config{}, fmt.Errorf("MAX_BODY_BYTES must be between %d and %d", minBodyBytes, maxBodyBytes)
	}
	cfg.MaxMetaBytes, err = getenvInt("MAX_META_BYTES", 16*1024)
	if err != nil {
		return Config{}, fmt.Errorf("MAX_META_BYTES: %w", err)
	}
	if cfg.MaxMetaBytes < 256 || cfg.MaxMetaBytes > 1024*1024 {
		return Config{}, fmt.Errorf("MAX_META_BYTES must be between 256 and 1048576")
	}
	cfg.MaxCipherBytes, err = getenvInt("MAX_CIPHERTEXT_BYTES", int(cfg.MaxBodyBytes)-cfg.MaxMetaBytes-1024)
	if err != nil {
		return Config{}, fmt.Errorf("MAX_CIPHERTEXT_BYTES: %w", err)
	}
	if cfg.MaxCipherBytes < 1024 {
		return Config{}, fmt.Errorf("MAX_CIPHERTEXT_BYTES must be at least 1024")
	}
	if int64(cfg.MaxCipherBytes+cfg.MaxMetaBytes+1024) > cfg.MaxBodyBytes {
		return Config{}, fmt.Errorf("MAX_BODY_BYTES too small for configured MAX_META_BYTES and MAX_CIPHERTEXT_BYTES")
	}
	cfg.MaxFileBytes, err = getenvInt64("MAX_FILE_BYTES", 50*1024*1024)
	if err != nil {
		return Config{}, fmt.Errorf("MAX_FILE_BYTES: %w", err)
	}
	if cfg.MaxFileBytes < 1024 || cfg.MaxFileBytes > 500*1024*1024 {
		return Config{}, fmt.Errorf("MAX_FILE_BYTES must be between 1024 and 524288000")
	}
	cfg.AllowedFileMIMEs = parseCSV(getenv("ALLOWED_FILE_MIME_TYPES", "application/pdf,image/png,image/jpeg,text/plain,application/octet-stream"))
	cfg.MaxTTLSeconds, err = getenvInt64("MAX_TTL_SECONDS", 86400)
	if err != nil {
		return Config{}, fmt.Errorf("MAX_TTL_SECONDS: %w", err)
	}
	cfg.MaxViews, err = getenvInt64("MAX_VIEWS", 100)
	if err != nil {
		return Config{}, fmt.Errorf("MAX_VIEWS: %w", err)
	}
	if cfg.MaxViews < 1 || cfg.MaxViews > 10000 {
		return Config{}, fmt.Errorf("MAX_VIEWS must be between 1 and 10000")
	}
	cfg.IDLengthBytes, err = getenvInt("ID_LENGTH_BYTES", 24)
	if err != nil {
		return Config{}, fmt.Errorf("ID_LENGTH_BYTES: %w", err)
	}
	if cfg.IDLengthBytes < 16 || cfg.IDLengthBytes > 64 {
		return Config{}, fmt.Errorf("ID_LENGTH_BYTES must be between 16 and 64")
	}
	cfg.RateLimitRPM, err = getenvInt("RATE_LIMIT_RPM", 60)
	if err != nil {
		return Config{}, fmt.Errorf("RATE_LIMIT_RPM: %w", err)
	}
	if cfg.RateLimitRPM < 1 || cfg.RateLimitRPM > 10000 {
		return Config{}, fmt.Errorf("RATE_LIMIT_RPM must be between 1 and 10000")
	}
	cfg.RateLimitBurst, err = getenvInt("RATE_LIMIT_BURST", 20)
	if err != nil {
		return Config{}, fmt.Errorf("RATE_LIMIT_BURST: %w", err)
	}
	if cfg.RateLimitBurst < 1 || cfg.RateLimitBurst > 10000 {
		return Config{}, fmt.Errorf("RATE_LIMIT_BURST must be between 1 and 10000")
	}
	cfg.MetricsEnabled, err = getenvBool("METRICS_ENABLED", false)
	if err != nil {
		return Config{}, fmt.Errorf("METRICS_ENABLED: %w", err)
	}
	cfg.RequirePassword, err = getenvBool("REQUIRE_PASSWORD", false)
	if err != nil {
		return Config{}, fmt.Errorf("REQUIRE_PASSWORD: %w", err)
	}
	cfg.StrictRedisEphemeral, err = getenvBool("STRICT_REDIS_EPHEMERAL", true)
	if err != nil {
		return Config{}, fmt.Errorf("STRICT_REDIS_EPHEMERAL: %w", err)
	}
	timeoutMs, err := getenvInt64("REQUEST_TIMEOUT_MS", 60000)
	if err != nil {
		return Config{}, fmt.Errorf("REQUEST_TIMEOUT_MS: %w", err)
	}
	cfg.RequestTimeout = time.Duration(timeoutMs) * time.Millisecond

	return cfg, nil
}

// LargestStorableFile returns the size of the largest file whose encrypted
// form fits within MaxCipherBytes. The browser adds a 16-byte authentication
// tag, base64-encodes the result, which makes it a third larger, and wraps it
// in a small JSON envelope together with the IV.
func (c Config) LargestStorableFile() int64 {
	const envelopeBytes = 64
	const tagBytes = 16
	encoded := int64(c.MaxCipherBytes) - envelopeBytes
	if encoded <= 0 {
		return 0
	}
	return encoded/4*3 - tagBytes
}

// Warnings returns problems in the configuration that do not stop the API
// from starting but are probably not what the operator meant.
func (c Config) Warnings() []string {
	var out []string
	if largest := c.LargestStorableFile(); c.MaxFileBytes > largest {
		out = append(out, fmt.Sprintf(
			"MAX_FILE_BYTES is %d, but MAX_CIPHERTEXT_BYTES only has room for files up to %d bytes once they are encrypted and encoded; larger files will be rejected. Raise MAX_CIPHERTEXT_BYTES and MAX_BODY_BYTES, or lower MAX_FILE_BYTES",
			c.MaxFileBytes, largest,
		))
	}
	if c.TrustedProxyCID != "" {
		if _, _, err := net.ParseCIDR(c.TrustedProxyCID); err != nil {
			out = append(out, "TRUSTED_PROXY_CIDR is not a valid CIDR and is ignored; every client behind the proxy shares one rate limit")
		}
	}
	return out
}

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func getenvInt(key string, fallback int) (int, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, err
	}
	return parsed, nil
}

func getenvInt64(key string, fallback int64) (int64, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, err
	}
	return parsed, nil
}

func getenvBool(key string, fallback bool) (bool, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, err
	}
	return parsed, nil
}

func parseAllowedOrigins(raw string) []string {
	out := parseCSV(raw)
	if len(out) == 0 {
		return []string{"http://localhost:5173"}
	}
	return out
}

func parseCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		value := strings.TrimSpace(p)
		if value == "" {
			continue
		}
		out = append(out, value)
	}
	return out
}
