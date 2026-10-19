package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/osmendes/db-auditor/internal/analyzer"
	"github.com/osmendes/db-auditor/internal/observability"
	"github.com/osmendes/db-auditor/internal/repository"
	"github.com/osmendes/db-auditor/internal/tsretention"
)

type timescaleSimulationStore interface {
	LoadHypertableSimulationInput(ctx context.Context, env, run, database, schema, name string) (*repository.HypertableSimulationInput, error)
	ListRetentionScenarios(ctx context.Context, environmentID string) ([]repository.RetentionScenario, error)
	SaveRetentionScenario(ctx context.Context, environmentID string, in repository.SaveRetentionScenarioInput) (*repository.RetentionScenario, error)
	CurrentCostPrice(ctx context.Context, environmentID string) (*repository.CostPriceVersion, error)
	ListChunkScanObservations(ctx context.Context, env, run, database, schema, name string) ([]tsretention.ChunkScanObservation, error)
	PreviousCompleteRunID(ctx context.Context, env, run string) (string, error)
	UpsertFinding(ctx context.Context, p repository.UpsertFindingParams) (*repository.Finding, error)
}

type simulationAPIResponse struct {
	Applicable          bool                        `json:"applicable"`
	Reason              string                      `json:"reason,omitempty"`
	Partial             bool                        `json:"partial"`
	RetentionApplicable bool                        `json:"retention_applicable"`
	ObservedRatio       float64                     `json:"observed_ratio"`
	RatioConfidenceLow  bool                        `json:"ratio_confidence_low"`
	CurrentBytes        int64                       `json:"current_bytes"`
	CompressedPct       float64                     `json:"compressed_pct"`
	AgeBuckets          []tsretention.AgeBucket     `json:"age_buckets"`
	Scenarios           []simulationScenarioAPI     `json:"scenarios"`
	Risks               []string                    `json:"risks"`
	Currency            string                      `json:"currency,omitempty"`
	PriceVersionID      string                      `json:"price_version_id,omitempty"`
	PriceEffectiveFrom  *time.Time                  `json:"price_effective_from,omitempty"`
	Gain                simulationGainAPI           `json:"gain"`
	Policies            []repository.HypertablePolicy `json:"policies,omitempty"`
}

type simulationScenarioAPI struct {
	Label             string   `json:"label"`
	RetentionDays     int      `json:"retention_days"`
	CompressAfterDays int      `json:"compress_after_days"`
	BytesAfter        int64    `json:"bytes_after"`
	BytesSaved        int64    `json:"bytes_saved"`
	GBSaved           float64  `json:"gb_saved"`
	MoneySaved        *float64 `json:"money_saved,omitempty"`
	Risks             []string `json:"risks"`
	Projection30GB    float64  `json:"projection_30_gb"`
	Projection90GB    float64  `json:"projection_90_gb"`
	Projection180GB   float64  `json:"projection_180_gb"`
}

type simulationGainAPI struct {
	Hypothesis string `json:"hypothesis"`
	Predicted  string `json:"predicted"`
	Measured   string `json:"measured"`
}

func registerTimescaleSimulationRoutes(mux *http.ServeMux, store InventoryStore) {
	backend, ok := store.(timescaleSimulationStore)
	if !ok {
		return
	}
	htBase := "/api/v1/environments/{id}/runs/{run}/databases/{database}/schemas/{schema}/hypertables/{hypertable}"
	mux.HandleFunc("GET "+htBase+"/simulation", getHypertableSimulation(backend))
	mux.HandleFunc("GET "+htBase+"/unscanned-chunks", getUnscannedChunks(backend))
	mux.HandleFunc("GET /api/v1/environments/{id}/timescale-scenarios", listTimescaleScenarios(backend))
	mux.HandleFunc("POST /api/v1/environments/{id}/timescale-scenarios", saveTimescaleScenario(backend))
	mux.HandleFunc("GET /api/v1/environments/{id}/timescale-scenarios/export", exportTimescaleScenarios(backend))
}

