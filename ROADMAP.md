# Evictor Roadmap

Evictor helps teams understand the cost of cold starts and idle warm workers
in serverless GPU systems. The roadmap follows that product boundary: first
make the data trustworthy, then add control features.

## Current direction

Phase 1 is the priority. It is a read-only observability product. No provider
state is changed in Phase 1, and no feature should depend on a future
prewarming system.

The repository foundation is being built first: a Go API, a Next.js dashboard,
PostgreSQL, Docker Compose, CI, strict type checking, and repeatable local
checks.

## Before implementation: validate the problem

Two decisions must be settled before the numbered work continues:

1. Speak with 15 teams and keep verbatim notes. At least five must confirm
   cold-start latency or idle GPU spend as a top-three infrastructure problem.
2. Complete the RunPod API spike. Record worker-state polling, workersMin
   mutation behavior, latency, and cost in an integration ADR.

The team reviews both results and decides whether to proceed, pivot, or stop.
Task 008 cannot begin without the provider ADR.

## Phase 1: observability

### Foundation

Build the platform needed by every later task:

- Monorepo setup, Docker Compose, and CI
- PostgreSQL schema, migrations, and prefixed IDs
- Deterministic mock provider with fault injection
- Configuration, structured logging, graceful shutdown, and bounded workers

Exit condition: a clean checkout starts a healthy API, migrated database, and
mock provider. CI is green.

### Ingestion and authentication

Add the authentication and ingestion spine:

- Ingestion and dashboard credentials
- Contract error responses and pagination
- `POST /api/v1/requests`
- Bounded request processing and backpressure
- Cold-start classification and rolling warm medians

Exit condition: ingestion meets the local throughput target and every cold-start
business rule has a focused test.

### Provider polling and metrics

Add read-only provider integration and the metrics engine:

- Mock-provider integration tests first
- RunPod status polling with backoff and visible health states
- Recorded fixtures for real-provider responses
- Cold-start, latency, idle-spend, and latency-cost calculations

Exit condition: hand-calculated synthetic workloads match the metrics engine.
CI makes no live provider calls.

### Dashboard and settings

Expose trusted data through the product:

- Session-authenticated dashboard API
- Overview and endpoint detail views
- Endpoint management
- Trust badges for inferred, estimated, calibrating, and degraded data
- Provider connection verification
- API-key rotation and pricing overrides

Exit condition: seeded data exercises every state described in the feature
specification without manual database edits.

### Proof and hardening

Prepare for an open-source release:

- Five-minute quick start and seed data
- Load, memory-soak, and chaos testing
- Kill-and-restore verification
- Seventy-two hours of dogfooding against one real endpoint

Exit condition: all hardening findings are recorded, the installer works, and
the release gates are met.

## Phase 1 release gate

Phase 1 is complete when:

- All foundation, ingestion, provider, metrics, dashboard, settings, seed, and
  hardening targets pass
- Twenty real installations have data flowing
- Three users report learning something from the dashboard that they did not
  know before using it

After this gate, the team focuses for two weeks on installation and onboarding
issues before starting Phase 2.

## Phase 2: controlled prewarming

Phase 2 starts only after the Phase 1 release gate passes. The order is fixed:

1. Durable revert records and startup reconciliation
2. Spend caps, cooldowns, and maximum warm duration
3. Read-only decision history and policy explanations
4. RunPod prewarm action
5. Pattern-based prediction
6. Decision and prewarm history in the dashboard
7. Benchmark against a simple min-workers schedule
8. Hosted product, billing, and first paying customer

No prewarm action ships before durable revert safety exists. No policy action
ships without a configured spend cap.

## Sequencing rules

- The mock provider comes before the metrics engine. It is the test harness for
  downstream work.
- The policy engine comes before prediction. A predictor must have a circuit
  breaker before it can influence spend.
- Revert safety comes before provider mutation. Phase 1 provider adapters are
  read-only and return not implemented for prewarming.

## Explicit non-goals

Until the Phase 1 gate passes, do not build:

- Provider mutation or prewarming
- Prediction models
- Spend-policy automation
- A logo, landing page, or dashboard mockup before the validation meeting
- Phase 3 routing, failover, RBAC, billing, or audit systems

When a new idea does not fit this roadmap, record it as a separate proposal or
ADR. Do not add it to an active task by implication.

## Source documents

- [Build order](docs/EVICTOR_BUILD_ORDER.md)
- [Planning](docs/EVICTOR_PLANNING.md)
- [Feature specification](docs/EVICTOR_FEATURE_SPEC.md)
- [Business rules](docs/EVICTOR_BUSINESS_RULES.md)
- [API contract](docs/EVICTOR_API_CONTRACT.md)
