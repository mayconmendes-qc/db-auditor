package api

import "net/http"

// API error codes (stable for clients).
const (
	CodeBadRequest          = "bad_request"
	CodeNotFound            = "not_found"
	CodeConflict            = "conflict"
	CodeInternal            = "internal_error"
	CodeUnavailable         = "unavailable"
	CodeValidation          = "validation_error"
	CodeEnvironmentRequired = "environment_required"
)

// ErrorBody is the standard JSON error envelope.
type ErrorBody struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

// writeError writes a Portuguese user-facing error with a stable code.
func writeError(w http.ResponseWriter, status int, code, message string) {
	if message == "" {
		message = defaultMessage(status)
	}
	if code == "" {
		code = defaultCode(status)
	}
	writeJSON(w, status, ErrorBody{Error: message, Code: code})
}

func defaultCode(status int) string {
	switch status {
	case http.StatusBadRequest:
		return CodeBadRequest
	case http.StatusNotFound:
		return CodeNotFound
	case http.StatusConflict:
		return CodeConflict
	case http.StatusServiceUnavailable:
		return CodeUnavailable
	default:
		return CodeInternal
	}
}

func defaultMessage(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "Requisição inválida."
	case http.StatusNotFound:
		return "Recurso não encontrado."
	case http.StatusConflict:
		return "Conflito ao processar a requisição."
	case http.StatusServiceUnavailable:
		return "Serviço temporariamente indisponível."
	default:
		return "Erro interno do servidor."
	}
}
