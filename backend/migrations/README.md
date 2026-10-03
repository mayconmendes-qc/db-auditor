# Snapshot store schema

O diretório tem dois scripts. O Postgres do Compose executa os `.sql` **somente na primeira inicialização** de um volume vazio, em ordem alfabética.

- `01_baseline.sql`: schema completo. As migrations das sprints 13–20 foram incorporadas aqui.
- `02_seed_demo.sql`: duas linhas locais de demonstração. A API não aplica este arquivo.
- `03_audit_schedule.sql`: agenda persistente. A API aplica no boot se ainda não estiver registrada.

Não reintroduza arquivos `.down.sql`: o entrypoint executaria todos os `.sql`.

Depois de alterar o baseline em desenvolvimento:

```bash
make reset-volume   # destrutivo: apaga o volume snapshot-store
make up
```

Um volume que já existe **não** reaplica este diretório. A API cria as tabelas de login (`auditor_user`, sessão e log) se elas ainda não existirem, para o primeiro acesso funcionar sem migration extra. O restante do schema antigo permanece como está. Se o volume for anterior às sprints 13–19 e a API passar a falhar por coluna ausente, recrie o volume em desenvolvimento com `make reset-volume`.

## Ambientes

`02_seed_demo.sql` não descreve os Timescale da empresa. Ele só nomeia, em volume vazio:

| id | nome |
| --- | --- |
| `00000000-0000-0000-0000-000000000001` | Demo Tiger Cloud |
| `00000000-0000-0000-0000-000000000002` | Demo Datacenter |

Host, porta, database, usuário e senha ficam em `AUDITOR_TARGET_N_*` no `.env`. O `ENVIRONMENT_ID` do slot tem de ser um `id` que já exista em `audit_environment`. Não crie outra migration de seed para os servidores reais: ela não roda de novo num volume existente e não deve carregar senha.
