.PHONY: up down logs reset-volume smoke backend-fmt backend-test backend-test-cover backend-check backend-lint backend-staticcheck frontend-install frontend-check frontend-test frontend-typecheck

up:
	podman compose up -d --build

down:
	podman compose down

logs:
	podman compose logs -f

# Destructive: removes the named Postgres volume used by compose.
reset-volume:
	podman compose down -v

smoke:
	@curl -sf http://localhost:8080/health >/dev/null
	@curl -sf http://localhost:8080/ready >/dev/null
	@echo "smoke ok: API healthy and ready"

backend-fmt:
	cd backend && $(MAKE) fmt

backend-test:
	cd backend && $(MAKE) test

backend-test-cover:
	cd backend && $(MAKE) test-cover

backend-check:
	cd backend && $(MAKE) check

backend-lint:
	cd backend && $(MAKE) lint

backend-staticcheck:
	cd backend && $(MAKE) staticcheck

frontend-install:
	cd frontend && bun install

frontend-check:
	cd frontend && bun run check

frontend-test:
	cd frontend && bun run test

frontend-typecheck:
	cd frontend && bun run typecheck
