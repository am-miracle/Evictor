-- Seeds provider default pricing. Rows are effective-dated; a rate change
-- adds a new row rather than mutating one. Reconcile against live pricing before launch.
INSERT INTO pricing_defaults (provider_kind, gpu_tier, hourly_rate_cents, effective_from) VALUES
    ('runpod', '16GB', 40,  '2026-01-01T00:00:00Z'),
    ('runpod', '24GB', 69,  '2026-01-01T00:00:00Z'),
    ('runpod', '48GB', 109, '2026-01-01T00:00:00Z'),
    ('runpod', '80GB', 217, '2026-01-01T00:00:00Z');
