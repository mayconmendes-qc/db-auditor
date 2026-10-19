package costlens

import "fmt"

// StorageItem is one relation (table or index) sized for storage cost.
type StorageItem struct {
	Kind        string // table | index
	Database    string
	Schema      string
	Name        string
	ObjectKey   string
	SizeBytes   int64
	FindingID   string
	FindingType string
}

// CostResult is one priced (or not) monthly estimate with AN-03 transparency.
type CostResult struct {
	Type           string         `json:"type"` // storage | unused_index | bloat | query
	ObjectKey      string         `json:"object_key"`
	Database       string         `json:"database,omitempty"`
	Schema         string         `json:"schema,omitempty"`
	Object         string         `json:"object,omitempty"`
	SizeBytes      int64          `json:"size_bytes,omitempty"`
	MonthAmount    *float64       `json:"month_amount,omitempty"`
	Currency       string         `json:"currency,omitempty"`
	Priced         bool           `json:"priced"`
	Reason         string         `json:"reason,omitempty"`
	Formula        string         `json:"formula,omitempty"`
	Confidence     string         `json:"confidence"`
	PriceVersion   int            `json:"price_version,omitempty"`
	PriceEffective string         `json:"price_effective,omitempty"`
	FindingID      string         `json:"finding_id,omitempty"`
	FindingType    string         `json:"finding_type,omitempty"`
	NextStep       string         `json:"next_step,omitempty"`
	Label          string         `json:"label"`
	ImpactBasis    map[string]any `json:"impact_basis,omitempty"`
	ShareOfTime    float64        `json:"share_of_time,omitempty"`
	DeltaCalls     int64          `json:"delta_calls,omitempty"`
	Fingerprint    string         `json:"fingerprint,omitempty"`
}

// EstimateStorage prices one object's size against the storage GB-month rate.
func EstimateStorage(item StorageItem, price *PriceVersion) CostResult {
	r := CostResult{
		Type:        "storage",
		ObjectKey:   item.ObjectKey,
		Database:    item.Database,
		Schema:      item.Schema,
		Object:      item.Name,
		SizeBytes:   item.SizeBytes,
		FindingID:   item.FindingID,
		FindingType: item.FindingType,
		Confidence:  "medium",
		Label:       ConfidenceEstimate,
		NextStep:    "Revise retenção e crescimento; confirme a fatura real antes de agir.",
	}
	r.ImpactBasis = map[string]any{
		"size_bytes": item.SizeBytes,
		"period":     "mês",
		"metric":     "storage_bytes",
	}
	if price == nil {
		r.Reason = ReasonNoPrice
		r.Label = ReasonNoPrice
		r.Confidence = "none"
		return r
	}
	amount := StorageMonthlyCost(item.SizeBytes, price.StorageGBMonth)
	r.Priced = true
	r.MonthAmount = &amount
	r.Currency = price.Currency
	r.PriceVersion = price.Version
	r.PriceEffective = price.EffectiveFrom.UTC().Format("2006-01-02")
	r.Formula = fmt.Sprintf("%.3f GB × %s %.4f/GB-mês = %s",
		BytesToGB(item.SizeBytes), price.Currency, price.StorageGBMonth, MoneyLabel(amount, price.Currency))
	r.Label = MoneyLabel(amount, price.Currency)
	r.ImpactBasis["storage_gb_month"] = price.StorageGBMonth
	r.ImpactBasis["price_version"] = price.Version
	r.ImpactBasis["price_effective_from"] = r.PriceEffective
	r.ImpactBasis["currency"] = price.Currency
	r.ImpactBasis["formula"] = r.Formula
	r.ImpactBasis["month_amount"] = amount
	return r
}

// EstimateUnusedIndexWaste prices an open unused index (RF-03 / CA-01).
func EstimateUnusedIndexWaste(sizeBytes int64, objectKey, database, schema, name, findingID string, price *PriceVersion) CostResult {
	item := StorageItem{
		Kind: "index", Database: database, Schema: schema, Name: name,
		ObjectKey: objectKey, SizeBytes: sizeBytes, FindingID: findingID, FindingType: "cost.unused_index_waste",
	}
	r := EstimateStorage(item, price)
	r.Type = "unused_index"
	r.FindingType = "cost.unused_index_waste"
	r.NextStep = "Confirme réplicas e consultas raras antes de considerar remoção. Nunca DROP automático."
	if price != nil {
		r.Formula = fmt.Sprintf("índice sem uso: %.3f GB × %s %.4f/GB-mês = %s",
			BytesToGB(sizeBytes), price.Currency, price.StorageGBMonth, MoneyLabel(*r.MonthAmount, price.Currency))
		r.ImpactBasis["formula"] = r.Formula
	}
	return r
}
