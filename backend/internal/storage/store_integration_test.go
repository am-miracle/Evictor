package storage_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/am-miracle/evictor/internal/models"
	"github.com/am-miracle/evictor/internal/primitives"
	"github.com/am-miracle/evictor/internal/storage"
)

var (
	testStore  *storage.Store
	setupErr   error
	externalDB bool
)

// TestMain runs the storage suite against either an ephemeral testcontainers
// Postgres or, when TEST_DATABASE_URL is set, an external database. The external
// path (e.g. a disposable Neon branch) needs no Docker. It MUST point at a
// throwaway database: the suite applies migrations and one test tears them down.
func TestMain(m *testing.M) {
	ctx := context.Background()

	if dsn := os.Getenv("TEST_DATABASE_URL"); dsn != "" {
		externalDB = true
		os.Exit(runAgainst(ctx, m, dsn, nil))
	}

	ctr, dsn, err := startPostgres(ctx)
	if err != nil {
		// No Docker and no TEST_DATABASE_URL: tests skip themselves.
		setupErr = err
		os.Exit(m.Run())
	}
	os.Exit(runAgainst(ctx, m, dsn, ctr))
}

// runAgainst migrates dsn, opens a Store, runs the suite, and cleans up. ctr may
// be nil in the external-database path.
func runAgainst(ctx context.Context, m *testing.M, dsn string, ctr *tcpostgres.PostgresContainer) int {
	terminate := func() {
		if ctr != nil {
			_ = ctr.Terminate(ctx)
		}
	}
	if err := storage.RunMigrations(dsn); err != nil {
		setupErr = fmt.Errorf("migrate: %w", err)
		terminate()
		return m.Run()
	}
	store, err := storage.New(ctx, dsn)
	if err != nil {
		setupErr = err
		terminate()
		return m.Run()
	}
	testStore = store

	code := m.Run()

	store.Close()
	terminate()
	return code
}

func startPostgres(ctx context.Context) (*tcpostgres.PostgresContainer, string, error) {
	ctr, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("evictor"),
		tcpostgres.WithUsername("evictor"),
		tcpostgres.WithPassword("evictor"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		return nil, "", err
	}
	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		_ = ctr.Terminate(ctx)
		return nil, "", err
	}
	return ctr, dsn, nil
}

func requireStore(t *testing.T) *storage.Store {
	t.Helper()
	if setupErr != nil {
		t.Skipf("skipping: no database available (%v)", setupErr)
	}
	return testStore
}

func ptr[T any](v T) *T { return &v }

func seedProject(t *testing.T, s *storage.Store) *models.Project {
	t.Helper()
	p := &models.Project{ID: primitives.NewProjectID(), Name: "acme"}
	if err := s.InsertProject(context.Background(), p); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	return p
}

func seedEndpoint(t *testing.T, s *storage.Store, projectID, name string) *models.Endpoint {
	t.Helper()
	e := &models.Endpoint{
		ID:          primitives.NewEndpointID(),
		ProjectID:   projectID,
		Name:        name,
		Status:      models.EndpointActive,
		PriceSource: models.PriceSourceDefault,
	}
	if err := s.InsertEndpoint(context.Background(), e); err != nil {
		t.Fatalf("seed endpoint: %v", err)
	}
	return e
}

