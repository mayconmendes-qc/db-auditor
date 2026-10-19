package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/osmendes/db-auditor/internal/config"
	"github.com/osmendes/db-auditor/internal/dqcontract"
	"github.com/osmendes/db-auditor/internal/repository"
)

type qualityContractStore interface {
	ListQualityContracts(context.Context, string) ([]repository.QualityContract, error)
	CreateQualityContract(context.Context, string, string, dqcontract.Document, string) (*repository.QualityContract, error)
	AddQualityContractVersion(context.Context, string, string, string, dqcontract.Document, string) (*repository.QualityContractVersion, error)
	ApproveQualityContractVersion(context.Context, string, string, string, string) (*repository.QualityContractVersion, error)
	ProposeQualityContractFromSnapshot(context.Context, string, string, string, string, string) (*repository.QualityContract, error)
	ImportQualityContractYAML(context.Context, string, string, []byte) (*repository.QualityContract, error)
	ExportQualityContractYAML(context.Context, string, string, string) (string, error)
	GetQualityContractVersion(context.Context, string, string, string) (*repository.QualityContractVersion, error)
	ExecuteQualityContractRun(context.Context, string, string, string, string, string, string, dqcontract.Document, string, string) (*repository.QualityContractRun, error)
	GetQualityContractRun(context.Context, string, string) (*repository.QualityContractRun, error)
	ListQualityContractRuns(context.Context, string, string, int) ([]repository.QualityContractRunSummary, error)
	ListQualityContractTrend(context.Context, string, string, int) ([]repository.QualityContractTrendPoint, error)
}

