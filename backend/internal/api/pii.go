package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/osmendes/db-auditor/internal/config"
	"github.com/osmendes/db-auditor/internal/observability"
	"github.com/osmendes/db-auditor/internal/repository"
)

type piiStore interface {
	GetEnvironmentEngine(context.Context, string) (string, error)
	GetPIISettings(context.Context, string, bool) (*repository.PIIEnvironmentSetting, error)
	PutPIISettings(context.Context, string, string, bool) (*repository.PIIEnvironmentSetting, error)
	ListPIIDictionary(context.Context, string) ([]repository.PIIDictionaryEntry, error)
	ReplacePIIDictionary(context.Context, string, string, []repository.PIIDictionaryEntry) ([]repository.PIIDictionaryEntry, error)
	CreatePIIScan(context.Context, string, string, []int, bool, string, config.Scope) (*repository.PIIScan, error)
	ListPIIScans(context.Context, string) ([]repository.PIIScan, error)
	GetPIIScan(context.Context, string, string) (*repository.PIIScan, error)
	ListPIIMap(context.Context, string, repository.PIIMapFilter) ([]repository.PIIColumnResult, int, error)
	ReviewPIIClassification(context.Context, string, string, string, string, string) (*repository.PIIClassificationReview, error)
	StreamPIIMapCSV(context.Context, string, io.Writer) (int, error)
	LogAuditorOperation(context.Context, *repository.AuditorUser, string, string, string, string) error
}

func piiProcessFlag() bool { return os.Getenv("AUDITOR_PII_CONTENT_SCAN_ENABLED") == "1" }

