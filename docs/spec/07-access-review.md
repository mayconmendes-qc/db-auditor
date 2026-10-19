# 07 — Revisão periódica de acessos (`access-review`)

## 0. Contexto e problema, objetivo, fora de escopo

**Problema.** SOC 2 e ISO 27001 pedem revisão periódica de acessos com decisão registrada. No PostgreSQL, a herança de roles torna difícil saber quem lê o quê.

O db-auditor já coleta:
- privilégio efetivo por conta com LOGIN × relação (`grant_snapshot`, resolvido pelo servidor, então já inclui herança);
- atributos e poderes herdados das contas (`account_role_snapshot`, `security.powerful_role`, PG 14–18);
- contas sem sessão observada (`security.account_inactive_review`).

Falta: o **caminho** da herança por privilégio, o ciclo de revisão com responsável e decisão, o diff com a revisão anterior e a exportação para auditoria.

**Objetivo.** Campanhas de revisão por ambiente. A matriz conta × objeto × privilégio efetivo vem com origem (direto ou via qual role), cada linha recebe aprovar, revogar ou justificar do responsável, há diff com a campanha anterior e exportação CSV como evidência (XLSX depois, ADR-0001 D7).

**Fora de escopo.** Executar revogação, mesmo que a decisão seja "revogar" (vira ação assistida para execução externa). Contas do próprio db-auditor (já em Contas). Integração com IdP/LDAP. MongoDB no MVP.

## 1. Requisitos funcionais

- **RF-01** Campanha de revisão: ambiente, escopo (bancos e schemas), coleta de referência (completa), responsáveis, prazo e estado (aberta, em revisão, concluída, vencida).
- **RF-02** Matriz efetiva a partir da coleta de referência:
  - conta com LOGIN × objeto (tabela, visão, sequence, função com EXECUTE, schema com CREATE ou USAGE) × privilégio;
  - **origem**: direto, via role X (cadeia), via PUBLIC, via role predefinida.

  A origem por privilégio é nova. Hoje ela só existe para os atributos poderosos.
- **RF-03** Privilégios por coluna (novo), quando existirem, como linhas próprias.
- **RF-04** Destacar na matriz:
  - superusuários e poderes de `security.powerful_role`;
  - contas sem sessão observada ou com validade vencida (`security.account_inactive_review`);
  - privilégio de escrita em schema de outra aplicação;
  - privilégio novo desde a campanha anterior.
- **RF-05** Agrupar por responsável: cada role ou conta recebe um responsável, que é uma **conta do auditor** com papel auditor ou acima no ambiente, mais um contato livre opcional (nome e/ou e-mail) para a planilha. O mapeamento é mantido no auditor (novo). Linhas sem responsável vão para o operator (ADR-0001 D7).
- **RF-06** Decisão por linha ou em lote: aprovar, revogar (com prazo), ou manter com justificativa obrigatória. Decide o responsável da linha. Nas linhas sem responsável, decide o operator (ADR-0001 D7). Registro de quem, quando e por quê, imutável.
- **RF-07** "Revogar" cria uma ação assistida existente (plano, execução externa, medição na próxima coleta). Na campanha seguinte, um privilégio revogado que continua presente aparece como "revogação não aplicada".
- **RF-08** Diff com a campanha anterior: privilégios novos, removidos, decisões que mudaram e contas novas ou removidas.
- **RF-09** Exportação CSV da matriz com as decisões (XLSX fica para depois, fora do v1, ADR-0001 D7). PDF de evidência da campanha: resumo, responsáveis, % decidido, pendências.

## 2. Requisitos não funcionais

- **RNF-01 Segurança e só leitura.** A matriz usa os snapshots. A resolução de origem e os privilégios por coluna são leituras de catálogo pela conexão só leitura, na coleta. Nada é revogado pelo auditor.
- **RNF-02 LGPD.** Só nomes de roles, objetos e privilégios. Nomes de role podem identificar pessoas, então os modos de redação do PDF valem (identificadores ocultam nomes de conta). Nenhum dado de tabela.
- **RNF-03 Desempenho e limites.** Matriz de até 500 contas × 5000 objetos, paginada e filtrável no servidor, com resposta em menos de 2 s por página. Exportação em streaming, com o marcador de fim (padrão do export de ações). (Quando o XLSX entrar, depois do v1: limite de 1 M linhas por aba, com aviso.)
- **RNF-04 Multi-tenant.** A campanha pertence ao ambiente. Responsável deve ter acesso ao ambiente (papel auditor ou acima). Viewer lê. Operator cria a campanha e o mapeamento de responsáveis. Um auditor que não é o responsável da linha recebe 403 ao decidir (ADR-0001 D7).
- **RNF-05 Observabilidade.** Métricas de campanhas por estado e de linhas pendentes. Toda decisão vai para `auditor_operation_log`.
- **RNF-06 Imutabilidade.** Uma campanha concluída não muda. Correção = nova decisão registrada, com referência à anterior.

## 3. Regras importantes

