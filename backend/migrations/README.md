# Snapshot store schema

## Baseline e migrations incrementais

O schema canônico está em **`01_baseline.sql`**.

As migrations posteriores são incrementais e append-only:

- `02_seed_demo.sql`: dados locais de demonstração.
- `03_sprint13_pipeline.sql`: cobertura por audit run e lifecycle da análise automática.

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
