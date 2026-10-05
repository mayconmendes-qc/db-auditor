package api

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestOpenAPICoversRegisteredRoutes(t *testing.T) {
	raw, err := os.ReadFile("../../api/openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	var spec struct {
		Paths map[string]map[string]any `json:"paths"`
	}
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile("health.go")
	if err != nil {
		t.Fatal(err)
	}
	_ = body
	registered := map[string]bool{}
	re := regexp.MustCompile(`HandleFunc\("([A-Z]+) (/api/v1/[^"]+)"`)
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		src, err := os.ReadFile(entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range re.FindAllStringSubmatch(string(src), -1) {
			registered[m[2]+" "+strings.ToLower(m[1])] = true
		}
		text := string(src)
		if strings.Contains(text, "registerAssessmentRoutes") {
			base := "/api/v1/environments/{id}/runs/{run}/databases/{database}/schemas/{schema}/tables/{table}"
			for _, kind := range []string{"assessment", "graph", "findings", "grants", "dependencies", "triggers", "rls-policies"} {
				registered[base+"/"+kind+" get"] = true
			}
		}
		if strings.Contains(text, "registerPDFReportRoutes") {
			base := "/api/v1/environments/{id}/reports"
			registered[base+" post"] = true
			registered[base+" get"] = true
			registered[base+"/{job} get"] = true
			registered[base+"/{job}/cancel post"] = true
			registered[base+"/{job}/retry post"] = true
			registered[base+"/{job}/download get"] = true
		}
	}
	for key := range registered {
		parts := strings.SplitN(key, " ", 2)
		path, method := parts[0], parts[1]
		item, ok := spec.Paths[path]
		if !ok {
			t.Errorf("openapi missing %s", path)
			continue
		}
		if _, ok := item[method]; !ok {
			t.Errorf("openapi missing %s %s", method, path)
		}
	}
	if strings.Contains(string(raw), "dsn") || strings.Contains(strings.ToLower(string(raw)), "password") {
		t.Fatal("spec must not publish DSN or password fields")
	}
}
