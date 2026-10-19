package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/osmendes/db-auditor/internal/costlens"
	"github.com/osmendes/db-auditor/internal/repository"
)

type costLensStore interface {
	ListCostPrices(context.Context, string) (*repository.CostPricesResponse, error)
	CreateCostPriceVersion(context.Context, string, string, costlens.PriceInput) (*repository.CostPriceVersion, error)
	ComputeCostRanking(context.Context, repository.CostRankingFilter) (*repository.CostRankingResponse, error)
	ListCostSeries(context.Context, string, time.Time, time.Time) ([]costlens.SeriesPoint, error)
	TableStorageCostMap(context.Context, string) (map[string]costlens.CostResult, error)
}

func registerCostLensRoutes(mux *http.ServeMux, store InventoryStore) {
	backend, ok := store.(costLensStore)
	if !ok {
		return
	}
	base := "/api/v1/environments/{id}/cost"
	mux.HandleFunc("GET "+base+"/prices", func(w http.ResponseWriter, r *http.Request) {
		env := r.PathValue("id")
		if !uuidPattern.MatchString(env) {
			writeError(w, http.StatusBadRequest, CodeValidation, "Ambiente inválido.")
			return
		}
		res, err := backend.ListCostPrices(r.Context(), env)
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Falha ao listar preços.")
			return
		}
		writeJSON(w, http.StatusOK, res)
	})
	mux.HandleFunc("PUT "+base+"/prices", func(w http.ResponseWriter, r *http.Request) {
		env := r.PathValue("id")
		var body struct {
			Currency        string   `json:"currency"`
			StorageGBMonth  float64  `json:"storage_gb_month"`
			IOPerMillion    *float64 `json:"io_per_million"`
			IOPerGBRead     *float64 `json:"io_per_gb_read"`
			ComputeVCPUHour *float64 `json:"compute_vcpu_hour"`
			InstanceMonth   *float64 `json:"instance_month"`
			ModelOrigin     string   `json:"model_origin"`
			EffectiveFrom   string   `json:"effective_from"`
		}
		if !uuidPattern.MatchString(env) || json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&body) != nil {
			writeError(w, http.StatusBadRequest, CodeValidation, "Preço inválido.")
			return
		}
		eff, err := time.Parse(time.RFC3339, strings.TrimSpace(body.EffectiveFrom))
		if err != nil {
			writeError(w, http.StatusBadRequest, CodeValidation, "Vigência inválida (RFC3339).")
			return
		}
		actor := "operator"
		if user := requestIdentity(r); user != nil {
			actor = user.Username
		}
		item, err := backend.CreateCostPriceVersion(r.Context(), env, actor, costlens.PriceInput{
			Currency: body.Currency, StorageGBMonth: body.StorageGBMonth,
			IOPerMillion: body.IOPerMillion, IOPerGBRead: body.IOPerGBRead,
			ComputeVCPUHour: body.ComputeVCPUHour, InstanceMonth: body.InstanceMonth,
			ModelOrigin: body.ModelOrigin, EffectiveFrom: eff.UTC(),
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, CodeValidation, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, item)
	})
	mux.HandleFunc("GET "+base+"/templates", func(w http.ResponseWriter, r *http.Request) {
		env := r.PathValue("id")
		if !uuidPattern.MatchString(env) {
			writeError(w, http.StatusBadRequest, CodeValidation, "Ambiente inválido.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": costlens.Templates()})
	})
	mux.HandleFunc("GET "+base+"/ranking", func(w http.ResponseWriter, r *http.Request) {
		env := r.PathValue("id")
		if !uuidPattern.MatchString(env) {
			writeError(w, http.StatusBadRequest, CodeValidation, "Ambiente inválido.")
			return
		}
		f := repository.CostRankingFilter{
			EnvironmentID: env,
			AuditRunID:    strings.TrimSpace(r.URL.Query().Get("run")),
			Database:      strings.TrimSpace(r.URL.Query().Get("database")),
			Schema:        strings.TrimSpace(r.URL.Query().Get("schema")),
			Type:          strings.TrimSpace(r.URL.Query().Get("type")),
		}
		res, err := backend.ComputeCostRanking(r.Context(), f)
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Falha ao calcular ranking de custo.")
			return
		}
		writeJSON(w, http.StatusOK, res)
	})
	mux.HandleFunc("GET "+base+"/series", func(w http.ResponseWriter, r *http.Request) {
		env := r.PathValue("id")
		if !uuidPattern.MatchString(env) {
			writeError(w, http.StatusBadRequest, CodeValidation, "Ambiente inválido.")
			return
		}
		to := time.Now().UTC()
		from := to.AddDate(0, 0, -90)
		if v := strings.TrimSpace(r.URL.Query().Get("to")); v != "" {
			parsed, err := time.Parse(time.RFC3339, v)
			if err != nil {
				writeError(w, http.StatusBadRequest, CodeValidation, "Parâmetro to inválido.")
				return
			}
			to = parsed.UTC()
		}
		if v := strings.TrimSpace(r.URL.Query().Get("from")); v != "" {
			parsed, err := time.Parse(time.RFC3339, v)
			if err != nil {
				writeError(w, http.StatusBadRequest, CodeValidation, "Parâmetro from inválido.")
				return
			}
			from = parsed.UTC()
		}
		items, err := backend.ListCostSeries(r.Context(), env, from, to)
		if err != nil {
			writeError(w, http.StatusBadRequest, CodeValidation, err.Error())
			return
		}
		if items == nil {
			items = []costlens.SeriesPoint{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items, "from": from.Format(time.RFC3339), "to": to.Format(time.RFC3339)})
	})
	mux.HandleFunc("GET "+base+"/ranking/export", func(w http.ResponseWriter, r *http.Request) {
		env := r.PathValue("id")
		if !uuidPattern.MatchString(env) {
			writeError(w, http.StatusBadRequest, CodeValidation, "Ambiente inválido.")
			return
		}
		if format := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format"))); format != "" && format != "csv" {
			writeError(w, http.StatusBadRequest, CodeValidation, "Formato suportado: csv.")
			return
		}
		res, err := backend.ComputeCostRanking(r.Context(), repository.CostRankingFilter{EnvironmentID: env})
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Falha ao exportar ranking.")
			return
		}
		csv := repository.FormatCostRankingCSV(res)
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="cost-ranking.csv"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(csv))
	})
	mux.HandleFunc("GET "+base+"/inventory", func(w http.ResponseWriter, r *http.Request) {
		env := r.PathValue("id")
		if !uuidPattern.MatchString(env) {
			writeError(w, http.StatusBadRequest, CodeValidation, "Ambiente inválido.")
			return
		}
		m, err := backend.TableStorageCostMap(r.Context(), env)
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Falha ao calcular custo do inventário.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": m})
	})
}
