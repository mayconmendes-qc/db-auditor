package report

// RedactSensitive removes fields from the rendered copy only. Source snapshots,
// job provenance, counts and artifact hash remain in the control database.
func (d *Document) RedactSensitive(level string) {
	if level != "identifiers" && level != "strict" {
		d.RedactionLevel = "none"
		return
	}
	d.RedactionLevel = level
	mask := "[oculto]"
	d.Environment, d.RequestedBy = mask, mask
	d.DatabaseFilter, d.SchemaFilter, d.TableFilter = mask, mask, mask
	for i := range d.Databases {
		d.Databases[i].Name = mask
	}
	for i := range d.Tables {
		d.Tables[i].Database, d.Tables[i].Schema, d.Tables[i].Name = mask, mask, mask
	}
	for i := range d.Findings {
		d.Findings[i].Database, d.Findings[i].Schema, d.Findings[i].Object = mask, mask, mask
		d.Findings[i].Evidence = "[evidência omitida]"
		d.Findings[i].Title = mask
		d.Findings[i].Summary = "[resumo omitido]"
		d.Findings[i].Recommendation = "[texto original omitido]"
	}
	for i := range d.CoverageNotes {
		d.CoverageNotes[i] = "Detalhes de cobertura omitidos neste nível de redação."
	}
}
