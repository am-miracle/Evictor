DELETE FROM pricing_defaults
WHERE provider_kind = 'runpod'
  AND effective_from = '2026-01-01T00:00:00Z'
  AND gpu_tier IN ('16GB', '24GB', '48GB', '80GB');
