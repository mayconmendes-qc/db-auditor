# 05 — Mapa de dados pessoais LGPD (`pii-scanner`)

## 0. Contexto e problema, objetivo, fora de escopo

**Problema.** A LGPD exige inventário de dados pessoais e de quem tem acesso a eles. Ferramentas genéricas (Presidio) não conhecem CPF, CNPJ ou chave PIX. O db-auditor já sabe colunas, tipos, estatísticas e privilégio efetivo por tabela, mas não classifica conteúdo.

**Objetivo.** Mapear `banco.schema.tabela.coluna` → categoria de dado pessoal, com confiança e com quem pode ler, **sem nunca expor ou guardar valores**. Gerar achados quando dado pessoal está amplamente acessível.

**Fora de escopo.** Detectar dados sensíveis do art. 5º, II (saúde, biometria, religião). Classificar texto livre ou JSONB aninhado no MVP. Mascarar ou anonimizar dados no banco. MongoDB (o auditor não lê documentos). Parecer jurídico.

## 1. Requisitos funcionais

- **RF-01** Etapa 1, só metadados: heurística por nome e tipo de coluna (cpf, cnpj, email, telefone/celular, cep, pix, nome, nascimento, endereço, rg), com dicionário editável por ambiente pelo operator (ADR-0001 D14). Sem ler linhas.
- **RF-02** Etapa 2, opcional e explicitamente habilitada: validação de conteúdo sobre uma amostra limitada de colunas candidatas (texto e numéricas). Detectores:
  - CPF e CNPJ com dígito verificador (CNPJ numérico e alfanumérico, ADR-0001 D14);
  - e-mail;
  - telefone BR (fixo e celular, com ou sem DDI);
  - CEP;
  - chave PIX: CPF, CNPJ, e-mail, telefone ou chave aleatória.
- **RF-03** Por coluna: categoria, confiança (alta, média, baixa), método (nome, conteúdo, ambos), tamanho da amostra, contagem e % de valores que validaram, e % nulo (estatística existente).
- **RF-04** Cruzar com o acesso: contas com LOGIN que têm SELECT efetivo na tabela (já coletado), privilégios por coluna (novo), presença de RLS na tabela (já coletado) e contas poderosas (`security.powerful_role`, `pg_read_all_data`).
- **RF-05** Achados:
  - `pii.column_detected` (informativa);
  - `pii.broad_read_access`: coluna pessoal legível por muitas contas ou por role ampla;
  - `pii.no_rls_hypothesis`: tabela com dado pessoal, sem RLS e com mais de uma aplicação lendo. É hipótese.
- **RF-06** Revisão humana: um auditor confirma ou descarta cada classificação. O descarte usa a supressão existente, com justificativa, e não reabre sem mudança de evidência.
- **RF-07** Exportação CSV do mapa e seção "Dados pessoais" no PDF, sempre sem valores.

## 2. Requisitos não funcionais

- **RNF-01 Segurança e só leitura.** A etapa 2 exige a flag própria e mais restrita `AUDITOR_PII_CONTENT_SCAN_ENABLED=1` no processo **e** a habilitação explícita do ambiente pelo operator. `AUDITOR_DATA_QUALITY_ENABLED` não basta (ADR-0001 D4). Usa a conta só leitura, transação só leitura e timeout por coluna.
- **RNF-02 LGPD (crítico).** Valores amostrados vivem só em memória durante a validação e são descartados. Não vão para log, store, evidência, PDF, CSV, mensagem de erro nem métrica. Nem mascarados, nem com hash: hash de CPF é reversível por força bruta. Persistir só contagens e proporções.
- **RNF-03 Desempenho e limites.** Amostra de 1000 linhas por tabela (o mesmo teto do diagnóstico atual, independente do teto dos contratos da 03; ADR-0001 D4), máximo de 50 colunas candidatas por tabela e 200 tabelas por execução (**valor inicial, calibrar**). Amostragem por páginas em tabelas grandes. Teto de duração por execução: 10 min.
- **RNF-04 Multi-tenant.** O mapa pertence ao ambiente. Ver o mapa exige viewer, mas a etapa 2 e a confirmação exigem auditor. O próprio mapa é informação sensível, então o download exige auditor (regra atual de download).
- **RNF-05 Observabilidade.** Métricas de colunas analisadas por método e de execuções interrompidas por teto. Um teste automatizado garante que nenhum valor amostrado aparece em log ou store (padrão dos testes de redação existentes).

## 3. Regras importantes

