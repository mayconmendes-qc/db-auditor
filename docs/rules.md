# Regras e heurísticas — Sprint 16

O catálogo `rule_catalog` é semeado na inicialização da API. Cada combinação
`rule_id`/`rule_version` é imutável: alterar critérios, texto ou parâmetros
exige nova versão. A API recusa iniciar se detectar uma versão existente com
conteúdo diferente. Findings persistem a versão, confiança, parâmetros,
recomendação, validação e referências usados na análise. Mudança de versão ou
de política gera outra identidade de deduplicação, sem reescrever o histórico.

## Configuração por ambiente e schema

`rule_policy` guarda overrides. `schema_name=''` vale para todo o ambiente;
uma linha para um schema específico tem precedência. A API de leitura é
`GET /api/v1/environments/{id}/rules?schema=public`, que retorna catálogo,
estado habilitado e parâmetros efetivos. Não há endpoint de escrita enquanto
o produto não tiver autenticação/RBAC (planejados para a Sprint 20). Um
operador do banco interno pode alterar a política com SQL controlado:

```sql
INSERT INTO rule_policy(environment_id,schema_name,rule_id,enabled,parameters)
VALUES ('<environment-id>', 'public', 'model.wide_table', true,
        '{"min_columns":70}'::jsonb)
ON CONFLICT(environment_id,schema_name,rule_id)
DO UPDATE SET enabled=EXCLUDED.enabled,
              parameters=EXCLUDED.parameters,updated_at=now();
```

Somente chaves do catálogo são aplicadas. Os parâmetros de contagem, tamanho,
idade e razão usam números positivos. Para `model.type_review` existem
`check_money`, `check_timestamp_without_timezone` e `check_status_text`.
`model.naming_inconsistent` aceita `snake_case`, `lowercase` ou `none`.

## Interpretação e limites

- Integridade: ausência de PK, FK sem índice de suporte, constraint não
  validada, tipo de FK divergente, sequence sem ownership e default apontando
  para sequence de outra coluna são sinais para
  investigação. PK em partições é ignorada; índice parcial ou inválido não
  conta como suporte universal de FK. Nunca se cria FK/PK automaticamente.
- Índices: duplicidade estrutural, prefixo, indisponibilidade, excesso sob
  escrita e carga de leitura sem índice geram hipóteses. Predicados,
  unicidade, `INCLUDE`, expressão e plano real podem invalidar a hipótese.
  Nenhuma regra recomenda `DROP` automático.
- Modelagem: largura, colunas numeradas opcionais, entidades semelhantes
  entre schemas, relacionamento sugerido por nome, tipos, JSONB e convenção
  de nomes têm confiança inferior às regras determinísticas. Nomes e
  estatísticas não revelam intenção de negócio.
- Manutenção: dead tuples e bloat são **estimativas**; `last_analyze` pode
  estar ausente ou desatualizado. Confirmar com métricas de autovacuum e
  diagnóstico autorizado antes de mudanças.
- Workload: a correlação com tabela exige uma referência qualificada única.
  Fingerprints não armazenam texto SQL nem literais. Contadores cobrem a
  janela desde `stats_reset`; reset, baixa amostra e consultas complexas
  reduzem confiança.

Para validar um finding, conferir o run e cobertura, revisar evidências,
comparar planos e métricas durante uma janela suficiente e testar qualquer
mudança em ambiente controlado. O auditor não executa DDL nem exclusões.
