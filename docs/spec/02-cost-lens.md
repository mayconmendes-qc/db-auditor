# 02 — Custo por tabela, índice e consulta (`cost-lens`)

## 0. Contexto e problema, objetivo, fora de escopo

**Problema.** Times pagam RDS, Cloud SQL ou VPS sem saber quais tabelas, índices inchados ou consultas geram o custo. O db-auditor já mede tamanho, uso de índice e workload, mas não fala em dinheiro.

**Objetivo.** Traduzir o que já é coletado em custo mensal estimado (R$ ou US$) por objeto e por consulta, a partir de uma tabela de preços configurável. Entregar o ranking "top 10 desperdícios" com hipótese, ganho previsto e confiança.

**Fora de escopo.** Integrar com a fatura real do provedor (Cost Explorer, billing). Buscar preços online automaticamente. Recomendar troca de instância. MongoDB no MVP. Executar limpeza.

## 1. Requisitos funcionais

- **RF-01** Tabela de preços por ambiente (operator), versionada, com moeda, armazenamento (por GB/mês), I/O (por milhão de requisições ou por GB lido), computação (por vCPU-hora ou custo mensal da instância) e data de vigência. Modelos iniciais editáveis: "RDS PostgreSQL", "Cloud SQL", "self-hosted".
- **RF-02** Custo de armazenamento por tabela e índice = tamanho do último snapshot completo × preço de armazenamento.
- **RF-03** Desperdício de índice sem uso: índices com `index.unused` ativo, valorados por tamanho × preço.
- **RF-04** Bloat recuperável **estimado** por tabela e índice (método estatístico sobre catálogo e estatísticas, sem ler linhas). Sempre rotulado "estimativa", nunca "medido".
- **RF-05** Custo de consulta por fingerprint: participação no tempo total de execução e em blocos lidos, na janela entre duas coletas completas (delta, não acumulado), × custo de computação e I/O. Usa os deltas da spec 06.
- **RF-06** Ranking "top 10 desperdícios" por ambiente, combinando os três tipos, com valor mensal, base de cálculo, confiança e próximo passo.
- **RF-07** Novos achados no catálogo: `cost.unused_index_waste`, `cost.bloat_recoverable`, `cost.query_expensive`, com `impact_estimate` em moeda.
- **RF-08** Seção "Custo" no PDF técnico e executivo. Exportação CSV do ranking.
- **RF-09** Histórico: série de custo estimado por coleta completa, com lacunas para coletas parciais (mesma regra da série de armazenamento).

## 2. Requisitos não funcionais

- **RNF-01 Segurança e só leitura.** Nenhuma consulta nova pesada ao alvo. O bloat estimado usa catálogo e estatísticas. Medir bloat de verdade com extensão de varredura física é opcional e desligado por padrão (ver perguntas).
- **RNF-02 LGPD.** Apenas tamanhos, contagens, fingerprints sem literais e valores monetários. Nenhum texto SQL com literal.
- **RNF-03 Desempenho.** O cálculo roda sobre o snapshot store, sem nova sessão no alvo. Ranking de um ambiente com 5000 tabelas em menos de 2 s. A série respeita a janela máxima de 90 dias já aplicada às séries.
- **RNF-04 Multi-tenant.** Tabela de preços por ambiente. Viewer lê, operator edita. A alteração vai para `auditor_operation_log`.
- **RNF-05 Observabilidade.** Métrica de cálculos com e sem preço configurado. Log de mudança de tabela de preços com quem e quando.
- **RNF-06 Transparência.** Todo valor mostra fórmula, preço usado (com vigência), denominador e período (AN-03). Sem preço: "não estimado" com motivo.

## 3. Regras importantes

| Regra | Condição | Severidade | Observação |
|---|---|---|---|
| `cost.unused_index_waste` | `index.unused` aberto, índice ≥ 100 MB, janela de estatísticas ≥ 14 dias sem reset | média; alta se ≥ 5 GB | limiares **valor inicial, calibrar**; nunca recomenda DROP automático |
| `cost.bloat_recoverable` | bloat estimado ≥ 30% e ≥ 1 GB | média; alta se ≥ 20 GB | **valor inicial, calibrar**; confiança média, no máximo |
| `cost.query_expensive` | fingerprint com ≥ 10% do tempo total na janela, ≥ 1000 chamadas | baixa; média se ≥ 25% | **valor inicial, calibrar**; janela inválida se houve `stats_reset` |
| Sem preço configurado | qualquer | — | o ranking mostra bytes e tempo, valor "não estimado" |
| Coleta parcial | qualquer lado | — | não calcula delta; não entra na série |

