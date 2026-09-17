# Timescale Auditor

Serviço read-only de inventário, comparação e diagnóstico dos ambientes TimescaleDB (Tiger Cloud e Datacenter Unifique).

## Layout do repositório

```text
.
├── backend/          # API Go 1.27 + pgx + sqlc + migrations
├── frontend/         # React + TypeScript + Tailwind + Bun
├── compose.yaml      # Podman Compose (api + postgres)
├── .env.example
├── Makefile
└── .github/workflows # CI (lint, testes, Trivy)
```

## Stack

| Camada | Tecnologia |
|--------|------------|
| Backend | Go 1.27.x, `net/http`, `pgx`/`pgxpool`, `sqlc`, PostgreSQL |
| Frontend | React, TypeScript strict, Tailwind CSS, Vitest, BiomeJS, **Bun** |
| Runtime local | Podman Compose |

## Ambiente local

1. `cp .env.example .env` e defina `POSTGRES_PASSWORD`.
2. Suba a API e o Snapshot Store:

```bash
podman compose up -d --build
```

3. Verifique:

```bash
curl http://localhost:8080/health
curl http://localhost:8080/ready
```

PostgreSQL interno fica na rede privada (sem porta publicada por padrão). Volume `snapshot-store` é persistente.

```bash
podman compose down
```

### Frontend (Bun)

```bash
cd frontend
bun install
bun run dev
```

O frontend consome **apenas** a API HTTP — nunca PostgreSQL/TimescaleDB diretamente.

## Qualidade

### Backend

```bash
make backend-fmt
make backend-check    # go vet
make backend-lint     # golangci-lint
make backend-test
```

Ou dentro de `backend/`:

```bash
make fmt check lint test
```

### Frontend

```bash
make frontend-install
make frontend-check
make frontend-typecheck
make frontend-test
```

Ou:

```bash
cd frontend
bun run check
bun run typecheck
bun run test
```

## CI (GitHub Actions)

Pipeline em `.github/workflows/ci.yml`:

- **Backend:** `gofmt`, `go vet`, `golangci-lint`, `go test`, Trivy (filesystem)
- **Frontend:** Biome, TypeScript, Vitest (via Bun), Trivy (filesystem)

## Segurança

- Ambientes auditados: somente leitura
- Secrets fora do repositório
- Connection strings sanitizadas em logs
- `statement_timeout`, `lock_timeout`, `application_name` configuráveis

## Documentação de produto

Especificação funcional, backlog e fluxo de desenvolvimento: Notion (*Anotações / Timescale Auditor*).
