# 09 — Simulação de retenção e compressão no TimescaleDB (`timescale-retention-advisor`)

## 0. Contexto e problema, objetivo, fora de escopo

**Problema.** No TimescaleDB, retenção e compressão mal configuradas (ou ausentes) inflam o disco.

O db-auditor já coleta:
- hypertables;
- chunks, com intervalo, flag de comprimido, tamanho e bytes antes e depois da compressão;
- políticas (retenção, compressão, refresh, reorder, columnstore), jobs e CAGGs;
- tuplas mortas por chunk.

Também já gera `chunk.*`, `cagg.*` e as opções de retenção e compressão em `maintenance.growth_trend` (BA-21). Falta responder: "quanto eu economizo com retenção de 90 dias ou comprimindo após 7 dias?"

**Objetivo.** Simular, por hypertable, políticas de retenção e de compressão sobre o tamanho real dos chunks e a taxa de compressão observada. Mostrar GB e custo economizados, riscos (conflito com CAGG) e chunks possivelmente sem consulta.

**Fora de escopo.** Aplicar ou alterar políticas. Simular desempenho de consulta após compressão. TimescaleDB multinó (distribuído). PostgreSQL sem Timescale (vira "não aplicável").

## 1. Requisitos funcionais

- **RF-01** Mostrar, por hypertable, o estado atual: políticas vigentes, nº e tamanho de chunks por faixa de idade, % comprimido e taxa de compressão observada (antes ÷ depois, nos chunks já comprimidos).
- **RF-02** Simular a retenção de N dias: chunks cujo fim de intervalo é mais antigo que N, com os GB liberados.
- **RF-03** Simular a compressão após N dias: chunks não comprimidos mais antigos que N × taxa observada da própria hypertable. Sem chunk comprimido, usar a taxa padrão configurável, marcada "sem taxa observada, confiança baixa".
- **RF-04** Simular cenários combinados (retenção + compressão), com até 5 cenários lado a lado por hypertable. Projetar o tamanho em 30, 90 e 180 dias usando o crescimento entre coletas completas (regra de `maintenance.growth_trend`).
- **RF-05** Converter os GB em custo pela tabela de preços do ambiente (spec 02), quando houver.
- **RF-06** Detectar riscos da política simulada:
  - (a) retenção mais curta que a janela de refresh de um CAGG dependente;
  - (b) retenção sobre dados ainda dentro de um CAGG sem política de refresh;
  - (c) compressão de chunks que ainda recebem escrita (chunks recentes com tuplas mortas ou inserções).
- **RF-07** Chunks possivelmente sem consulta: por contadores de varredura por chunk entre duas coletas completas (proxy, novo), sempre como hipótese rotulada **aproximada**. Um reset de estatística invalida a janela (ADR-0001 D9).
- **RF-08** Achados: `timescale.retention_missing`, `timescale.compression_missing`, `timescale.retention_cagg_conflict` e `timescale.compression_ratio_low`.
- **RF-09** Seção no painel da hypertable (UI), no PDF técnico e no CSV dos cenários.

## 2. Requisitos não funcionais

- **RNF-01 Segurança e só leitura.** A simulação é cálculo sobre snapshots. Coletar contadores por chunk (RF-07) é leitura de catálogo e estatísticas pela conexão só leitura. Nenhuma política é criada.
- **RNF-02 LGPD.** Só tamanhos, contagens e intervalos de tempo dos chunks (metadado do catálogo). Nenhum dado das hypertables.
- **RNF-03 Desempenho e limites.** Simulação em menos de 1 s por hypertable com até 10 000 chunks. Respeitar o truncamento existente do inventário de chunks (`chunk.inventory_truncated`): com inventário truncado, a simulação fica marcada "parcial".
- **RNF-04 Multi-tenant.** Cenários salvos por ambiente. Viewer simula e lê. Salvar cenário exige auditor.
- **RNF-05 Observabilidade.** Métricas de simulações e de hypertables sem taxa observada.
- **RNF-06 Hipótese × ganho.** A UI e o PDF separam hipótese, ganho previsto e ganho medido (padrão BA-21). O ganho medido só aparece após a política ser aplicada fora do auditor e uma coleta completa comparável.

## 3. Regras importantes

