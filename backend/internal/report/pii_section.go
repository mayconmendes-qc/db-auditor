package report

import "fmt"

func emitPersonalDataSection(d Document, out func(...Line) error) error {
	if d.PersonalData == nil {
		return nil
	}
	pd := d.PersonalData
	if err := out(Line{"Dados pessoais", "heading"}); err != nil {
		return err
	}
	if err := out(Line{fmt.Sprintf("Colunas classificadas: %d (sem valores amostrados).", pd.TotalColumns), "body"}); err != nil {
		return err
	}
	if pd.JSONNote != "" {
		if err := out(Line{pd.JSONNote, "body"}); err != nil {
			return err
		}
	}
	if err := out(Line{"Totais por categoria", "subheading"}); err != nil {
		return err
	}
	for _, c := range pd.ByCategory {
		if err := out(Line{fmt.Sprintf("%s: %d", c.Category, c.Count), "body"}); err != nil {
			return err
		}
	}
	if len(pd.DetailColumns) > 0 {
		if err := out(Line{"Detalhe por coluna", "subheading"}); err != nil {
			return err
		}
		for _, col := range pd.DetailColumns {
			rls := "sem RLS"
			if col.HasRLS {
				rls = "com RLS"
			}
			if err := out(Line{fmt.Sprintf("%s · %s · %s · %s · %d contas · %s",
				col.Qualified, col.Category, col.Confidence, col.Method, col.ReadCount, rls), "body"}); err != nil {
				return err
			}
		}
	}
	return nil
}
