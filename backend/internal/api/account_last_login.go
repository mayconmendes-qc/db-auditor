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

	"github.com/osmendes/db-auditor/internal/observability"
	"github.com/osmendes/db-auditor/internal/repository"
)

const maxAccountLastLoginBody = 512 << 10 // 512 KiB

type accountLastLoginStore interface {
	UpsertAccountLastLogins(context.Context, string, *string, []repository.AccountLastLoginInput) (int, error)
	ListAccountLastLogins(context.Context, string) ([]repository.AccountLastLoginObservation, error)
}

var accountLastLoginTopKeys = map[string]bool{
	"schema_version": true,
	"entries":        true,
}

var accountLastLoginEntryKeys = map[string]bool{
	"role":          true,
	"last_login_at": true,
}

func registerAccountLastLoginRoutes(mux *http.ServeMux, store InventoryStore) {
	backend, ok := store.(accountLastLoginStore)
	if !ok {
		return
	}
	mux.HandleFunc("POST /api/v1/environments/{id}/account-last-logins", func(w http.ResponseWriter, r *http.Request) {
		env := r.PathValue("id")
		if !uuidPattern.MatchString(env) {
			observability.DefaultMetrics.IncLastLoginIngest("rejected")
			writeError(w, http.StatusBadRequest, CodeValidation, "Ambiente inválido.")
			return
		}
		tok := requestEnvToken(r)
		if tok == nil {
			observability.DefaultMetrics.IncLastLoginIngest("rejected")
			writeError(w, http.StatusForbidden, CodeUnavailable, "Somente token de ambiente com escopo last-login:write.")
			return
		}
		raw, err := io.ReadAll(io.LimitReader(r.Body, maxAccountLastLoginBody+1))
		if err != nil {
			observability.DefaultMetrics.IncLastLoginIngest("rejected")
			writeError(w, http.StatusBadRequest, CodeValidation, "Corpo inválido.")
			return
		}
		if len(raw) > maxAccountLastLoginBody {
			observability.DefaultMetrics.IncLastLoginIngest("rejected")
			writeError(w, http.StatusRequestEntityTooLarge, CodeValidation, "Corpo excede o limite.")
			return
		}
		entries, err := decodeAccountLastLoginBody(raw)
		if err != nil {
			observability.DefaultMetrics.IncLastLoginIngest("rejected")
			writeError(w, http.StatusBadRequest, CodeValidation, err.Error())
			return
		}
		tokenID := tok.ID
		n, err := backend.UpsertAccountLastLogins(r.Context(), env, &tokenID, entries)
		if errors.Is(err, repository.ErrAccountLastLoginTooMany) {
			observability.DefaultMetrics.IncLastLoginIngest("rejected")
			writeError(w, http.StatusBadRequest, CodeValidation, "Demasiadas entradas.")
			return
		}
		if errors.Is(err, repository.ErrAccountLastLoginValidation) {
			observability.DefaultMetrics.IncLastLoginIngest("rejected")
			writeError(w, http.StatusBadRequest, CodeValidation, "Entrada inválida (role e last_login_at obrigatórios).")
			return
		}
		if err != nil {
			observability.DefaultMetrics.IncLastLoginIngest("rejected")
			writeError(w, http.StatusInternalServerError, CodeInternal, "Falha ao gravar último login.")
			return
		}
		observability.DefaultMetrics.IncLastLoginIngest("accepted")
		writeJSON(w, http.StatusOK, map[string]any{"upserted": n, "count": len(entries)})
	})
}

func decodeAccountLastLoginBody(raw []byte) ([]repository.AccountLastLoginInput, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil || top == nil {
		return nil, errors.New("corpo deve ser um objeto JSON")
	}
	for k := range top {
		if !accountLastLoginTopKeys[k] {
			return nil, errors.New("campo não permitido: " + k)
		}
	}
	entriesRaw, ok := top["entries"]
	if !ok {
		return nil, errors.New("entries obrigatório")
	}
	var rawEntries []json.RawMessage
	if err := json.Unmarshal(entriesRaw, &rawEntries); err != nil {
		return nil, errors.New("entries deve ser um array")
	}
	if len(rawEntries) > repository.MaxAccountLastLoginEntries {
		return nil, errors.New("demasiadas entradas")
	}
	out := make([]repository.AccountLastLoginInput, 0, len(rawEntries))
	for _, er := range rawEntries {
		var keys map[string]json.RawMessage
		if err := json.Unmarshal(er, &keys); err != nil || keys == nil {
			return nil, errors.New("cada entrada deve ser um objeto")
		}
		for k := range keys {
			if !accountLastLoginEntryKeys[k] {
				return nil, errors.New("campo não permitido: " + k)
			}
		}
		dec := json.NewDecoder(bytes.NewReader(er))
		dec.DisallowUnknownFields()
		var row struct {
			Role        string `json:"role"`
			LastLoginAt string `json:"last_login_at"`
		}
		if err := dec.Decode(&row); err != nil {
			return nil, errors.New("entrada inválida")
		}
		role := strings.TrimSpace(row.Role)
		if role == "" || len(role) > 128 {
			return nil, errors.New("role inválida")
		}
		ts, err := time.Parse(time.RFC3339, strings.TrimSpace(row.LastLoginAt))
		if err != nil {
			return nil, errors.New("last_login_at deve ser RFC3339")
		}
		out = append(out, repository.AccountLastLoginInput{Role: role, LastLoginAt: ts.UTC()})
	}
	return out, nil
}
