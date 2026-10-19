package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/osmendes/db-auditor/internal/config"
	"github.com/osmendes/db-auditor/internal/observability"
	"github.com/osmendes/db-auditor/internal/repository"
	"github.com/osmendes/db-auditor/internal/restoreverify"
)

type restoreDrillStore interface {
	GetRestoreDrillContext(context.Context, string, []string) (*repository.RestoreDrillContext, error)
	RecordTargetRestoreDrill(context.Context, string, *string, repository.RecordTargetRestoreDrillInput) (*repository.TargetRestoreDrill, error)
	ListTargetRestoreDrills(context.Context, string, int, int) ([]repository.TargetRestoreDrill, int, error)
	GetTargetRestoreDrill(context.Context, string, string) (*repository.TargetRestoreDrill, error)
	GetTargetRestorePolicy(context.Context, string) (*repository.TargetRestorePolicy, error)
	UpsertTargetRestorePolicy(context.Context, string, string, int, int) (*repository.TargetRestorePolicy, error)
	LastSuccessfulTargetRestoreDrill(context.Context, string) (*repository.TargetRestoreDrill, error)
	UpsertFinding(context.Context, repository.UpsertFindingParams) (*repository.Finding, error)
}

var restoreDrillEvidenceKeys = map[string]bool{
	"kind": true, "environment_id": true, "artifact_sha256": true, "succeeded": true,
	"rto_seconds": true, "prevalidate_seconds": true, "verify_seconds": true,
	"checks": true, "divergences": true, "cli_version": true, "note": true,
	"performed_at": true, "integrity": true,
}

func registerRestoreDrillRoutes(mux *http.ServeMux, store InventoryStore, targets map[string]string) {
	backend, ok := store.(restoreDrillStore)
	if !ok {
		return
	}
	base := "/api/v1/environments/{id}"

	mux.HandleFunc("GET "+base+"/restore-drill-context", func(w http.ResponseWriter, r *http.Request) {
		env := r.PathValue("id")
		if !uuidPattern.MatchString(env) {
			writeError(w, http.StatusBadRequest, CodeValidation, "Ambiente inválido.")
			return
		}
		fps := fingerprintsForEnvironment(targets, env)
		out, err := backend.GetRestoreDrillContext(r.Context(), env, fps)
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Falha ao carregar o contexto de restauração.")
			return
		}
		writeJSON(w, http.StatusOK, out)
	})

	mux.HandleFunc("POST "+base+"/restore-drills", func(w http.ResponseWriter, r *http.Request) {
		env := r.PathValue("id")
		if !uuidPattern.MatchString(env) {
			writeError(w, http.StatusBadRequest, CodeValidation, "Ambiente inválido.")
			return
		}
		raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			writeError(w, http.StatusBadRequest, CodeValidation, "Corpo inválido.")
			return
		}
		in, err := decodeRestoreDrillEvidence(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, CodeValidation, err.Error())
			return
		}
		var tokenID *string
		if tok := requestEnvToken(r); tok != nil {
			id := tok.ID
			tokenID = &id
		}
		drill, err := backend.RecordTargetRestoreDrill(r.Context(), env, tokenID, in)
		if err != nil {
			writeError(w, http.StatusBadRequest, CodeValidation, "Evidência recusada.")
			return
		}
		emitRestoreDrillFindings(r.Context(), backend, env, drill)
		updateRestoreDrillMetrics(r.Context(), backend, env)
		writeJSON(w, http.StatusCreated, drill)
	})

	mux.HandleFunc("GET "+base+"/restore-drills", func(w http.ResponseWriter, r *http.Request) {
		env := r.PathValue("id")
		if !uuidPattern.MatchString(env) {
			writeError(w, http.StatusBadRequest, CodeValidation, "Ambiente inválido.")
			return
		}
		items, total, err := backend.ListTargetRestoreDrills(r.Context(), env, 50, 0)
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Falha ao listar ensaios.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": total})
	})

	mux.HandleFunc("GET "+base+"/restore-drills/{drill}", func(w http.ResponseWriter, r *http.Request) {
		env := r.PathValue("id")
		drillID := r.PathValue("drill")
		if !uuidPattern.MatchString(env) || !uuidPattern.MatchString(drillID) {
			writeError(w, http.StatusBadRequest, CodeValidation, "Identificador inválido.")
			return
		}
		item, err := backend.GetTargetRestoreDrill(r.Context(), env, drillID)
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, CodeNotFound, "Ensaio não encontrado.")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Falha ao carregar o ensaio.")
			return
		}
		writeJSON(w, http.StatusOK, item)
	})

	mux.HandleFunc("GET "+base+"/restore-drill-policy", func(w http.ResponseWriter, r *http.Request) {
		env := r.PathValue("id")
		if !uuidPattern.MatchString(env) {
			writeError(w, http.StatusBadRequest, CodeValidation, "Ambiente inválido.")
			return
		}
		pol, err := backend.GetTargetRestorePolicy(r.Context(), env)
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Falha ao carregar a política.")
			return
		}
		writeJSON(w, http.StatusOK, pol)
	})

	mux.HandleFunc("PUT "+base+"/restore-drill-policy", func(w http.ResponseWriter, r *http.Request) {
		env := r.PathValue("id")
		user := requestIdentity(r)
		if user == nil || user.Role != "operator" {
			writeError(w, http.StatusForbidden, CodeUnavailable, "Somente um operador configura RTO e prazo.")
			return
		}
		var body struct {
			RTOTargetSeconds int `json:"rto_target_seconds"`
			MaxAgeDays       int `json:"max_age_days"`
		}
		if !uuidPattern.MatchString(env) || json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body) != nil {
			writeError(w, http.StatusBadRequest, CodeValidation, "Política inválida.")
			return
		}
		pol, err := backend.UpsertTargetRestorePolicy(r.Context(), env, user.Username, body.RTOTargetSeconds, body.MaxAgeDays)
		if err != nil {
			writeError(w, http.StatusBadRequest, CodeValidation, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, pol)
	})
}

