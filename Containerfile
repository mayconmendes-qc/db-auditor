FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/auditor ./cmd/auditor

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/auditor /auditor
COPY --from=busybox:1.37.0-musl /bin/busybox /busybox
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/auditor"]