func registerPIIRoutes(mux *http.ServeMux, store InventoryStore, targets map[string]string) {
	backend, ok := store.(piiStore)
	if !ok {
		return
	}
	base := "/api/v1/environments/{id}"

	mux.HandleFunc("GET "+base+"/pii-settings", func(w http.ResponseWriter, r *http.Request) {
		env := r.PathValue("id")
		if !uuidPattern.MatchString(env) {
			writeError(w, http.StatusBadRequest, CodeValidation, "Ambiente inválido.")
			return
		}
		item, err := backend.GetPIISettings(r.Context(), env, piiProcessFlag())
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Falha ao ler configuração PII.")
			return
		}
		writeJSON(w, http.StatusOK, item)
	})

	mux.HandleFunc("PUT "+base+"/pii-settings", func(w http.ResponseWriter, r *http.Request) {
		env := r.PathValue("id")
		user := requestIdentity(r)
		if !uuidPattern.MatchString(env) || user == nil {
			writeError(w, http.StatusBadRequest, CodeValidation, "Ambiente inválido.")
			return
		}
		var body struct {
			ContentScanEnabled bool `json:"content_scan_enabled"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body) != nil {
			writeError(w, http.StatusBadRequest, CodeValidation, "Corpo inválido.")
			return
		}
		item, err := backend.PutPIISettings(r.Context(), env, user.Username, body.ContentScanEnabled)
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Falha ao gravar configuração PII.")
			return
		}
		item.ProcessFlagEnabled = piiProcessFlag()
		item.EffectiveEnabled = item.ProcessFlagEnabled && item.ContentScanEnabled
		_ = backend.LogAuditorOperation(r.Context(), user, "pii.settings_updated", env, "", "success")
		writeJSON(w, http.StatusOK, item)
	})

	mux.HandleFunc("GET "+base+"/pii-dictionary", func(w http.ResponseWriter, r *http.Request) {
		env := r.PathValue("id")
		if !uuidPattern.MatchString(env) {
			writeError(w, http.StatusBadRequest, CodeValidation, "Ambiente inválido.")
			return
		}
		items, err := backend.ListPIIDictionary(r.Context(), env)
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Falha ao listar dicionário.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	})

	mux.HandleFunc("PUT "+base+"/pii-dictionary", func(w http.ResponseWriter, r *http.Request) {
		env := r.PathValue("id")
		user := requestIdentity(r)
		if !uuidPattern.MatchString(env) || user == nil {
			writeError(w, http.StatusBadRequest, CodeValidation, "Ambiente inválido.")
			return
		}
		var body struct {
			Items []repository.PIIDictionaryEntry `json:"items"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body) != nil {
			writeError(w, http.StatusBadRequest, CodeValidation, "Corpo inválido.")
			return
		}
		items, err := backend.ReplacePIIDictionary(r.Context(), env, user.Username, body.Items)
		if errors.Is(err, repository.ErrPIIValidation) {
			writeError(w, http.StatusBadRequest, CodeValidation, "Padrão ou categoria inválidos.")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Falha ao gravar dicionário.")
			return
		}
		_ = backend.LogAuditorOperation(r.Context(), user, "pii.dictionary_replaced", env, "", "success")
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	})

	mux.HandleFunc("POST "+base+"/pii-scans", func(w http.ResponseWriter, r *http.Request) {
		env := r.PathValue("id")
		user := requestIdentity(r)
		if !uuidPattern.MatchString(env) || user == nil {
			writeError(w, http.StatusBadRequest, CodeValidation, "Ambiente inválido.")
			return
		}
		engine, err := backend.GetEnvironmentEngine(r.Context(), env)
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Falha ao consultar ambiente.")
			return
		}
		if engine == "mongodb" {
			writeError(w, http.StatusUnprocessableEntity, CodeUnavailable, "não aplicável a este mecanismo")
			return
		}
		var body struct {
			Stages []int `json:"stages"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body) != nil {
			writeError(w, http.StatusBadRequest, CodeValidation, "Corpo inválido.")
			return
		}
		if len(body.Stages) == 0 {
			body.Stages = []int{1}
		}
		configured := targets
		if configured == nil {
			configured = config.LoadTargetDSNs()
		}
		dsn := config.DSNForEnvironment(configured, env)
		scope := config.Scope{
			DatabaseDenylist: []string{"template0", "template1", "postgres"},
			SchemaDenylist:   []string{"pg_catalog", "information_schema"},
		}
		if cfg, err := config.Load(); err == nil {
			scope = cfg.Scope
		}
		item, err := backend.CreatePIIScan(r.Context(), env, user.Username, body.Stages, piiProcessFlag(), dsn, scope)
		if errors.Is(err, repository.ErrPIIContentDisabled) {
			writeError(w, http.StatusConflict, CodeConflict, "Etapa 2 desabilitada: exige AUDITOR_PII_CONTENT_SCAN_ENABLED=1 e habilitação do ambiente.")
			return
		}
		if errors.Is(err, repository.ErrPIINotApplicable) {
			writeError(w, http.StatusUnprocessableEntity, CodeUnavailable, "não aplicável a este mecanismo")
			return
		}
		if errors.Is(err, repository.ErrPIINoReferenceRun) {
			writeError(w, http.StatusConflict, CodeConflict, "Varredura exige uma coleta completa de referência.")
			return
		}
		if errors.Is(err, repository.ErrPIIValidation) {
			writeError(w, http.StatusBadRequest, CodeValidation, "Etapas inválidas.")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Falha ao executar varredura PII.")
			return
		}
		observability.DefaultMetrics.IncPIIScan(item.Status, item.ContentColumns > 0)
		_ = backend.LogAuditorOperation(r.Context(), user, "pii.scan_created", env, item.ID, "success")
		writeJSON(w, http.StatusCreated, item)
	})

	mux.HandleFunc("GET "+base+"/pii-scans", func(w http.ResponseWriter, r *http.Request) {
		env := r.PathValue("id")
		if !uuidPattern.MatchString(env) {
			writeError(w, http.StatusBadRequest, CodeValidation, "Ambiente inválido.")
			return
		}
		engine, _ := backend.GetEnvironmentEngine(r.Context(), env)
		if engine == "mongodb" {
			writeJSON(w, http.StatusOK, map[string]any{"items": []any{}, "applicable": false, "reason": "não aplicável a este mecanismo", "content_scan_enabled": false})
			return
		}
		items, err := backend.ListPIIScans(r.Context(), env)
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Falha ao listar varreduras.")
			return
		}
		settings, _ := backend.GetPIISettings(r.Context(), env, piiProcessFlag())
		enabled := settings != nil && settings.EffectiveEnabled
		writeJSON(w, http.StatusOK, map[string]any{"items": items, "applicable": true, "content_scan_enabled": enabled})
	})

	mux.HandleFunc("GET "+base+"/pii-scans/{scan}", func(w http.ResponseWriter, r *http.Request) {
		env, id := r.PathValue("id"), r.PathValue("scan")
		if !uuidPattern.MatchString(env) || !uuidPattern.MatchString(id) {
			writeError(w, http.StatusBadRequest, CodeValidation, "Ambiente ou varredura inválidos.")
			return
		}
		item, err := backend.GetPIIScan(r.Context(), env, id)
		if errors.Is(err, repository.ErrPIINotFound) {
			writeError(w, http.StatusNotFound, CodeNotFound, "Varredura não encontrada.")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Falha ao carregar varredura.")
			return
		}
		writeJSON(w, http.StatusOK, item)
	})

	mux.HandleFunc("GET "+base+"/pii-map", func(w http.ResponseWriter, r *http.Request) {
		env := r.PathValue("id")
		limit, offset, ok := parseListPage(r, 50)
		if !uuidPattern.MatchString(env) || !ok {
			writeError(w, http.StatusBadRequest, CodeValidation, "Ambiente ou paginação inválidos.")
			return
		}
		engine, _ := backend.GetEnvironmentEngine(r.Context(), env)
		if engine == "mongodb" {
			writeJSON(w, http.StatusOK, map[string]any{"items": []any{}, "total": 0, "limit": limit, "offset": offset, "applicable": false, "reason": "não aplicável a este mecanismo"})
			return
		}
		q := r.URL.Query()
		items, total, err := backend.ListPIIMap(r.Context(), env, repository.PIIMapFilter{Category: q.Get("category"), Confidence: q.Get("confidence"), Limit: limit, Offset: offset})
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Falha ao carregar mapa PII.")
			return
		}
		writePage(w, items, limit, offset, total)
	})

	mux.HandleFunc("PATCH "+base+"/pii-classifications/{column}", func(w http.ResponseWriter, r *http.Request) {
		env, column := r.PathValue("id"), r.PathValue("column")
		user := requestIdentity(r)
		if !uuidPattern.MatchString(env) || user == nil || strings.TrimSpace(column) == "" {
			writeError(w, http.StatusBadRequest, CodeValidation, "Ambiente ou coluna inválidos.")
			return
		}
		var body struct {
			Decision      string `json:"decision"`
			Justification string `json:"justification"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&body) != nil {
			writeError(w, http.StatusBadRequest, CodeValidation, "Corpo inválido.")
			return
		}
		item, err := backend.ReviewPIIClassification(r.Context(), env, column, user.Username, body.Decision, body.Justification)
		if errors.Is(err, repository.ErrPIIValidation) {
			writeError(w, http.StatusBadRequest, CodeValidation, "Decisão inválida; descarte exige justificativa.")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Falha ao registrar revisão.")
			return
		}
		_ = backend.LogAuditorOperation(r.Context(), user, "pii.classification_reviewed", env, column, "success")
		writeJSON(w, http.StatusOK, item)
	})

	mux.HandleFunc("GET "+base+"/pii-map/export/download", func(w http.ResponseWriter, r *http.Request) {
		env := r.PathValue("id")
		if !uuidPattern.MatchString(env) {
			writeError(w, http.StatusBadRequest, CodeValidation, "Ambiente inválido.")
			return
		}
		if f := r.URL.Query().Get("format"); f != "" && f != "csv" {
			writeError(w, http.StatusBadRequest, CodeValidation, "Formato inválido.")
			return
		}
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="pii-map.csv"`)
		n, err := backend.StreamPIIMapCSV(r.Context(), env, w)
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Falha ao exportar mapa PII.")
			return
		}
		observability.DefaultMetrics.AddPIIExportRows(n)
	})
}
