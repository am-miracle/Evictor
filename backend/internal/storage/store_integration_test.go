//go:build integration

package storage_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/am-miracle/evictor/internal/models"
	"github.com/am-miracle/evictor/internal/primitives"
	"github.com/am-miracle/evictor/internal/storage"
)

var testStore *storage.Store

// TestMain requires two disposable databases: one for the storage suite and one
// for the destructive migration-down test.
func TestMain(m *testing.M) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		_, _ = fmt.Fprintln(os.Stderr, "integration tests require TEST_DATABASE_URL")
		os.Exit(1)
	}
	migrationDSN := os.Getenv("MIGRATION_TEST_DATABASE_URL")
	if migrationDSN == "" {
		_, _ = fmt.Fprintln(os.Stderr, "integration tests require MIGRATION_TEST_DATABASE_URL")
		os.Exit(1)
	}
	if err := validateIntegrationDatabaseURLs(dsn, migrationDSN); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "unsafe integration database configuration: %v\n", err)
		os.Exit(1)
	}

	os.Exit(runAgainst(context.Background(), m, dsn))
}

// runAgainst migrates dsn, opens a Store, and runs the suite. A configured
// database that cannot be prepared is a test failure rather than a skip.
func runAgainst(ctx context.Context, m *testing.M, dsn string) int {
	if err := storage.RunMigrations(dsn); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "prepare integration database: migrate: %v\n", err)
		return 1
	}
	store, err := storage.New(ctx, dsn)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "prepare integration database: connect: %v\n", err)
		return 1
	}
	testStore = store

	code := m.Run()

	store.Close()
	return code
}

func requireStore(t *testing.T) *storage.Store {
	t.Helper()
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

func TestEndpointRequiresProviderEndpointIDWhenProviderIsSet(t *testing.T) {
	s := requireStore(t)
	ctx := context.Background()
	p := seedProject(t, s)

	provider := &models.Provider{
		ID:                   primitives.NewProviderID(),
		ProjectID:            p.ID,
		Kind:                 models.ProviderRunPod,
		Name:                 "provider-for-invalid-endpoint",
		CredentialsEncrypted: []byte("ciphertext"),
		KeyLast4:             "9f2c",
		PollingHealth:        models.PollingHealthOK,
	}
	if err := s.InsertProvider(ctx, provider); err != nil {
		t.Fatalf("seed provider: %v", err)
	}

	endpoint := &models.Endpoint{
		ID:          primitives.NewEndpointID(),
		ProjectID:   p.ID,
		Name:        "missing-provider-endpoint-id",
		Status:      models.EndpointActive,
		ProviderID:  &provider.ID,
		PriceSource: models.PriceSourceDefault,
	}
	err := s.InsertEndpoint(ctx, endpoint)

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23514" ||
		pgErr.ConstraintName != "endpoints_provider_endpoint_required" {
		t.Fatalf("expected provider endpoint check violation, got %v", err)
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

// TestMigrateDownReverses proves migrate-down fully reverses on a separate,
// throwaway database because it drops the complete application schema.
func TestMigrateDownReverses(t *testing.T) {
	dsn := os.Getenv("MIGRATION_TEST_DATABASE_URL")
	ctx := context.Background()

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
