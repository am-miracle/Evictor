package logging_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/am-miracle/evictor/internal/config"
	"github.com/am-miracle/evictor/internal/logging"
)

func TestProductionLogsJSONAndDevelopmentLogsText(t *testing.T) {
	var production bytes.Buffer
	logging.New(config.Production, &production).Info("started")
	if !strings.HasPrefix(production.String(), "{") {
		t.Fatalf("production log is not JSON: %s", production.String())
	}

	var development bytes.Buffer
	logging.New(config.Development, &development).Info("started")
	if strings.HasPrefix(development.String(), "{") || !strings.Contains(development.String(), `msg=started`) {
		t.Fatalf("development log is not readable text: %s", development.String())
	}
}
