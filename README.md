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

Defina `AUDITOR_REPORT_API_TOKEN` com pelo menos 32 caracteres aleatórios no `.env` do servidor (por exemplo, gerado com `openssl rand -hex 32`). Sem o token, as rotas de relatório e a aprovação de baseline ficam desativadas. Informe o mesmo valor na página **Relatórios** ou no campo de aprovação de baseline; a interface mantém o valor apenas na memória da aba, sem persisti-lo no navegador. Use HTTPS em produção.

Os pedidos são assíncronos e idempotentes por execução, versão de regras, tipo e filtros. Os PDFs ficam no snapshot store por 30 dias; os metadados do histórico são preservados por mais 90 dias. Uma execução parcial aparece com cobertura limitada e não é usada para inferir resolução de findings. A geração é limitada a 500 bancos, 5.000 tabelas, 2.000 findings e 16 MiB por PDF, com truncamento declarado no documento.

Em uma instalação com volume PostgreSQL existente, aplique as migrações `08_sprint18_history.sql` e `09_sprint19_reports.sql` nesta ordem antes de iniciar a API atualizada. Não reinicialize o volume de produção; veja `backend/migrations/README.md`.

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
