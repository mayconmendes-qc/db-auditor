# 10 — Anomalias em logs estruturados (`log-anomaly`)

## 0. Contexto e problema, objetivo, fora de escopo

**Problema.** Times pequenos não têm Datadog. Um arquivo de log JSON já bastaria para ver picos de erro e padrões novos. Hoje o db-auditor não lê logs: só catálogo e estatísticas.

**Objetivo.** Um modo CLI do mesmo binário que lê logs estruturados (arquivo ou entrada padrão), agrupa linhas em templates (estilo Drain), compara contagens por janela com uma linha de base e aponta anomalias ("padrão X cresceu 8× às 14h", "padrões novos"). Foco inicial no log JSON do PostgreSQL, com perfil JSON genérico. No v1, o processamento é local. O único dado que sobe ao auditor é o agregado de último login por role (ADR-0001 D10).

**Fora de escopo.** Coletar logs remotamente (agente, syslog, nuvem). Busca de texto completo. Guardar linhas brutas. Logs não estruturados no MVP. Substituir uma stack de observabilidade.

## 1. Requisitos funcionais

- **RF-01** Ler JSON por linha de um arquivo, de vários arquivos ou da entrada padrão. Perfis de campo:
  - "postgres-jsonlog" (log JSON nativo do PostgreSQL 15+);
  - "genérico", com mapeamento configurável de horário, nível e mensagem.
- **RF-02** Extrair o template de cada mensagem, substituindo partes variáveis (números, identificadores, literais, IPs, e-mails, hashes) por marcadores. Agrupar por template com um id estável (hash do template).
- **RF-03** Contar por template e janela (padrão 5 min, configurável) e por nível (erro, aviso, info).
- **RF-04** Linha de base por template: média e desvio das últimas K janelas (padrão 12), ou de um arquivo ou período de referência informado. Calcular o z-score por janela.
- **RF-05** Anomalias:
  - (a) pico de template;
  - (b) template novo sem histórico na base;
  - (c) pico de volume por nível de erro;
  - (d) template que sumiu (informativa).
- **RF-06** Saída no terminal e em Markdown: top anomalias com template mascarado, janela, contagem observada × esperada, fator e z-score. Saída JSON com os mesmos campos.
- **RF-07** *(fora do v1, ADR-0001 D10)* Envio ao auditor de agregados de anomalia (template mascarado, id, contagens por janela, anomalias), vinculados a um ambiente, e lista "Anomalias de log" na UI. No v1, templates, contagens e anomalias não saem da máquina.
- **RF-08** No perfil postgres-jsonlog, eventos de conexão autenticada fornecem o "último login por role" (só nome da role e horário mais recente) para a spec 07. A CLI gera o arquivo local e, opcionalmente, o envia ao ambiente com o token por ambiente (escopo `last-login:write`). É o único envio à API no v1 (ADR-0001 D1, D10).

## 2. Requisitos não funcionais

- **RNF-01 Segurança e só leitura.** A CLI não conecta ao banco auditado. Só lê arquivos locais. O único envio (último login) usa o token do ambiente com escopo `last-login:write`. Nenhuma linha de log, template ou contagem trafega.
- **RNF-02 LGPD (crítico).** Logs contêm valores (parâmetros de consulta, e-mails, IPs). As saídas locais têm só templates mascarados e contagens. O envio tem só role e horário. Linhas brutas e exemplos nunca saem da memória. Um teste garante que nenhum valor variável sobrevive ao mascaramento. Se o template inteiro for variável (sem parte fixa), ele vira "não classificável" e não é exibido.
- **RNF-03 Desempenho e limites.** Ao menos 50 MB/s de leitura em streaming com memória limitada (padrão 256 MB). Máximo de 10 000 templates distintos, com o excedente agregado em "outros" e aviso. Linha acima de 64 KB descartada e contada.
- **RNF-04 Multi-tenant.** O agregado de último login enviado pertence a um ambiente e é lido na revisão de acessos (07) por quem tem acesso ao ambiente. A credencial de CLI só escreve no seu ambiente.
- **RNF-05 Observabilidade.** A CLI reporta linhas lidas, descartadas e inválidas e templates. Na API, métricas de envios de último login recebidos e rejeitados.
- **RNF-06 Determinismo.** A mesma entrada com a mesma configuração gera os mesmos ids de template e as mesmas anomalias.

## 3. Regras importantes

