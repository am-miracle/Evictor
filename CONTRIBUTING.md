# Contributing to Evictor

Thanks for helping improve Evictor. Keep changes focused, tested, and easy to
review. Read the relevant contract, business rules, and planning document
before changing product behavior.

## Local setup

```sh
npm install
npm --prefix frontend install
./scripts/setup-secrets.sh dev
make dev
```

The API is available at `http://localhost:8080/healthz` and the dashboard at
`http://localhost:3000`.

## Checks

Run the same checks required by hooks and CI:

```sh
make lint
make test
```

Install `golangci-lint` at the version used by CI before committing. The root
Husky hooks run formatting, linting, tests, and Conventional Commit checks.
The same checks run in CI for every pull request.

### Database integration tests

Database integration tests are separate from the default local test command and
run in CI against two disposable PostgreSQL databases. To run them locally,
provide both database URLs explicitly:

```sh
TEST_DATABASE_URL='postgres://...' \
MIGRATION_TEST_DATABASE_URL='postgres://...' \
make test-integration
```

The URLs must point to separate throwaway databases whose names contain `test`.
The test runner rejects identical databases and names without that marker. The
migration test drops the complete application schema. Running the integration
target without both URLs fails instead of silently skipping the database suite.

Commit messages must use Conventional Commits, for example:

```text
feat(ingest): accept inference requests
fix(frontend): handle missing endpoint data
docs: clarify local setup
```

## Branch and pull request flow

The branch flow is `feature/*` to `dev`, then `dev` to `master`. Create work
from the latest staging branch:

```sh
git fetch origin dev
git switch -c feat/<name> origin/dev
```

Push the feature branch and open a reviewed pull request into `dev`. Direct
pushes to `dev` and `master` are prohibited. Only the repository owner
promotes reviewed changes from `dev` to `master`.

Before opening a pull request:

- run `make lint`
- run `make test`
- explain behavior changes and migration impact
- add or update tests for changed behavior
- keep secrets and environment-specific values out of tracked files

Pull requests should have one purpose, a clear description, and no unrelated
formatting or generated-file changes.

Never add credentials, private keys, real domains, registry identifiers, or
deployment secrets to tracked files. Use the ignored `secrets/` directory or a
secret manager.
