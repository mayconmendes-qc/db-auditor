# Timescale Auditor

Serviço **read-only** de inventário, comparação e diagnóstico dos ambientes TimescaleDB (Tiger Cloud e Datacenter Unifique).

**MVP 0.12** — sprints 0–12 no backlog Notion concluídas no repositório (fundação → inventário → findings → performance → ops → dashboard → release).

## Pré-requisito no host

Apenas **Podman** (ou Docker) com **Compose**. Não é necessário instalar Go, Bun, Node, sqlc ou curl na máquina local — o `Makefile` executa ferramentas em containers.

```bash
# exemplo Fedora / RHEL
sudo dnf install podman podman-compose
```

## Estrutura canônica

```text
.
├── backend/          # API Go + sqlc + migrations + Containerfile
├── frontend/         # React + TS + Tailwind + Bun
├── deploy/           # compose.prod, Caddy, Grafana stub
├── docs/             # ops, mvp-release, hardening
├── compose.yaml
├── Makefile
└── README.md
```

## Ambiente local

```bash
cp .env.example .env   # defina POSTGRES_PASSWORD
make up
make smoke
```

| Serviço | URL |
|---------|-----|
| Frontend | http://localhost:5173 |
| API | http://localhost:8080 |
| Metrics | http://localhost:8080/metrics |
| Status | http://localhost:8080/api/v1/status |
| Dashboard KPIs | http://localhost:8080/api/v1/analytics/kpis |

Produção (VPS): ver `docs/ops.md` e `deploy/compose.prod.yaml`.

Release MVP: `docs/mvp-release.md`.

## Qualidade

```bash
make backend-fmt backend-check backend-staticcheck backend-lint backend-test
make frontend-check frontend-typecheck frontend-test
```

## Segurança

- Ambientes auditados: somente leitura
- Secrets fora do repositório
- Connection strings sanitizadas em logs
- Allowlist/denylist de databases e schemas
- Analyzers **nunca** recomendam DROP/REVOKE/VACUUM automático

## Documentação de produto

Notion: *Anotações / Timescale Auditor* (EF, Backlog, Fluxo de Desenvolvimento).