func getHypertableSimulation(store timescaleSimulationStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		env, run := r.PathValue("id"), r.PathValue("run")
		database, schema, name := r.PathValue("database"), r.PathValue("schema"), r.PathValue("hypertable")
		if !uuidPattern.MatchString(env) || !uuidPattern.MatchString(run) || database == "" || schema == "" || name == "" {
			writeError(w, http.StatusBadRequest, CodeValidation, "Escopo da simulação inválido.")
			return
		}
		scenarios, err := parseSimulationScenarios(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, CodeValidation, err.Error())
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		input, err := store.LoadHypertableSimulationInput(ctx, env, run, database, schema, name)
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Não foi possível carregar a hypertable.")
			return
		}
		if input.NotApplicable || !input.Found {
			writeJSON(w, http.StatusOK, simulationAPIResponse{
				Applicable: false,
				Reason:     "não aplicável",
				Gain:       defaultGain(),
			})
			return
		}
		price, _ := store.CurrentCostPrice(ctx, env)
		res := runSimulation(input, scenarios, time.Now().UTC())
		observability.DefaultMetrics.IncRetentionSimulation(res.RatioConfidenceLow)
		writeJSON(w, http.StatusOK, toSimulationAPI(res, input.Policies, price))
	}
}

func listTimescaleScenarios(store timescaleSimulationStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		env := r.PathValue("id")
		if !uuidPattern.MatchString(env) {
			writeError(w, http.StatusBadRequest, CodeValidation, "Ambiente inválido.")
			return
		}
		items, err := store.ListRetentionScenarios(r.Context(), env)
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Falha ao listar cenários.")
			return
		}
		if items == nil {
			items = []repository.RetentionScenario{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	}
}