func registerQualityContractRoutes(mux *http.ServeMux, store InventoryStore, targets map[string]string) {
	backend, ok := store.(qualityContractStore)
	if !ok {
		return
	}
	base := "/api/v1/environments/{id}"

	mux.HandleFunc("GET "+base+"/quality-contracts", func(w http.ResponseWriter, r *http.Request) {
		env := r.PathValue("id")
		if !uuidPattern.MatchString(env) {
			writeError(w, 400, CodeValidation, "Ambiente inválido.")
			return
		}
		items, err := backend.ListQualityContracts(r.Context(), env)
		if err != nil {
			writeError(w, 500, CodeInternal, "Falha ao listar contratos.")
			return
		}
		viewer := isViewerOnly(r)
		if viewer {
			for i := range items {
				if items[i].LatestVersion != nil {
					dqcontract.RedactDomainForViewer(&items[i].LatestVersion.Document)
				}
			}
		}
		writeJSON(w, 200, map[string]any{
			"items":   items,
			"enabled": dqcontract.DataQualityEnabled(),
		})
	})

	mux.HandleFunc("POST "+base+"/quality-contracts", func(w http.ResponseWriter, r *http.Request) {
		env := r.PathValue("id")
		user := requestIdentity(r)
		if !uuidPattern.MatchString(env) || user == nil {
			writeError(w, 400, CodeValidation, "Ambiente inválido.")
			return
		}
		var body struct {
			Document dqcontract.Document `json:"document"`
			YAML     string              `json:"yaml"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
			writeError(w, 400, CodeValidation, "Corpo inválido.")
			return
		}
		var item *repository.QualityContract
		var err error
		if strings.TrimSpace(body.YAML) != "" {
			item, err = backend.ImportQualityContractYAML(r.Context(), env, user.Username, []byte(body.YAML))
		} else {
			item, err = backend.CreateQualityContract(r.Context(), env, user.Username, body.Document, "")
		}
		writeContractWriteResult(w, item, err)
	})

	mux.HandleFunc("POST "+base+"/quality-contracts/import", func(w http.ResponseWriter, r *http.Request) {
		env := r.PathValue("id")
		user := requestIdentity(r)
		if !uuidPattern.MatchString(env) || user == nil {
			writeError(w, 400, CodeValidation, "Ambiente inválido.")
			return
		}
		var body struct {
			YAML string `json:"yaml"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil || strings.TrimSpace(body.YAML) == "" {
			writeError(w, 400, CodeValidation, "YAML inválido.")
			return
		}
		item, err := backend.ImportQualityContractYAML(r.Context(), env, user.Username, []byte(body.YAML))
		writeContractWriteResult(w, item, err)
	})

	mux.HandleFunc("POST "+base+"/quality-contracts/proposals", func(w http.ResponseWriter, r *http.Request) {
		env := r.PathValue("id")
		user := requestIdentity(r)
		if !uuidPattern.MatchString(env) || user == nil {
			writeError(w, 400, CodeValidation, "Ambiente inválido.")
			return
		}
		var body struct {
			Database string `json:"database"`
			Schema   string `json:"schema"`
			Table    string `json:"table"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body); err != nil ||
			body.Database == "" || body.Schema == "" || body.Table == "" {
			writeError(w, 400, CodeValidation, "Informe database, schema e table.")
			return
		}
		item, err := backend.ProposeQualityContractFromSnapshot(r.Context(), env, user.Username, body.Database, body.Schema, body.Table)
		if errors.Is(err, repository.ErrQualityContractNoSnapshot) {
			writeError(w, 409, CodeConflict, "Proposta exige coleta completa com a tabela no snapshot.")
			return
		}
		writeContractWriteResult(w, item, err)
	})

	mux.HandleFunc("POST "+base+"/quality-contracts/{contract}/versions", func(w http.ResponseWriter, r *http.Request) {
		env, contractID := r.PathValue("id"), r.PathValue("contract")
		user := requestIdentity(r)
		if !uuidPattern.MatchString(env) || !uuidPattern.MatchString(contractID) || user == nil {
			writeError(w, 400, CodeValidation, "Ambiente ou contrato inválido.")
			return
		}
		var body struct {
			Document dqcontract.Document `json:"document"`
			YAML     string              `json:"yaml"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
			writeError(w, 400, CodeValidation, "Corpo inválido.")
			return
		}
		doc := body.Document
		src := ""
		if strings.TrimSpace(body.YAML) != "" {
			parsed, norm, err := dqcontract.ParseYAML([]byte(body.YAML), dqcontract.ProcessMaxSample())
			if err != nil {
				writeYAMLError(w, err)
				return
			}
			doc, src = parsed, norm
		}
		item, err := backend.AddQualityContractVersion(r.Context(), env, contractID, user.Username, doc, src)
		if errors.Is(err, repository.ErrQualityContractNotFound) {
			writeError(w, 404, CodeNotFound, "Contrato não encontrado.")
			return
		}
		if err != nil {
			if ye, ok := err.(*dqcontract.YAMLError); ok {
				writeYAMLError(w, ye)
				return
			}
			writeError(w, 400, CodeValidation, err.Error())
			return
		}
		writeJSON(w, 201, item)
	})

	mux.HandleFunc("POST "+base+"/quality-contracts/{contract}/versions/{version}/approve", func(w http.ResponseWriter, r *http.Request) {
		env, contractID, versionID := r.PathValue("id"), r.PathValue("contract"), r.PathValue("version")
		user := requestIdentity(r)
		if !uuidPattern.MatchString(env) || !uuidPattern.MatchString(contractID) || !uuidPattern.MatchString(versionID) || user == nil {
			writeError(w, 400, CodeValidation, "Parâmetros inválidos.")
			return
		}
		item, err := backend.ApproveQualityContractVersion(r.Context(), env, contractID, versionID, user.Username)
		if errors.Is(err, repository.ErrQualityContractNotFound) {
			writeError(w, 404, CodeNotFound, "Versão não encontrada.")
			return
		}
		if errors.Is(err, repository.ErrQualityContractBadState) {
			writeError(w, 409, CodeConflict, "Somente versões propostas podem ser aprovadas.")
			return
		}
		if err != nil {
			writeError(w, 500, CodeInternal, "Falha ao aprovar.")
			return
		}
		writeJSON(w, 200, item)
	})

	mux.HandleFunc("GET "+base+"/quality-contracts/{contract}/versions/{version}/yaml", func(w http.ResponseWriter, r *http.Request) {
		env, contractID, versionID := r.PathValue("id"), r.PathValue("contract"), r.PathValue("version")
		if !uuidPattern.MatchString(env) || !uuidPattern.MatchString(contractID) || !uuidPattern.MatchString(versionID) {
			writeError(w, 400, CodeValidation, "Parâmetros inválidos.")
			return
		}
		yamlText, err := backend.ExportQualityContractYAML(r.Context(), env, contractID, versionID)
		if errors.Is(err, repository.ErrQualityContractNotFound) {
			writeError(w, 404, CodeNotFound, "Versão não encontrada.")
			return
		}
		if err != nil {
			writeError(w, 500, CodeInternal, "Falha ao exportar YAML.")
			return
		}
		w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(yamlText))
	})

	// Ad hoc / token runs without contract id in path.
	mux.HandleFunc("POST "+base+"/quality-contracts/runs", func(w http.ResponseWriter, r *http.Request) {
		handleContractRun(w, r, backend, targets, "")
	})

	mux.HandleFunc("POST "+base+"/quality-contracts/{contract}/runs", func(w http.ResponseWriter, r *http.Request) {
		handleContractRun(w, r, backend, targets, r.PathValue("contract"))
	})

	mux.HandleFunc("GET "+base+"/quality-contracts/{contract}/runs", func(w http.ResponseWriter, r *http.Request) {
		env, contractID := r.PathValue("id"), r.PathValue("contract")
		if !uuidPattern.MatchString(env) || !uuidPattern.MatchString(contractID) {
			writeError(w, 400, CodeValidation, "Parâmetros inválidos.")
			return
		}
		items, err := backend.ListQualityContractRuns(r.Context(), env, contractID, 50)
		if err != nil {
			writeError(w, 500, CodeInternal, "Falha ao listar execuções.")
			return
		}
		writeJSON(w, 200, map[string]any{"items": items})
	})

	mux.HandleFunc("GET "+base+"/quality-contracts/{contract}/runs/{run}", func(w http.ResponseWriter, r *http.Request) {
		env, runID := r.PathValue("id"), r.PathValue("run")
		if !uuidPattern.MatchString(env) || !uuidPattern.MatchString(runID) {
			writeError(w, 400, CodeValidation, "Parâmetros inválidos.")
			return
		}
		item, err := backend.GetQualityContractRun(r.Context(), env, runID)
		if errors.Is(err, repository.ErrQualityContractNotFound) {
			writeError(w, 404, CodeNotFound, "Execução não encontrada.")
			return
		}
		if err != nil {
			writeError(w, 500, CodeInternal, "Falha ao carregar execução.")
			return
		}
		writeJSON(w, 200, item)
	})

	mux.HandleFunc("GET "+base+"/quality-contracts/{contract}/runs/{run}/markdown", func(w http.ResponseWriter, r *http.Request) {
		env, runID := r.PathValue("id"), r.PathValue("run")
		if !uuidPattern.MatchString(env) || !uuidPattern.MatchString(runID) {
			writeError(w, 400, CodeValidation, "Parâmetros inválidos.")
			return
		}
		item, err := backend.GetQualityContractRun(r.Context(), env, runID)
		if errors.Is(err, repository.ErrQualityContractNotFound) {
			writeError(w, 404, CodeNotFound, "Execução não encontrada.")
			return
		}
		if err != nil {
			writeError(w, 500, CodeInternal, "Falha ao carregar execução.")
			return
		}
		md := dqcontract.RenderMarkdown(item.Report)
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(md))
	})

	mux.HandleFunc("GET "+base+"/quality-contracts/{contract}/trend", func(w http.ResponseWriter, r *http.Request) {
		env, contractID := r.PathValue("id"), r.PathValue("contract")
		if !uuidPattern.MatchString(env) || !uuidPattern.MatchString(contractID) {
			writeError(w, 400, CodeValidation, "Parâmetros inválidos.")
			return
		}
		items, err := backend.ListQualityContractTrend(r.Context(), env, contractID, 90)
		if err != nil {
			writeError(w, 500, CodeInternal, "Falha ao carregar tendência.")
			return
		}
		writeJSON(w, 200, map[string]any{"items": items, "sampling_note": dqcontract.SamplingNote})
	})

	// Immutable versions: reject PUT/PATCH with 405.
	mux.HandleFunc("PUT "+base+"/quality-contracts/{contract}/versions/{version}", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusMethodNotAllowed, CodeConflict, "Versões de contrato são imutáveis.")
	})
	mux.HandleFunc("PATCH "+base+"/quality-contracts/{contract}/versions/{version}", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusMethodNotAllowed, CodeConflict, "Versões de contrato são imutáveis.")
	})
}

