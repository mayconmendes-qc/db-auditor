package report

import "fmt"

// RestoreVerifiedSummary feeds the "Restauração verificada" PDF section (RF-07).
type RestoreVerifiedSummary struct {
	HasDrill         bool
	Succeeded        bool
	PerformedAt      string
	RTOSeconds       *int
	RTOTargetSeconds int
	MaxAgeDays       int
	DivergenceCount  int
	ChecksOK         int
	ChecksTotal      int
	Note             string
	Overdue          bool
}

func emitRestoreVerified(d Document, out func(...Line) error) error {
	if d.Type != "technical" && d.Type != "executive" {
		return nil
	}
	rv := d.RestoreVerified
	if rv == nil {
		return nil
	}
	if err := out(Line{"Restauração verificada", "heading"}); err != nil {
		return err
	}
	if !rv.HasDrill {
		msg := "Nenhum ensaio target_restore_drill registrado para este ambiente."
		if rv.Overdue {
			msg += fmt.Sprintf(" Prazo máximo entre ensaios: %d dias.", rv.MaxAgeDays)
		}
		return out(Line{msg, "body"})
	}
	status := "falhou"
	if rv.Succeeded {
		status = "sucesso"
	}
	rto := "n/d"
	if rv.RTOSeconds != nil {
		rto = fmt.Sprintf("%d s", *rv.RTOSeconds)
		if rv.RTOTargetSeconds > 0 {
			rto = fmt.Sprintf("%d s (alvo %d s)", *rv.RTOSeconds, rv.RTOTargetSeconds)
		}
	}
	lines := []Line{
		{fmt.Sprintf("Último ensaio: %s em %s · RTO %s", status, rv.PerformedAt, rto), "body"},
		{fmt.Sprintf("Verificações: %d/%d ok · divergências: %d", rv.ChecksOK, rv.ChecksTotal, rv.DivergenceCount), "body"},
	}
	if rv.Overdue {
		lines = append(lines, Line{"Ensaio em atraso em relação ao prazo do ambiente.", "body"})
	}
	if d.RedactionLevel != "strict" && rv.Note != "" {
		lines = append(lines, Line{"Nota: " + rv.Note, "body"})
	}
	return out(lines...)
}
