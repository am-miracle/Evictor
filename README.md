# Evictor

An observability and control layer for AI inference workloads running on serverless GPU providers.

## Local development

Requirements: Docker with Compose v2, Go 1.24+, and Node.js 22+.

```sh
npm install
npm --prefix frontend install
cp .env.example .env
docker compose up --build
```

Once the containers are healthy:

- API health: <http://localhost:8080/healthz>
- Dashboard: <http://localhost:3000>
- PostgreSQL: `localhost:5432`

Run all local checks with `make test` and `make lint`. Database migrations are
introduced by task 002; until then, `make migrate` is intentionally a no-op.

## Contribution rules

Repository hooks are installed by the root `npm install` command. They format
and lint staged frontend files, format staged Go files, enforce Conventional
Commits, and require the complete `make lint` and `make test` suites to pass
before both commit and push. Install `golangci-lint` locally before committing;
CI uses the same linter version declared in the workflow.

The branch flow is:

1. Update staging: `git fetch origin dev`.
2. Create work from staging: `git switch -c feat/<name> origin/dev`.
3. Push the feature branch and open a reviewed pull request into `dev`.


Direct pushes to `dev` and `master` are rejected. Feature branches that do not
contain the current `origin/dev` history are also rejected.