func saveTimescaleScenario(store timescaleSimulationStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		env := r.PathValue("id")
		var body struct {
			DatabaseName      string `json:"database_name"`
			SchemaName        string `json:"schema_name"`
			HypertableName    string `json:"hypertable_name"`
			Label             string `json:"label"`
			RetentionDays     int    `json:"retention_days"`
			CompressAfterDays int    `json:"compress_after_days"`
			AuditRunID        string `json:"audit_run_id"`
		}
		if !uuidPattern.MatchString(env) || json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&body) != nil {
			writeError(w, http.StatusBadRequest, CodeValidation, "Cenário inválido.")
			return
		}
		if body.RetentionDays < 0 || body.CompressAfterDays < 0 || body.RetentionDays == 0 && body.CompressAfterDays == 0 {
			writeError(w, http.StatusBadRequest, CodeValidation, "Informe retenção e/ou compressão com N > 0.")
			return
		}
		if body.DatabaseName == "" || body.SchemaName == "" || body.HypertableName == "" || body.AuditRunID == "" || !uuidPattern.MatchString(body.AuditRunID) {
			writeError(w, http.StatusBadRequest, CodeValidation, "Hypertable e execução de referência são obrigatórios.")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		input, err := store.LoadHypertableSimulationInput(ctx, env, body.AuditRunID, body.DatabaseName, body.SchemaName, body.HypertableName)
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Não foi possível carregar a hypertable.")
			return
		}
		if input.NotApplicable || !input.Found {
			writeError(w, http.StatusBadRequest, CodeValidation, "não aplicável")
			return
		}
		sc := tsretention.Scenario{
			Label:             strings.TrimSpace(body.Label),
			RetentionDays:     body.RetentionDays,
			CompressAfterDays: body.CompressAfterDays,
		}
		if sc.Label == "" {
			sc.Label = "cenário"
		}
		res := runSimulation(input, []tsretention.Scenario{sc}, time.Now().UTC())
		if len(res.Scenarios) == 0 {
			writeError(w, http.StatusBadRequest, CodeValidation, "Cenário inválido.")
			return
		}
		sr := res.Scenarios[0]
		price, _ := store.CurrentCostPrice(ctx, env)
		currency, money, priceID, effective := repository.ApplyStoragePrice(sr.GBSaved, price)
		actor := "auditor"
		if user := requestIdentity(r); user != nil {
			actor = user.Username
		}
		saved, err := store.SaveRetentionScenario(ctx, env, repository.SaveRetentionScenarioInput{
			DatabaseName: body.DatabaseName, SchemaName: body.SchemaName, HypertableName: body.HypertableName,
			Label: sc.Label, RetentionDays: body.RetentionDays, CompressAfterDays: body.CompressAfterDays,
			AuditRunID: body.AuditRunID, Result: res, Scenario: sr,
			Currency: currency, MoneySaved: money, PriceVersionID: priceID, PriceEffectiveFrom: effective, SavedBy: actor,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Falha ao salvar o cenário.")
			return
		}
		// ADR-0001 D9: create conflict finding on save, linked to last complete run.
		for _, dep := range input.CAGGDeps {
			if body.RetentionDays > 0 && dep.HasRefreshPolicy && dep.RefreshWindowDays > 0 && body.RetentionDays < dep.RefreshWindowDays {
				f := analyzer.ConflictFindingForSavedScenario(env, body.AuditRunID, body.DatabaseName, body.SchemaName, body.HypertableName, dep.ViewName, body.RetentionDays, dep.RefreshWindowDays)
				ev, _ := json.Marshal(f.Evidence)
				_, _ = store.UpsertFinding(ctx, repository.UpsertFindingParams{
					EnvironmentID: env, AuditRunID: body.AuditRunID, FindingType: f.FindingType, Severity: string(f.Severity),
					Title: f.Title, Summary: f.Summary, ObjectType: f.ObjectType, ObjectKey: f.ObjectKey,
					DatabaseName: f.DatabaseName, SchemaName: f.SchemaName, ObjectName: f.ObjectName,
					Evidence: ev, DedupKey: f.DedupKey, RuleID: f.FindingType, RuleVersion: "1", Category: "timescale",
					Confidence: 0.85, Impact: "alto", Risk: "perda de dados necessários ao CAGG",
					Recommendation: "Ajuste a retenção simulada para cobrir a janela de refresh ou revise a política do CAGG. Nenhuma exclusão automática.",
					Validation:     "Confirme start_offset do refresh e drop_after da retenção no catálogo Timescale.",
				})
			}
		}
		observability.DefaultMetrics.IncRetentionSimulation(res.RatioConfidenceLow)
		writeJSON(w, http.StatusCreated, saved)
	}
}

func exportTimescaleScenarios(store timescaleSimulationStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		env := r.PathValue("id")
		if !uuidPattern.MatchString(env) {
			writeError(w, http.StatusBadRequest, CodeValidation, "Ambiente inválido.")
			return
		}
		if format := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format"))); format != "" && format != "csv" {
			writeError(w, http.StatusBadRequest, CodeValidation, "Formato suportado: csv.")
			return
		}
		items, err := store.ListRetentionScenarios(r.Context(), env)
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Falha ao exportar cenários.")
			return
		}
		csv := repository.FormatRetentionScenariosCSV(items)
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="timescale-scenarios.csv"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(csv))
	}
}

