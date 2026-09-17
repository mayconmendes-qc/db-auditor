# Timescale Auditor

Serviço **read-only** de inventário, comparação e diagnóstico dos ambientes TimescaleDB (Tiger Cloud e Datacenter Unifique).

## Estrutura canônica do repositório

Monorepo com **dois pacotes** e artefatos de operação na raiz:

```text
.
├── backend/                 # API Go (única fonte de verdade do backend)
│   ├── cmd/auditor/         # entrypoint
│   ├── internal/            # api, config, database, …
│   ├── migrations/          # SQL do Snapshot Store
│   ├── sql/                 # queries sqlc
│   ├── Containerfile
│   ├── go.mod / go.sum
│   ├── Makefile
│   ├── sqlc.yaml
│   └── .golangci.yml
├── frontend/                # UI React + TS + Tailwind (Bun)
│   ├── src/
│   ├── package.json
│   ├── biome.json
│   ├── vite.config.ts
│   └── tsconfig.json
├── .github/workflows/ci.yml # CI (lint, testes, Trivy)
├── compose.yaml             # Podman Compose (api + postgres)
├── Makefile                 # atalhos raiz (backend-*, frontend-*)
├── .env.example
├── .gitignore
└── README.md
```

### O que **não** deve existir na raiz

| Path legado | Destino correto |
|-------------|-----------------|
| `internal/`, `cmd/`, `migrations/`, `sql/`, `sqlc.yaml`, `go.mod` | `backend/` |
| `web/` (npm) | `frontend/` (Bun) |

A CI, o Compose e o Makefile apontam **apenas** para `backend/` e `frontend/`.

## Stack

| Camada | Tecnologia |
|--------|------------|
| Backend | Go 1.27.x, `net/http`, `pgx`/`pgxpool`, `sqlc`, PostgreSQL |
| Frontend | React, TypeScript strict, Tailwind CSS, Vitest, Biome, **Bun** |
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

```bash
make backend-fmt backend-check backend-lint backend-test
make frontend-install frontend-check frontend-typecheck frontend-test
```

## CI (GitHub Actions)

`.github/workflows/ci.yml`:

- **Backend:** `gofmt`, `go vet`, `golangci-lint`, `go test`, Trivy FS em `backend/`
- **Frontend:** Biome, TypeScript, Vitest (Bun), Trivy FS em `frontend/`

## Segurança

- Ambientes auditados: somente leitura
- Secrets fora do repositório (`.env` no `.gitignore`)
- Connection strings sanitizadas em logs
- `statement_timeout`, `lock_timeout`, `application_name` configuráveis

## Documentação de produto

Especificação funcional, backlog e fluxo de desenvolvimento: Notion (*Anotações / Timescale Auditor*).
