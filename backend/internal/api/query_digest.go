package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/osmendes/db-auditor/internal/repository"
)

type queryDigestStore interface {
	ListQueryDigests(context.Context, string, int, int) ([]repository.QueryDigest, int, error)
	GetQueryDigest(context.Context, string, string) (*repository.QueryDigest, error)
	GenerateQueryDigest(context.Context, string, time.Time) (*repository.QueryDigest, error)
	GetQueryDigestConfig(context.Context, string) (*repository.QueryDigestConfig, error)
	UpsertQueryDigestConfig(context.Context, string, int, json.RawMessage, bool, string) (*repository.QueryDigestConfig, error)
	DeliverQueryDigest(context.Context, *repository.QueryDigest) error
}

func registerQueryDigestRoutes(mux *http.ServeMux, store InventoryStore) {
	backend, ok := store.(queryDigestStore)
	if !ok {
		return
	}
	base := "/api/v1/environments/{id}"
	mux.HandleFunc("GET "+base+"/query-digests", func(w http.ResponseWriter, r *http.Request) {
		env := r.PathValue("id")
		limit, offset, ok := parseListPage(r, 20)
		if !uuidPattern.MatchString(env) || !ok {
			writeError(w, http.StatusBadRequest, CodeValidation, "Ambiente ou paginação inválidos.")
			return
		}
		items, total, err := backend.ListQueryDigests(r.Context(), env, limit, offset)
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Falha ao listar digests.")
			return
		}
		writePage(w, items, limit, offset, total)
	})
	mux.HandleFunc("GET "+base+"/query-digests/{digest}", func(w http.ResponseWriter, r *http.Request) {
		env, id := r.PathValue("id"), r.PathValue("digest")
		if !uuidPattern.MatchString(env) || !uuidPattern.MatchString(id) {
			writeError(w, http.StatusBadRequest, CodeValidation, "Ambiente ou digest inválido.")
			return
		}
		item, err := backend.GetQueryDigest(r.Context(), env, id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Falha ao carregar digest.")
			return
		}
		if item == nil {
			writeError(w, http.StatusNotFound, CodeNotFound, "Digest não encontrado.")
			return
		}
		writeJSON(w, http.StatusOK, item)
	})
	mux.HandleFunc("GET "+base+"/query-digests/{digest}/markdown", func(w http.ResponseWriter, r *http.Request) {
		env, id := r.PathValue("id"), r.PathValue("digest")
		if !uuidPattern.MatchString(env) || !uuidPattern.MatchString(id) {
			writeError(w, http.StatusBadRequest, CodeValidation, "Ambiente ou digest inválido.")
			return
		}
		item, err := backend.GetQueryDigest(r.Context(), env, id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Falha ao carregar digest.")
			return
		}
		if item == nil {
			writeError(w, http.StatusNotFound, CodeNotFound, "Digest não encontrado.")
			return
		}
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(item.Markdown))
	})
	mux.HandleFunc("POST "+base+"/query-digests", func(w http.ResponseWriter, r *http.Request) {
		env := r.PathValue("id")
		if !uuidPattern.MatchString(env) {
			writeError(w, http.StatusBadRequest, CodeValidation, "Ambiente inválido.")
			return
		}
		var body struct {
			Reference string `json:"reference"`
		}
		_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048)).Decode(&body)
		ref := time.Now().UTC()
		if body.Reference != "" {
			parsed, err := time.Parse(time.RFC3339, body.Reference)
			if err != nil {
				writeError(w, http.StatusBadRequest, CodeValidation, "Data de referência inválida.")
				return
			}
			ref = parsed.UTC()
		}
		item, err := backend.GenerateQueryDigest(r.Context(), env, ref)
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Falha ao gerar digest.")
			return
		}
		cfg, _ := backend.GetQueryDigestConfig(r.Context(), env)
		if cfg != nil && cfg.DeliveryEnabled {
			_ = backend.DeliverQueryDigest(r.Context(), item)
		}
		writeJSON(w, http.StatusCreated, item)
	})
	mux.HandleFunc("GET "+base+"/query-digest-config", func(w http.ResponseWriter, r *http.Request) {
		env := r.PathValue("id")
		if !uuidPattern.MatchString(env) {
			writeError(w, http.StatusBadRequest, CodeValidation, "Ambiente inválido.")
			return
		}
		item, err := backend.GetQueryDigestConfig(r.Context(), env)
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Falha ao carregar configuração.")
			return
		}
		writeJSON(w, http.StatusOK, item)
	})
	mux.HandleFunc("PUT "+base+"/query-digest-config", func(w http.ResponseWriter, r *http.Request) {
		env := r.PathValue("id")
		var body struct {
			WindowDays      int             `json:"window_days"`
			Thresholds      json.RawMessage `json:"thresholds"`
			DeliveryEnabled *bool           `json:"delivery_enabled"`
		}
		if !uuidPattern.MatchString(env) || json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&body) != nil {
			writeError(w, http.StatusBadRequest, CodeValidation, "Configuração inválida.")
			return
		}
		if body.WindowDays == 0 {
			body.WindowDays = 7
		}
		delivery := true
		if body.DeliveryEnabled != nil {
			delivery = *body.DeliveryEnabled
		}
		actor := "operator"
		if user := requestIdentity(r); user != nil {
			actor = user.Username
		}
		item, err := backend.UpsertQueryDigestConfig(r.Context(), env, body.WindowDays, body.Thresholds, delivery, actor)
		if err != nil {
			writeError(w, http.StatusBadRequest, CodeValidation, "Não foi possível salvar a configuração.")
			return
		}
		writeJSON(w, http.StatusOK, item)
	})
}
