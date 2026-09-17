# Container-first workflow: only Podman (or Docker) Compose is required on the host.
# No local Go, Bun, Node, sqlc, or curl installation is needed for day-to-day work.

COMPOSE ?= podman compose

.PHONY: up down logs reset-volume smoke frontend-up \
	backend-fmt backend-test backend-test-cover backend-check backend-lint backend-staticcheck \
	frontend-install frontend-check frontend-test frontend-typecheck frontend-dev

## Runtime -----------------------------------------------------------------

up:
	$(COMPOSE) up -d --build postgres api

down:
	$(COMPOSE) --profile frontend --profile tools down

logs:
	$(COMPOSE) logs -f postgres api

# Destructive: removes named volumes (Postgres data + tool caches).
reset-volume:
	$(COMPOSE) --profile frontend --profile tools down -v

# Smoke via the API container (no host curl required).
smoke:
	@$(COMPOSE) exec -T api /busybox wget -q -O - http://localhost:8080/health >/dev/null
	@$(COMPOSE) exec -T api /busybox wget -q -O - http://localhost:8080/ready >/dev/null
	@echo "smoke ok: API healthy and ready"

## Frontend runtime --------------------------------------------------------

frontend-up:
	$(COMPOSE) --profile frontend up -d --build frontend

frontend-dev: frontend-up
	@echo "frontend: http://localhost:$${FRONTEND_HTTP_PORT:-5173}"

## Backend quality (containers) --------------------------------------------

backend-fmt:
	$(COMPOSE) --profile tools run --rm --no-deps backend-tools \
		sh -c 'gofmt -w $$(find . -name "*.go" -not -path "./internal/database/sqlc/*")'

backend-test:
	$(COMPOSE) --profile tools run --rm --no-deps backend-tools go test ./...

backend-test-cover:
	$(COMPOSE) --profile tools run --rm --no-deps backend-tools \
		sh -c 'go test ./... -coverprofile=coverage.out && go tool cover -func=coverage.out'

backend-check:
	$(COMPOSE) --profile tools run --rm --no-deps backend-tools go vet ./...

backend-staticcheck:
	$(COMPOSE) --profile tools run --rm --no-deps backend-tools \
		sh -c 'go install honnef.co/go/tools/cmd/staticcheck@latest && staticcheck ./...'

backend-lint:
	$(COMPOSE) --profile tools run --rm --no-deps golangci-lint golangci-lint run

## Frontend quality (containers) -------------------------------------------

frontend-install:
	$(COMPOSE) --profile tools run --rm --no-deps frontend-tools bun install

frontend-check:
	$(COMPOSE) --profile tools run --rm --no-deps frontend-tools sh -c 'bun install && bun run check'

frontend-test:
	$(COMPOSE) --profile tools run --rm --no-deps frontend-tools sh -c 'bun install && bun run test'

frontend-typecheck:
	$(COMPOSE) --profile tools run --rm --no-deps frontend-tools sh -c 'bun install && bun run typecheck'
