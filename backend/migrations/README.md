# Snapshot store schema

O diretório contém o baseline, o seed local e migrações incrementais. O Postgres do Compose executa os `.sql` **somente na primeira inicialização** de um volume vazio, em ordem alfabética; a API registra e aplica as migrações incrementais pendentes em volumes existentes.

- `01_baseline.sql`: schema completo. As migrations das sprints 13–20 foram incorporadas aqui.
- A API não aplica arquivo de seed. Depois das migrations, ela cria os ambientes de `AUDITOR_TARGET_N_ENVIRONMENT_ID` e `AUDITOR_TARGET_N_NAME`. Se o id já existe, só o nome é atualizado. Sem nenhum id, uma instalação sem essa configuração recebe dois ambientes de demonstração.
- `03_audit_schedule.sql`: agenda persistente. A API aplica no boot se ainda não estiver registrada.
- `04_p1.sql`: primeira ampliação incremental.
- `05_p2.sql`: melhorias da primeira etapa.
- `06_second_stage.sql`: planos de ação, eventos, anotações e regressões da segunda etapa.
- `07_third_stage.sql`: mecanismo do ambiente, diagnósticos agregados, decisões de saneamento e medições antes/depois. A constraint `audit_environment_engine_format` é criada de forma idempotente (reaplicável após apply parcial).
- `08_action_workload.sql`: registra a confirmação explícita de carga semelhante nas medições antes/depois; medições antigas permanecem como não confirmadas.
- `09_query_action_metrics.sql`: permite registrar latência média e leituras por mil chamadas de uma consulta observada, preservando as medições anteriores.
- `10_account_activity.sql`: guarda apenas validade e presença de sessão no instante da coleta para sugerir revisão de contas possivelmente inativas. Não guarda senhas nem comprova ausência de uso entre amostras.
- `11_quality_validation.sql`: vincula a decisão de validação de qualidade ao diagnóstico posterior comparável, preservando o vínculo também no evento.
- `12_view_detail.sql`: adiciona colunas, opções de segurança e estado de população às visões. Snapshots antigos ficam com valor ausente; uma nova coleta preenche os campos. A migração não remove dados.
- `13_index_detail.sql`: registra colunas incluídas e se as estatísticas de uso do índice estavam disponíveis. Em snapshots anteriores, esses campos ficam ausentes até uma nova coleta; os índices históricos permanecem preservados.
- `14_function_detail.sql`: registra retorno, configuração segura do caminho de busca, papéis com EXECUTE observado e estatísticas agregadas das funções. Execuções antigas mantêm esses campos como não coletados; o corpo SQL permanece protegido na API.
- `15_cagg_source.sql`: acrescenta origem e intervalo de bucket do agregado contínuo quando o catálogo Timescale os fornece. Snapshots antigos mantêm esses campos ausentes; uma nova coleta pode preenchê-los.
- `16_identity.sql`: garante usuário, sessão e tentativas de login em volumes antigos, com ociosidade e prazo absoluto da sessão. `IF NOT EXISTS` não apaga sessões já gravadas.
- `17_report_retention.sql`: guarda o prazo de retenção do PDF. Um artefato vencido deixa de ser baixado; execuções, achados e o job permanecem.
- `18_totp_discard_retention.sql`: TOTP opcional, assinatura de descarte e a linha de política de retenção. Não apaga dados existentes.
- `19_architecture_report.sql`: permite artefato Markdown e registra decisões de domínio versionadas, congeladas no pedido do relatório. Atua somente no banco interno.
- `20_architecture_insertion.sql`: guarda a evidência de inserção colhida em auditoria manual, inclusive consultas puladas por orçamento ou permissão. Atua somente no banco interno.
- `21_identity_login_block.sql`: acrescenta `blocked_until` ao contador de tentativas de login em volumes antigos. Substitui a criação ad hoc das tabelas de login que a API fazia no boot (`EnsureIdentitySchema`); agora todo o schema de identidade vem de `16_identity.sql`, `18_totp_discard_retention.sql` e desta migração, registradas em `schema_migration`.
- `22_finding_queue_order_idx.sql`: índice `finding_queue_order_idx` na ordem da fila de achados (ambiente, severidade, confiança, `last_seen_at`, id). A primeira página da fila deixa de ordenar o ambiente inteiro. Criado com `CREATE INDEX CONCURRENTLY`, fora de transação; o arquivo tem um único comando.
- `23_finding_env_last_seen_idx.sql`: índice `finding_env_last_seen_idx` (ambiente, `last_seen_at`, id) para as listas filtradas por categoria ou tipo. Também `CONCURRENTLY`. Se um índice ficar inválido após falha, `DROP INDEX CONCURRENTLY` e reinicie a API; o `IF NOT EXISTS` não recria índice inválido.
- `24_report_limits_retention_governance.sql`: acrescenta `report_job.limit_note` (limite ou truncamento aplicado ao relatório), aprovação da política em `retention_policy` (`approved_by`, `approved_at`, `archive_note`) e a tabela `retention_evidence` (backup verificado e restauração de teste). Só adiciona; não altera nem remove linhas.
- `25_role_attributes_regression_workload.sql`: guarda `is_superuser`, `bypass_rls`, `replication`, `create_role` e `inherited_privileges` (poder herdado por membership, `privilégio:role de origem`) em `account_role_snapshot`, para `security.powerful_role` cobrir toda role com login, e permite a categoria `workload` em `regression_alert` (consultas lentas). Só adiciona; snapshots antigos ficam com os atributos em `false`.
- `26_query_digest.sql`: digest periódico de regressões de consultas (`query_digest`, itens, entregas, configuração por ambiente) e destinatários por ambiente (`notification_destination`). Só `CREATE … IF NOT EXISTS`.
- `27_schema_timeline.sql`: atributo `is_production` em `audit_environment`, eventos `schema_change_event` e lacunas `schema_timeline_gap` (spec 04).
- `28_migration_registry.sql`: configuração `environment_migration_registry` e snapshot `migration_registry_snapshot` para correlação com ferramentas de migration (ADR-0001 D5).
- `29_environment_api_token.sql`: tokens de API por ambiente (sha256, escopos fechados, validade obrigatória, revogação). Nunca guarda o segredo em claro (ADR-0001 D1).
- `30_migration_analysis.sql`: histórico de análises de migration por ambiente (cabeçalho + itens sem SQL/literais).
- `33_grant_origin_column_privilege.sql`: origem do privilégio efetivo (`grant_origin_snapshot`) e privilégios por coluna (`column_grant_snapshot`). Snapshots antigos ficam sem origem até a próxima coleta; `grant_snapshot` permanece inalterado.
- `34_access_review.sql`: campanhas de revisão de acessos, matriz congelada, responsáveis (conta do auditor + contato opcional) e decisões append-only; amplia `report_job.report_type` com `access_review`.
- `35_pii_scanner.sql`: mapa de dados pessoais LGPD (dicionário, settings, scans, resultados sem valores, revisão humana; ADR-0001 D4/D14/D11).
- `36_dq_contracts.sql`: contratos de qualidade versionados, execuções e resultados (só contagens; ADR-0001 D4/D13).
- `37_retention_scenarios.sql`: cenários salvos de retenção/compressão Timescale (spec 09, ADR-0001 D9/D11).
- `38_chunk_scan_counters.sql`: contadores de varredura/escrita por chunk em `chunk_snapshot` (RF-07; nulos = não coletado).
- `39_target_restore_drill.sql`: ensaios `target_restore_drill` e política RTO/prazo por ambiente (plano 08; distinto de `retention_evidence`).
- `40_account_last_login.sql`: agregado de último login por role (`account_last_login_observation`: ambiente, role, horário, fonte `log`, token; ADR-0001 D10 / plano 07 L6).

