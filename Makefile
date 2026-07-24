GOLANGCI_LINT ?= $(shell command -v golangci-lint 2>/dev/null || printf '%s/bin/golangci-lint' "$$(go env GOPATH)")

# DATABASE_URL points make migrate at the local Postgres. It defaults to the
# compose-generated secret with the host rewritten to localhost. Override on the
# command line for other environments, e.g. make migrate DATABASE_URL=postgres://...
DATABASE_URL ?= $(shell sed 's#@postgres:#@localhost:#' secrets/dev/database_url 2>/dev/null)
MIGRATE_DATABASE_URL = $(shell printf '%s' '$(DATABASE_URL)' | sed -E 's#^postgres(ql)?://#pgx5://#')
MIGRATE = go run -tags 'pgx5' github.com/golang-migrate/migrate/v4/cmd/migrate -path internal/migrations -database "$(MIGRATE_DATABASE_URL)"

.PHONY: dev test lint migrate migrate-down changelog deploy-staging deploy-production

dev:
	./scripts/setup-secrets.sh dev
	docker compose up --build

test:
	cd backend && go test ./...
	cd frontend && npm test

lint:
	cd backend && go vet ./...
	cd backend && $(GOLANGCI_LINT) run ./...
	cd frontend && npm run format:check
	cd frontend && npm run lint
	cd frontend && npm run typecheck

migrate:
	cd backend && $(MIGRATE) up

migrate-down:
	cd backend && $(MIGRATE) down -all

changelog:
	git-cliff --output CHANGELOG.md

deploy-staging:
	./scripts/deploy-staging.sh

deploy-production:
	./scripts/deploy-production.sh
