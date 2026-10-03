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
- `10_sprint20_identity.sql`: contas locais, sessões, trilha de operações e índices de paginação/inventário.

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

Para a Sprint 20, veja [o procedimento de migração, bootstrap e retenção](../../docs/sprint20-operations.md).

## Ambientes: não criar outro `02_seed_*`

`02_seed_demo.sql` não é a fonte dos dois Timescale da empresa. Ele só insere,
em volume vazio, duas linhas de demonstração em `audit_environment`:

| id | nome | papel |
| --- | --- | --- |
| `00000000-0000-0000-0000-000000000001` | Demo Tiger Cloud | rótulo local |
| `00000000-0000-0000-0000-000000000002` | Demo Datacenter | rótulo local |

Host, porta, database, usuário e senha **não** estão nessa migration. Já vêm de
`AUDITOR_TARGET_N_*` no `.env` (ou do fallback `AUDITOR_TARGET_DSN_<uuid>`).
Os coletores conectam com esse DSN e leem catálogo, hypertables, chunks e jobs
do servidor. Isso já é dinâmico e não precisa de seed.

O que o seed ainda faz, e o `.env` sozinho não faz, é criar a linha de catálogo.
Runs, snapshots e findings referenciam `audit_environment.id`. O
`AUDITOR_TARGET_N_ENVIRONMENT_ID` tem de ser o mesmo UUID. No laboratório, o
contrato é manual com os dois IDs acima. Em volume já existente o entrypoint
não reaplica `02_seed_demo.sql`.

Não adicionar `02_seed_empresa.sql` (nem editar o seed demo) com os ambientes reais:

- migration já aplicada não roda de novo; um volume de produção não vê o arquivo novo se ele for encaixado no meio da sequência já executada pelo entrypoint;
- nome, tipo e modo de descoberta mudam sem ser mudança de schema;
- senha de banco não pode ir para SQL versionado.

O passo seguinte, se o catálogo tiver de acompanhar o `.env` sem SQL manual, é a API fazer upsert na subida a partir de variáveis (id, nome, tipo, `discovery_mode`), sem gravar DSN. Até lá, os dois servidores continuam nos slots do `.env`, apontando para linhas que já existem em `audit_environment`.
