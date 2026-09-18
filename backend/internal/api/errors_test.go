package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteErrorEnvelope(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	writeError(w, http.StatusNotFound, CodeNotFound, "Finding não encontrado.")
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d", w.Code)
	}
	var body ErrorBody
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Code != CodeNotFound {
		t.Fatalf("code = %q", body.Code)
	}
	if body.Error != "Finding não encontrado." {
		t.Fatalf("error = %q", body.Error)
	}
}