| Regra | Condição | Severidade |
|---|---|---|
| `timescale.retention_missing` | hypertable ≥ 10 GB sem política de retenção e com chunks > 365 dias | baixa (retenção exige decisão de negócio e compliance) |
| `timescale.compression_missing` | hypertable ≥ 5 GB, compressão desabilitada ou sem política, chunks não comprimidos > 30 dias somando ≥ 50% do tamanho | média |
| `timescale.retention_cagg_conflict` | retenção vigente ou salva menor que a janela de refresh de um CAGG dependente | alta |
| `timescale.compression_ratio_low` | taxa observada < 2× em ≥ 5 chunks comprimidos | baixa |
| Taxa padrão sem observação | 5× | — (confiança baixa) |

Todos os limiares e a taxa padrão são **valor inicial, calibrar**. Nenhuma recomendação sugere exclusão automática. Retenção exige aprovação de negócio e backup restaurável (texto BA-21 existente).

## 4. Respostas esperadas

- **Entradas.** Ambiente, hypertable (ou todas), cenários (retenção N dias, compressão após N dias), horizonte de projeção.
- **UI.** No painel **Tabelas temporais**, nova seção **Simulação**: tabela de cenários com GB atuais, GB após o cenário, economia em GB e em moeda, riscos e confiança, mais um gráfico de projeção.
- **API.** Simular (viewer, sem gravar). Salvar e listar cenários (auditor). Ler chunks possivelmente sem consulta (viewer).
- **CSV.** Uma linha por hypertable × cenário. O arquivo termina com o marcador de exportação completa.
- **PDF.** Seção "Retenção e compressão (simulação)", com a ressalva de hipótese.
- **Erros.**
  - ambiente sem TimescaleDB: "não aplicável";
  - hypertable com dimensão inteira sem função de "agora": simulação de retenção por idade "não aplicável" (intervalo não temporal);
  - inventário truncado: resultado "parcial";
  - cenário inválido (N ≤ 0): 400.

## 5. Critérios de aceite

- **CA-01** Dado uma hypertable com 400 chunks diários de 1 GB, sem compressão, quando simulada uma retenção de 90 dias, então a economia mostrada é de cerca de 310 GB, com a lista de faixas de idade.
- **CA-02** Dado chunks comprimidos com antes = 10 GB e depois = 2 GB, quando simulada a compressão após 7 dias, então a taxa usada é 5× observada e a confiança não é baixa.
- **CA-03** Dado uma hypertable sem nenhum chunk comprimido, quando simulada a compressão, então a taxa padrão é usada e marcada "confiança baixa".
- **CA-04** Dado um CAGG com janela de refresh de 120 dias sobre uma hypertable, quando o cenário de retenção de 90 dias é salvo, então surge `timescale.retention_cagg_conflict` alta, criado no salvamento e vinculado à última coleta completa (ADR-0001 D9).
- **CA-05** Dado um ambiente PostgreSQL sem Timescale, quando a seção é aberta, então mostra "não aplicável" e nenhum coletor falha (regressão do #125).
- **CA-06** Dado preços configurados (02), quando a simulação roda, então a economia aparece também em moeda, com a vigência do preço.
- **CA-07** Dado um viewer, quando ele tenta salvar um cenário, então recebe 403.

## 6. Dependências, reaproveitamento, riscos e perguntas em aberto

**Reaproveita.**
- os coletores `collectors/timescale` (hypertable, dimension, chunk com `before_compression_bytes`, `after_compression_bytes`, `is_compressed` e intervalos; policy e policy_inventory; job; cagg);
- `chunk_snapshot`, `policy_snapshot`, `job_snapshot`, `continuous_aggregate_snapshot` e `chunk_vacuum_sample`;
- as regras `chunk.high_count`, `chunk.size_skew`, `chunk.inventory_truncated`, `cagg.missing_refresh_policy`, `cagg.refresh_window_exceeded` e `timescale.chunk_dead_tuples`;
- `maintenance.growth_trend` e a separação hipótese/ganho (BA-21);
- o painel de hypertable e `internal/impact`.

**Novo.**
- o motor de simulação e os cenários salvos;
- a projeção por cenário;
- os contadores de varredura por chunk;
- as regras `timescale.retention_*` e `timescale.compression_*`;
- a seção de UI e de PDF.

**Dependências.** 02 (tabela de preços), opcional. Sem a 02, só GB.

**Riscos.**
- A taxa de compressão varia ao longo do tempo (mudança de cardinalidade).
- Contadores por chunk zeram com reset, então "sem consulta" é fraco.
- Retenção é decisão de compliance, não técnica.

**Perguntas resolvidas** ([ADR-0001](../adr/0001-decisoes-v1-features.md), D9).
- P-01: chunks sem consulta no MVP? **Resolvida:** sim, com o proxy de contadores por chunk, rotulado aproximado.
