# Hardening notes (Sprint 12)

## Backend

- Collectors e analyzers: **read-only**; evidências com `safety_note` onde aplicável
- Partial failures: AuditRunner já modela SUCCESS / PARTIAL_SUCCESS / FAILED
- Timeouts: `AUDITOR_STATEMENT_TIMEOUT`, `AUDITOR_LOCK_TIMEOUT`, context cancel em collectors
- Migrations SQL versionadas em `backend/migrations`
- Compatibilidade Timescale: registry em collectors/timescale

## Frontend

- Sem acesso direto a bancos
- Listas grandes: paginação API (Inventory Explorer)
- Estados loading/error/empty nas páginas de operação e dashboard
- Navegação por teclado nos botões da shell; labels em filtros

## Integração

- Fluxos críticos cobertos por testes Go/Vitest e smoke `make smoke`
- Relatórios JSON em `/api/v1/reports/*` para exportação posterior CSV se necessário
