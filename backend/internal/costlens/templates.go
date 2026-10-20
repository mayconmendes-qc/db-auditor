package costlens

// PriceTemplate is an editable starter model (no online price fetch).
type PriceTemplate struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	Currency        string  `json:"currency"`
	StorageGBMonth  float64 `json:"storage_gb_month"`
	IOPerMillion    float64 `json:"io_per_million"`
	IOPerGBRead     float64 `json:"io_per_gb_read"`
	ComputeVCPUHour float64 `json:"compute_vcpu_hour"`
	InstanceMonth   float64 `json:"instance_month"`
	ModelOrigin     string  `json:"model_origin"`
	Notes           string  `json:"notes"`
}

// Templates returns the v1 starter models (ADR-0001 D3 / P-03).
// Values are illustrative public list prices, always labeled estimated.
func Templates() []PriceTemplate {
	return []PriceTemplate{
		{
			ID: "rds_postgresql", Name: "RDS PostgreSQL", Currency: CurrencyUSD,
			StorageGBMonth: 0.115, IOPerMillion: 0.20, IOPerGBRead: 0.10,
			ComputeVCPUHour: 0.041, InstanceMonth: 60,
			ModelOrigin: "rds_postgresql",
			Notes:       "Preset editável (lista pública ilustrativa). Não é a fatura real.",
		},
		{
			ID: "cloud_sql", Name: "Cloud SQL", Currency: CurrencyUSD,
			StorageGBMonth: 0.17, IOPerMillion: 0.0, IOPerGBRead: 0.12,
			ComputeVCPUHour: 0.0413, InstanceMonth: 70,
			ModelOrigin: "cloud_sql",
			Notes:       "Preset editável (lista pública ilustrativa). Não é a fatura real.",
		},
		{
			ID: "self_hosted", Name: "Self-hosted", Currency: CurrencyBRL,
			StorageGBMonth: 0.50, IOPerMillion: 0.0, IOPerGBRead: 0.05,
			ComputeVCPUHour: 0.08, InstanceMonth: 400,
			ModelOrigin: "self_hosted",
			Notes:       "Preset editável para VPS/bare metal. Ajuste à sua fatura.",
		},
	}
}
