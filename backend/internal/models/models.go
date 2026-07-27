package models

import (
	"log/slog"
	"time"
)

type Project struct {
	ID        string
	Name      string
	CreatedAt time.Time
}

type APIKeyKind string

const (
	APIKeyIngestion APIKeyKind = "ingestion"
	APIKeySession   APIKeyKind = "session"
)

type APIKey struct {
	ID        string
	ProjectID string
	KeyHash   APIKeyHash
	Last4     string
	Kind      APIKeyKind
	ExpiresAt *time.Time
	CreatedAt time.Time
}

type APIKeyHash []byte

func (APIKeyHash) LogValue() slog.Value { return slog.StringValue("[REDACTED]") }

func (key APIKey) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("id", key.ID),
		slog.String("project_id", key.ProjectID),
		slog.String("last4", key.Last4),
		slog.String("kind", string(key.Kind)),
		slog.String("key_hash", "[REDACTED]"),
	)
}

type ProviderKind string

const (
	ProviderRunPod ProviderKind = "runpod"
)

type PollingHealth string

const (
	PollingHealthOK       PollingHealth = "ok"
	PollingHealthDegraded PollingHealth = "degraded"
	PollingHealthStopped  PollingHealth = "stopped"
)

type Provider struct {
	ID                   string
	ProjectID            string
	Kind                 ProviderKind
	Name                 string
	CredentialsEncrypted ProviderCredentials
	KeyLast4             string
	PollingHealth        PollingHealth
	CreatedAt            time.Time
}

type ProviderCredentials []byte

func (ProviderCredentials) LogValue() slog.Value { return slog.StringValue("[REDACTED]") }

func (provider Provider) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("id", provider.ID),
		slog.String("project_id", provider.ProjectID),
		slog.String("kind", string(provider.Kind)),
		slog.String("name", provider.Name),
		slog.String("key_last4", provider.KeyLast4),
		slog.String("polling_health", string(provider.PollingHealth)),
		slog.String("credentials_encrypted", "[REDACTED]"),
	)
}

type EndpointStatus string

const (
	EndpointActive  EndpointStatus = "active"
	EndpointDeleted EndpointStatus = "deleted"
)

type PriceSource string

const (
	PriceSourceDefault       PriceSource = "default"
	PriceSourceUserConfirmed PriceSource = "user_confirmed"
)

type Endpoint struct {
	ID                   string
	ProjectID            string
	Name                 string
	Status               EndpointStatus
	ProviderID           *string
	ProviderEndpointID   *string
	HourlyRateCents      *int64
	PriceSource          PriceSource
	ColdStartThresholdMs *int64
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type Classification string

const (
	ClassificationCold    Classification = "cold"
	ClassificationWarm    Classification = "warm"
	ClassificationPending Classification = "pending"
)

type ColdStartSource string

const (
	ColdStartReported ColdStartSource = "reported"
	ColdStartInferred ColdStartSource = "inferred"
)

type InferenceRequest struct {
	ID                string
	EndpointID        string
	LatencyMs         int64
	WasColdStart      *bool
	Classification    Classification
	ColdStartSource   ColdStartSource
	ProviderRequestID *string
	OccurredAt        time.Time
	ReceivedAt        time.Time
	Metadata          map[string]string
}

type WorkerState string

const (
	WorkerStateCold    WorkerState = "cold"
	WorkerStateWarming WorkerState = "warming"
	WorkerStateWarm    WorkerState = "warm"
	WorkerStateUnknown WorkerState = "unknown"
)

type StatusSnapshot struct {
	ID          string
	EndpointID  string
	WorkerState WorkerState
	WorkerCount *int32
	TakenAt     time.Time
}

type PricingDefault struct {
	ProviderKind    ProviderKind
	GPUTier         string
	HourlyRateCents int64
	EffectiveFrom   time.Time
}

type WarmMedian struct {
	EndpointID  string
	MedianMs    int64
	SampleCount int32
	ComputedAt  time.Time
}