func handleContractRun(w http.ResponseWriter, r *http.Request, backend qualityContractStore, targets map[string]string, contractID string) {
	env := r.PathValue("id")
	if !uuidPattern.MatchString(env) {
		writeError(w, 400, CodeValidation, "Ambiente inválido.")
		return
	}
	if contractID != "" && !uuidPattern.MatchString(contractID) {
		writeError(w, 400, CodeValidation, "Contrato inválido.")
		return
	}
	if !dqcontract.DataQualityEnabled() {
		writeError(w, 409, CodeConflict, "Diagnóstico de dados desativado. Solicite habilitação operacional.")
		return
	}
	var body struct {
		YAML      string `json:"yaml"`
		Adhoc     bool   `json:"adhoc"`
		VersionID string `json:"version_id"`
		FailOn    string `json:"fail_on"`
	}
	_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body)

	actor := "local"
	mode := dqcontract.ModeOnDemand
	tokenID := ""
	if tok := requestEnvToken(r); tok != nil {
		actor = "token:" + tok.PublicPrefix
		mode = dqcontract.ModeCLI
		tokenID = tok.ID
	} else if user := requestIdentity(r); user != nil {
		actor = user.Username
	}

	configured := targets
	if configured == nil {
		configured = config.LoadTargetDSNs()
	}
	dsn := config.DSNForEnvironment(configured, env)
	if dsn == "" {
		writeError(w, 503, CodeUnavailable, "Conexão de leitura do ambiente não configurada.")
		return
	}

	var adhoc dqcontract.Document
	if body.Adhoc || (contractID == "" && strings.TrimSpace(body.YAML) != "") {
		doc, _, err := dqcontract.ParseYAML([]byte(body.YAML), dqcontract.ProcessMaxSample())
		if err != nil {
			writeYAMLError(w, err)
			return
		}
		adhoc = doc
		mode = dqcontract.ModeCLI
		contractID = ""
	}

	run, err := backend.ExecuteQualityContractRun(r.Context(), env, dsn, actor, mode, contractID, body.VersionID, adhoc, tokenID, "")
	if errors.Is(err, repository.ErrQualityContractDisabled) {
		writeError(w, 409, CodeConflict, "Diagnóstico de dados desativado. Solicite habilitação operacional.")
		return
	}
	if errors.Is(err, repository.ErrQualityContractNotFound) {
		writeError(w, 404, CodeNotFound, "Contrato ou versão não encontrados.")
		return
	}
	if errors.Is(err, repository.ErrQualityContractBadState) {
		writeError(w, 409, CodeConflict, "Somente versões aprovadas podem ser executadas sob demanda.")
		return
	}
	if err != nil {
		msg := err.Error()
		if ye, ok := err.(*dqcontract.YAMLError); ok {
			writeYAMLError(w, ye)
			return
		}
		if stringsContainsValidation(msg) {
			writeError(w, 400, CodeValidation, msg)
			return
		}
		writeError(w, 500, CodeInternal, "Falha ao executar contrato: "+config.SanitizeError(err))
		return
	}
	writeJSON(w, 201, run.Report)
}

