package quality

type Plan struct {
	Meaning       string `json:"meaning"`
	Confirmation  string `json:"confirmation"`
	ExternalSteps string `json:"external_steps"`
	Validation    string `json:"validation"`
	Risk          string `json:"risk"`
}

func PlanFor(kind string) Plan {
	common := Plan{Risk: "Qualquer correção exige aprovação de negócio, backup, janela, teste e rollback fora do auditor.", Validation: "Repita o diagnóstico com limite e condições comparáveis; registre contagens pré/pós e evidência externa."}
	switch kind {
	case "null":
		common.Meaning = "A amostra contém valores ausentes em uma coluna indicada como obrigatória."
		common.Confirmation = "Confirme a regra de negócio e a proporção na população antes de mudar a coluna."
		common.ExternalSteps = "Planeje correção por lotes, trate a origem dos nulos e só depois avalie NOT NULL."
	case "duplicate":
		common.Meaning = "A chave candidata escolhida repete valores na amostra."
		common.Confirmation = "Confirme se a chave representa identidade de negócio e verifique dependências e concorrência."
		common.ExternalSteps = "Planeje deduplicação com regra de sobrevivência aprovada e valide unicidade antes de uma constraint."
	case "orphan":
		common.Meaning = "A amostra contém referências sem linha correspondente no pai."
		common.Confirmation = "Confirme o relacionamento, estado da constraint e regras de exclusão."
		common.ExternalSteps = "Planeje reconciliação ou reparo dos vínculos; valide a constraint após a correção externa."
	case "date_range":
		common.Meaning = "A amostra contém datas fora do intervalo informado."
		common.Confirmation = "Confirme fuso, período válido e exceções históricas com a área responsável."
		common.ExternalSteps = "Registre contagem total e dependências. Se houver política de retenção aprovada, planeje cópia para arquivo com verificação de integridade e teste de restauração antes de qualquer exclusão externa; se não houver, apenas investigue a origem e corrija datas por lotes."
	default:
		common.Meaning = "Uma distribuição concentrada foi observada na amostra."
		common.Confirmation = "Confirme se a concentração é esperada no período e na população."
		common.ExternalSteps = "Revise regras de entrada e classificação; só planeje alteração após confirmação do domínio."
	}
	return common
}
