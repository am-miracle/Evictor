package models_test

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/am-miracle/evictor/internal/models"
)

func TestCredentialTypesRedactSecretMaterialFromLogs(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	logger.Info("secrets",
		"api_key", models.APIKey{KeyHash: []byte("api-key-material"), Last4: "1234"},
		"provider", models.Provider{CredentialsEncrypted: []byte("provider-material"), KeyLast4: "5678"},
		"api_key_hash", models.APIKeyHash("api-key-material"),
		"provider_credentials", models.ProviderCredentials("provider-material"),
	)

	got := output.String()
	for _, secret := range []string{"api-key-material", "provider-material"} {
		if strings.Contains(got, secret) {
			t.Fatalf("log contains credential material %q: %s", secret, got)
		}
	}
	if !strings.Contains(got, "[REDACTED]") {
		t.Fatalf("log does not record redaction: %s", got)
	}
}
