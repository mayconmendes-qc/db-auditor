package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/osmendes/db-auditor/internal/observability"
	"github.com/osmendes/db-auditor/internal/repository"
)

type envTokenStore interface {
	CreateEnvAPIToken(context.Context, repository.CreateEnvAPITokenParams) (*repository.CreateEnvAPITokenResult, error)
	ListEnvAPITokens(context.Context, string) ([]repository.EnvAPIToken, error)
	RevokeEnvAPIToken(context.Context, string, string) error
}

func registerEnvironmentTokenRoutes(mux *http.ServeMux, store InventoryStore) {
	backend, ok := store.(envTokenStore)
	if !ok {
		return
	}
	base := "/api/v1/environments/{id}/api-tokens"
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		env := r.PathValue("id")
		user := requestIdentity(r)
		if user == nil || user.Role != "operator" {
			writeError(w, http.StatusForbidden, CodeUnavailable, "Somente um operador cria tokens de API.")
			return
		}
		if !uuidPattern.MatchString(env) {
			writeError(w, http.StatusBadRequest, CodeValidation, "Ambiente inválido.")
			return
		}
		var body struct {
			Name      string   `json:"name"`
			Scopes    []string `json:"scopes"`
			ExpiresAt string   `json:"expires_at"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body) != nil {
			writeError(w, http.StatusBadRequest, CodeValidation, "Corpo inválido.")
			return
		}
		expiresAt := time.Now().UTC().Add(90 * 24 * time.Hour)
		if strings.TrimSpace(body.ExpiresAt) != "" {
			parsed, err := time.Parse(time.RFC3339, body.ExpiresAt)
			if err != nil {
				writeError(w, http.StatusBadRequest, CodeValidation, "expires_at inválido (RFC3339).")
				return
			}
			expiresAt = parsed.UTC()
		}
		result, err := backend.CreateEnvAPIToken(r.Context(), repository.CreateEnvAPITokenParams{
			EnvironmentID: env,
			Name:          body.Name,
			Scopes:        body.Scopes,
			CreatedBy:     user.ID,
			ExpiresAt:     expiresAt,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, CodeValidation, "Não foi possível criar o token.")
			return
		}
		observability.DefaultMetrics.IncEnvTokenRequest("create", "success")
		// Plaintext returned once; never logged.
		writeJSON(w, http.StatusCreated, map[string]any{
			"token":     result.Token,
			"plaintext": result.Plaintext,
		})
	})
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		env := r.PathValue("id")
		user := requestIdentity(r)
		if user == nil || user.Role != "operator" {
			writeError(w, http.StatusForbidden, CodeUnavailable, "Somente um operador lista tokens de API.")
			return
		}
		if !uuidPattern.MatchString(env) {
			writeError(w, http.StatusBadRequest, CodeValidation, "Ambiente inválido.")
			return
		}
		items, err := backend.ListEnvAPITokens(r.Context(), env)
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Falha ao listar tokens.")
			return
		}
		if items == nil {
			items = []repository.EnvAPIToken{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	})
	mux.HandleFunc("DELETE "+base+"/{token}", func(w http.ResponseWriter, r *http.Request) {
		env, id := r.PathValue("id"), r.PathValue("token")
		user := requestIdentity(r)
		if user == nil || user.Role != "operator" {
			writeError(w, http.StatusForbidden, CodeUnavailable, "Somente um operador revoga tokens.")
			return
		}
		if !uuidPattern.MatchString(env) || !uuidPattern.MatchString(id) {
			writeError(w, http.StatusBadRequest, CodeValidation, "Identificador inválido.")
			return
		}
		err := backend.RevokeEnvAPIToken(r.Context(), env, id)
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, CodeNotFound, "Token não encontrado.")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Falha ao revogar o token.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
	})
}
