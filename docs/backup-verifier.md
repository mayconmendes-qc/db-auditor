# Backup verifier — prova de restauração do banco auditado

Runbook do plano 08 (`auditor restore-verify`). Restaura um `pg_dump` do **banco
auditado** num container efêmero sem rede de saída, mede o RTO, confere inventário,
contagens e integridade física (amcheck) e, opcionalmente, hash de amostra.
A evidência `target_restore_drill` vai para o ambiente no auditor (escopo
`restore-evidence:write`).

Isto é distinto do ensaio do **snapshot store** (`deploy/snapshot-restore-verify.sh`
→ `retention_evidence`), que alimenta só a política de retenção (BA-03/10/16).

## Pré-requisitos

- Runner com Docker ou Podman (a API **nunca** recebe o socket de container).
- Imagem PostgreSQL com major ≥ origem (`--image postgres:16`).
- Disco livre ≥ 2× o tamanho do artefato.
- Token de ambiente com `restore-evidence:write` em `AUDITOR_CLI_TOKEN` / `_FILE`.
- `AUDITOR_API_URL` apontando para a API.

## Uso básico (CA-01)

```bash
export AUDITOR_API_URL=https://auditor.exemplo
export AUDITOR_CLI_TOKEN=dba_...
auditor restore-verify \
  --backup /secure/app.dump \
  --sha256 "$(cat /secure/app.dump.sha256)" \
  --environment "$ENV_UUID" \
  --runtime podman \
  --image postgres:16 \
  --confirm-isolated-target \
  --submit \
  --format md
```

Sem `--sample-hash`, a verificação (c) fica **desabilitada** e isso **não** conta
como execução parcial (ADR-0001 D8).

## Hash de amostra (opt-in, CA-09)

Somente com `--sample-hash` **e** DSN só leitura da origem no runner:

```bash
export AUDITOR_RESTORE_VERIFY_SOURCE_DSN_FILE=/run/secrets/ro-dsn
auditor restore-verify ... --sample-hash --confirm-isolated-target --submit
```

- Conta com escrita → código 4.
- Sem DSN → código 2 **antes** de restaurar.
- Origem inacessível → (c) "não executada", código 3.
- O DSN nunca vai em argumento, log, evidência nem `ps`.

## Isolamento e fingerprints

- Sem `--confirm-isolated-target` → código 4, nenhum container.
- Destino efêmero `127.0.0.1:5432/restore` é comparado aos fingerprints
  `sha256(host:porta/banco)` devolvidos por `GET .../restore-drill-context`.
  Nunca DSNs (ADR-0001 D8).

## Códigos de saída

| Código | Significado |
|---|---|
| 0 | sucesso sem divergência relevante |
| 1 | bloqueio (checksum, restauração, divergência crítica) |
| 2 | uso / runtime / disco / major / DSN ausente |
| 3 | contexto insuficiente ou (c) não executada |
| 4 | segurança (isolamento, fingerprint, conta com escrita) |

## Política e UI

Operator configura RTO-alvo e prazo máximo entre ensaios em Ambientes →
**Restauração verificada**. Achados: `backup.restore_failed`,
`backup.restore_divergence`, `backup.rto_exceeded`, `backup.drill_overdue`.