Não reintroduza arquivos `.down.sql`: o entrypoint executaria todos os `.sql`.

Depois de alterar o baseline em desenvolvimento:

```bash
make reset-volume   # destrutivo: apaga o volume snapshot-store
make up
```

Um volume que já existe **não** reaplica o baseline. A API executa as migrações incrementais pendentes, inclusive as de identidade (`16_identity.sql`, `18_totp_discard_retention.sql`, `21_identity_login_block.sql`); ela não cria tabelas fora de `schema_migration`. Faça backup antes de atualizar um volume persistente; confira o registro `schema_migration` e não altere arquivos já aplicados. Veja o [guia de operação](../../docs/ops.md) para backup e restauração.

Se o checksum de um arquivo **já aplicado** mudar (por correção de idempotência), a API recusa subir. Em desenvolvimento, atualize o registro após validar o schema:

```bash
sha256sum backend/migrations/07_third_stage.sql
# no snapshot store:
# UPDATE schema_migration SET checksum = '<hex>' WHERE version = '07_third_stage.sql';
```

Volumes em que a migração **falhou a meio** (objeto já existe, versão ausente em `schema_migration`) podem subir de novo após o pull: o SQL idempotente completa o que faltava e grava o checksum.

## Ambientes

Os ambientes iniciais não ficam numa migration. A API, logo depois de aplicar as migrations, cria uma linha para cada `AUDITOR_TARGET_N_ENVIRONMENT_ID` e usa `AUDITOR_TARGET_N_NAME` como nome na tela. Sem nenhum id, entram dois ambientes de demonstração:

| id | nome |
| --- | --- |
| `00000000-0000-0000-0000-000000000001` | Demo Tiger Cloud |
| `00000000-0000-0000-0000-000000000002` | Demo Datacenter |

Host, porta, database, usuário e senha ficam em `AUDITOR_TARGET_N_*` no `.env`. O nome da empresa na tela é `AUDITOR_COMPANY_NAME`. Um volume existente não perde ambientes antigos. A API atualiza o nome quando o id é o mesmo.

- `29_environment_api_token.sql`: tokens de API por ambiente (sha256, escopos fechados, validade obrigatória, revogação). Nunca guarda o segredo em claro (ADR-0001 D1).
- `30_migration_analysis.sql`: histórico de análises de migration por ambiente (cabeçalho + itens sem SQL/literais).
- `31_cost_price_table.sql`: tabela de preços versionada por ambiente (`cost_price_version`), moeda BRL/USD travada após a primeira versão (ADR-0001 D3).
- `32_cost_estimate_series.sql`: totais de custo estimado por coleta completa (`cost_estimate_snapshot`) para a série de 90 dias.
