package api

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/osmendes/db-auditor/internal/repository"
	"github.com/osmendes/db-auditor/internal/schemadiff"
)

type schemaDiffStore interface {
	BuildSchemaDiff(context.Context, repository.SchemaDiffRequest) (*repository.SchemaDiffResult, error)
}

type schemaTimelineStore interface {
	ListSchemaTimeline(context.Context, string, time.Time, time.Time, int, int) (*repository.SchemaTimelinePage, error)
	SetEnvironmentProduction(context.Context, string, bool, string) error
	ListMigrationRegistry(context.Context, string) ([]repository.EnvironmentMigrationRegistry, error)
	ReplaceMigrationRegistry(context.Context, string, string, []repository.MigrationRegistryInput) ([]repository.EnvironmentMigrationRegistry, error)
}

func registerSchemaDriftRoutes(mux *http.ServeMux, store InventoryStore) {
	base := "/api/v1/environments/{id}"
	mux.HandleFunc("GET "+base+"/schema-diff", func(w http.ResponseWriter, r *http.Request) {
		backend, ok := store.(schemaDiffStore)
		if !ok {
			writeError(w, http.StatusServiceUnavailable, CodeUnavailable, "Diff de schema indisponível.")
			return
		}
		env := r.PathValue("id")
		if !uuidPattern.MatchString(env) {
			writeError(w, http.StatusBadRequest, CodeValidation, "Ambiente inválido.")
			return
		}
		q := r.URL.Query()
		targetEnv := strings.TrimSpace(q.Get("target_environment_id"))
		if targetEnv == "" {
			targetEnv = env
		}
		if !uuidPattern.MatchString(targetEnv) {
			writeError(w, http.StatusBadRequest, CodeValidation, "Ambiente de destino inválido.")
			return
		}
		sourceRun := strings.TrimSpace(q.Get("source_run"))
		targetRun := strings.TrimSpace(q.Get("target_run"))
		if sourceRun != "" && !uuidPattern.MatchString(sourceRun) {
			writeError(w, http.StatusBadRequest, CodeValidation, "Execução de origem inválida.")
			return
		}
		if targetRun != "" && !uuidPattern.MatchString(targetRun) {
			writeError(w, http.StatusBadRequest, CodeValidation, "Execução de destino inválida.")
			return
		}
		limit, offset, valid := parseListPage(r, 50)
		if !valid {
			writeError(w, http.StatusBadRequest, CodeValidation, "Paginação inválida.")
			return
		}
		res, err := backend.BuildSchemaDiff(r.Context(), repository.SchemaDiffRequest{
			SourceEnv:  env,
			SourceRun:  sourceRun,
			TargetEnv:  targetEnv,
			TargetRun:  targetRun,
			ObjectType: strings.TrimSpace(q.Get("object_type")),
			Status:     strings.TrimSpace(q.Get("status")),
			Limit:      limit,
			Offset:     offset,
		})
		if errors.Is(err, schemadiff.ErrMechanismMismatch) {
			writeError(w, http.StatusBadRequest, CodeValidation, "Mecanismos de banco diferentes não podem ser comparados.")
			return
		}
		if errors.Is(err, repository.ErrMissingCompleteRun) {
			writeError(w, http.StatusBadRequest, CodeValidation, "Não há execução completa para comparar.")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Falha ao calcular o diff de schema.")
			return
		}
		writeJSON(w, http.StatusOK, res)
	})

	mux.HandleFunc("GET "+base+"/schema-diff/export", func(w http.ResponseWriter, r *http.Request) {
		backend, ok := store.(schemaDiffStore)
		if !ok {
			writeError(w, http.StatusServiceUnavailable, CodeUnavailable, "Diff de schema indisponível.")
			return
		}
		env := r.PathValue("id")
		format := strings.TrimSpace(r.URL.Query().Get("format"))
		if !uuidPattern.MatchString(env) || (format != "csv" && format != "md") {
			writeError(w, http.StatusBadRequest, CodeValidation, "Ambiente ou formato inválido.")
			return
		}
		q := r.URL.Query()
		targetEnv := strings.TrimSpace(q.Get("target_environment_id"))
		if targetEnv == "" {
			targetEnv = env
		}
		res, err := backend.BuildSchemaDiff(r.Context(), repository.SchemaDiffRequest{
			SourceEnv:  env,
			SourceRun:  strings.TrimSpace(q.Get("source_run")),
			TargetEnv:  targetEnv,
			TargetRun:  strings.TrimSpace(q.Get("target_run")),
			ObjectType: strings.TrimSpace(q.Get("object_type")),
			Status:     strings.TrimSpace(q.Get("status")),
			Limit:      10000,
			Offset:     0,
		})
		if errors.Is(err, schemadiff.ErrMechanismMismatch) {
			writeError(w, http.StatusBadRequest, CodeValidation, "Mecanismos de banco diferentes não podem ser comparados.")
			return
		}
		if errors.Is(err, repository.ErrMissingCompleteRun) {
			writeError(w, http.StatusBadRequest, CodeValidation, "Não há execução completa para comparar.")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Falha ao exportar o diff de schema.")
			return
		}
		w.Header().Set("Cache-Control", "private, no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="schema-diff-%s.%s"`, env, format))
		if format == "md" {
			w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
			_, _ = fmt.Fprintf(w, "# Schema diff\n\nOrigem `%s` → Destino `%s`\n\n", res.SourceRunID, res.TargetRunID)
			for _, o := range res.Objects {
				_, _ = fmt.Fprintf(w, "- **%s** `%s` %s (fp %s → %s)\n", o.Status, o.ObjectType, o.ObjectKey, o.SourceFP, o.TargetFP)
			}
			return
		}
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		cw := csv.NewWriter(w)
		_ = cw.Write([]string{"object_type", "object_key", "status", "source_fingerprint", "target_fingerprint", "changed_fields"})
		for _, o := range res.Objects {
			fields := make([]string, 0, len(o.FieldDiffs))
			for _, f := range o.FieldDiffs {
				fields = append(fields, f.Field)
			}
			_ = cw.Write([]string{o.ObjectType, o.ObjectKey, string(o.Status), o.SourceFP, o.TargetFP, strings.Join(fields, "|")})
		}
		_ = cw.Write([]string{"# export_complete", fmt.Sprint(len(res.Objects)), "", "", "", ""})
		cw.Flush()
	})

	mux.HandleFunc("GET "+base+"/schema-timeline", func(w http.ResponseWriter, r *http.Request) {
		backend, ok := store.(schemaTimelineStore)
		if !ok {
			writeError(w, http.StatusServiceUnavailable, CodeUnavailable, "Linha do tempo indisponível.")
			return
		}
		env := r.PathValue("id")
		if !uuidPattern.MatchString(env) {
			writeError(w, http.StatusBadRequest, CodeValidation, "Ambiente inválido.")
			return
		}
		from, to := parseTimelineWindow(r)
		limit, offset, valid := parseListPage(r, 50)
		if !valid {
			writeError(w, http.StatusBadRequest, CodeValidation, "Paginação inválida.")
			return
		}
		page, err := backend.ListSchemaTimeline(r.Context(), env, from, to, limit, offset)
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Falha ao listar a linha do tempo.")
			return
		}
		writeJSON(w, http.StatusOK, page)
	})

	mux.HandleFunc("GET "+base+"/schema-timeline/export", func(w http.ResponseWriter, r *http.Request) {
		backend, ok := store.(schemaTimelineStore)
		if !ok {
			writeError(w, http.StatusServiceUnavailable, CodeUnavailable, "Linha do tempo indisponível.")
			return
		}
		env := r.PathValue("id")
		format := strings.TrimSpace(r.URL.Query().Get("format"))
		if !uuidPattern.MatchString(env) || (format != "csv" && format != "md") {
			writeError(w, http.StatusBadRequest, CodeValidation, "Ambiente ou formato inválido.")
			return
		}
		from, to := parseTimelineWindow(r)
		page, err := backend.ListSchemaTimeline(r.Context(), env, from, to, 10000, 0)
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Falha ao exportar a linha do tempo.")
			return
		}
		w.Header().Set("Cache-Control", "private, no-store")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="schema-timeline-%s.%s"`, env, format))
		if format == "md" {
			w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
			_, _ = fmt.Fprintf(w, "# Schema timeline\n\n")
			for _, e := range page.Events {
				_, _ = fmt.Fprintf(w, "- %s **%s** `%s` corr=%s\n", e.DetectedAt.Format(time.RFC3339), e.ChangeKind, e.ObjectKey, e.Correlation)
			}
			return
		}
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		cw := csv.NewWriter(w)
		_ = cw.Write([]string{"detected_at", "object_type", "object_key", "change_kind", "correlation", "prior_fingerprint", "current_fingerprint"})
		for _, e := range page.Events {
			_ = cw.Write([]string{e.DetectedAt.Format(time.RFC3339), e.ObjectType, e.ObjectKey, e.ChangeKind, e.Correlation, e.PriorFingerprint, e.CurrentFingerprint})
		}
		_ = cw.Write([]string{"# export_complete", fmt.Sprint(len(page.Events)), "", "", "", "", ""})
		cw.Flush()
	})

	mux.HandleFunc("PATCH "+base, func(w http.ResponseWriter, r *http.Request) {
		backend, ok := store.(schemaTimelineStore)
		if !ok {
			writeError(w, http.StatusServiceUnavailable, CodeUnavailable, "Atualização de ambiente indisponível.")
			return
		}
		env := r.PathValue("id")
		if !uuidPattern.MatchString(env) {
			writeError(w, http.StatusBadRequest, CodeValidation, "Ambiente inválido.")
			return
		}
		user := requestIdentity(r)
		actor := "operator"
		if user != nil {
			actor = user.Username
		}
		var body struct {
			IsProduction *bool `json:"is_production"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.IsProduction == nil {
			writeError(w, http.StatusBadRequest, CodeValidation, "Corpo inválido.")
			return
		}
		if err := backend.SetEnvironmentProduction(r.Context(), env, *body.IsProduction, actor); err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Falha ao atualizar o ambiente.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"id": env, "is_production": *body.IsProduction})
	})

	mux.HandleFunc("GET "+base+"/migration-registry", func(w http.ResponseWriter, r *http.Request) {
		backend, ok := store.(schemaTimelineStore)
		if !ok {
			writeError(w, http.StatusServiceUnavailable, CodeUnavailable, "Registro de migrations indisponível.")
			return
		}
		env := r.PathValue("id")
		if !uuidPattern.MatchString(env) {
			writeError(w, http.StatusBadRequest, CodeValidation, "Ambiente inválido.")
			return
		}
		items, err := backend.ListMigrationRegistry(r.Context(), env)
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Falha ao listar o registro de migrations.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	})

	mux.HandleFunc("PUT "+base+"/migration-registry", func(w http.ResponseWriter, r *http.Request) {
		backend, ok := store.(schemaTimelineStore)
		if !ok {
			writeError(w, http.StatusServiceUnavailable, CodeUnavailable, "Registro de migrations indisponível.")
			return
		}
		env := r.PathValue("id")
		if !uuidPattern.MatchString(env) {
			writeError(w, http.StatusBadRequest, CodeValidation, "Ambiente inválido.")
			return
		}
		user := requestIdentity(r)
		actor := "operator"
		if user != nil {
			actor = user.Username
		}
		var body struct {
			Items []repository.MigrationRegistryInput `json:"items"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, CodeValidation, "Corpo inválido.")
			return
		}
		for _, in := range body.Items {
			if err := repository.ValidateMigrationRegistryInput(in); err != nil {
				writeError(w, http.StatusBadRequest, CodeValidation, err.Error())
				return
			}
		}
		items, err := backend.ReplaceMigrationRegistry(r.Context(), env, actor, body.Items)
		if err != nil {
			if strings.Contains(err.Error(), "inválid") {
				writeError(w, http.StatusBadRequest, CodeValidation, err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, CodeInternal, "Falha ao salvar o registro de migrations.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	})
}

func parseTimelineWindow(r *http.Request) (time.Time, time.Time) {
	q := r.URL.Query()
	to := time.Now().UTC()
	from := to.Add(-90 * 24 * time.Hour)
	if v := strings.TrimSpace(q.Get("to")); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			to = t
		}
	}
	if v := strings.TrimSpace(q.Get("from")); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			from = t
		}
	}
	return from, to
}
