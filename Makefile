.PHONY: test check staticcheck lint fmt up down logs

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