func getUnscannedChunks(store timescaleSimulationStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		env, run := r.PathValue("id"), r.PathValue("run")
		database, schema, name := r.PathValue("database"), r.PathValue("schema"), r.PathValue("hypertable")
		if !uuidPattern.MatchString(env) || !uuidPattern.MatchString(run) || database == "" || schema == "" || name == "" {
			writeError(w, http.StatusBadRequest, CodeValidation, "Escopo inválido.")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		input, err := store.LoadHypertableSimulationInput(ctx, env, run, database, schema, name)
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Falha ao carregar chunks.")
			return
		}
		if input.NotApplicable || !input.Found {
			writeJSON(w, http.StatusOK, map[string]any{
				"applicable": false, "reason": "não aplicável",
				"approximate": true, "label": tsretention.ApproximateUnscannedLabel,
				"items": []string{},
			})
			return
		}
		cur, err := store.ListChunkScanObservations(ctx, env, run, database, schema, name)
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Falha ao ler contadores.")
			return
		}
		prevRun, err := store.PreviousCompleteRunID(ctx, env, run)
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Falha ao localizar coleta anterior.")
			return
		}
		var prev []tsretention.ChunkScanObservation
		if prevRun != "" {
			prev, err = store.ListChunkScanObservations(ctx, env, prevRun, database, schema, name)
			if err != nil {
				writeError(w, http.StatusInternalServerError, CodeInternal, "Falha ao ler contadores anteriores.")
				return
			}
		}
		res := tsretention.FindUnscannedChunks(prev, cur)
		writeJSON(w, http.StatusOK, map[string]any{
			"applicable": true, "comparable": res.Comparable, "reason": res.Reason,
			"approximate": true, "label": res.Label,
			"items": res.Unscanned, "not_collected": res.NotCollected,
		})
	}
}

func parseSimulationScenarios(r *http.Request) ([]tsretention.Scenario, error) {
	q := r.URL.Query()
	var out []tsretention.Scenario
	if raw := strings.TrimSpace(q.Get("scenario")); raw != "" {
		parts := strings.Split(raw, ",")
		for i, part := range parts {
			if i >= 5 {
				break
			}
			sc, err := parseScenarioToken(part)
			if err != nil {
				return nil, err
			}
			out = append(out, sc)
		}
		return out, nil
	}
	ret, err := optionalNonNegInt(q.Get("retention_days"))
	if err != nil {
		return nil, err
	}
	comp, err := optionalNonNegInt(q.Get("compress_after_days"))
	if err != nil {
		return nil, err
	}
	if ret == 0 && comp == 0 {
		// Default preview scenarios when none provided.
		return []tsretention.Scenario{
			{Label: "ret-90", RetentionDays: 90},
			{Label: "comp-7", CompressAfterDays: 7},
			{Label: "ret-90+comp-7", RetentionDays: 90, CompressAfterDays: 7},
		}, nil
	}
	label := strings.TrimSpace(q.Get("label"))
	if label == "" {
		label = "custom"
	}
	return []tsretention.Scenario{{Label: label, RetentionDays: ret, CompressAfterDays: comp}}, nil
}

var errInvalidScenario = &validationError{msg: "Cenário inválido: N deve ser > 0."}

type validationError struct{ msg string }

func (e *validationError) Error() string { return e.msg }

func optionalNonNegInt(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, errInvalidScenario
	}
	if n <= 0 {
		return 0, errInvalidScenario
	}
	return n, nil
}

func parseScenarioToken(token string) (tsretention.Scenario, error) {
	token = strings.TrimSpace(token)
	sc := tsretention.Scenario{Label: token}
	for _, part := range strings.Split(token, "+") {
		part = strings.TrimSpace(part)
		switch {
		case strings.HasPrefix(part, "ret-"):
			n, err := strconv.Atoi(strings.TrimPrefix(part, "ret-"))
			if err != nil || n <= 0 {
				return sc, errInvalidScenario
			}
			sc.RetentionDays = n
		case strings.HasPrefix(part, "comp-"):
			n, err := strconv.Atoi(strings.TrimPrefix(part, "comp-"))
			if err != nil || n <= 0 {
				return sc, errInvalidScenario
			}
			sc.CompressAfterDays = n
		default:
			if sc.Label == token {
				sc.Label = part
			}
		}
	}
	if sc.RetentionDays == 0 && sc.CompressAfterDays == 0 {
		return sc, errInvalidScenario
	}
	return sc, nil
}

