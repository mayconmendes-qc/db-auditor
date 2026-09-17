.PHONY: fmt test test-cover check staticcheck lint build sqlc

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './internal/database/sqlc/*')

test:
	go test ./...

test-cover:
	go test ./... -coverprofile=coverage.out
	go tool cover -func=coverage.out

check:
	go vet ./...

staticcheck:
	staticcheck ./...

lint:
	golangci-lint run

build:
	CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o bin/auditor ./cmd/auditor

sqlc:
	sqlc generate
