package config

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var knownKeys = []string{
	"LISTEN_ADDR", "REDIS_URL", "MAX_BODY_BYTES", "MAX_META_BYTES", "MAX_CIPHERTEXT_BYTES",
	"MAX_FILE_BYTES", "ALLOWED_FILE_MIME_TYPES", "MAX_TTL_SECONDS", "MAX_VIEWS", "ID_LENGTH_BYTES",
	"REQUEST_TIMEOUT_MS", "ALLOWED_ORIGINS", "ALLOWED_ORIGIN", "TRUSTED_PROXY_CIDR", "RATE_LIMIT_RPM",
	"RATE_LIMIT_BURST", "METRICS_ENABLED", "REQUIRE_PASSWORD", "STRICT_REDIS_EPHEMERAL",
}

// cleanEnv makes the test independent of whatever the developer has exported.
func cleanEnv(t *testing.T) {
	t.Helper()
	for _, key := range knownKeys {
		t.Setenv(key, "")
	}
}

func setEnv(t *testing.T, values map[string]string) {
	t.Helper()
	for key, value := range values {
		t.Setenv(key, value)
	}
}

func TestLoadDefaults(t *testing.T) {
	cleanEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("defaults must load: %v", err)
	}
	if cfg.MaxFileBytes != 50*1024*1024 {
		t.Errorf("default MAX_FILE_BYTES = %d, want 50 MiB", cfg.MaxFileBytes)
	}
	if !cfg.StrictRedisEphemeral {
		t.Error("STRICT_REDIS_EPHEMERAL must default to true")
	}
	if cfg.MetricsEnabled {
		t.Error("METRICS_ENABLED must default to false")
	}
	if warnings := cfg.Warnings(); len(warnings) != 0 {
		t.Errorf("defaults must not produce warnings, got %v", warnings)
	}
}

func TestLoadRejectsBodyLimitOutOfRange(t *testing.T) {
	for _, value := range []string{"0", "-1", "100", "2147483648", "9223372036854775807"} {
		t.Run(value, func(t *testing.T) {
			cleanEnv(t)
			t.Setenv("MAX_BODY_BYTES", value)
			if _, err := Load(); err == nil {
				t.Fatalf("MAX_BODY_BYTES=%s must be rejected", value)
			}
		})
	}
}

func TestLoadRejectsBodyTooSmallForCiphertext(t *testing.T) {
	cleanEnv(t)
	setEnv(t, map[string]string{
		"MAX_BODY_BYTES":       "1048576",
		"MAX_CIPHERTEXT_BYTES": "1048576",
	})
	if _, err := Load(); err == nil {
		t.Fatal("a body limit without room for ciphertext and metadata must be rejected")
	}
}

func TestLargestStorableFile(t *testing.T) {
	cases := []struct {
		name        string
		cipherBytes int
		fileBytes   int64
		fits        bool
	}{
		{"default limits hold a 50 MiB file", 80 * 1024 * 1024, 50 * 1024 * 1024, true},
		{"140 MiB of ciphertext holds a 100 MiB file", 140 * 1024 * 1024, 100 * 1024 * 1024, true},
		{"300 MiB of ciphertext does not hold a 250 MiB file", 300 * 1024 * 1024, 250 * 1024 * 1024, false},
		{"340 MiB of ciphertext holds a 250 MiB file", 340 * 1024 * 1024, 250 * 1024 * 1024, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg := Config{MaxCipherBytes: c.cipherBytes}
			if got := cfg.LargestStorableFile() >= c.fileBytes; got != c.fits {
				t.Fatalf("LargestStorableFile() = %d, file of %d bytes fits = %v, want %v",
					cfg.LargestStorableFile(), c.fileBytes, got, c.fits)
			}
		})
	}
}

// The largest file reported as storable must really fit once it is encrypted
// and encoded the way the browser does it.
func TestLargestStorableFileFitsWhenEncoded(t *testing.T) {
	for _, cipherBytes := range []int{1024, 4096, 1000003, 80 * 1024 * 1024} {
		cfg := Config{MaxCipherBytes: cipherBytes}
		file := cfg.LargestStorableFile()
		if file <= 0 {
			t.Fatalf("no file fits in %d bytes of ciphertext", cipherBytes)
		}
		encrypted := file + 16                           // AES-GCM tag
		base64URL := (encrypted*4 + 2) / 3               // unpadded base64url
		envelope := int64(len(`{"iv":"","ct":""}`)) + 16 // 12-byte IV is 16 characters
		if total := base64URL + envelope; total > int64(cipherBytes) {
			t.Fatalf("file of %d bytes needs %d bytes of ciphertext, limit is %d", file, total, cipherBytes)
		}
	}
}

