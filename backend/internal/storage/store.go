package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/am-miracle/evictor/internal/models"
)

func (s *Store) InsertProject(ctx context.Context, p *models.Project) error {
	return s.pool.QueryRow(ctx,
		`INSERT INTO projects (id, name) VALUES ($1, $2) RETURNING created_at`,
		p.ID, p.Name,
	).Scan(&p.CreatedAt)
}

func (s *Store) GetProject(ctx context.Context, id string) (*models.Project, error) {
	var p models.Project
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, created_at FROM projects WHERE id = $1`, id,
	).Scan(&p.ID, &p.Name, &p.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get project %s: %w", id, err)
	}
	return &p, nil
}

func (s *Store) InsertAPIKey(ctx context.Context, k *models.APIKey) error {
	return s.pool.QueryRow(ctx,
		`INSERT INTO api_keys (id, project_id, key_hash, last4, kind, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING created_at`,
		k.ID, k.ProjectID, k.KeyHash, k.Last4, string(k.Kind), k.ExpiresAt,
	).Scan(&k.CreatedAt)
}

func (s *Store) GetAPIKey(ctx context.Context, id string) (*models.APIKey, error) {
	var k models.APIKey
	var kind string
	err := s.pool.QueryRow(ctx,
		`SELECT id, project_id, key_hash, last4, kind, expires_at, created_at
		 FROM api_keys WHERE id = $1`, id,
	).Scan(&k.ID, &k.ProjectID, &k.KeyHash, &k.Last4, &kind, &k.ExpiresAt, &k.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get api_key %s: %w", id, err)
	}
	k.Kind = models.APIKeyKind(kind)
	return &k, nil
}

func (s *Store) InsertProvider(ctx context.Context, p *models.Provider) error {
	return s.pool.QueryRow(ctx,
		`INSERT INTO providers (id, project_id, kind, name, credentials_encrypted, key_last4, polling_health)
		 VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING created_at`,
		p.ID, p.ProjectID, string(p.Kind), p.Name, p.CredentialsEncrypted, p.KeyLast4, string(p.PollingHealth),
	).Scan(&p.CreatedAt)
}

func (s *Store) GetProvider(ctx context.Context, id string) (*models.Provider, error) {
	var p models.Provider
	var kind, pollingHealth string
	err := s.pool.QueryRow(ctx,
		`SELECT id, project_id, kind, name, credentials_encrypted, key_last4, polling_health, created_at
		 FROM providers WHERE id = $1`, id,
	).Scan(&p.ID, &p.ProjectID, &kind, &p.Name, &p.CredentialsEncrypted, &p.KeyLast4, &pollingHealth, &p.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get provider %s: %w", id, err)
	}
	p.Kind = models.ProviderKind(kind)
	p.PollingHealth = models.PollingHealth(pollingHealth)
	return &p, nil
}

func (s *Store) InsertEndpoint(ctx context.Context, e *models.Endpoint) error {
	return s.pool.QueryRow(ctx,
		`INSERT INTO endpoints
		   (id, project_id, name, status, provider_id, provider_endpoint_id,
		    hourly_rate_cents, price_source, cold_start_threshold_ms)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 RETURNING created_at, updated_at`,
		e.ID, e.ProjectID, e.Name, string(e.Status), e.ProviderID, e.ProviderEndpointID,
		e.HourlyRateCents, string(e.PriceSource), e.ColdStartThresholdMs,
	).Scan(&e.CreatedAt, &e.UpdatedAt)
}

func (s *Store) GetEndpoint(ctx context.Context, id string) (*models.Endpoint, error) {
	var e models.Endpoint
	var status, priceSource string
	err := s.pool.QueryRow(ctx,
		`SELECT id, project_id, name, status, provider_id, provider_endpoint_id,
		        hourly_rate_cents, price_source, cold_start_threshold_ms, created_at, updated_at
		 FROM endpoints WHERE id = $1`, id,
	).Scan(&e.ID, &e.ProjectID, &e.Name, &status, &e.ProviderID, &e.ProviderEndpointID,
		&e.HourlyRateCents, &priceSource, &e.ColdStartThresholdMs, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get endpoint %s: %w", id, err)
	}
	e.Status = models.EndpointStatus(status)
	e.PriceSource = models.PriceSource(priceSource)
	return &e, nil
}

func (s *Store) InsertInferenceRequest(ctx context.Context, r *models.InferenceRequest) error {
	metadata, err := json.Marshal(r.Metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}
	return s.pool.QueryRow(ctx,
		`INSERT INTO inference_requests
		   (id, endpoint_id, latency_ms, was_cold_start, classification, cold_start_source,
		    provider_request_id, occurred_at, metadata)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 RETURNING received_at`,
		r.ID, r.EndpointID, r.LatencyMs, r.WasColdStart, string(r.Classification), string(r.ColdStartSource),
		r.ProviderRequestID, r.OccurredAt, metadata,
	).Scan(&r.ReceivedAt)
}

func (s *Store) GetInferenceRequest(ctx context.Context, id string) (*models.InferenceRequest, error) {
	var r models.InferenceRequest
	var metadata []byte
	var classification, coldStartSource string
	err := s.pool.QueryRow(ctx,
		`SELECT id, endpoint_id, latency_ms, was_cold_start, classification, cold_start_source,
		        provider_request_id, occurred_at, received_at, metadata
		 FROM inference_requests WHERE id = $1`, id,
	).Scan(&r.ID, &r.EndpointID, &r.LatencyMs, &r.WasColdStart, &classification, &coldStartSource,
		&r.ProviderRequestID, &r.OccurredAt, &r.ReceivedAt, &metadata)
	if err != nil {
		return nil, fmt.Errorf("get inference_request %s: %w", id, err)
	}
	if err := json.Unmarshal(metadata, &r.Metadata); err != nil {
		return nil, fmt.Errorf("unmarshal metadata for %s: %w", id, err)
	}
	r.Classification = models.Classification(classification)
	r.ColdStartSource = models.ColdStartSource(coldStartSource)
	return &r, nil
}

func (s *Store) InsertStatusSnapshot(ctx context.Context, snap *models.StatusSnapshot) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO status_snapshots (id, endpoint_id, worker_state, worker_count, taken_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		snap.ID, snap.EndpointID, string(snap.WorkerState), snap.WorkerCount, snap.TakenAt,
	)
	if err != nil {
		return fmt.Errorf("insert status_snapshot %s: %w", snap.ID, err)
	}
	return nil
}

func (s *Store) GetStatusSnapshot(ctx context.Context, id string) (*models.StatusSnapshot, error) {
	var snap models.StatusSnapshot
	var workerState string
	err := s.pool.QueryRow(ctx,
		`SELECT id, endpoint_id, worker_state, worker_count, taken_at
		 FROM status_snapshots WHERE id = $1`, id,
	).Scan(&snap.ID, &snap.EndpointID, &workerState, &snap.WorkerCount, &snap.TakenAt)
	if err != nil {
		return nil, fmt.Errorf("get status_snapshot %s: %w", id, err)
	}
	snap.WorkerState = models.WorkerState(workerState)
	return &snap, nil
}

func (s *Store) InsertPricingDefault(ctx context.Context, d *models.PricingDefault) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO pricing_defaults (provider_kind, gpu_tier, hourly_rate_cents, effective_from)
		 VALUES ($1, $2, $3, $4)`,
		string(d.ProviderKind), d.GPUTier, d.HourlyRateCents, d.EffectiveFrom,
	)
	if err != nil {
		return fmt.Errorf("insert pricing_default %s/%s: %w", d.ProviderKind, d.GPUTier, err)
	}
	return nil
}

