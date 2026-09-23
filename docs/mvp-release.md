# MVP Release — DB Auditor (Sprint 12)

Versão alvo: **0.12.0-mvp**

## Quality gates (obrigatório)

```bash
make backend-fmt backend-check backend-staticcheck backend-lint backend-test
make frontend-check frontend-typecheck frontend-test
```

CI em `.github/workflows/ci.yml` cobre backend Go e frontend Bun.

## Checklist de release

- [ ] CI verde em `main`
- [ ] `make smoke` com compose local
- [ ] Secrets apenas em `.env` / secret manager (nunca no git)
- [ ] Ambientes auditados com credenciais **read-only**
- [ ] Allow/deny lists revisadas
- [ ] `docs/ops.md` e este documento revisados
- [ ] Backup do volume `snapshot-store` testado (pg_dump ou snapshot de volume)
- [ ] Rollback: tag anterior + `podman compose ... up -d` com imagem anterior

## Instalação resumida

1. `cp .env.example .env` e defina `POSTGRES_PASSWORD`
2. `make up` (dev) ou `podman compose -f deploy/compose.prod.yaml --env-file .env.prod up -d --build`
3. Frontend / API atrás do Caddy em produção

## Configuração de ambientes auditados

- Connection strings só no backend (secrets files / env)
- `AUDITOR_DATABASE_DENYLIST` / `AUDITOR_SCHEMA_DENYLIST`
- Coletores são somente leitura; analyzers nunca emit DROP/REVOKE/VACUUM automático

## Backup / restore (banco interno)

- Volume Compose: `snapshot-store`
- Backup: `pg_dump` via container tools ou cópia do volume parado
- Restore: recriar volume e aplicar dump; migrations em `backend/migrations`

## Hardening coberto neste MVP

- Middleware request_id + métricas
- Timeouts de statement/lock
- Paginação server-side no inventário
- Testes unitários de analyzers, API health/status/analytics
- Acessibilidade básica: labels, `role="alert"`, navegação por botões
- Feedback loading/error/empty nas páginas principais

## Pós-MVP (backlog)

Ver Notion: baseline contínuo, workflow de consolidação, IA explicativa, auth multi-usuário.
