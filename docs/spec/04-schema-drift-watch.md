# 04 — Linha do tempo e alerta de drift de schema (`schema-drift-watch`)

## 0. Contexto e problema, objetivo, fora de escopo

**Problema.** Dev, homologação e produção divergem (índice criado à mão, coluna esquecida), e ninguém sabe quando nem por qual caminho a mudança entrou.

O que já existe no db-auditor:
- snapshots estruturais por coleta;
- comparação genérica de conjuntos (`POST /api/v1/compare`, `internal/comparison`);
- comparação de servidores (versão, extensões, GUCs, hypertables, políticas e CAGGs);
- regressão `structure` contra o baseline.

Lacuna verificada: a tela **Desvio de schema** (`SchemaDriftPage`) envia **dados de demonstração fixos** ao endpoint de comparação, não os snapshots reais. Também não há linha do tempo de mudanças nem detecção de "mudou fora do pipeline".

**Objetivo.** Diff de schema real entre ambientes e entre duas coletas, linha do tempo de mudanças por ambiente e alerta quando a mudança não corresponde a uma implantação registrada.

**Fora de escopo.** Gerar ou aplicar migration de correção. Gerenciar migrations (papel do Atlas e similares). Diff de dados. Diff de MongoDB além de coleções e índices já coletados.

## 1. Requisitos funcionais

- **RF-01** Ligar a tela Desvio de schema aos snapshots reais: escolher ambiente e execução de origem e de destino (padrão: última completa de cada). Remover os dados de demonstração.
- **RF-02** Diff entre duas execuções do **mesmo** ambiente ou de ambientes diferentes, com os estados existentes (igual, diferente, só na origem, só no destino), cobrindo tabelas, colunas (tipo, nulidade, default por fingerprint), índices, constraints, visões, funções (assinatura e fingerprint do corpo), triggers, sequences e, no Timescale, hypertables e políticas.
- **RF-03** Usar os mapeamentos (`object_mapping`) para parear objetos com nomes diferentes entre ambientes.
- **RF-04** Linha do tempo por ambiente: a cada coleta completa, registrar os eventos de mudança em relação à coleta completa anterior (objeto, tipo de mudança, campos, execução, intervalo de tempo em que ocorreu).
- **RF-05** Correlacionar cada evento com a evidência de pipeline:
  - (a) anotação de implantação existente (`audit_annotation`, tela Acompanhamento) no intervalo;
  - (b) opcional: o registro de migrations da ferramenta usada pela aplicação no alvo (golang-migrate, goose, Flyway, Alembic, Rails ou Prisma; versão mais recente e momento de aplicação, quando a ferramenta grava), configurado por ambiente.

  Qualquer uma das duas fontes correlaciona o evento, e o registro não é obrigatório. Para ferramentas sem carimbo de aplicação (golang-migrate, Alembic, Rails), a correlação é "a versão mudou entre as duas coletas", com confiança rebaixada. Sem correlação, o evento é marcado como "fora do pipeline" (ADR-0001 D5).
- **RF-06** Achados novos: `drift.out_of_pipeline` (mudança sem implantação correspondente) e `drift.environment_divergence` (divergência entre ambientes pareados que persiste em duas coletas completas de cada lado). O par vem de `object_mapping`, e o atributo `is_production` define o lado de produção. Não há entidade nova de par (ADR-0001 D5).
- **RF-07** Webhook e e-mail para os destinos do ambiente, com fallback nos canais globais quando não há destino (ADR-0001 D6). Payload sanitizado (tipo, ambiente, objeto qualificado, tipo de mudança, link), sem definição SQL. Cada webhook tem o seu formato, JSON genérico ou nativo do Slack. O webhook global continua no JSON genérico, e os alertas existentes não mudam (ADR-0001 D15).
- **RF-08** Exportar o diff e a timeline em Markdown e CSV.

## 2. Requisitos não funcionais

- **RNF-01 Segurança e só leitura.** O diff e a timeline usam só o snapshot store, sem nova sessão no alvo. O RF-05(b) é uma leitura de metadado de uma tabela de controle de migrations, pela conexão só leitura, e só se o operator configurar.
- **RNF-02 LGPD.** Definições de visão, função, índice e default aparecem como fingerprint (padrão atual). Nunca como texto. Do registro de migrations, só a versão e o carimbo de aplicação.
- **RNF-03 Desempenho.** Diff de 5000 tabelas e 50 000 colunas em menos de 5 s, paginado. Timeline paginada, com janela padrão de 90 dias (teto das séries). Evento gravado na ingestão, não recalculado na leitura.
- **RNF-04 Multi-tenant.** Diff entre ambientes exige acesso aos dois. Sem acesso a um lado: 403, sem revelar existência de objetos.
- **RNF-05 Observabilidade.** Métricas de eventos gravados por tipo e de alertas enviados ou falhos. Contar falhas é novo: hoje `alert_delivery` só registra entregas, para dedup.
- **RNF-06 Comparabilidade.** Só coletas completas, com o mesmo perfil e versão de coletor, geram eventos. Uma coleta parcial no meio cria lacuna, não evento (regra AN-11).