| Anomalia | Condição | Severidade |
|---|---|---|
| Pico de template | z ≥ 3 e contagem ≥ 20 na janela | baixa; média se nível erro |
| Pico forte | fator ≥ 8× a média e contagem ≥ 50 | média; alta se nível erro ou FATAL/PANIC |
| Template novo | sem ocorrência na base e ≥ 10 ocorrências na janela | baixa; média se nível erro |
| Pico de erros | volume de nível erro ≥ 5× a média da base | alta |
| Template sumiu | presente em ≥ 50% das janelas da base e ausente nas últimas 3 | informativa |
| Base insuficiente | < 6 janelas de histórico | — (só "templates novos", com confiança baixa) |

Todos os limiares são **valor inicial, calibrar**. Gate de CLI: bloqueia a partir de alta.

## 4. Respostas esperadas

- **Entradas.** Arquivos ou entrada padrão, perfil de campos, tamanho de janela, base (K janelas ou referência), ambiente para envio do último login (opcional), severidade de bloqueio.
- **Terminal/Markdown.** Resumo (linhas, período, templates, anomalias por severidade), tabela de anomalias e ressalvas (linhas descartadas, base insuficiente).
- **JSON.** Mesmos campos, mais a versão do esquema.
- **API.** Receber o agregado de último login (token do ambiente, escopo `last-login:write`), exibido na revisão de acessos (07). A lista de anomalias e templates por ambiente na UI fica fora do v1 (RF-07).
- **Códigos de saída.** Os do README. 3 quando mais de 10% das linhas forem inválidas ou a base for insuficiente para picos.
- **Erros.**
  - arquivo inexistente ou ilegível: código 2;
  - perfil sem campo de horário: código 2;
  - envio do último login rejeitado (credencial, escopo, tamanho): código 2, com a saída local preservada.

## 5. Critérios de aceite

- **CA-01** Dado um log com 10 ocorrências/janela de "connection authorized: user=? database=?" e uma janela com 90, quando analisado, então a saída aponta pico forte (9×) com o template mascarado.
- **CA-02** Dado linhas com e-mails, IPs e parâmetros de consulta, quando as saídas (terminal, Markdown, JSON, arquivo de último login) e o envio de último login são inspecionados, então nenhum desses valores aparece.
- **CA-03** Dado um template de nível erro que nunca apareceu e surge 15 vezes numa janela, quando analisado, então aparece como template novo médio.
- **CA-04** Dado um arquivo com 20% de linhas que não são JSON, quando analisado, então a CLI processa o resto, informa as descartadas e sai com código 3.
- **CA-05** Dado a mesma entrada processada duas vezes, quando comparadas as saídas JSON, então são idênticas.
- **CA-06** Dado um envio de último login com credencial de outro ambiente, quando recebido, então a API rejeita com 403 e a CLI sai com 2.
- **CA-07** Dado eventos de conexão no perfil postgres-jsonlog, quando o insumo de último login é gerado, então contém só a role e o horário mais recente, sem IP nem banco de origem.

## 6. Dependências, reaproveitamento, riscos e perguntas em aberto

**Reaproveita.**
- o binário e a convenção de CLI e códigos (README, base da 01);
- a normalização de literais do coletor de workload, extraída para `internal/fingerprint` na 01 (ADR-0001 D16), como ponto de partida do mascaramento;
- o modelo de achado e severidade;
- a credencial de escopo de ambiente da 01;
- a sanitização de log de `config`.

**Novo.**
- a leitura de logs;
- o clustering de templates;
- a linha de base e o z-score;
- os perfis de campo;
- o insumo de último login para a 07 e o seu envio ao ambiente (o armazenamento de agregados de anomalia fica fora do v1).

**Dependências.** 01 (modo CLI e credencial). 07 (rota que recebe e exibe o último login).

**Riscos.**
- Mascaramento incompleto vaza dado pessoal. Mitigação: lista de padrões testada e regra de "não classificável".
- É a feature menos alinhada ao foco em banco e pode competir por esforço.
- Templates instáveis entre versões do PostgreSQL.

**Perguntas resolvidas** ([ADR-0001](../adr/0001-decisoes-v1-features.md), D10).
- P-01: só logs do PostgreSQL ou também JSON genérico? **Resolvida:** os dois (perfis `postgres-jsonlog` e genérico).
- P-02: agregados no snapshot store ou só local? **Resolvida:** só local no v1, com uma exceção: o agregado de último login por role (role e horário) sobe ao ambiente para a 07.
