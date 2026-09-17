.PHONY: up down logs backend-fmt backend-test backend-check backend-lint frontend-install frontend-check frontend-test frontend-typecheck

up:
	podman compose up -d --build

down:
	podman compose down

logs:
	podman compose logs -f

backend-fmt:
	cd backend && $(MAKE) fmt

backend-test:
	cd backend && $(MAKE) test

backend-check:
	cd backend && $(MAKE) check

backend-lint:
	cd backend && $(MAKE) lint

frontend-install:
	cd frontend && bun install

frontend-check:
	cd frontend && bun run check

frontend-test:
	cd frontend && bun run test

frontend-typecheck:
	cd frontend && bun run typecheck
