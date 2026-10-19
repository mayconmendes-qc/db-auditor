package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/osmendes/db-auditor/internal/observability"
	"github.com/osmendes/db-auditor/internal/repository"
)

type accessReviewStore interface {
	ListAccessReviewOwners(context.Context, string) ([]repository.AccessReviewOwner, error)
	ReplaceAccessReviewOwners(context.Context, string, string, []repository.AccessReviewOwnerInput) ([]repository.AccessReviewOwner, error)
	CreateAccessReviewCampaign(context.Context, string, repository.CreateAccessReviewCampaignInput) (*repository.AccessReviewCampaign, error)
	ListAccessReviewCampaigns(context.Context, string, int, int) ([]repository.AccessReviewCampaign, int, error)
	GetAccessReviewCampaign(context.Context, string, string) (*repository.AccessReviewCampaign, error)
	ListAccessReviewMatrix(context.Context, string, string, repository.AccessReviewMatrixFilter) ([]repository.AccessReviewItem, int, error)
	CloseAccessReviewCampaign(context.Context, string, string, string) (*repository.AccessReviewCampaign, error)
	PostAccessReviewDecisions(context.Context, string, string, string, string, string, []repository.AccessReviewDecisionInput) ([]repository.AccessReviewDecision, error)
	DiffAccessReviewCampaign(context.Context, string, string) (*repository.AccessReviewDiff, error)
	StreamAccessReviewCSV(context.Context, string, string, io.Writer) (int, error)
	LogAuditorOperation(context.Context, *repository.AuditorUser, string, string, string, string) error
}

