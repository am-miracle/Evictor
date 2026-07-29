package config_test

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/am-miracle/evictor/internal/config"
)

func TestLoadNamesMissingRequiredVariable(t *testing.T) {
	_, err := config.Load(func(key string) string {
		if key == "EVICTOR_ENCRYPTION_KEY" {
			return "encryption-secret"
		}
		return ""
	})
	if err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("expected DATABASE_URL error, got %v", err)
	}
}

func TestLoadReadsDirectValuesAndDefaults(t *testing.T) {
	values := map[string]string{
		"DATABASE_URL":           "postgres://localhost/evictor",
		"EVICTOR_ENCRYPTION_KEY": "encryption-secret",
	}
	got, err := config.Load(func(key string) string { return values[key] })
	if err != nil {
		t.Fatal(err)
	}
	if got.Port != 8080 || got.Environment != config.Development ||
		got.LogLevel != slog.LevelInfo || got.WorkerCount != 4 ||
		got.QueueCapacity != 100 || got.WorkerMaxAttempts != 3 {
		t.Fatalf("unexpected defaults: %+v", got)
	}
}

func TestLoadReadsLogLevel(t *testing.T) {
	values := map[string]string{
		"DATABASE_URL":           "postgres://localhost/evictor",
		"EVICTOR_ENCRYPTION_KEY": "encryption-secret",
		"LOG_LEVEL":              "warn",
	}
	got, err := config.Load(func(key string) string { return values[key] })
	if err != nil {
		t.Fatal(err)
	}
	if got.LogLevel != slog.LevelWarn {
		t.Fatalf("got log level %s, want WARN", got.LogLevel)
	}
}

func TestLoadRejectsInvalidLogLevel(t *testing.T) {
	values := map[string]string{
		"DATABASE_URL":           "postgres://localhost/evictor",
		"EVICTOR_ENCRYPTION_KEY": "encryption-secret",
		"LOG_LEVEL":              "verbose",
	}
	_, err := config.Load(func(key string) string { return values[key] })
	if err == nil || !strings.Contains(err.Error(), "LOG_LEVEL") {
		t.Fatalf("expected LOG_LEVEL error, got %v", err)
	}
}

func TestLoadRejectsInvalidRole(t *testing.T) {
	values := map[string]string{
		"DATABASE_URL":           "postgres://localhost/evictor",
		"EVICTOR_ENCRYPTION_KEY": "encryption-secret",
		"EVICTOR_ROLE":           "scheduler",
	}
	_, err := config.Load(func(key string) string { return values[key] })
	if err == nil || !strings.Contains(err.Error(), "EVICTOR_ROLE") {
		t.Fatalf("expected EVICTOR_ROLE error, got %v", err)
	}
}

func TestLoadReadsDockerSecretFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "database_url")
	if err := os.WriteFile(path, []byte("postgres://localhost/evictor\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	values := map[string]string{
		"DATABASE_URL_FILE":      path,
		"EVICTOR_ENCRYPTION_KEY": "encryption-secret",
	}
	got, err := config.Load(func(key string) string { return values[key] })
	if err != nil {
		t.Fatal(err)
	}
	if got.DatabaseURL.Reveal() != "postgres://localhost/evictor" {
		t.Fatal("database secret file was not loaded")
	}
}

func TestSecretIsRedactedBySlog(t *testing.T) {
	const material = "credential-material"
	value := slog.AnyValue(config.Secret(material)).Resolve()
	if strings.Contains(value.String(), material) || value.String() != "[REDACTED]" {
		t.Fatalf("secret rendered as %q", value.String())
	}
}
