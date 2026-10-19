# 08 — Prova de restauração do backup do banco auditado (`backup-verifier`)

## 0. Contexto e problema, objetivo, fora de escopo

**Problema.** "Backup não testado não é backup." pgBackRest e WAL-G fazem o backup, mas não provam que os dados restaurados batem com a origem nem medem o RTO real.

**O que já existe e NÃO cobre isso.** O #106 (BA-03/10/16) entregou backup verificado e restauração de teste do **snapshot store do próprio auditor**:
- `deploy/snapshot-backup.sh` e `deploy/snapshot-restore-verify.sh`: sha256, validação da lista do arquivo, restauração em destino isolado com confirmação explícita, RTO e comparação de contagens;
- evidência `restore_drill` e `backup_verified` em `POST /api/v1/retention-policy/evidence`, que alimenta os bloqueios da política de retenção.

Nada disso olha os backups dos **bancos auditados**. A regra `replication.archive_stalled` só sinaliza arquivamento de WAL parado.

**Objetivo.** Um modo CLI do mesmo binário restaura um backup do banco auditado num **container efêmero e isolado**, roda verificações (inventário, contagens, checksums de amostra, integridade física), mede o RTO e envia a evidência ao auditor, que mostra o histórico e gera achados.

**Fora de escopo.** Fazer o backup. Restaurar em produção ou em qualquer destino não isolado. Recuperação point-in-time no MVP. Gerenciar a retenção dos backups. MongoDB.

## 1. Requisitos funcionais

- **RF-01** Subcomando CLI que recebe:
  - local do backup (arquivo `pg_dump` em formato custom ou diretório no MVP);
  - checksum esperado (opcional);
  - ambiente de origem;
  - runtime de container (Docker ou Podman);
  - imagem PostgreSQL da mesma major ou superior à origem.