func registerAccessReviewRoutes(mux *http.ServeMux, store InventoryStore) {
	backend, ok := store.(accessReviewStore)
	if !ok {
		return
	}
	base := "/api/v1/environments/{id}"

	mux.HandleFunc("GET "+base+"/access-review-owners", func(w http.ResponseWriter, r *http.Request) {
		env := r.PathValue("id")
		if !uuidPattern.MatchString(env) {
			writeError(w, http.StatusBadRequest, CodeValidation, "Ambiente inválido.")
			return
		}
		items, err := backend.ListAccessReviewOwners(r.Context(), env)
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Falha ao listar responsáveis.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	})

	mux.HandleFunc("PUT "+base+"/access-review-owners", func(w http.ResponseWriter, r *http.Request) {
		env := r.PathValue("id")
		user := requestIdentity(r)
		if !uuidPattern.MatchString(env) || user == nil {
			writeError(w, http.StatusBadRequest, CodeValidation, "Ambiente inválido.")
			return
		}
		var body struct {
			Items []repository.AccessReviewOwnerInput `json:"items"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body) != nil {
			writeError(w, http.StatusBadRequest, CodeValidation, "Corpo inválido.")
			return
		}
		items, err := backend.ReplaceAccessReviewOwners(r.Context(), env, user.Username, body.Items)
		if errors.Is(err, repository.ErrAccessReviewOwnerAccess) {
			writeError(w, http.StatusBadRequest, CodeValidation, "responsável sem acesso ao ambiente")
			return
		}
		if errors.Is(err, repository.ErrAccessReviewBadContact) || errors.Is(err, repository.ErrAccessReviewValidation) {
			writeError(w, http.StatusBadRequest, CodeValidation, "Responsável ou contato inválido.")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Falha ao gravar responsáveis.")
			return
		}
		_ = backend.LogAuditorOperation(r.Context(), user, "access_review.owners_replaced", env, "", "success")
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	})

	mux.HandleFunc("POST "+base+"/access-reviews", func(w http.ResponseWriter, r *http.Request) {
		env := r.PathValue("id")
		user := requestIdentity(r)
		if !uuidPattern.MatchString(env) || user == nil {
			writeError(w, http.StatusBadRequest, CodeValidation, "Ambiente inválido.")
			return
		}
		var body struct {
			Name           string          `json:"name"`
			ReferenceRunID string          `json:"reference_run_id"`
			DueAt          string          `json:"due_at"`
			Scope          json.RawMessage `json:"scope"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body) != nil {
			writeError(w, http.StatusBadRequest, CodeValidation, "Corpo inválido.")
			return
		}
		due, err := time.Parse(time.RFC3339, strings.TrimSpace(body.DueAt))
		if err != nil {
			writeError(w, http.StatusBadRequest, CodeValidation, "Prazo inválido.")
			return
		}
		camp, err := backend.CreateAccessReviewCampaign(r.Context(), env, repository.CreateAccessReviewCampaignInput{
			Name: body.Name, ReferenceRunID: body.ReferenceRunID, DueAt: due, Scope: body.Scope,
			CreatedBy: user.Username, CreatedByID: user.ID,
		})
		if errors.Is(err, repository.ErrAccessReviewIncompleteRun) {
			writeError(w, http.StatusConflict, CodeConflict, "campanha exige coleta completa")
			return
		}
		if errors.Is(err, repository.ErrAccessReviewValidation) {
			writeError(w, http.StatusBadRequest, CodeValidation, "Dados da campanha inválidos.")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Falha ao criar campanha.")
			return
		}
		observability.DefaultMetrics.IncAccessReviewCampaign(camp.Status)
		_ = backend.LogAuditorOperation(r.Context(), user, "access_review.campaign_created", env, camp.ID, "success")
		writeJSON(w, http.StatusCreated, camp)
	})

	mux.HandleFunc("GET "+base+"/access-reviews", func(w http.ResponseWriter, r *http.Request) {
		env := r.PathValue("id")
		limit, offset, ok := parseListPage(r, 20)
		if !uuidPattern.MatchString(env) || !ok {
			writeError(w, http.StatusBadRequest, CodeValidation, "Ambiente ou paginação inválidos.")
			return
		}
		items, total, err := backend.ListAccessReviewCampaigns(r.Context(), env, limit, offset)
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Falha ao listar campanhas.")
			return
		}
		writePage(w, items, limit, offset, total)
	})

	mux.HandleFunc("GET "+base+"/access-reviews/{campaign}", func(w http.ResponseWriter, r *http.Request) {
		env, id := r.PathValue("id"), r.PathValue("campaign")
		if !uuidPattern.MatchString(env) || !uuidPattern.MatchString(id) {
			writeError(w, http.StatusBadRequest, CodeValidation, "Ambiente ou campanha inválidos.")
			return
		}
		camp, err := backend.GetAccessReviewCampaign(r.Context(), env, id)
		if errors.Is(err, repository.ErrAccessReviewNotFound) {
			writeError(w, http.StatusNotFound, CodeNotFound, "Campanha não encontrada.")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Falha ao carregar campanha.")
			return
		}
		writeJSON(w, http.StatusOK, camp)
	})

	mux.HandleFunc("GET "+base+"/access-reviews/{campaign}/matrix", func(w http.ResponseWriter, r *http.Request) {
		env, id := r.PathValue("id"), r.PathValue("campaign")
		limit, offset, ok := parseListPage(r, 50)
		if !uuidPattern.MatchString(env) || !uuidPattern.MatchString(id) || !ok {
			writeError(w, http.StatusBadRequest, CodeValidation, "Ambiente, campanha ou paginação inválidos.")
			return
		}
		q := r.URL.Query()
		f := repository.AccessReviewMatrixFilter{
			Account: q.Get("account"), Object: q.Get("object"), Privilege: q.Get("privilege"),
			Origin: q.Get("origin"), Highlight: q.Get("highlight"), Owner: q.Get("owner"),
			Limit: limit, Offset: offset,
		}
		if v := q.Get("pending"); v == "true" || v == "1" {
			t := true
			f.Pending = &t
		} else if v == "false" || v == "0" {
			t := false
			f.Pending = &t
		}
		if q.Get("mine") == "true" || q.Get("mine") == "1" {
			if user := requestIdentity(r); user != nil {
				f.MineUser = user.ID
			}
		}
		items, total, err := backend.ListAccessReviewMatrix(r.Context(), env, id, f)
		if errors.Is(err, repository.ErrAccessReviewNotFound) {
			writeError(w, http.StatusNotFound, CodeNotFound, "Campanha não encontrada.")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Falha ao carregar matriz.")
			return
		}
		writePage(w, items, limit, offset, total)
	})

	mux.HandleFunc("POST "+base+"/access-reviews/{campaign}/decisions", func(w http.ResponseWriter, r *http.Request) {
		env, id := r.PathValue("id"), r.PathValue("campaign")
		user := requestIdentity(r)
		if !uuidPattern.MatchString(env) || !uuidPattern.MatchString(id) || user == nil {
			writeError(w, http.StatusBadRequest, CodeValidation, "Ambiente ou campanha inválidos.")
			return
		}
		var body struct {
			Decisions []repository.AccessReviewDecisionInput `json:"decisions"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body) != nil || len(body.Decisions) == 0 {
			writeError(w, http.StatusBadRequest, CodeValidation, "Decisões inválidas.")
			return
		}
		out, err := backend.PostAccessReviewDecisions(r.Context(), env, id, user.Username, user.ID, user.Role, body.Decisions)
		switch {
		case errors.Is(err, repository.ErrAccessReviewKeepJustify):
			writeError(w, http.StatusBadRequest, CodeValidation, "Justificativa obrigatória para manter.")
		case errors.Is(err, repository.ErrAccessReviewNotOwner):
			writeError(w, http.StatusForbidden, CodeUnavailable, "Apenas o responsável da linha ou o operator pode decidir.")
		case errors.Is(err, repository.ErrAccessReviewClosed):
			writeError(w, http.StatusConflict, CodeConflict, "Campanha concluída.")
		case errors.Is(err, repository.ErrAccessReviewValidation):
			writeError(w, http.StatusBadRequest, CodeValidation, "Decisão inválida.")
		case err != nil:
			writeError(w, http.StatusInternalServerError, CodeInternal, "Falha ao registrar decisões.")
		default:
			observability.DefaultMetrics.AddAccessReviewPendingDelta(-len(out))
			_ = backend.LogAuditorOperation(r.Context(), user, "access_review.decisions", env, id, "success")
			writeJSON(w, http.StatusCreated, map[string]any{"items": out})
		}
	})

	mux.HandleFunc("POST "+base+"/access-reviews/{campaign}/close", func(w http.ResponseWriter, r *http.Request) {
		env, id := r.PathValue("id"), r.PathValue("campaign")
		user := requestIdentity(r)
		if !uuidPattern.MatchString(env) || !uuidPattern.MatchString(id) || user == nil {
			writeError(w, http.StatusBadRequest, CodeValidation, "Ambiente ou campanha inválidos.")
			return
		}
		camp, err := backend.CloseAccessReviewCampaign(r.Context(), env, id, user.Username)
		if errors.Is(err, repository.ErrAccessReviewClosed) {
			writeError(w, http.StatusConflict, CodeConflict, "Campanha já concluída.")
			return
		}
		if errors.Is(err, repository.ErrAccessReviewNotFound) {
			writeError(w, http.StatusNotFound, CodeNotFound, "Campanha não encontrada.")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Falha ao encerrar campanha.")
			return
		}
		observability.DefaultMetrics.IncAccessReviewCampaign("closed")
		_ = backend.LogAuditorOperation(r.Context(), user, "access_review.campaign_closed", env, id, "success")
		writeJSON(w, http.StatusOK, camp)
	})

	mux.HandleFunc("GET "+base+"/access-reviews/{campaign}/diff", func(w http.ResponseWriter, r *http.Request) {
		env, id := r.PathValue("id"), r.PathValue("campaign")
		if !uuidPattern.MatchString(env) || !uuidPattern.MatchString(id) {
			writeError(w, http.StatusBadRequest, CodeValidation, "Ambiente ou campanha inválidos.")
			return
		}
		diff, err := backend.DiffAccessReviewCampaign(r.Context(), env, id)
		if errors.Is(err, repository.ErrAccessReviewNotFound) {
			writeError(w, http.StatusNotFound, CodeNotFound, "Campanha não encontrada.")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Falha ao calcular diff.")
			return
		}
		writeJSON(w, http.StatusOK, diff)
	})

	mux.HandleFunc("GET "+base+"/access-reviews/{campaign}/export/download", func(w http.ResponseWriter, r *http.Request) {
		env, id := r.PathValue("id"), r.PathValue("campaign")
		if !uuidPattern.MatchString(env) || !uuidPattern.MatchString(id) {
			writeError(w, http.StatusBadRequest, CodeValidation, "Ambiente ou campanha inválidos.")
			return
		}
		format := strings.TrimSpace(r.URL.Query().Get("format"))
		if format != "" && format != "csv" {
			writeError(w, http.StatusBadRequest, CodeValidation, "Formato inválido (use csv).")
			return
		}
		w.Header().Set("Cache-Control", "private, no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="access-review-%s.csv"`, id))
		n, err := backend.StreamAccessReviewCSV(r.Context(), env, id, w)
		if errors.Is(err, repository.ErrAccessReviewNotFound) {
			writeError(w, http.StatusNotFound, CodeNotFound, "Campanha não encontrada.")
			return
		}
		if err != nil {
			// Headers may already be sent; best-effort.
			return
		}
		_ = n
	})
}
