package report

import "fmt"

func emitRetentionSimulation(d Document, out func(...Line) error) error {
	if d.Type != "technical" && d.RetentionSimulation == nil {
		return nil
	}
	if d.RetentionSimulation == nil {
		return nil
	}
	if err := out(Line{"Retenção e compressão (simulação)", "heading"}); err != nil {
		return err
	}
	rs := d.RetentionSimulation
	if !rs.Applicable {
		reason := rs.Reason
		if reason == "" {
			reason = "não aplicável"
		}
		return out(Line{reason, "body"},
			Line{"Hipótese: a simulação não aplica políticas; confirme compliance e backup antes de qualquer retenção (BA-21).", "body"})
	}
	if err := out(Line{"Hipótese: cálculo sobre metadados de chunks. Ganho previsto abaixo; ganho medido só após política aplicada fora do auditor e nova coleta completa comparável (BA-21).", "body"}); err != nil {
		return err
	}
	if rs.Partial {
		if err := out(Line{"Resultado parcial: inventário de chunks truncado.", "body"}); err != nil {
			return err
		}
	}
	if len(rs.Items) == 0 {
		return out(Line{"Nenhum cenário salvo para esta execução.", "body"})
	}
	for _, item := range rs.Items {
		money := ""
		if item.MoneySaved != nil && item.Currency != "" {
			money = fmt.Sprintf(" | %s %.2f/mês", item.Currency, *item.MoneySaved)
		}
		risks := ""
		if len(item.Risks) > 0 {
			risks = " | riscos: " + fmt.Sprintf("%v", item.Risks)
		}
		line := fmt.Sprintf("%s · %s · economia prevista %.2f GB%s · confiança %s%s",
			item.Hypertable, item.Label, item.GBSaved, money, item.Confidence, risks)
		if err := out(Line{line, "body"}); err != nil {
			return err
		}
	}
	return out(Line{"Ganho medido: não medido nesta simulação.", "body"})
}
