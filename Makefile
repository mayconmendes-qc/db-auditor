# Container-first workflow: only Podman (or Docker) Compose is required on the host.
# No local Go, Bun, Node, sqlc, or curl installation is needed for day-to-day work.

COMPOSE ?= podman compose
AUDITOR_HTTP_PORT ?= 8080
FRONTEND_HTTP_PORT ?= 5173

export AUDITOR_HTTP_PORT
export FRONTEND_HTTP_PORT

.PHONY: up down logs reset-volume smoke ps \
	backend-fmt backend-test backend-test-cover backend-check backend-lint backend-staticcheck \
	frontend-install frontend-check frontend-test frontend-typecheck

## Runtime -----------------------------------------------------------------

# Starts postgres + api (backend) + frontend.
# Override ports: FRONTEND_HTTP_PORT=5174 make up
up:
	@echo "API      → http://localhost:$(AUDITOR_HTTP_PORT)"
	@echo "Frontend → http://localhost:$(FRONTEND_HTTP_PORT)"
	@echo "(override with AUDITOR_HTTP_PORT / FRONTEND_HTTP_PORT in .env or the environment)"
	$(COMPOSE) up -d --build postgres api frontend

down:
	$(COMPOSE) --profile tools down

logs:
	$(COMPOSE) logs -f postgres api frontend

ps:
	$(COMPOSE) ps

# Destructive: removes named volumes (Postgres data + tool caches).
reset-volume:
	$(COMPOSE) --profile tools down -v

# Smoke via the API container (no host curl required).
smoke:
	@$(COMPOSE) exec -T api /busybox wget -q -O - http://localhost:8080/health >/dev/null
	@$(COMPOSE) exec -T api /busybox wget -q -O - http://localhost:8080/ready >/dev/null
	@echo "smoke ok: API healthy and ready"

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
