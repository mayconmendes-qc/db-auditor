const guidance: Record<string, { meaning: string; next: string }> = {
  "integrity.missing_primary_key": {
    meaning: "Esta tabela não tem uma chave primária identificada na coleta.",
    next: "Confirme como os registros são identificados e valide os dados antes de considerar uma chave primária.",
  },
  "integrity.fk_without_index": {
    meaning:
      "Uma relação entre tabelas pode estar sem um índice útil para as consultas.",
    next: "Confira as consultas e seus planos de execução antes de avaliar um novo índice.",
  },
  "index.unused": {
    meaning: "Este índice não teve uso observado no período disponível.",
    next: "Confira o período das estatísticas e as consultas antes de decidir se o índice ainda é necessário.",
  },
  "security.auditor_not_readonly": {
    meaning:
      "A conta usada para auditar o banco possui permissão de escrita ou privilégios elevados.",
    next: "Peça ao administrador que revise as permissões e use uma conta de somente leitura para a coleta.",
  },
  "security.auditor_privilege_unknown": {
    meaning: "Não foi possível confirmar as permissões da conta de auditoria.",
    next: "Repita a verificação com acesso ao catálogo de permissões. Não presuma que a conta é de somente leitura.",
  },
};

const byCategory: Record<string, { meaning: string; next: string }> = {
  integrity: {
    meaning: "Uma regra de integridade dos dados merece revisão.",
    next: "Confira os registros e os relacionamentos afetados antes de alterar a estrutura do banco.",
  },
  security: {
    meaning:
      "Foi encontrado um ponto de atenção nas permissões ou na execução de código do banco.",
    next: "Revise as evidências com o administrador e confirme a necessidade de cada privilégio antes de alterar acessos.",
  },
  performance: {
    meaning:
      "Uma métrica sugere possível impacto no tempo das consultas ou gravações.",
    next: "Compare as métricas e os planos de consultas representativas antes de ajustar o banco.",
  },
  index: {
    meaning: "O uso ou a estrutura de um índice merece investigação.",
    next: "Verifique as consultas, a frequência de uso e o custo de escrita antes de criar ou remover índices.",
  },
  model: {
    meaning: "A estrutura dos dados apresenta uma hipótese de melhoria.",
    next: "Converse com a equipe responsável para entender o uso dos dados antes de mudar o modelo.",
  },
  storage: {
    meaning: "O espaço ocupado por este objeto merece acompanhamento.",
    next: "Confira o crescimento e a política de retenção antes de ajustar armazenamento ou compressão.",
  },
  maintenance: {
    meaning: "Uma tabela pode precisar de atenção na manutenção automática.",
    next: "Confirme as estatísticas e a atividade da tabela antes de mudar a configuração de manutenção.",
  },
  vacuum: {
    meaning: "Há sinal de acúmulo de registros antigos na tabela.",
    next: "Verifique a manutenção automática e o volume de gravações antes de planejar uma intervenção.",
  },
  policy: {
    meaning: "Uma política de manutenção ou retenção merece revisão.",
    next: "Confira a configuração, a última execução e a necessidade de negócio antes de alterá-la.",
  },
  replication: {
    meaning: "Há sinal de atraso ou falha na replicação ou no arquivamento.",
    next: "Verifique as métricas e os logs da réplica antes de agir.",
  },
  config: {
    meaning: "A configuração deste ambiente difere da referência comparada.",
    next: "Confirme se a diferença é intencional antes de alterar parâmetros.",
  },
};

export function findingGuidance(type: string) {
  return (
    guidance[type] ??
    byCategory[type.split(".")[0]] ?? {
      meaning: "A auditoria encontrou um sinal que precisa de análise humana.",
      next: "Confira as evidências e valide o impacto com a equipe responsável antes de fazer alterações.",
    }
  );
}
