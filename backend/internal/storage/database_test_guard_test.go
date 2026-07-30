package storage_test

import (
	"fmt"
	"net/url"
	"strings"
	"testing"
)

type testDatabaseIdentity struct {
	name string
}

func validateIntegrationDatabaseURLs(mainDSN, migrationDSN string) error {
	mainDB, err := parseTestDatabaseURL("TEST_DATABASE_URL", mainDSN)
	if err != nil {
		return err
	}
	migrationDB, err := parseTestDatabaseURL("MIGRATION_TEST_DATABASE_URL", migrationDSN)
	if err != nil {
		return err
	}
	if mainDB.name == migrationDB.name {
		return fmt.Errorf("TEST_DATABASE_URL and MIGRATION_TEST_DATABASE_URL must use different database names")
	}
	return nil
}

func parseTestDatabaseURL(variable, dsn string) (testDatabaseIdentity, error) {
	parsed, err := url.Parse(dsn)
	if err != nil {
		return testDatabaseIdentity{}, fmt.Errorf("%s must be a valid PostgreSQL URL: %w", variable, err)
	}
	if parsed.Scheme != "postgres" && parsed.Scheme != "postgresql" {
		return testDatabaseIdentity{}, fmt.Errorf("%s must use the postgres or postgresql URL scheme", variable)
	}
	if parsed.Hostname() == "" {
		return testDatabaseIdentity{}, fmt.Errorf("%s must include a database host", variable)
	}

	name, err := url.PathUnescape(strings.TrimPrefix(parsed.EscapedPath(), "/"))
	if err != nil || name == "" || strings.Contains(name, "/") {
		return testDatabaseIdentity{}, fmt.Errorf("%s must include exactly one database name", variable)
	}
	name = strings.ToLower(name)
	if !strings.Contains(name, "test") {
		return testDatabaseIdentity{}, fmt.Errorf("%s database name %q must contain \"test\"", variable, name)
	}

	return testDatabaseIdentity{
		name: name,
	}, nil
}

func TestValidateIntegrationDatabaseURLs(t *testing.T) {
	t.Run("accepts separate test databases", func(t *testing.T) {
		err := validateIntegrationDatabaseURLs(
			"postgres://user:secret@localhost:5432/evictor_test?sslmode=disable",
			"postgres://user:secret@localhost:5432/evictor_migration_test?sslmode=disable",
		)
		if err != nil {
			t.Fatalf("expected safe URLs, got %v", err)
		}
	})

	t.Run("rejects a non-test database", func(t *testing.T) {
		err := validateIntegrationDatabaseURLs(
			"postgres://localhost/evictor_test",
			"postgres://localhost/evictor_production",
		)
		if err == nil || !strings.Contains(err.Error(), `must contain "test"`) {
			t.Fatalf("expected test-name error, got %v", err)
		}
	})

	t.Run("rejects the same database name despite different connection details", func(t *testing.T) {
		err := validateIntegrationDatabaseURLs(
			"postgres://first:secret@DB.EXAMPLE.com/evictor_test?sslmode=require",
			"postgresql://second:secret@pooler.example.com:5432/evictor_test?sslmode=disable",
		)
		if err == nil || !strings.Contains(err.Error(), "must use different database names") {
			t.Fatalf("expected separate-database error, got %v", err)
		}
	})

	t.Run("rejects malformed and unsupported URLs", func(t *testing.T) {
		for _, dsn := range []string{
			"://broken",
			"https://localhost/evictor_test",
			"postgres:///evictor_test",
			"postgres://localhost/",
			"postgres://localhost/one/two",
		} {
			err := validateIntegrationDatabaseURLs(
				"postgres://localhost/evictor_test",
				dsn,
			)
			if err == nil {
				t.Errorf("expected %q to be rejected", dsn)
			}
		}
	})
}
