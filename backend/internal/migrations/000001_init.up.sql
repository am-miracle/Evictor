--Field names follow the API contract (snake_case). All money
-- (_cents) and durations (_ms) are integers; all timestamps are timestamptz.

CREATE TABLE projects (
    id         text        PRIMARY KEY,
    name       text        NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE api_keys (
    id         text        PRIMARY KEY,
    project_id text        NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    key_hash   bytea       NOT NULL,
    last4      text        NOT NULL,
    kind       text        NOT NULL CHECK (kind IN ('ingestion', 'session')),
    expires_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE providers (
    id                     text        PRIMARY KEY,
    project_id             text        NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    kind                   text        NOT NULL CHECK (kind IN ('runpod')),
    name                   text        NOT NULL,
    credentials_encrypted  bytea       NOT NULL,
    key_last4              text        NOT NULL,
    polling_health         text        NOT NULL DEFAULT 'ok'
                               CHECK (polling_health IN ('ok', 'degraded', 'stopped')),
    created_at             timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE endpoints (
    id                      text        PRIMARY KEY,
    project_id              text        NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    name                    text        NOT NULL,
    status                  text        NOT NULL DEFAULT 'active'
                                CHECK (status IN ('active', 'deleted')),
    provider_id             text        REFERENCES providers (id) ON DELETE SET NULL,
    provider_endpoint_id    text,
    hourly_rate_cents       bigint,
    price_source            text        NOT NULL DEFAULT 'default'
                                CHECK (price_source IN ('default', 'user_confirmed')),
    cold_start_threshold_ms bigint,
    created_at              timestamptz NOT NULL DEFAULT now(),
    updated_at              timestamptz NOT NULL DEFAULT now()
);

-- Name is unique within a project only among active endpoints; a soft-deleted
-- name can be reused.
CREATE UNIQUE INDEX endpoints_project_active_name_uniq
    ON endpoints (project_id, name)
    WHERE status = 'active';

CREATE TABLE inference_requests (
    id                  text        PRIMARY KEY,
    endpoint_id         text        NOT NULL REFERENCES endpoints (id) ON DELETE CASCADE,
    latency_ms          bigint      NOT NULL,
    was_cold_start      boolean,
    classification      text        NOT NULL CHECK (classification IN ('cold', 'warm', 'pending')),
    cold_start_source   text        NOT NULL CHECK (cold_start_source IN ('reported', 'inferred')),
    provider_request_id text,
    occurred_at         timestamptz NOT NULL,
    received_at         timestamptz NOT NULL DEFAULT now(),
    metadata            jsonb       NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX inference_requests_endpoint_occurred_idx
    ON inference_requests (endpoint_id, occurred_at DESC);

-- Deliberately not UNIQUE. BR-12 lets a provider_request_id recur as a new
-- request after 24h, which a static unique index would permanently forbid
-- (Postgres index predicates can't reference now(), so a rolling 24h window
-- isn't expressible as a constraint here). The write path is responsible for
-- closing the check-then-insert race (e.g. an advisory lock keyed on
-- endpoint_id+provider_request_id, or a serializable transaction), not this
-- index. Concurrent duplicate inserts are an ingestion-layer concern.
CREATE INDEX inference_requests_provider_request_idx
    ON inference_requests (endpoint_id, provider_request_id, occurred_at DESC)
    WHERE provider_request_id IS NOT NULL;

CREATE TABLE status_snapshots (
    id           text        PRIMARY KEY,
    endpoint_id  text        NOT NULL REFERENCES endpoints (id) ON DELETE CASCADE,
    worker_state text        NOT NULL CHECK (worker_state IN ('cold', 'warming', 'warm', 'unknown')),
    worker_count integer,
    taken_at     timestamptz NOT NULL
);

CREATE INDEX status_snapshots_endpoint_taken_idx
    ON status_snapshots (endpoint_id, taken_at DESC);

-- Versioned, effective-dated provider pricing
CREATE TABLE pricing_defaults (
    provider_kind     text        NOT NULL,
    gpu_tier          text        NOT NULL,
    hourly_rate_cents bigint      NOT NULL,
    effective_from    timestamptz NOT NULL,
    PRIMARY KEY (provider_kind, gpu_tier, effective_from)
);

-- Cached rolling warm median per endpoint; rebuildable from raw data
CREATE TABLE warm_medians (
    endpoint_id  text        PRIMARY KEY REFERENCES endpoints (id) ON DELETE CASCADE,
    median_ms    bigint      NOT NULL,
    sample_count integer     NOT NULL,
    computed_at  timestamptz NOT NULL DEFAULT now()
);
