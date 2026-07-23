# Evictor

An observability and control layer for AI inference workloads running on serverless GPU providers.

## Local development

Requirements: Docker with Compose v2, Go 1.24+, and Node.js 22+.

```sh
npm install
npm --prefix frontend install
cp .env.example .env
./scripts/setup-secrets.sh dev
docker compose up --build
```

Once the containers are healthy:

- API health: <http://localhost:8080/healthz>
- Dashboard: <http://localhost:3000>
- PostgreSQL: `localhost:5432`

Run all local checks with `make test` and `make lint`. Database migrations are
introduced by task 002; until then, `make migrate` is intentionally a no-op.
Generate the Conventional Commit changelog with `make changelog` after
installing [git-cliff](https://git-cliff.org/).

Production-like deployments use the same service shape for staging and
production. The deployment examples are in `deploy/`, and the compose files
are `docker-compose.staging.yml` and `docker-compose.production.yml`.

See [CONTRIBUTING.md](CONTRIBUTING.md) for the contributor workflow and
[SECURITY.md](SECURITY.md) for reporting security issues.