func writeContractWriteResult(w http.ResponseWriter, item *repository.QualityContract, err error) {
	if errors.Is(err, repository.ErrQualityContractConflict) {
		writeError(w, 409, CodeConflict, "Já existe contrato para esta tabela; crie uma nova versão.")
		return
	}
	if ye, ok := err.(*dqcontract.YAMLError); ok {
		writeYAMLError(w, ye)
		return
	}
	if err != nil {
		writeError(w, 400, CodeValidation, err.Error())
		return
	}
	writeJSON(w, 201, item)
}

func writeYAMLError(w http.ResponseWriter, err error) {
	ye, ok := err.(*dqcontract.YAMLError)
	if !ok {
		writeError(w, 400, CodeValidation, err.Error())
		return
	}
	writeJSON(w, 400, map[string]any{
		"error": map[string]any{
			"code":    CodeValidation,
			"message": ye.Error(),
			"line":    ye.Line,
			"field":   ye.Field,
		},
	})
}

func isViewerOnly(r *http.Request) bool {
	user := requestIdentity(r)
	if user == nil {
		return true
	}
	return user.Role == "viewer"
}

func stringsContainsValidation(msg string) bool {
	lower := strings.ToLower(msg)
	return strings.Contains(lower, "sample_limit") || strings.Contains(lower, "inválid") || strings.Contains(lower, "obrigat")
}