func TestProjectsRoundTrip(t *testing.T) {
	s := requireStore(t)
	ctx := context.Background()
	p := seedProject(t, s)

	got, err := s.GetProject(ctx, p.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "acme" || got.CreatedAt.IsZero() {
		t.Fatalf("got %+v", got)
	}
}

func TestAPIKeysRoundTrip(t *testing.T) {
	s := requireStore(t)
	ctx := context.Background()
	p := seedProject(t, s)

	k := &models.APIKey{
		ID:        primitives.NewAPIKeyID(),
		ProjectID: p.ID,
		KeyHash:   []byte{0xde, 0xad, 0xbe, 0xef},
		Last4:     "b8d1",
		Kind:      models.APIKeyIngestion,
		ExpiresAt: ptr(time.Now().Add(24 * time.Hour).UTC()),
	}
	if err := s.InsertAPIKey(ctx, k); err != nil {
		t.Fatalf("insert: %v", err)
	}
	got, err := s.GetAPIKey(ctx, k.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Kind != models.APIKeyIngestion || got.Last4 != "b8d1" || len(got.KeyHash) != 4 {
		t.Fatalf("got %+v", got)
	}
}

func TestProvidersRoundTrip(t *testing.T) {
	s := requireStore(t)
	ctx := context.Background()
	p := seedProject(t, s)

	prov := &models.Provider{
		ID:                   primitives.NewProviderID(),
		ProjectID:            p.ID,
		Kind:                 "runpod",
		Name:                 "prod-account",
		CredentialsEncrypted: []byte("ciphertext"),
		KeyLast4:             "9f2c",
		PollingHealth:        "ok",
	}
	if err := s.InsertProvider(ctx, prov); err != nil {
		t.Fatalf("insert: %v", err)
	}
	got, err := s.GetProvider(ctx, prov.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.KeyLast4 != "9f2c" || string(got.CredentialsEncrypted) != "ciphertext" {
		t.Fatalf("got %+v", got)
	}
}

func TestEndpointsRoundTrip(t *testing.T) {
	s := requireStore(t)
	ctx := context.Background()
	p := seedProject(t, s)

	e := &models.Endpoint{
		ID:                   primitives.NewEndpointID(),
		ProjectID:            p.ID,
		Name:                 "image-generator",
		Status:               "active",
		HourlyRateCents:      ptr(int64(210)),
		PriceSource:          "user_confirmed",
		ColdStartThresholdMs: nil,
	}
	if err := s.InsertEndpoint(ctx, e); err != nil {
		t.Fatalf("insert: %v", err)
	}
	got, err := s.GetEndpoint(ctx, e.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "image-generator" || got.HourlyRateCents == nil || *got.HourlyRateCents != 210 {
		t.Fatalf("got %+v", got)
	}
	if got.ProviderID != nil {
		t.Fatalf("expected nil provider_id, got %v", *got.ProviderID)
	}
}

func TestInferenceRequestsRoundTrip(t *testing.T) {
	s := requireStore(t)
	ctx := context.Background()
	p := seedProject(t, s)
	e := seedEndpoint(t, s, p.ID, "infer-ep")

	r := &models.InferenceRequest{
		ID:                primitives.NewRequestID(),
		EndpointID:        e.ID,
		LatencyMs:         14200,
		WasColdStart:      ptr(true),
		Classification:    "cold",
		ColdStartSource:   "reported",
		ProviderRequestID: ptr("runpod-abc123"),
		OccurredAt:        time.Now().UTC(),
		Metadata:          map[string]string{"region": "eu-west", "plan": "pro"},
	}
	if err := s.InsertInferenceRequest(ctx, r); err != nil {
		t.Fatalf("insert: %v", err)
	}
	got, err := s.GetInferenceRequest(ctx, r.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.LatencyMs != 14200 || got.Metadata["region"] != "eu-west" {
		t.Fatalf("got %+v", got)
	}
	if got.ReceivedAt.IsZero() {
		t.Fatalf("received_at not set")
	}
}

func TestStatusSnapshotsRoundTrip(t *testing.T) {
	s := requireStore(t)
	ctx := context.Background()
	p := seedProject(t, s)
	e := seedEndpoint(t, s, p.ID, "status-ep")

	snap := &models.StatusSnapshot{
		ID:          primitives.NewSnapshotID(),
		EndpointID:  e.ID,
		WorkerState: "warm",
		WorkerCount: ptr(int32(1)),
		TakenAt:     time.Now().UTC(),
	}
	if err := s.InsertStatusSnapshot(ctx, snap); err != nil {
		t.Fatalf("insert: %v", err)
	}
	got, err := s.GetStatusSnapshot(ctx, snap.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.WorkerState != "warm" || got.WorkerCount == nil || *got.WorkerCount != 1 {
		t.Fatalf("got %+v", got)
	}
}

func TestPricingDefaultsRoundTrip(t *testing.T) {
	s := requireStore(t)
	ctx := context.Background()

	// Insert a new tier, then read one of the migration-seeded rows.
	effective := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	d := &models.PricingDefault{ProviderKind: "runpod", GPUTier: "141GB", HourlyRateCents: 468, EffectiveFrom: effective}
	if err := s.InsertPricingDefault(ctx, d); err != nil {
		t.Fatalf("insert: %v", err)
	}
	got, err := s.GetPricingDefault(ctx, "runpod", "141GB", effective)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.HourlyRateCents != 468 {
		t.Fatalf("got %+v", got)
	}

	seeded, err := s.GetPricingDefault(ctx, "runpod", "80GB", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("get seeded: %v", err)
	}
	if seeded.HourlyRateCents != 217 {
		t.Fatalf("seed BR-8 rate = %d, want 217", seeded.HourlyRateCents)
	}
}

func TestWarmMediansRoundTrip(t *testing.T) {
	s := requireStore(t)
	ctx := context.Background()
	p := seedProject(t, s)
	e := seedEndpoint(t, s, p.ID, "median-ep")

	m := &models.WarmMedian{EndpointID: e.ID, MedianMs: 610, SampleCount: 42}
	if err := s.UpsertWarmMedian(ctx, m); err != nil {
		t.Fatalf("insert: %v", err)
	}
	// Upsert replaces in place, keeping the cache rebuildable (BR-23).
	m.MedianMs = 720
	m.SampleCount = 55
	if err := s.UpsertWarmMedian(ctx, m); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	got, err := s.GetWarmMedian(ctx, e.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.MedianMs != 720 || got.SampleCount != 55 {
		t.Fatalf("got %+v", got)
	}
}

// TestBR15_ActiveNameUniqueAfterSoftDelete proves the partial unique index:
// a duplicate active name is rejected, but the name frees up after soft delete.
func TestBR15_ActiveNameUniqueAfterSoftDelete(t *testing.T) {
	s := requireStore(t)
	ctx := context.Background()
	p := seedProject(t, s)

	seedEndpoint(t, s, p.ID, "dup-name")

	dupe := &models.Endpoint{
		ID:          primitives.NewEndpointID(),
		ProjectID:   p.ID,
		Name:        "dup-name",
		Status:      models.EndpointActive,
		PriceSource: models.PriceSourceDefault,
	}
	err := s.InsertEndpoint(ctx, dupe)
	if !isUniqueViolation(err) {
		t.Fatalf("expected unique violation on duplicate active name, got %v", err)
	}

	if _, err := s.Pool().Exec(ctx,
		`UPDATE endpoints SET status = 'deleted' WHERE project_id = $1 AND name = 'dup-name'`,
		p.ID); err != nil {
		t.Fatalf("soft delete: %v", err)
	}

	dupe.ID = primitives.NewEndpointID()
	if err := s.InsertEndpoint(ctx, dupe); err != nil {
		t.Fatalf("reusing name after soft delete should succeed, got %v", err)
	}
}

// TestMigrateDownReverses proves migrate-down fully reverses on a fresh database.
// It provisions its own throwaway Postgres, so it is skipped in the external-DB
// path where dropping every table would clobber the shared test database.
func TestMigrateDownReverses(t *testing.T) {
	if externalDB {
		t.Skip("skipping: destructive; covered by CI/testcontainers or `make migrate-down`")
	}
	if setupErr != nil {
		t.Skipf("skipping: no database available (%v)", setupErr)
	}
	ctx := context.Background()
	ctr, dsn, err := startPostgres(ctx)
	if err != nil {
		t.Skipf("skipping: no database available (%v)", err)
	}
	defer func() { _ = ctr.Terminate(ctx) }()

	if err := storage.RunMigrations(dsn); err != nil {
		t.Fatalf("up: %v", err)
	}
	if err := storage.MigrateDown(dsn); err != nil {
		t.Fatalf("down: %v", err)
	}

	store, err := storage.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer store.Close()

	var count int
	if err := store.Pool().QueryRow(ctx,
		`SELECT count(*) FROM information_schema.tables
		 WHERE table_schema = 'public' AND table_name = 'projects'`).Scan(&count); err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != 0 {
		t.Fatalf("projects table still present after migrate down")
	}
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
