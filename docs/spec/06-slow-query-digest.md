# 06 — Digest semanal de regressões de consultas (`slow-query-digest`)

## 0. Contexto e problema, objetivo, fora de escopo

**Problema.** pgBadger depende de logs e PgHero mostra o estado atual. Nenhum responde "o que piorou esta semana?". O db-auditor já guarda `workload_snapshot` por coleta (fingerprint sem literais, `query_id`, chamadas, tempo total e médio, blocos, `stats_reset`). Também já alerta regressão `workload` contra o baseline. Mas não calcula delta entre janelas nem entrega um resumo periódico.

**Objetivo.** Digest periódico (semanal por padrão) por ambiente: top regressões, consultas novas, consultas que sumiram e sugestões de índice já existentes, pronto para e-mail, webhook e UI.

**Fora de escopo.** Ler logs do servidor. Capturar planos reais (`EXPLAIN ANALYZE`). Criar índices. Consultas fora do top N coletado.

## 1. Requisitos funcionais

- **RF-01** Calcular, por fingerprint (com `query_id` quando estável), o delta de contadores entre a primeira e a última coleta completa de cada janela: chamadas, tempo total, tempo médio da janela (Δtempo ÷ Δchamadas), linhas e blocos lidos.
- **RF-02** Comparar a janela atual com a anterior (padrão 7 + 7 dias, configurável por ambiente) e classificar cada fingerprint como regressão, melhora, nova, sumiu ou estável.
- **RF-03** Invalidar a janela quando houver `stats_reset` (de `pg_stat_statements` ou do banco) entre as coletas usadas, inclusive a passagem de NULL para valor (regra AN-11). Nesse caso, tentar a maior subjanela contígua sem reset. Se não houver, o digest diz "sem janela comparável".
- **RF-04** Digest com, no máximo:
  - top 10 regressões;
  - top 10 consultas novas por tempo total;
  - top 5 que sumiram;
  - top 5 melhoras.

  Cada item traz o fingerprint resumido, os objetos referenciados, os números das duas janelas, a variação e o link para o achado correlato.
- **RF-05** Sugestões de índice: anexar `index.candidate` e `performance.workload_scan` já existentes que referenciem as mesmas tabelas. Com HypoPG disponível e permitido (pergunta em aberto), anexar a estimativa de custo hipotética, rotulada "hipotético".
- **RF-06** Agendamento: gerar o digest após a última coleta completa da janela, pelo agendador existente. Também sob demanda.
- **RF-07** Entrega: e-mail e webhook pelos canais de alerta existentes. Digest em Markdown (corpo) e na UI (histórico de digests por ambiente).
- **RF-08** Regressões do digest não criam um alerta novo se já houver alerta `workload` aberto para o mesmo fingerprint (dedup existente). O digest só referencia.

## 2. Requisitos não funcionais

- **RNF-01 Segurança e só leitura.** Nenhuma sessão nova no alvo: tudo vem do snapshot store. A eventual chamada a HypoPG é uma sessão só leitura com índices hipotéticos de sessão, sem efeito persistente, e só se o operator permitir.
- **RNF-02 LGPD.** Só fingerprint normalizado sem literais (normalização atual do coletor) e números agregados. Nunca o texto original de `pg_stat_statements`.
- **RNF-03 Desempenho.** Digest de um ambiente com 500 fingerprints por coleta × 42 coletas por janela em menos de 3 s. Corpo do e-mail ≤ 100 KB.
- **RNF-04 Multi-tenant.** Digest por ambiente. Destinatários configurados por ambiente pelo operator. Um digest nunca mistura ambientes.
- **RNF-05 Observabilidade.** Métricas de digests gerados, sem janela comparável e entregas falhas. Falha de entrega é nova: hoje `internal/notify` ignora o erro e `alert_delivery` só registra entregas, para dedup.
- **RNF-06 Cobertura honesta.** Como o coletor guarda só o top N por tempo total (`AUDITOR_WORKLOAD_LIMIT`, padrão 500), o digest informa N. Uma consulta "nova" pode só ter entrado no top N. Isso aparece como ressalva.

## 3. Regras importantes