func TestWarnsWhenFileLimitExceedsCiphertextRoom(t *testing.T) {
	cleanEnv(t)
	setEnv(t, map[string]string{
		"MAX_FILE_BYTES":       "262144000",
		"MAX_CIPHERTEXT_BYTES": "314572800",
		"MAX_BODY_BYTES":       "367001600",
	})

	cfg, err := Load()
	if err != nil {
		t.Fatalf("the configuration is valid and must load: %v", err)
	}
	warnings := cfg.Warnings()
	if len(warnings) != 1 || !strings.Contains(warnings[0], "MAX_FILE_BYTES") {
		t.Fatalf("expected one warning about MAX_FILE_BYTES, got %v", warnings)
	}
}

func TestWarnsOnInvalidTrustedProxyCIDR(t *testing.T) {
	cleanEnv(t)
	t.Setenv("TRUSTED_PROXY_CIDR", "172.16.0.0")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("an invalid proxy range must not stop startup: %v", err)
	}
	warnings := cfg.Warnings()
	if len(warnings) != 1 || !strings.Contains(warnings[0], "TRUSTED_PROXY_CIDR") {
		t.Fatalf("expected one warning about TRUSTED_PROXY_CIDR, got %v", warnings)
	}
}

// readEnvFile returns the assignments in an env file. With commented set to
// true it returns the ones that are commented out, which is how the templates
// show alternatives.
func readEnvFile(t *testing.T, path string, commented bool) map[string]string {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer file.Close()

	active := regexp.MustCompile(`^([A-Z][A-Z0-9_]*)=(.*)$`)
	inactive := regexp.MustCompile(`^#\s+([A-Z][A-Z0-9_]*)=(\S+)$`)
	pattern := active
	if commented {
		pattern = inactive
	}

	out := map[string]string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if m := pattern.FindStringSubmatch(scanner.Text()); m != nil {
			out[m[1]] = m[2]
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return out
}

// The templates are what people copy. They must be accepted as they are.
func TestEnvTemplatesLoadWithoutWarnings(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	for _, name := range []string{filepath.Join("infra", ".env.example"), ".env.example"} {
		t.Run(name, func(t *testing.T) {
			cleanEnv(t)
			values := readEnvFile(t, filepath.Join(root, name), false)
			if len(values) < 10 {
				t.Fatalf("expected the template to set most variables, found %d", len(values))
			}
			for key, value := range values {
				if strings.HasPrefix(key, "VITE_") {
					continue
				}
				t.Setenv(key, value)
			}

			cfg, err := Load()
			if err != nil {
				t.Fatalf("%s must load: %v", name, err)
			}
			if warnings := cfg.Warnings(); len(warnings) != 0 {
				t.Fatalf("%s must not produce warnings, got %v", name, warnings)
			}
		})
	}
}

// infra/.env.example shows larger limits as commented-out lines. Whoever
// uncomments them must end up with a configuration that works.
func TestCommentedExamplesInProductionTemplateAreConsistent(t *testing.T) {
	cleanEnv(t)
	path := filepath.Join("..", "..", "..", "infra", ".env.example")
	for key, value := range readEnvFile(t, path, false) {
		if !strings.HasPrefix(key, "VITE_") {
			t.Setenv(key, value)
		}
	}
	examples := readEnvFile(t, path, true)
	for _, key := range []string{"MAX_FILE_BYTES", "MAX_CIPHERTEXT_BYTES", "MAX_BODY_BYTES", "ALLOWED_FILE_MIME_TYPES"} {
		value, ok := examples[key]
		if !ok {
			t.Fatalf("expected a commented example for %s", key)
		}
		t.Setenv(key, value)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("the commented examples must load: %v", err)
	}
	if cfg.MaxFileBytes != 100*1024*1024 {
		t.Errorf("example MAX_FILE_BYTES = %d, want 100 MiB", cfg.MaxFileBytes)
	}
	if warnings := cfg.Warnings(); len(warnings) != 0 {
		t.Fatalf("the commented examples must not produce warnings, got %v", warnings)
	}
}
