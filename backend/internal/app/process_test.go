package app_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"

	"github.com/am-miracle/evictor/internal/app"
	"github.com/am-miracle/evictor/internal/workers"
)

type fakeDatabase struct{ closed bool }

func (fakeDatabase) Ping(context.Context) error { return nil }
func (f *fakeDatabase) Close()                  { f.closed = true }

type fakeDrainer struct {
	started bool
	waitErr error
}

func (f *fakeDrainer) Start(context.Context) { f.started = true }
func (f *fakeDrainer) Wait(context.Context) error {
	return f.waitErr
}

type harness struct {
	process  app.Process
	logs     *bytes.Buffer
	failures *bytes.Buffer
	database *fakeDatabase
	drainer  *fakeDrainer
}

func newHarness(t *testing.T, overrides map[string]string, args ...string) *harness {
	t.Helper()
	values := map[string]string{
		"DATABASE_URL":           "postgres://localhost/evictor",
		"EVICTOR_ENCRYPTION_KEY": "encryption-secret",
	}
	for key, value := range overrides {
		values[key] = value
	}
	h := &harness{
		logs:     &bytes.Buffer{},
		failures: &bytes.Buffer{},
		database: &fakeDatabase{},
		drainer:  &fakeDrainer{},
	}
	if len(args) == 0 {
		args = []string{"evictor"}
	}
	h.process = app.Process{
		Lookup:      func(key string) string { return values[key] },
		Args:        args,
		Output:      h.logs,
		ErrorOutput: h.failures,
		Listen: func(string, string) (net.Listener, error) {
			return net.Listen("tcp", "127.0.0.1:0")
		},
		OpenDatabase: func(context.Context, string) (app.Database, error) {
			return h.database, nil
		},
		NewWorkers: func(workers.Options) app.Drainer { return h.drainer },
	}
	return h
}

// cancelled returns a context already past shutdown, so Run walks its whole
// startup path and then unwinds immediately.
func cancelled() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

func TestRunExitsZeroOnCleanShutdown(t *testing.T) {
	h := newHarness(t, nil)

	if code := h.process.Run(cancelled()); code != 0 {
		t.Fatalf("exit code = %d, want 0; logs:\n%s", code, h.logs)
	}
	if !h.drainer.started {
		t.Error("worker pool was never started")
	}
	if !h.database.closed {
		t.Error("database was never closed")
	}
}

func TestRunExitsZeroWhenWorkersAbandonWork(t *testing.T) {
	h := newHarness(t, nil)
	h.drainer.waitErr = fmt.Errorf("%w: 2 job(s) unaccounted for", workers.ErrDrainIncomplete)

	code := h.process.Run(cancelled())

	if code != 0 {
		t.Fatalf("exit code = %d, want 0 -- an orderly shutdown that abandoned work is not a failed process", code)
	}
	if !strings.Contains(h.logs.String(), "abandoned") {
		t.Fatalf("abandoned work was not logged:\n%s", h.logs)
	}
}

func TestRunExitsOneWhenDrainTimesOut(t *testing.T) {
	h := newHarness(t, nil)
	h.drainer.waitErr = context.DeadlineExceeded

	if code := h.process.Run(cancelled()); code != 1 {
		t.Fatalf("exit code = %d, want 1; logs:\n%s", code, h.logs)
	}
}

func TestRunReportsConfigFailureBeforeStarting(t *testing.T) {
	h := newHarness(t, map[string]string{"LOG_LEVEL": "verbose"})

	if code := h.process.Run(cancelled()); code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(h.failures.String(), "LOG_LEVEL") {
		t.Fatalf("failure did not name the variable: %q", h.failures.String())
	}
	if h.drainer.started {
		t.Error("workers started despite unusable configuration")
	}
}

func TestRunReportsUnavailableDatabase(t *testing.T) {
	h := newHarness(t, nil)
	h.process.OpenDatabase = func(context.Context, string) (app.Database, error) {
		return nil, errors.New("offline")
	}

	if code := h.process.Run(cancelled()); code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if h.drainer.started {
		t.Error("workers started without a database")
	}
}

func TestRunReportsListenFailure(t *testing.T) {
	h := newHarness(t, nil)
	h.process.Listen = func(string, string) (net.Listener, error) {
		return nil, errors.New("address in use")
	}

	if code := h.process.Run(cancelled()); code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !h.database.closed {
		t.Error("database was not closed after a failed listen")
	}
}

func TestRunDispatchesWorkerRoleFromArguments(t *testing.T) {
	h := newHarness(t, nil, "evictor", "worker")

	if code := h.process.Run(cancelled()); code != 0 {
		t.Fatalf("exit code = %d, want 0; logs:\n%s", code, h.logs)
	}
	if h.drainer.started {
		t.Error("worker role started the API's pool")
	}
}

func TestRunRejectsInvalidRoleArgument(t *testing.T) {
	h := newHarness(t, nil, "evictor", "nonsense")

	if code := h.process.Run(cancelled()); code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
}