func (s *Store) GetPricingDefault(ctx context.Context, providerKind models.ProviderKind, gpuTier string, effectiveFrom time.Time) (*models.PricingDefault, error) {
	var d models.PricingDefault
	var kind string
	err := s.pool.QueryRow(ctx,
		`SELECT provider_kind, gpu_tier, hourly_rate_cents, effective_from
		 FROM pricing_defaults
		 WHERE provider_kind = $1 AND gpu_tier = $2 AND effective_from = $3`,
		string(providerKind), gpuTier, effectiveFrom,
	).Scan(&kind, &d.GPUTier, &d.HourlyRateCents, &d.EffectiveFrom)
	if err != nil {
		return nil, fmt.Errorf("get pricing_default %s/%s: %w", providerKind, gpuTier, err)
	}
	d.ProviderKind = models.ProviderKind(kind)
	return &d, nil
}

func (s *Store) UpsertWarmMedian(ctx context.Context, m *models.WarmMedian) error {
	return s.pool.QueryRow(ctx,
		`INSERT INTO warm_medians (endpoint_id, median_ms, sample_count)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (endpoint_id)
		 DO UPDATE SET median_ms = EXCLUDED.median_ms,
		               sample_count = EXCLUDED.sample_count,
		               computed_at = now()
		 RETURNING computed_at`,
		m.EndpointID, m.MedianMs, m.SampleCount,
	).Scan(&m.ComputedAt)
}

func (s *Store) GetWarmMedian(ctx context.Context, endpointID string) (*models.WarmMedian, error) {
	var m models.WarmMedian
	err := s.pool.QueryRow(ctx,
		`SELECT endpoint_id, median_ms, sample_count, computed_at
		 FROM warm_medians WHERE endpoint_id = $1`, endpointID,
	).Scan(&m.EndpointID, &m.MedianMs, &m.SampleCount, &m.ComputedAt)
	if err != nil {
		return nil, fmt.Errorf("get warm_median %s: %w", endpointID, err)
	}
	return &m, nil
}