func fingerprintsForEnvironment(targets map[string]string, env string) []string {
	if targets == nil {
		targets = config.LoadTargetDSNs()
	}
	dsn := config.DSNForEnvironment(targets, env)
	if dsn == "" {
		return []string{}
	}
	fp := restoreverify.FingerprintDSN(dsn)
	if fp == "" {
		return []string{}
	}
	return []string{fp}
}

func decodeRestoreDrillEvidence(raw []byte) (repository.RecordTargetRestoreDrillInput, error) {
	var in repository.RecordTargetRestoreDrillInput
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(raw, &keys); err != nil || keys == nil {
		return in, errors.New("body must be a JSON object")
	}
	for key := range keys {
		if !restoreDrillEvidenceKeys[key] {
			return in, errors.New("campo não permitido: " + key)
		}
	}
	if err := restoreverify.RefuseRowValues(raw); err != nil {
		return in, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		return in, err
	}
	if in.Kind == "" {
		in.Kind = restoreverify.KindTargetRestoreDrill
	}
	if in.Kind != restoreverify.KindTargetRestoreDrill {
		return in, errors.New("kind inválido")
	}
	return in, nil
}

func emitRestoreDrillFindings(ctx context.Context, store restoreDrillStore, env string, drill *repository.TargetRestoreDrill) {
	if drill == nil {
		return
	}
	upsert := func(findingType, severity, title, summary string, evidence map[string]any) {
		ev, _ := json.Marshal(evidence)
		_, _ = store.UpsertFinding(ctx, repository.UpsertFindingParams{
			EnvironmentID: env,
			FindingType:   findingType,
			Severity:      severity,
			Title:         title,
			Summary:       summary,
			ObjectType:    "environment",
			ObjectKey:     env,
			DedupKey:      findingType + "|" + env + "|" + drill.ID,
			RuleID:        findingType,
			RuleVersion:   "1",
			Category:      "backup",
			Evidence:      ev,
		})
	}
	if !drill.Succeeded {
		upsert("backup.restore_failed", "critical",
			"Ensaio de restauração falhou",
			"Pré-validação, restauração ou verificação física falhou no ensaio target_restore_drill.",
			map[string]any{"drill_id": drill.ID, "artifact_sha256": drill.ArtifactSHA256, "note": drill.Note})
	}
	var divs []restoreverify.Divergence
	_ = json.Unmarshal(drill.Divergences, &divs)
	for _, d := range divs {
		sev := d.Severity
		if sev == "" {
			sev = "medium"
		}
		upsert("backup.restore_divergence", sev,
			"Divergência na restauração: "+d.Table,
			"Verificação "+d.Kind+" divergiu na cópia restaurada.",
			map[string]any{"drill_id": drill.ID, "table": d.Table, "kind": d.Kind, "detail": d.Detail})
	}
	pol, err := store.GetTargetRestorePolicy(ctx, env)
	if err == nil && pol != nil && drill.RTOSeconds != nil && *drill.RTOSeconds > pol.RTOTargetSeconds {
		upsert("backup.rto_exceeded", "high",
			"RTO do ensaio excedeu o alvo",
			"RTO medido acima do RTO-alvo do ambiente.",
			map[string]any{"drill_id": drill.ID, "rto_seconds": *drill.RTOSeconds, "rto_target_seconds": pol.RTOTargetSeconds})
	}
}

func updateRestoreDrillMetrics(ctx context.Context, store restoreDrillStore, env string) {
	last, err := store.LastSuccessfulTargetRestoreDrill(ctx, env)
	age := uint64(0)
	rto := uint64(0)
	if err == nil && last != nil {
		sec := time.Since(last.PerformedAt).Seconds()
		if sec < 0 {
			sec = 0
		}
		age = uint64(sec)
		if last.RTOSeconds != nil && *last.RTOSeconds >= 0 {
			rto = uint64(*last.RTOSeconds)
		}
	}
	observability.DefaultMetrics.SetTargetRestoreDrillAge(env, age)
	observability.DefaultMetrics.SetTargetRestoreRTO(env, rto)
}
