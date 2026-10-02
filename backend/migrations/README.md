# Snapshot store schema

## Baseline e migrations incrementais

O schema canônico está em **`01_baseline.sql`**.

As migrations posteriores são incrementais e append-only:

- `02_seed_demo.sql`: dados locais de demonstração.
- `03_sprint13_pipeline.sql`: cobertura por audit run e lifecycle da análise automática.
- `04_sprint14_structural.sql`: inventário estrutural de relações, constraints, sequences, triggers e RLS.
- `05_sprint15_history_stats_workload.sql`: índices temporais, estatísticas agregadas de colunas e workload sanitizado.
- `06_sprint16_rules.sql`: versão de regras, políticas por escopo e metadados explicativos dos findings; também completa metadados da Sprint 15.
- `07_sprint17_assessment.sql`: comentários de coluna, grants efetivos, dependências e índices do assessment; separa policies RLS das policies TimescaleDB, que já usam `policy_snapshot`.
- `08_sprint18_history.sql`: baseline auditável, comparação temporal, eventos imutáveis de findings e supressão com validade.
- `09_sprint19_reports.sql`: fila persistente de relatórios e artefatos PDF com retenção.

Nunca altere uma migration que já possa ter sido aplicada.

O serviço Postgres do Compose monta este diretório em `/docker-entrypoint-initdb.d`.
Scripts `.sql` rodam **apenas na primeira inicialização** de um volume vazio.

### Desenvolvimento

Após alterar o baseline:

```bash
make reset-volume   # destrutivo: apaga o volume snapshot-store
make up
```

### Histórico de sprints

As migrations numeradas `000001`–`000008` das sprints 0–6 foram consolidadas neste baseline.
Não reintroduza arquivos `.down.sql` neste diretório: o entrypoint do Postgres executaria todos os `.sql` em ordem alfabética.

### Volumes existentes e produção

O entrypoint só executa scripts em volumes vazios. Em um volume existente, aplique somente a nova migration incremental por um processo controlado antes de subir a nova API. Nunca reaplique o baseline nem edite `01_baseline.sql` em uma instalação já provisionada.
