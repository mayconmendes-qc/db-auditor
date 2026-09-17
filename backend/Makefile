.PHONY: fmt test check staticcheck lint build

fmt:
	gofmt -w $$(find . -name '*.go')

test:
	go test ./...

check:
	go vet ./...

staticcheck:
	staticcheck ./...

lint:
	golangci-lint run

build:
	CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o bin/auditor ./cmd/auditor
