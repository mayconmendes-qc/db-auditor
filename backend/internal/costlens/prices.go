// Package costlens turns snapshot sizes, unused indexes, estimated bloat and
// workload deltas into monthly cost estimates (spec 02, ADR-0001 D3/D11).
package costlens

import (
	"fmt"
	"math"
	"strings"
	"time"
)

const (
	CurrencyBRL = "BRL"
	CurrencyUSD = "USD"

	ReasonNoPrice       = "não estimado — preço não configurado"
	ReasonInvalidWindow = "janela inválida"
	LabelEstimatedMonth = "estimado/mês"
	ConfidenceEstimate  = "estimativa"
)

// PriceVersion is one immutable price table version for an environment.
type PriceVersion struct {
	ID              string
	EnvironmentID   string
	Version         int
	Currency        string
	StorageGBMonth  float64
	IOPerMillion    *float64
	IOPerGBRead     *float64
	ComputeVCPUHour *float64
	InstanceMonth   *float64
	ModelOrigin     string
	EffectiveFrom   time.Time
	CreatedBy       string
	CreatedAt       time.Time
}

// PriceInput is the operator write model for a new version.
type PriceInput struct {
	Currency        string
	StorageGBMonth  float64
	IOPerMillion    *float64
	IOPerGBRead     *float64
	ComputeVCPUHour *float64
	InstanceMonth   *float64
	ModelOrigin     string
	EffectiveFrom   time.Time
}

// ValidateCurrency accepts only BRL or USD.
func ValidateCurrency(c string) error {
	switch strings.ToUpper(strings.TrimSpace(c)) {
	case CurrencyBRL, CurrencyUSD:
		return nil
	default:
		return fmt.Errorf("moeda inválida: use BRL ou USD")
	}
}

// ValidatePriceInput checks non-negative amounts and currency (ADR-0001 D3).
func ValidatePriceInput(in PriceInput, lockedCurrency string) error {
	if err := ValidateCurrency(in.Currency); err != nil {
		return err
	}
	cur := strings.ToUpper(strings.TrimSpace(in.Currency))
	if lockedCurrency != "" && cur != lockedCurrency {
		return fmt.Errorf("moeda travada em %s para este ambiente", lockedCurrency)
	}
	if in.StorageGBMonth < 0 || math.IsNaN(in.StorageGBMonth) || math.IsInf(in.StorageGBMonth, 0) {
		return fmt.Errorf("preço de armazenamento não pode ser negativo")
	}
	for _, p := range []*float64{in.IOPerMillion, in.IOPerGBRead, in.ComputeVCPUHour, in.InstanceMonth} {
		if p != nil && (*p < 0 || math.IsNaN(*p) || math.IsInf(*p, 0)) {
			return fmt.Errorf("preço não pode ser negativo")
		}
	}
	if in.EffectiveFrom.IsZero() {
		return fmt.Errorf("vigência obrigatória")
	}
	switch strings.TrimSpace(in.ModelOrigin) {
	case "", "rds_postgresql", "cloud_sql", "self_hosted", "custom":
	default:
		return fmt.Errorf("modelo de origem inválido")
	}
	return nil
}

// NormalizeCurrency uppercases a validated currency.
func NormalizeCurrency(c string) string {
	return strings.ToUpper(strings.TrimSpace(c))
}

// RoundMoney rounds to 2 decimal places (currency).
func RoundMoney(v float64) float64 {
	return math.Round(v*100) / 100
}

// BytesToGB converts bytes to gigabytes (decimal GB for pricing).
func BytesToGB(bytes int64) float64 {
	if bytes <= 0 {
		return 0
	}
	return float64(bytes) / 1e9
}

// StorageMonthlyCost is size × storage price per GB-month.
func StorageMonthlyCost(sizeBytes int64, pricePerGBMonth float64) float64 {
	return RoundMoney(BytesToGB(sizeBytes) * pricePerGBMonth)
}

// MoneyLabel formats a monetary value with currency and the estimated/month label.
func MoneyLabel(amount float64, currency string) string {
	cur := NormalizeCurrency(currency)
	if cur == "" {
		cur = CurrencyBRL
	}
	return fmt.Sprintf("%s %.2f/%s", cur, RoundMoney(amount), LabelEstimatedMonth)
}
