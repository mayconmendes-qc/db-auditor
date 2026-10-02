package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mayconmendes-qc/db-auditor/internal/repository"
)

type fakeAuthStore struct {
	user        *repository.AuditorUser
	environment string
}

func (s fakeAuthStore) FindAuditorUser(context.Context, string) (*repository.AuditorUser, error) {
	return s.user, nil
}
func (s fakeAuthStore) CreateAuditorSession(context.Context, string, []byte, time.Time) error {
	return nil
}
func (s fakeAuthStore) GetAuditorSession(context.Context, []byte) (*repository.AuditorUser, error) {
	return s.user, nil
}
func (s fakeAuthStore) DeleteAuditorSession(context.Context, []byte) error { return nil }
func (s fakeAuthStore) CreateAuditorUser(context.Context, string, string, string, []string) (*repository.AuditorUser, error) {
	return nil, nil
}
func (s fakeAuthStore) LogAuditorOperation(context.Context, *repository.AuditorUser, string, string, string, string) error {
	return nil
}
func (s fakeAuthStore) ResolveAuditorResourceEnvironment(context.Context, string, string) (string, error) {
	return s.environment, nil
}

func TestAuthMiddlewareRoleAndEnvironment(t *testing.T) {
	allowed := "00000000-0000-0000-0000-000000000001"
	other := "00000000-0000-0000-0000-000000000002"
	resource := "00000000-0000-0000-0000-000000000003"
	for _, tc := range []struct {
		name, role, method, path, resourceEnv string
		want                                  int
	}{
		{"viewer_read", "viewer", "GET", "/api/v1/environments/" + allowed + "/tables", "", 200},
		{"viewer_write", "viewer", "PATCH", "/api/v1/findings/" + resource, allowed, 403},
		{"auditor_triage", "auditor", "PATCH", "/api/v1/findings/" + resource, allowed, 200},
		{"auditor_cross_env", "auditor", "PATCH", "/api/v1/findings/" + resource, other, 403},
		{"viewer_logout", "viewer", "POST", "/api/v1/auth/logout", "", 200},
		{"viewer_admin", "viewer", "POST", "/api/v1/auth/users", "", 403},
		{"mapping_query_cannot_bypass_scope", "viewer", "GET", "/api/v1/mappings?environment_id=" + allowed, "", 403},
		{"status_query_cannot_bypass_scope", "viewer", "GET", "/api/v1/status?environment_id=" + allowed, "", 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := fakeAuthStore{user: &repository.AuditorUser{Role: tc.role, Environments: []string{allowed}}, environment: tc.resourceEnv}
			h := authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }), store)
			r := httptest.NewRequest(tc.method, tc.path, nil)
			r.Header.Set("Authorization", "Bearer valid-token")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.want {
				t.Fatalf("status %d want %d", w.Code, tc.want)
			}
		})
	}
}

func TestAuthMiddlewareRejectsMissingSession(t *testing.T) {
	h := authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }), fakeAuthStore{})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/environments", nil))
	if w.Code != 401 {
		t.Fatalf("status %d", w.Code)
	}
}
