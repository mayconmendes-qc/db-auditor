# Timescale Auditor

Serviço **read-only** de inventário, comparação e diagnóstico dos ambientes TimescaleDB (Tiger Cloud e Datacenter Unifique).

## Pré-requisito no host

Apenas **Podman** (ou Docker) com **Compose**. Não é necessário instalar Go, Bun, Node, sqlc ou curl na máquina local — o `Makefile` executa ferramentas em containers.

```bash
# exemplo Fedora / RHEL
sudo dnf install podman podman-compose
# ou Docker Desktop / docker compose plugin
```

## Estrutura canônica do repositório

```text
.
├── backend/          # API Go + sqlc + migrations + Containerfile
├── frontend/         # React + TS + Tailwind + Bun (+ Containerfile opcional)
├── compose.yaml      # postgres, api, frontend (profile), tools (profile)
├── Makefile          # todos os comandos via containers
├── .env.example
└── README.md
```

### Responsabilidades dos packages (backend)

| Package | Responsabilidade |
|---------|------------------|
| `cmd/auditor` | Bootstrap do processo: config, pool, HTTP server, shutdown |
| `internal/api` | Fronteira HTTP; serialização JSON; sem SQL direto |
| `internal/config` | Validação de env; sanitização de secrets; allow/deny de escopo |
| `internal/database` | `pgxpool` e runtime params (`statement_timeout`, …) |
| `internal/database/sqlc` | Código **gerado** por sqlc — não editar à mão |
| `internal/repository` | Orquestra queries tipadas e `pgx.Tx` |

## Stack

| Camada | Tecnologia |
|--------|------------|
| Backend | Go 1.27.x, `net/http`, `pgx`/`pgxpool`, `sqlc`, PostgreSQL **18** |
| Frontend | React, TypeScript strict, Tailwind CSS, Vitest, Biome, **Bun** |
| Runtime local | Podman / Docker Compose |

## Ambiente local (container-first)

1. Configure o ambiente:

```bash
cp .env.example .env
# edite POSTGRES_PASSWORD
```

2. Suba API + Snapshot Store (PostgreSQL 18):

```bash
make up
# podman compose up -d --build postgres api
```

> **PostgreSQL 18:** o volume nomeado monta em `/var/lib/postgresql` (não mais `/var/lib/postgresql/data`). O cluster fica em `…/18/docker` dentro do volume.

3. Smoke check (sem curl no host):

```bash
make smoke
```

4. Frontend de desenvolvimento (profile `frontend`):

```bash
make frontend-up
# http://localhost:5173 — API em http://localhost:8080
```

Logs e parada:

```bash
make logs
make down
```

Reset **destrutivo** de volumes (dados do Postgres + caches de ferramentas):

```bash
make reset-volume
```

Se você já tentou subir com o mount antigo (`…/data`), rode `make reset-volume` antes de `make up` para recriar o volume no path correto.

## Qualidade (também via containers)

```bash
make backend-fmt backend-check backend-staticcheck backend-lint backend-test
make backend-test-cover
make frontend-check frontend-typecheck frontend-test
```

Esses alvos usam os serviços `backend-tools`, `frontend-tools` e `golangci-lint` do `compose.yaml` (profile `tools`).

## CI

- Backend: gofmt, vet, staticcheck, golangci-lint, `go test -cover`, Trivy
- Frontend: Biome, typecheck, Vitest, Trivy

## Segurança

- Ambientes auditados: somente leitura
- Secrets fora do repositório
- Connection strings sanitizadas em logs
- Allowlist/denylist de databases e schemas via env
- `statement_timeout`, `lock_timeout`, `application_name`

## Documentação de produto

Notion: *Anotações / Timescale Auditor* (EF, Backlog, Fluxo de Desenvolvimento).