Arredondamento: moeda com 2 casas e o rótulo "estimado/mês". Não haverá conversão cambial automática no MVP.

## 4. Respostas esperadas

- **Entradas.** Ambiente, execução (padrão: última completa), tabela de preços vigente, filtros (banco, schema, tipo).
- **UI.** Aba **Custo** no Dashboard e coluna de custo no Inventário. Ranking com tipo, objeto, valor/mês, base, confiança e link para o achado.
- **API.** Leitura do ranking e da série por ambiente (viewer). Leitura e gravação da tabela de preços (gravação: operator).
- **CSV.** Uma linha por item do ranking, com moeda e vigência do preço, terminando no marcador de exportação completa (padrão do export de ações).
- **PDF.** Seção "Custo estimado", respeitando a redação. No modo estrito sai só o total por tipo.
- **Erros.** Ambiente sem coleta completa: lista vazia com motivo. Preço com valor negativo ou moeda inválida: 400. Janela com reset: item marcado "janela inválida".

## 5. Critérios de aceite

- **CA-01** Dado um índice de 2 GB com `index.unused` aberto e preço de armazenamento de R$ 0,50/GB-mês, quando o ranking é calculado, então o item mostra R$ 1,00/mês, a fórmula e a vigência do preço.
- **CA-02** Dado um ambiente sem tabela de preços, quando o ranking é aberto, então os itens aparecem com bytes e tempo, e o valor fica "não estimado — preço não configurado".
- **CA-03** Dado duas coletas com `stats_reset` diferente, quando o custo de consulta é calculado, então nenhum `cost.query_expensive` é criado e o motivo aparece na tela.
- **CA-04** Dado uma tabela com bloat estimado de 35% e 3 GB, quando a análise roda, então surge `cost.bloat_recoverable` média, rotulado "estimativa".
- **CA-05** Dado um viewer, quando ele tenta alterar a tabela de preços, então recebe 403.
- **CA-06** Dado um PDF com redação estrita, quando é gerado, então a seção de custo não lista nomes de objetos.
- **CA-07** Dado a exportação CSV, quando é baixada, então termina com o marcador de exportação completa e nenhuma célula contém texto SQL.

## 6. Dependências, reaproveitamento, riscos e perguntas em aberto

**Reaproveita.**
- `table_snapshot` e `index_snapshot` (tamanho, `idx_scan`, `stats_reset`);
- regras `index.unused`, `storage.top_consumer`, `storage.large_table`, `maintenance.dead_tuple_pressure` (estimativa de tuplas mortas, não é bloat);
- `workload_snapshot` (tempo, chamadas, blocos lidos, fingerprint);
- `internal/impact` (AN-03), `internal/priority`;
- série `analytics/storage` e PDF (`internal/report`), com a separação hipótese / ganho previsto / ganho medido do BA-21;
- ações assistidas com medição antes e depois, para provar o ganho.

**Novo.**
- tabela de preços;
- modelo de custo;
- estimativa estatística de bloat (hoje não existe; `maintenance.dead_tuple_pressure` mede outra coisa);
- regras `cost.*`;
- aba Custo e seção do PDF.

**Dependências.** 06, para os deltas por janela de consulta. Sem a 06, RF-05 fica fora e o resto entrega.

**Riscos.**
- Preço público ≠ fatura real (descontos, reservas).
- Bloat estatístico erra em tabelas com tipos de largura variável.
- Gestão pode ler estimativa como promessa. Mitigação: rótulo "estimado" e faixa de confiança.

**Perguntas em aberto.**
- P-01: moedas aceitas (BRL e USD com câmbio manual por tabela, ou uma moeda só por ambiente)?
- P-02: permitir, como opt-in, a medição física de bloat por extensão no alvo (mais I/O) ou ficar só na estimativa?
- P-03: quais provedores entram como modelos iniciais além de RDS, Cloud SQL e self-hosted?