| Item | Regra | Severidade / confiança |
|---|---|---|
| Confiança alta | ≥ 90% dos valores não nulos da amostra validam, com ≥ 50 valores não nulos | — |
| Confiança média | 50–90% validam, ou só nome com tipo compatível | — |
| Confiança baixa | só nome, ou < 50 valores não nulos na amostra | — |
| `pii.column_detected` | qualquer categoria com confiança ≥ média | informativa |
| `pii.broad_read_access` | coluna de confiança alta legível por ≥ 5 contas com LOGIN, ou por membro de `pg_read_all_data` que não seja o auditor | média; alta se CPF/CNPJ/PIX |
| `pii.no_rls_hypothesis` | tabela com dado pessoal de confiança alta, sem RLS e com ≥ 2 contas de aplicação com SELECT | baixa (hipótese) |

Os limiares de %, quantidade de contas e tamanho de amostra são **valor inicial, calibrar**.

## 4. Respostas esperadas

- **Entradas.** Ambiente, escopo (bancos e schemas, respeitando a allowlist e a denylist existentes), etapas (1, ou 1+2) e dicionário de nomes.
- **UI.** Nova aba **Dados pessoais** em Segurança. Mapa por tabela/coluna com categoria, confiança, método, contas com leitura e RLS. Filtros por categoria e confiança. Ações confirmar e descartar.
- **API.** Iniciar varredura (auditor). Ler o mapa e as contas com acesso (viewer). Confirmar ou descartar (auditor).
- **CSV.** Colunas: coluna qualificada, categoria, confiança, método, % validado, amostra, nº de contas com leitura, RLS. O arquivo termina com o marcador de exportação completa.
- **PDF.** Seção com totais por categoria. Detalhe por coluna só sem redação. No modo "identificadores" ou "estrita", só contagens.
- **Erros.**
  - etapa 2 desabilitada: 409;
  - sem permissão de SELECT na tabela: coluna "não verificada — sem permissão", com cobertura parcial;
  - teto de tempo atingido: execução `partial_success`, com o que faltou.

## 5. Critérios de aceite

- **CA-01** Dado uma coluna `documento` com 95% de CPFs válidos numa amostra de 500 valores não nulos, quando a etapa 2 roda, então a coluna é classificada como CPF, com confiança alta e método "conteúdo".
- **CA-02** Dado a mesma varredura, quando o store, os logs, a evidência e o CSV são inspecionados, então nenhum CPF (completo, mascarado ou em hash) aparece.
- **CA-03** Dado uma coluna `cpf` com 11 dígitos sem dígito verificador válido em 80% da amostra, quando analisada, então a confiança é média, não alta.
- **CA-04** Dado uma coluna CPF de confiança alta legível por 7 contas com LOGIN, quando a análise termina, então surge `pii.broad_read_access` alta, com a lista de contas na evidência.
- **CA-05** Dado a etapa 2 desabilitada, quando um auditor pede a varredura com conteúdo, então recebe 409 e só a etapa 1 pode rodar.
- **CA-06** Dado um ambiente MongoDB, quando a aba é aberta, então ela mostra "não aplicável a este mecanismo".
- **CA-07** Dado uma classificação descartada com justificativa, quando a próxima varredura encontra a mesma evidência, então o descarte é mantido.

## 6. Dependências, reaproveitamento, riscos e perguntas em aberto

**Reaproveita.**
- `column_snapshot` (nome e tipo) e `column_stat_snapshot` (`null_fraction`, `distinct_estimate`);
- `grant_snapshot` (privilégio efetivo por conta com LOGIN × tabela) e `rls_policy_snapshot`;
- `account_role_snapshot`, `security.powerful_role` e `security.excessive_privilege`;
- a amostragem limitada e os testes de redação de `internal/quality`;
- allowlist e denylist de schemas da coleta;
- redação do PDF (`internal/report/redaction`, `sensitive`) e supressão de achados com justificativa.

**Novo.**
- detectores brasileiros;
- dicionário de nomes;
- privilégio por coluna (hoje só por tabela);
- regras `pii.*`;
- aba Dados pessoais.

**Dependências.** 07 (matriz de privilégio efetivo com caminho de herança e privilégio por coluna). Sem a 07, usar `grant_snapshot` por tabela e marcar a evidência "nível tabela".

**Riscos.**
- Falso positivo em números de 11 dígitos sem DV (telefones).
- Coluna JSONB com dado pessoal fica fora do escopo.
- O próprio mapa é um alvo de ataque (onde estão os CPFs). Mitigação: download só para auditor e redação.

**Perguntas resolvidas** ([ADR-0001](../adr/0001-decisoes-v1-features.md)).
- P-01: mesma flag do diagnóstico ou uma dedicada? **Resolvida (D4):** flag dedicada e mais restrita, com habilitação por ambiente. A conta continua sendo a só leitura existente.
- P-02: JSONB e texto livre? **Resolvida (D14):** fora do v1.
