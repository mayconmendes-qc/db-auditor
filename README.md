# Timescale Auditor

Serviço read-only de inventário, comparação e diagnóstico dos ambientes TimescaleDB.

## Ambiente local

1. Copie `.env.example` para `.env` e defina uma senha local.
2. Execute `podman compose up -d --build`.
3. Verifique `curl http://localhost:8080/health` e `curl http://localhost:8080/ready`.

O PostgreSQL interno permanece na rede privada do compose e não publica porta externa. Para encerrar, use `podman compose down`. O volume `snapshot-store` é persistente; remova-o explicitamente apenas quando quiser reiniciar os dados locais.

## Qualidade

- Backend: `make fmt`, `make check`, `make test`.
- Frontend: `cd web && npm run check && npm run test && npm run typecheck`.

Os ambientes auditados serão sempre acessados somente para leitura. Credenciais reais devem ficar fora do repositório.
