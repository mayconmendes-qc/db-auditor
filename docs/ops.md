# Operação em VPS — DB Auditor

## Schema do snapshot store

O schema canônico está em `backend/migrations/01_baseline.sql`. O seed local é `02_seed_demo.sql`. A API aplica o baseline só quando o banco está vazio e registra `schema_migration`. Um volume que já tem o schema atual recebe só o que falta (hoje `03_audit_schedule.sql`) e não reaplica o baseline. O segundo boot não executa SQL de novo. Se o checksum de um arquivo já aplicado mudar, a API recusa subir.

## Produção

Copie [deploy/env.prod.example](../deploy/env.prod.example) para `.env.prod` na raiz do repositório. Preencha a senha do snapshot store, a senha inicial do operador e os dois slots `AUDITOR_TARGET_1_*` e `AUDITOR_TARGET_2_*`.

```bash
podman compose -f deploy/compose.prod.yaml --env-file .env.prod up -d
```

O compose falha se faltarem `POSTGRES_PASSWORD`, `AUDITOR_BOOTSTRAP_USER`, `AUDITOR_BOOTSTRAP_PASSWORD`, `AUDITOR_CORS_ORIGINS` ou `AUDITOR_TARGET_ALLOWED_HOSTS`. Os DSNs não entram no YAML. O Postgres interno não publica porta. `GET /metrics` não passa pelo Caddy; o scrape fica em `api:9090`, dentro da rede do compose.
O Postgres do Compose aplica scripts deste diretório **somente na primeira inicialização** do volume.

Após alterar o baseline em desenvolvimento:

```bash
make reset-volume   # destrutivo
make up
```

**Nome do banco interno:** default `db_auditor` (`.env` / `POSTGRES_DB`). Se o volume foi criado com o nome legado `timescale_auditor`, alinhe o `.env` ou recrie o volume.

Detalhes: `backend/migrations/README.md`.

## Observabilidade (Sprint 10)

| Endpoint | Uso |
|----------|-----|
| `GET /health` | Liveness |
| `GET /ready` | Readiness (snapshot store) |
| `GET /metrics` | Prometheus, só na porta interna 9090 |
| `GET /api/v1/status` | Visão operacional (UI Status) |

Logs da API são **JSON estruturados** por padrão (`AUDITOR_LOG_FORMAT=json`).  
Use `AUDITOR_LOG_FORMAT=text` e `AUDITOR_LOG_LEVEL=debug` em desenvolvimento.

Cada resposta HTTP inclui `X-Request-ID` (ou propaga o valor enviado pelo cliente) para correlacionar logs e traces manuais.

Erros de conexão a targets **não** incluem senha (ver `config.SanitizeDSN`). Warnings parciais multi-database aparecem no `collector_run.warning`.

### Métricas principais

- `auditor_up`
- `auditor_http_requests_total{method,path,code}`
- `auditor_http_request_duration_seconds_{sum,count}`
- `auditor_audit_runs_total` / `auditor_audit_runs_failed_total`

Paths com UUIDs são normalizados para `:id` para manter cardinalidade baixa.

OpenTelemetry e Sentry são opcionais (P1): configure DSN/endpoints via env quando forem adotados — o núcleo atual não exige dependências externas de APM.

## Deploy com Podman Compose

1. Crie `.env.prod` (fora do git) com `POSTGRES_PASSWORD`, `AUDITOR_DOMAIN`, etc.
2. Build e subida:

```bash
podman compose -f deploy/compose.prod.yaml --env-file .env.prod up -d --build
```

3. Caddy termina TLS (quando o domínio aponta para a VPS) e encaminha:
   - `/api/*`, `/health`, `/ready` → API
   - `/metrics` não é publicado; scrape em `api:9090`
   - resto → frontend estático

### Volumes e secrets

- Volume `snapshot-store`: dados do PostgreSQL interno (backup/restore = dump deste volume ou `pg_dump`).
- Senhas apenas em env files locais ou secret managers; nunca no repositório.
- Rede `auditor` é privada; Postgres **não** publica porta no compose de produção.
- Bancos **auditados** (Tiger Cloud / self-hosted): connection strings só no env da API, credenciais **read-only**.

### Health e restart

- `restart: unless-stopped` em todos os serviços long-running.
- Healthchecks de Postgres e API controlam dependências de startup.

## Checklist rápido de homologação

1. `GET /health` → `ok`
2. `GET /ready` → `ready`
3. `GET` na porta 9090 (`/metrics`) contém `auditor_up 1`. O mesmo caminho em 8080 responde 404.
4. UI **Status** lista API, store e runs
5. Logs JSON incluem `request_id` e `duration_ms`
6. UI **Documentação** descreve o fluxo de configuração via `.env`
