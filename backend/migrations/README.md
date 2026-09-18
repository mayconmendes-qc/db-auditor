# Snapshot store schema

## Baseline único (pré-produção)

O schema canônico está em **`01_baseline.sql`**.

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

### Produção futura

Quando houver ambientes reais, adote uma ferramenta de migrate versionada (ex.: golang-migrate) e trate mudanças como migrations incrementais a partir deste baseline — nunca edite `01_baseline.sql` em produção já provisionada.
