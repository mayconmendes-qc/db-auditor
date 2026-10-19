package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/osmendes/db-auditor/internal/costlens"
)

// CostPriceVersion is the persisted price table version.
type CostPriceVersion struct {
	ID              string    `json:"id"`
	EnvironmentID   string    `json:"environment_id"`
	Version         int       `json:"version"`
	Currency        string    `json:"currency"`
	StorageGBMonth  float64   `json:"storage_gb_month"`
	IOPerMillion    *float64  `json:"io_per_million,omitempty"`
	IOPerGBRead     *float64  `json:"io_per_gb_read,omitempty"`
	ComputeVCPUHour *float64  `json:"compute_vcpu_hour,omitempty"`
	InstanceMonth   *float64  `json:"instance_month,omitempty"`
	ModelOrigin     string    `json:"model_origin"`
	EffectiveFrom   time.Time `json:"effective_from"`
	CreatedBy       string    `json:"created_by"`
	CreatedAt       time.Time `json:"created_at"`
}

// CostPricesResponse is GET /cost/prices.
type CostPricesResponse struct {
	Current  *CostPriceVersion  `json:"current"`
	History  []CostPriceVersion `json:"history"`
	Currency string             `json:"currency,omitempty"` // locked currency when any version exists
	Locked   bool               `json:"currency_locked"`
}

func scanCostPrice(row pgx.Row) (CostPriceVersion, error) {
	var v CostPriceVersion
	err := row.Scan(
		&v.ID, &v.EnvironmentID, &v.Version, &v.Currency, &v.StorageGBMonth,
		&v.IOPerMillion, &v.IOPerGBRead, &v.ComputeVCPUHour, &v.InstanceMonth,
		&v.ModelOrigin, &v.EffectiveFrom, &v.CreatedBy, &v.CreatedAt,
	)
	return v, err
}

const costPriceCols = `id, environment_id, version, currency, storage_gb_month,
 io_per_million, io_per_gb_read, compute_vcpu_hour, instance_month,
 model_origin, effective_from, created_by, created_at`

// ListCostPrices returns current + history for an environment.
func (s *Store) ListCostPrices(ctx context.Context, environmentID string) (*CostPricesResponse, error) {
	rows, err := s.pool.Query(ctx, `
SELECT `+costPriceCols+`
FROM cost_price_version
WHERE environment_id=$1::uuid
ORDER BY version DESC`, environmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := &CostPricesResponse{History: []CostPriceVersion{}}
	for rows.Next() {
		v, err := scanCostPrice(rows)
		if err != nil {
			return nil, err
		}
		out.History = append(out.History, v)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out.History) > 0 {
		cur := out.History[0]
		out.Current = &cur
		// Locked currency is from the first (lowest) version.
		first := out.History[len(out.History)-1]
		out.Currency = first.Currency
		out.Locked = true
	}
	return out, nil
}

// CurrentCostPrice returns the latest version or nil.
func (s *Store) CurrentCostPrice(ctx context.Context, environmentID string) (*CostPriceVersion, error) {
	v, err := scanCostPrice(s.pool.QueryRow(ctx, `
SELECT `+costPriceCols+`
FROM cost_price_version
WHERE environment_id=$1::uuid
ORDER BY version DESC
LIMIT 1`, environmentID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// LockedCostCurrency returns the currency of version 1, if any.
func (s *Store) LockedCostCurrency(ctx context.Context, environmentID string) (string, error) {
	var cur string
	err := s.pool.QueryRow(ctx, `
SELECT currency FROM cost_price_version
WHERE environment_id=$1::uuid
ORDER BY version ASC LIMIT 1`, environmentID).Scan(&cur)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return cur, err
}

// CreateCostPriceVersion inserts a new immutable version (operator).
func (s *Store) CreateCostPriceVersion(ctx context.Context, environmentID, actor string, in costlens.PriceInput) (*CostPriceVersion, error) {
	locked, err := s.LockedCostCurrency(ctx, environmentID)
	if err != nil {
		return nil, err
	}
	if err := costlens.ValidatePriceInput(in, locked); err != nil {
		return nil, err
	}
	cur := costlens.NormalizeCurrency(in.Currency)
	origin := strings.TrimSpace(in.ModelOrigin)
	if origin == "" {
		origin = "custom"
	}
	var next int
	if err := s.pool.QueryRow(ctx, `
SELECT COALESCE(MAX(version), 0) + 1 FROM cost_price_version WHERE environment_id=$1::uuid`, environmentID).Scan(&next); err != nil {
		return nil, err
	}
	v, err := scanCostPrice(s.pool.QueryRow(ctx, `
INSERT INTO cost_price_version (
  environment_id, version, currency, storage_gb_month,
  io_per_million, io_per_gb_read, compute_vcpu_hour, instance_month,
  model_origin, effective_from, created_by)
VALUES ($1::uuid,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
RETURNING `+costPriceCols, environmentID, next, cur, in.StorageGBMonth,
		in.IOPerMillion, in.IOPerGBRead, in.ComputeVCPUHour, in.InstanceMonth,
		origin, in.EffectiveFrom.UTC(), actor))
	if err != nil {
		if strings.Contains(err.Error(), "currency locked") {
			return nil, fmt.Errorf("moeda travada em %s para este ambiente", locked)
		}
		return nil, err
	}
	return &v, nil
}

// ToCostlensPrice maps a repository row to the costlens model.
func ToCostlensPrice(v *CostPriceVersion) *costlens.PriceVersion {
	if v == nil {
		return nil
	}
	return &costlens.PriceVersion{
		ID: v.ID, EnvironmentID: v.EnvironmentID, Version: v.Version, Currency: v.Currency,
		StorageGBMonth: v.StorageGBMonth, IOPerMillion: v.IOPerMillion, IOPerGBRead: v.IOPerGBRead,
		ComputeVCPUHour: v.ComputeVCPUHour, InstanceMonth: v.InstanceMonth,
		ModelOrigin: v.ModelOrigin, EffectiveFrom: v.EffectiveFrom, CreatedBy: v.CreatedBy, CreatedAt: v.CreatedAt,
	}
}
