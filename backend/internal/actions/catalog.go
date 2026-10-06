// Package actions turns a diagnostic into a reviewable, read-only action plan.
// The auditor never executes these steps against a target database.
package actions

import "strings"

type Plan struct {
	Category      string `json:"category"`
	Benefit       string `json:"expected_benefit"`
	Risk          string `json:"risk"`
	Prerequisites string `json:"prerequisites"`
	Confirmation  string `json:"confirmation"`
	Validation    string `json:"validation"`
	FalsePositive string `json:"false_positive_risk"`
	ReadOnlyQuery string `json:"read_only_query,omitempty"`
}

func For(rule string) Plan {
	p := Plan{
		Category:      "investigation",
		Benefit:       "Impacto ainda não estimado; meça a condição antes de planejar uma alteração.",
		Risk:          "Uma mudança sem conhecer dependências e carga pode afetar a aplicação.",
		Prerequisites: "Confirme a cobertura da coleta, dependências e janela de manutenção.",
		Confirmation:  "Compare o achado com uma nova coleta e com a equipe responsável pelo objeto.",
		Validation:    "Registre as métricas anteriores e posteriores e repita a auditoria.",
		FalsePositive: "Uma observação isolada ou uma coleta parcial pode não representar o uso normal.",
	}
	switch {
	case strings.Contains(rule, "primary_key"), strings.Contains(rule, "constraint"), strings.Contains(rule, "invalid_index"), rule == "index.invalid", strings.HasPrefix(rule, "model."), strings.HasPrefix(rule, "inactivity."):
		p.Category = "structure"
		p.Confirmation = "Inspecione as dependências e o catálogo do objeto; confirme com o dono da aplicação antes de alterar chaves, constraints ou índices."
		p.Validation = "Após correção externa, confira o catálogo e compare uma nova coleta com a anterior."
		p.FalsePositive = "Ausência de uso observado não comprova que um objeto está órfão ou pode ser removido."
	case strings.HasPrefix(rule, "index."), strings.HasPrefix(rule, "performance."):
		p.Category = "query_and_index"
		p.Prerequisites = "Confira estatísticas, taxa de escrita, espaço e plano da consulta em ambiente seguro."
		p.Confirmation = "Compare EXPLAIN (sem executar alterações) e o fingerprint da consulta; confirme cobertura e período das estatísticas."
		p.Validation = "Meça latência, leituras, escrita e espaço antes/depois sob carga comparável; documente rollback."
		p.Risk = "Criar ou remover índice pode aumentar escrita, ocupar espaço ou causar bloqueios; planeje execução concorrente quando aplicável."
		p.FalsePositive = "Contadores zerados após reset ou uma janela curta não provam inutilidade de índice."
	case strings.HasPrefix(rule, "vacuum."), strings.HasPrefix(rule, "storage."), strings.HasPrefix(rule, "policy."), strings.HasPrefix(rule, "job."), strings.HasPrefix(rule, "chunk."), strings.HasPrefix(rule, "cagg."):
		p.Category = "maintenance"
		p.Prerequisites = "Confirme tendência, volume, custo operacional, SLA e janela aprovada. Retenção depende de decisão de negócio."
		p.Confirmation = "Compare duas ou mais coletas completas e verifique a configuração e as estatísticas do objeto."
		p.Validation = "Após ação externa, compare tamanho, atraso e saúde dos jobs em novas coletas."
		p.Risk = "Manutenção e retenção podem consumir recursos ou remover dados; exija backup e plano de recuperação."
		p.FalsePositive = "Um único pico ou coleta parcial não estabelece tendência de crescimento."
	case strings.HasPrefix(rule, "security."), strings.HasPrefix(rule, "privilege."), strings.HasPrefix(rule, "rls."), strings.HasPrefix(rule, "config."):
		p.Category = "security"
		p.Prerequisites = "Confirme o papel da aplicação, a permissão efetiva e a política de acesso antes de revogar privilégios."
		p.Confirmation = "Revise grants, políticas e dependências com o responsável pela aplicação."
		p.Validation = "Teste o fluxo autorizado e repita a coleta para verificar a permissão efetiva."
		p.Risk = "Uma revogação pode interromper um serviço legítimo; mudanças devem ter rollback externo."
		p.FalsePositive = "Permissão aparentemente ampla pode ser necessária a uma função operacional conhecida."
	}
	switch rule {
	case "model.no_primary_key", "structure.no_primary_key":
		p.ReadOnlyQuery = "SELECT conname, contype, convalidated FROM pg_catalog.pg_constraint WHERE conrelid = $1::regclass AND contype = 'p';"
	case "index.invalid":
		p.ReadOnlyQuery = "SELECT indexrelid::regclass, indisvalid, indisready FROM pg_catalog.pg_index WHERE indexrelid = $1::regclass;"
	}
	return p
}
