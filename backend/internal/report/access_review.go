package report

import "fmt"

// accessReviewLines renders the evidence section for report_type=access_review.
func accessReviewLines(d Document) []Line {
	ar := d.AccessReview
	if ar == nil {
		return []Line{{"Revisão de acessos", "heading"}, {"Nenhuma campanha vinculada a este relatório.", "body"}}
	}
	lines := []Line{
		{"Revisão de acessos", "heading"},
		{fmt.Sprintf("Campanha: %s (%s)", ar.CampaignName, ar.CampaignID), "body"},
		{fmt.Sprintf("Estado: %s | Prazo: %s", ar.Status, ar.DueAt), "body"},
		{fmt.Sprintf("Progresso: %d/%d decididos (%.1f%%). Pendências: %d.", ar.DecidedItems, ar.TotalItems, ar.DecidedPercent, ar.PendingItems), "body"},
		{"Responsáveis", "heading"},
	}
	if len(ar.Owners) == 0 {
		lines = append(lines, Line{"Nenhum responsável mapeado; linhas vão para o operator.", "body"})
	}
	for _, o := range ar.Owners {
		contact := o.Contact
		if contact != "" {
			contact = " (" + contact + ")"
		}
		lines = append(lines, Line{fmt.Sprintf("%s → %s%s", o.TargetRole, o.OwnerName, contact), "body"})
	}
	lines = append(lines, Line{"Pendências (amostra)", "heading"})
	if len(ar.PendingSamples) == 0 {
		lines = append(lines, Line{"Nenhuma pendência na amostra.", "body"})
	}
	for _, p := range ar.PendingSamples {
		lines = append(lines, Line{fmt.Sprintf("%s | %s | %s | %s", p.Account, p.Object, p.Privilege, p.Origin), "body"})
	}
	return lines
}