## 3. Regras importantes

| Situação | Severidade | Observação |
|---|---|---|
| Tabela ou coluna removida fora do pipeline, em ambiente marcado como produção | alta | — |
| Tipo de coluna ou PK alterados fora do pipeline | alta | — |
| Índice criado ou removido fora do pipeline | média | índice manual é o caso mais comum |
| Função ou visão com fingerprint alterado fora do pipeline | média | alta se a função é SECURITY DEFINER |
| Objeto novo fora do pipeline | baixa | — |
| Divergência entre ambientes pareados, persistente | média | informativa se o par não é "produção × homologação" |
| Mudança com implantação correspondente | informativa | só aparece na timeline, sem achado |

- Janela de correlação: a anotação vale se cair entre as duas coletas, com tolerância de ±30 min (**valor inicial, calibrar**).
- A marcação "ambiente de produção" é um atributo novo do ambiente. Sem ela, as severidades caem um nível.

## 4. Respostas esperadas

- **Entradas.** Origem (ambiente e execução), destino (ambiente e execução), filtros de tipo e estado, período da timeline.
- **UI.**
  - Desvio de schema com dados reais e campos alterados por objeto;
  - nova aba **Linha do tempo** por ambiente, com evento, execução, correlação (anotação ou migration) e selo "fora do pipeline";
  - link do evento para o objeto no Inventário.
- **API.** Diff entre execuções (viewer, com acesso aos dois lados). Timeline paginada por ambiente (viewer). Configuração do registro de migrations (operator).
- **Markdown/CSV.** Diff e timeline exportáveis. O CSV termina com o marcador de exportação completa.
- **Webhook.** Um evento por episódio, com o dedup das regressões.
- **Erros.**
  - execução parcial escolhida: aviso e diff marcado "parcial", sem achados;
  - mecanismos diferentes (PostgreSQL × MongoDB): 400 com mensagem;
  - registro de migrations configurado mas inacessível: correlação "não verificada" e cobertura parcial.

## 5. Critérios de aceite

- **CA-01** Dado dois ambientes com coletas completas, quando o usuário abre Desvio de schema, então o resultado vem dos snapshots dessas execuções e nenhum dado de demonstração aparece.
- **CA-02** Dado um índice criado manualmente entre duas coletas completas sem anotação de implantação, quando a segunda coleta é analisada, então a timeline mostra o evento "fora do pipeline" e um achado `drift.out_of_pipeline` médio é criado.
- **CA-03** Dado a mesma mudança com uma anotação de implantação no intervalo, quando analisada, então o evento aparece correlacionado e nenhum achado é criado.
- **CA-04** Dado uma coleta parcial entre duas completas, quando a timeline é calculada, então o intervalo aparece como lacuna e não gera eventos.
- **CA-05** Dado uma função com corpo alterado, quando o diff é exibido, então mostra os fingerprints antigo e novo e não mostra o corpo.
- **CA-06** Dado um viewer com acesso só ao ambiente A, quando pede o diff A × B, então recebe 403.
- **CA-07** Dado o webhook configurado, quando ocorre `drift.out_of_pipeline`, então um único POST sai com o payload sanitizado, e a repetição na coleta seguinte não reenvia.

## 6. Dependências, reaproveitamento, riscos e perguntas em aberto

**Reaproveita.**
- snapshots `table_snapshot`, `column_snapshot`, `index_snapshot`, `constraint_snapshot`, `view_snapshot`, `function_snapshot`, `trigger_snapshot`, `sequence_snapshot`, `hypertable_snapshot` e `policy_snapshot`, com fingerprints;
- `internal/comparison` e `POST /api/v1/compare`;
- `SchemaDriftPage`, Mapeamentos (`object_mapping`) e `findings-diff`;
- comparação de servidores e regras `config.*_drift` (drift de configuração; continuam lá, sem duplicar);
- regressão `structure` e baseline (AN-11);
- anotações de Acompanhamento e `internal/notify` com `alert_delivery` (dedup de entregas).

**Novo.**
- seleção de execuções reais na tela;
- eventos de timeline gravados na ingestão;
- correlação com pipeline;
- leitura opcional do registro de migrations;
- atributo "produção" do ambiente;
- regras `drift.*`.

**Dependências.** Nenhuma bloqueante. A 01 e a 07 usam a timeline de forma opcional.

**Riscos.**
- Intervalo longo entre coletas mascara mudanças revertidas.
- Anotação manual esquecida gera falso "fora do pipeline". Mitigação: a CLI de CI ou deploy registra a anotação automaticamente.
- Sobreposição com a regressão `structure`. Regra: a regressão compara com o baseline, a timeline compara coleta a coleta. Os textos devem deixar isso claro.

**Perguntas resolvidas** ([ADR-0001](../adr/0001-decisoes-v1-features.md)).
- P-01: quais ferramentas no RF-05(b)? **Resolvida (D5):** as seis listadas, além da anotação de implantação. Qualquer fonte correlaciona.
- P-02: formato nativo do Slack? **Resolvida (D5, D15):** sim, escolhido por webhook. O JSON genérico continua o padrão.