func runSimulation(input *repository.HypertableSimulationInput, scenarios []tsretention.Scenario, now time.Time) tsretention.SimulationResult {
	chunks := make([]tsretention.Chunk, 0, len(input.Chunks))
	for _, c := range input.Chunks {
		chunks = append(chunks, tsretention.Chunk{
			Name: c.ChunkSchema + "." + c.ChunkName, RangeStart: c.RangeStart, RangeEnd: c.RangeEnd,
			RangeEndInteger: c.RangeEndInteger, IsCompressed: c.IsCompressed, TotalSizeBytes: c.TotalSizeBytes,
			BeforeCompressionBytes: c.BeforeCompressionBytes, AfterCompressionBytes: c.AfterCompressionBytes,
			DeadTuples: c.DeadTuples,
		})
	}
	deps := make([]tsretention.CAGGDep, 0, len(input.CAGGDeps))
	for _, d := range input.CAGGDeps {
		deps = append(deps, tsretention.CAGGDep{ViewName: d.ViewName, RefreshWindowDays: d.RefreshWindowDays, HasRefreshPolicy: d.HasRefreshPolicy})
	}
	return tsretention.Simulate(tsretention.HypertableState{
		Chunks: chunks, CAGGDeps: deps, InventoryTruncated: input.InventoryTruncated,
		IntegerDimensionOnly: input.IntegerDimensionOnly, GrowthPerDayBytes: input.GrowthPerDayBytes, Now: now,
	}, scenarios)
}

func toSimulationAPI(res tsretention.SimulationResult, policies []repository.HypertablePolicy, price *repository.CostPriceVersion) simulationAPIResponse {
	out := simulationAPIResponse{
		Applicable: true, Partial: res.Partial, RetentionApplicable: res.RetentionApplicable,
		ObservedRatio: res.ObservedRatio, RatioConfidenceLow: res.RatioConfidenceLow,
		CurrentBytes: res.CurrentBytes, CompressedPct: res.CompressedPct,
		AgeBuckets: res.AgeBuckets, Risks: res.Risks, Policies: policies, Gain: defaultGain(),
	}
	if out.AgeBuckets == nil {
		out.AgeBuckets = []tsretention.AgeBucket{}
	}
	if out.Risks == nil {
		out.Risks = []string{}
	}
	currency, _, priceID, effective := repository.ApplyStoragePrice(0, price)
	out.Currency = currency
	out.PriceVersionID = priceID
	out.PriceEffectiveFrom = effective
	for _, sr := range res.Scenarios {
		item := simulationScenarioAPI{
			Label: sr.Label, RetentionDays: sr.RetentionDays, CompressAfterDays: sr.CompressAfterDays,
			BytesAfter: sr.BytesAfter, BytesSaved: sr.BytesSaved, GBSaved: sr.GBSaved, Risks: sr.Risks,
			Projection30GB: sr.Projection30GB, Projection90GB: sr.Projection90GB, Projection180GB: sr.Projection180GB,
		}
		if item.Risks == nil {
			item.Risks = []string{}
		}
		_, money, _, _ := repository.ApplyStoragePrice(sr.GBSaved, price)
		item.MoneySaved = money
		out.Scenarios = append(out.Scenarios, item)
	}
	if len(out.Scenarios) > 0 {
		best := out.Scenarios[0]
		out.Gain.Predicted = "economia prevista de até " + formatGB(best.GBSaved) + " no cenário \"" + best.Label + "\" (hipótese até confirmação humana)."
		if best.MoneySaved != nil && out.Currency != "" {
			out.Gain.Predicted += " Equivale a cerca de " + out.Currency + " " + strconv.FormatFloat(*best.MoneySaved, 'f', 2, 64) + "/mês na vigência do preço."
		}
	}
	return out
}

func defaultGain() simulationGainAPI {
	return simulationGainAPI{
		Hypothesis: "Simulação sobre metadados de chunks; não aplica políticas.",
		Predicted:  "não estimado até escolher um cenário.",
		Measured:   "não medido: exige política aplicada fora do auditor e nova coleta completa comparável.",
	}
}

func formatGB(v float64) string {
	return strconv.FormatFloat(v, 'f', 2, 64) + " GB"
}
