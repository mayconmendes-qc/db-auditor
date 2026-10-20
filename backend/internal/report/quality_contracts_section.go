package report

import "fmt"

func emitQualityContracts(d Document, out func(...Line) error) error {
	if d.Type != "technical" || d.QualityContracts == nil {
		return nil
	}
	if err := out(Line{"Contratos de qualidade", "heading"}); err != nil {
		return err
	}
	qc := d.QualityContracts
	if len(qc.Items) == 0 {
		return out(Line{"Nenhuma execução de contrato registrada para este ambiente.", "body"})
	}
	if qc.SamplingNote != "" {
		if err := out(Line{qc.SamplingNote, "body"}); err != nil {
			return err
		}
	}
	for _, item := range qc.Items {
		sample := "—"
		if item.SampleSize != nil {
			sample = fmt.Sprintf("%d", *item.SampleSize)
		}
		line := fmt.Sprintf("%s · %s · %s · observado %s · amostra %s · %s",
			item.CheckKey, item.Result, item.Severity, item.Observed, sample, item.Reason)
		if err := out(Line{line, "body"}); err != nil {
			return err
		}
	}
	return nil
}
