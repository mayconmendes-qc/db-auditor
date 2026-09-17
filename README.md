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
├── frontend/         # React + TS + Tailwind + Bun
├── compose.yaml      # postgres, api, frontend + profile tools
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

2. Suba **postgres + api + frontend** com um único comando:

```bash
make up
```

| Serviço | URL / nota |
|---------|------------|
| Frontend | http://localhost:5173 |
| API | http://localhost:8080 |
| Postgres | só na rede interna do compose (volume `snapshot-store` em `/var/lib/postgresql`) |

> **PostgreSQL 18:** o volume monta em `/var/lib/postgresql` (não `/var/lib/postgresql/data`). Se você já usou o path antigo, rode `make reset-volume` antes de `make up`.

3. Smoke check (sem curl no host):

```bash
make smoke
make ps
make logs
```

Parada e reset:

```bash
make down
make reset-volume   # destrutivo: apaga volumes nomeados
```

## Qualidade (também via containers)

```bash
make backend-fmt backend-check backend-staticcheck backend-lint backend-test
make backend-test-cover
make frontend-check frontend-typecheck frontend-test
```

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
