# Contributing to Evictor

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

Install `golangci-lint` at the version used by CI before committing. Husky
also enforces formatting, linting, tests, and Conventional Commits.

## Branch and pull request flow

Create feature branches from `dev`. Pull requests merge into `dev`; only the
repository owner promotes `dev` into `master` after review. Direct pushes to
either protected branch are prohibited.

Never add credentials, private keys, real domains, registry identifiers, or
deployment secrets to tracked files. Use the ignored `secrets/` directory or a
secret manager.