| Situação | Severidade | Observação |
|---|---|---|
| Superusuário ou BYPASSRLS sem decisão "aprovar" na campanha vigente | alta | reaproveita a severidade de `security.powerful_role` quando maior |
| Conta com LOGIN sem sessão observada há ≥ 90 dias e com privilégio de escrita | média | limiar **valor inicial, calibrar**; ausência de sessão na amostra não prova falta de uso (ressalva existente) |
| Privilégio concedido via PUBLIC em tabela fora de `public` | média | — |
| Privilégio novo desde a campanha anterior, sem decisão | média | — |
| Decisão "revogar" não aplicada após o prazo | alta | verificado na coleta completa seguinte |
| Campanha vencida com ≥ 10% de linhas pendentes | média | limiar **valor inicial, calibrar** |
| Periodicidade sugerida | — | trimestral (**valor inicial, calibrar**) |

## 4. Respostas esperadas

- **Entradas.** Ambiente, escopo, coleta de referência, responsáveis, prazo.
- **UI.**
  - Em Segurança, nova área **Revisão de acessos**: lista de campanhas;
  - matriz com filtros (conta, objeto, privilégio, origem, destaque, pendente);
  - decisão por linha e em lote;
  - diff com a anterior;
  - visão "minhas pendências" para o responsável.
- **API.** Criar e encerrar campanha e mapear responsáveis (operator). Decidir (auditor responsável). Ler matriz, diff e progresso (viewer). Exportar (auditor, regra de download).
- **CSV.** Colunas: conta, objeto qualificado, tipo, privilégio, origem, destaque, responsável, decisão, justificativa, decidido por, decidido em.
- **PDF.** Evidência da campanha, respeitando a redação.
- **Erros.**
  - coleta de referência parcial: 409 (campanha exige coleta completa);
  - responsável sem acesso ao ambiente: 400;
  - justificativa vazia em "manter": 400;
  - decisão em campanha concluída: 409.

## 5. Critérios de aceite

- **CA-01** Dado uma conta `app_report` que lê `vendas.pedido` só por ser membro de `g_leitura`, quando a matriz é aberta, então a linha mostra SELECT com origem "via g_leitura".
- **CA-02** Dado uma campanha anterior em que `app_etl` não tinha UPDATE em `vendas.pedido`, quando a nova campanha é criada após a concessão, então o diff lista o privilégio como novo e destacado.
- **CA-03** Dado uma decisão "revogar" com prazo vencido e o privilégio ainda presente na coleta completa seguinte, quando a análise roda, então surge um achado alto de "revogação não aplicada", vinculado à ação assistida.
- **CA-04** Dado uma decisão "manter" sem justificativa, quando enviada, então a API responde 400 e nada é gravado.
- **CA-05** Dado um viewer, quando ele tenta decidir uma linha, então recebe 403.
- **CA-06** Dado a exportação CSV, quando baixada, então termina com o marcador de exportação completa e o número de linhas confere com o total da matriz.
- **CA-07** Dado PDF com redação "identificadores", quando gerado, então os nomes de conta não aparecem e as contagens sim.
- **CA-08** Dado PostgreSQL 14 e 16, quando a origem via role com e sem herança é resolvida, então o resultado segue as regras por versão já documentadas em `docs/rules.md` (#126, #134), testado na matriz da CI.

## 6. Dependências, reaproveitamento, riscos e perguntas em aberto

**Reaproveita.**
- `grant_snapshot` (coletor `effective_grants`);
- `account_role_snapshot` e `security.powerful_role` (atributos, `inherited`, `source_roles`, regras por versão);
- `security.account_inactive_review`, `security.excessive_privilege` e `security.public_schema_create`;
- `rls_policy_snapshot` e `schema_snapshot.nspacl`;
- EXECUTE observado de funções;
- ações assistidas e medições;
- export streaming CSV/JSONL;
- PDF e redação;
- RBAC e `auditor_operation_log`.

**Novo.**
- entidade campanha;
- origem por privilégio;
- privilégio por coluna;
- mapeamento de responsáveis;
- decisões imutáveis;
- diff entre campanhas;
- último login por role vindo do agregado de logs da 10 (ADR-0001 D10);
- regras de revogação não aplicada.

**Dependências.** É base da 05. Usa a 10 para o último login vindo de log (o agregado de role e horário, ADR-0001 D10; sem ele vale só a amostra de sessões) e, de forma opcional, a 04 (timeline de quando o privilégio apareceu).

**Riscos.**
- Matriz grande demais para revisão humana. Mitigação: lote e filtros por destaque.
- "Sem sessão observada" é amostral, então não pode embasar revogação sozinho.
- Nomes de role são dados pessoais em alguns contextos.

**Perguntas resolvidas** ([ADR-0001](../adr/0001-decisoes-v1-features.md), D7 e D10).
- P-01: o responsável é conta do auditor ou campo livre? **Resolvida:** conta do auditor com papel auditor ou acima no ambiente, mais contato livre opcional.
- P-02: XLSX? **Resolvida:** CSV no v1. XLSX depois.
- P-03: fonte de último login? **Resolvida:** a amostragem de sessões atual e o agregado de último login da spec 10 (só role e horário), com a fonte explícita.
