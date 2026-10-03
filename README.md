# DB Auditor

Serviço **read-only** de inventário, comparação e diagnóstico de ambientes PostgreSQL/TimescaleDB (ex.: Tiger Cloud e datacenter self-hosted).

**MVP 0.12** — sprints 0–12 no backlog concluídas no repositório (fundação → inventário → findings → performance → ops → dashboard → release).

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

### Relatórios PDF (Sprints 18–19)

Na Sprint 20, relatórios e aprovação de baseline usam a sessão da conta local e suas permissões. Crie o segredo inicial em `secrets/bootstrap-password` e siga o [guia operacional](docs/sprint20-operations.md); não coloque credenciais no frontend. Use HTTPS em produção.

Os pedidos são assíncronos e idempotentes por execução, versão de regras, tipo e filtros. Os PDFs ficam no snapshot store por 30 dias; os metadados do histórico são preservados por mais 90 dias. Uma execução parcial aparece com cobertura limitada e não é usada para inferir resolução de findings. A geração é limitada a 500 bancos, 5.000 tabelas, 2.000 findings e 16 MiB por PDF, com truncamento declarado no documento.

Em uma instalação com volume PostgreSQL existente, aplique apenas as migrações incrementais ainda pendentes, incluindo `10_sprint20_identity.sql`, antes de iniciar a API atualizada. Não reinicialize o volume de produção; veja [o guia operacional da Sprint 20](docs/sprint20-operations.md) e `backend/migrations/README.md`.

Relatórios só podem ser solicitados para execuções de auditoria concluídas cuja análise de regras também tenha terminado com sucesso. O trabalho registra o hash do catálogo de regras da análise (ou a versão do analisador para execuções anteriores à migração).

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

Notion: *Anotações / DB Auditor* (EF, Backlog, Fluxo de Desenvolvimento).
