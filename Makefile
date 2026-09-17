.PHONY: test check staticcheck lint fmt up down logs web-check web-test web-typecheck web-install

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './web/*')

test:
	go test ./...

check:
	go vet ./...

staticcheck:
	staticcheck ./...

lint:
	golangci-lint run

up:
	podman compose up -d --build

down:
	podman compose down

logs:
	podman compose logs -f

web-install:
	cd web && npm install

web-check:
	cd web && npm run check

web-test:
	cd web && npm run test

web-typecheck:
	cd web && npm run typecheck
