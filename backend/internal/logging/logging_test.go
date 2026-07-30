package logging_test

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/am-miracle/evictor/internal/config"
	"github.com/am-miracle/evictor/internal/logging"
)

func TestProductionLogsJSONAndDevelopmentLogsText(t *testing.T) {
	var production bytes.Buffer
	logging.New(config.Production, slog.LevelInfo, &production).Info("started")
	if !strings.HasPrefix(production.String(), "{") {
		t.Fatalf("production log is not JSON: %s", production.String())
	}

	var development bytes.Buffer
	logging.New(config.Development, slog.LevelInfo, &development).Info("started")
	if strings.HasPrefix(development.String(), "{") || !strings.Contains(development.String(), `msg=started`) {
		t.Fatalf("development log is not readable text: %s", development.String())
	}
}

func TestNewFiltersEventsBelowConfiguredLevel(t *testing.T) {
	var output bytes.Buffer
	logger := logging.New(config.Production, slog.LevelWarn, &output)

	logger.Info("routine request")
	logger.Warn("degraded dependency")

	if strings.Contains(output.String(), "routine request") {
		t.Fatalf("info event passed warn filter: %s", output.String())
	}
	if !strings.Contains(output.String(), "degraded dependency") {
		t.Fatalf("warn event was filtered out: %s", output.String())
	}
}