| Classificação | Condição | Severidade no digest |
|---|---|---|
| Regressão | tempo médio +50% **e** +10 ms absolutos **e** ≥ 100 chamadas em cada janela | média; alta se tempo total da janela +100% e ≥ 5% do tempo total do banco |
| Nova pesada | ausente na janela anterior e ≥ 5% do tempo total da janela atual | média |
| Sumiu | presente na anterior com ≥ 1% do tempo total, ausente na atual | informativa |
| Melhora | tempo médio −30% com o mesmo critério de chamadas | informativa |
| Janela inválida | reset, coleta parcial ou lacuna maior que `AUDITOR_REGRESSION_MAX_GAP` | — (digest diz o motivo) |

Todos os limiares são **valor inicial, calibrar**, por ambiente. A calibração segue o procedimento BA-15 (#104).

## 4. Respostas esperadas

- **Entradas.** Ambiente, tamanho da janela, data de referência, destinatários.
- **UI.** Em Performance, nova aba **Digest**, com a lista de digests e o detalhe nas seções do RF-04 e as ressalvas de cobertura.
- **API.** Listar e ler digests (viewer). Gerar sob demanda e configurar janela e destinatários (operator).
- **Markdown.** Cabeçalho com ambiente, janelas, coletas usadas e N do top. Seções com tabelas. Rodapé com ressalvas.
- **Webhook.** Evento resumido (contagens por classificação e link), sem a lista completa.
- **Erros.**
  - menos de duas coletas completas em alguma janela: "sem janela comparável";
  - `pg_stat_statements` ausente no alvo: "não aplicável — extensão ausente" (capacidade existente);
  - falha de entrega: registrada no histórico do digest, com nova tentativa no próximo ciclo (novo).

## 5. Critérios de aceite

- **CA-01** Dado um fingerprint com 1000 chamadas e 20 ms médios na janela anterior e 1200 chamadas e 35 ms na atual, quando o digest é gerado, então ele aparece como regressão média com +75%.
- **CA-02** Dado um `pg_stat_statements_reset` dentro da janela atual, quando o digest é gerado, então usa a maior subjanela contígua sem reset e a indica no cabeçalho. Se não houver, diz "sem janela comparável".
- **CA-03** Dado uma coleta `partial_success` na janela, quando o delta é calculado, então essa coleta não é usada como extremo da janela.
- **CA-04** Dado um alerta `workload` aberto para o mesmo fingerprint, quando o digest o lista, então nenhum alerta novo é criado e o item linka o alerta existente.
- **CA-05** Dado o Markdown e o e-mail gerados, quando inspecionados, então nenhum literal da consulta original aparece.
- **CA-06** Dado um ambiente MongoDB, quando o digest é pedido, então responde "não aplicável a este mecanismo".
- **CA-07** Dado um viewer, quando ele tenta mudar os destinatários, então recebe 403.

## 6. Dependências, reaproveitamento, riscos e perguntas em aberto

**Reaproveita.**
- `workload_snapshot` e o coletor `stats_workload`, com normalização sem literais e `referenced_objects`;
- regras `performance.slow_query`, `performance.workload_*` e `index.candidate`;
- categoria `workload` da regressão, dedup e `stats_reset` (AN-11, #104, #128);
- `internal/scheduler` e o padrão de relatório agendado (`scheduled_report`, PDF executivo por coleta diária ou semanal);
- `internal/notify` (webhook e SMTP) e `alert_delivery` (dedup de entregas).

**Novo.**
- cálculo de delta por janela;
- classificação regressão, nova, sumiu, melhora;
- entidade digest com histórico;
- aba Digest;
- destinatários por ambiente (hoje os canais de alerta são globais, por variável de ambiente);
- integração opcional com HypoPG.

**Dependências.** Nenhuma. É base da 02 (custo por consulta).

**Riscos.**
- Top N esconde consultas medianas que pioraram.
- Fingerprint muda com uma refatoração do SQL e vira "sumiu + nova".
- Cadência de coleta baixa gera janelas pobres.

**Perguntas em aberto.**
- P-01: permitir HypoPG (índice hipotético na sessão) como opt-in, ou ficar só nas sugestões `index.candidate` existentes?
- P-02: destinatários por ambiente (novo) ou os canais globais atuais?