- **RF-02** Pré-validação: checksum (quando informado), legibilidade do arquivo e lista de objetos, antes de restaurar. Falha → evidência `succeeded=false` e nada restaurado (mesmo comportamento do #106).
- **RF-03** Isolamento: recusar sem confirmação explícita de destino isolado, e recusar se o destino coincidir com qualquer DSN de ambiente configurado (padrão do `snapshot-restore-verify`). O container fica sem rede de saída e é destruído ao final, com sucesso ou falha.
- **RF-04** Medir o RTO: do início da restauração até o banco aceitar conexões com todos os objetos restaurados. Registrar também o tempo da pré-validação e das verificações.
- **RF-05** Verificações na cópia restaurada:
  - (a) inventário: executar os coletores existentes contra a cópia e comparar com a última coleta completa da origem (diff da 04 ou `internal/comparison`);
  - (b) contagens exatas por tabela na cópia, contra as linhas estimadas da origem, com tolerância;
  - (c) checksums de amostra: hash de uma amostra determinística de linhas por tabela, calculado nos dois lados;
  - (d) integridade física de índices B-tree e heap com a extensão de verificação do PostgreSQL, **na cópia**.
- **RF-06** Evidência estruturada: `kind=target_restore_drill`, ambiente, sha256 do artefato, RTO, resultado por verificação, divergências (tabela, tipo, contagens) e versão da CLI. Enviada à API do auditor com credencial de escopo do ambiente.
- **RF-07** Na UI, em Ambientes ou Status, área **Restauração verificada**: histórico de ensaios, último sucesso, RTO e tendência, divergências.
- **RF-08** Achados: `backup.restore_failed`, `backup.restore_divergence`, `backup.rto_exceeded` e `backup.drill_overdue` (sem ensaio bem-sucedido no prazo).
- **RF-09** RTO-alvo por ambiente (operator). Prazo máximo entre ensaios por ambiente.

## 2. Requisitos não funcionais

- **RNF-01 Segurança e só leitura.**
  - A origem só é lida pela conexão só leitura (para os checksums de amostra).
  - Toda escrita acontece no container efêmero.
  - **A API do auditor nunca recebe acesso ao socket de container.** O runtime só existe no host onde a CLI roda (runner de CI ou máquina de operação).
  - Credenciais da imagem restaurada são geradas por execução e descartadas.
- **RNF-02 LGPD.**
  - Os dados restaurados nunca saem do container.
  - A evidência tem só contagens, hashes agregados por tabela e nomes de objetos.
  - O hash de amostra é por tabela (agregado), nunca por linha.
  - Volumes do container são removidos ao final.
  - O artefato de backup pode estar cifrado (`age`, padrão do BA-16). A decifração acontece na memória ou no volume efêmero.
- **RNF-03 Desempenho e limites.**
  - A amostra de checksum vai de 1000 linhas por tabela até as 200 maiores tabelas (**valor inicial, calibrar**).
  - Teto de execução configurável, padrão 2 h.
  - Disco mínimo exigido = 2 × tamanho da origem, verificado antes de restaurar.
  - Na origem, as leituras de amostra seguem os timeouts da coleta.
- **RNF-04 Multi-tenant.** Evidência por ambiente. A credencial da CLI só envia para o seu ambiente. Viewer lê o histórico, operator configura RTO-alvo e prazo.
- **RNF-05 Observabilidade.** Saída de progresso na CLI. Métricas na API de último ensaio por ambiente (idade, mesmo padrão de `auditor_snapshot_backup_age_seconds`) e de RTO.
- **RNF-06 Consistência temporal.** A origem muda depois do backup. Por isso as contagens comparam com tolerância, e o checksum de amostra só é exigido para tabelas sem escrita entre o momento do backup e o da verificação (pelos contadores de escrita das coletas). As outras ficam como "não comparável".

## 3. Regras importantes

| Regra | Condição | Severidade |
|---|---|---|
| `backup.restore_failed` | pré-validação, restauração ou verificação física falhou | crítica |
| `backup.restore_divergence` (integridade) | verificação física aponta corrupção na cópia | crítica |
| `backup.restore_divergence` (checksum) | hash de amostra diferente em tabela comparável | alta |
| `backup.restore_divergence` (contagem) | diferença > 0,5% e > 1000 linhas em tabela comparável | média (**valor inicial, calibrar**) |
| `backup.restore_divergence` (inventário) | tabela, índice ou função da origem ausente na cópia | alta |
| `backup.rto_exceeded` | RTO medido > RTO-alvo do ambiente | alta |
| `backup.drill_overdue` | sem ensaio bem-sucedido há > 30 dias | média (**valor inicial, calibrar**; mesmo prazo de 30 dias da restauração de teste da política BA-03) |

## 4. Respostas esperadas

- **Entradas (CLI).** Backup, checksum, ambiente, runtime, imagem, confirmação de isolamento, envio à API (sim ou não).
- **Saída CLI.** Resumo legível ("restaurou em X min, N tabelas conferidas, K divergências"), mais JSON da evidência. Códigos do README, com 4 para falta de confirmação de isolamento ou destino igual a um ambiente.
- **API.** Receber evidência (credencial do ambiente). Listar ensaios e o detalhe (viewer). Configurar RTO-alvo e prazo (operator).
- **UI.** Histórico com estado, RTO versus alvo, divergências por tabela e link para os achados.
- **PDF.** Seção "Restauração verificada" no relatório técnico e executivo (último ensaio, RTO, divergências).
- **Erros.**
  - runtime de container ausente: código 2;
  - disco insuficiente: código 2, sem iniciar;
  - major da imagem menor que a da origem: código 2;
  - origem inacessível para os checksums: verificação (c) "não executada" e código 3.

## 5. Critérios de aceite

- **CA-01** Dado um dump válido e confirmação de isolamento, quando a CLI roda, então restaura no container, mede o RTO, executa as quatro verificações, destrói o container e sai com 0 se não houver divergência.
- **CA-02** Dado um dump com 1 byte alterado e checksum informado, quando a CLI roda, então a evidência tem `succeeded=false`, nada é restaurado e o código é 1 (achado crítico).
- **CA-03** Dado a ausência da confirmação de isolamento, quando a CLI roda, então recusa com código 4 sem criar container.
- **CA-04** Dado um destino que coincide com o DSN de um ambiente configurado, quando a CLI roda, então recusa com código 4.
- **CA-05** Dado uma tabela sem escrita desde o backup cuja amostra difere entre origem e cópia, quando verificada, então surge `backup.restore_divergence` alta, com a tabela e as contagens, sem nenhum valor de linha.
- **CA-06** Dado um RTO-alvo de 10 min e um RTO medido de 14 min, quando a evidência chega, então surge `backup.rto_exceeded` alta.
- **CA-07** Dado 31 dias sem ensaio bem-sucedido, quando a análise do ambiente roda, então surge `backup.drill_overdue` média.
- **CA-08** Dado uma falha no meio da restauração, quando a CLI termina, então nenhum container ou volume da execução permanece no host.

## 6. Dependências, reaproveitamento, riscos e perguntas em aberto

**Reaproveita.**
- O procedimento e o formato de evidência do #106 (`deploy/snapshot-restore-verify.sh`, testes em `internal/ops`): sha256, validação da lista do arquivo, confirmação de isolamento, recusa de destino igual à origem, RTO, `succeeded=false` em qualquer falha.
- O padrão de evidência registrada via API (`retention_evidence`).
- Os coletores existentes, executados contra a cópia, e `internal/comparison` (ou o diff da 04).
- `table_snapshot` (linhas estimadas, contadores de escrita).
- A conexão só leitura, a métrica de idade (padrão de `snapshot_backup`), o PDF e `replication.archive_stalled` como sinal complementar.

**Novo.**
- o subcomando CLI e a orquestração de container;
- os checksums de amostra;
- a verificação física;
- a evidência `target_restore_drill` por ambiente (separada da evidência do snapshot store, que continua alimentando só a retenção);
- RTO-alvo, prazo e as regras `backup.*`.

**Dependências.** 01 (modo CLI e credencial de CI). A 04 é opcional, para o diff de inventário.

**Riscos.**
- O runner precisa de acesso ao backup de produção, que é um ativo sensível.
- Restaurações grandes custam tempo e disco.
- Comparar uma origem que muda gera falsos positivos. Mitigação: tabelas "comparáveis" pelo contador de escrita.

**Perguntas em aberto.**
- P-01: quem executa o ensaio — só a CLI em runner externo (recomendado), ou também um worker gerenciado pelo auditor com acesso a container?
- P-02: formatos no MVP — só `pg_dump`, ou já pgBackRest e WAL-G (restauração física, que exige outra imagem e outro fluxo)?
- P-03: os checksums de amostra leem linhas da produção (só hash agregado sai da memória). Isso é aceitável, ou ficamos só com contagens e inventário?
