GOLANGCI_LINT ?= $(shell command -v golangci-lint 2>/dev/null || printf '%s/bin/golangci-lint' "$$(go env GOPATH)")

.PHONY: dev test lint migrate

dev:
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
	@echo "No migrations yet; task 002 adds the migration tool and schema."
