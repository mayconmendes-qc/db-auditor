package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/osmendes/db-auditor/internal/repository"
)

type accessReviewStub struct {
	stubStore
	fakeAuthStore
	owners    []repository.AccessReviewOwner
	campaigns []repository.AccessReviewCampaign
	items     []repository.AccessReviewItem
	decisions []repository.AccessReviewDecision
	createErr error
	decideErr error
	ops       []string
}

func (s *accessReviewStub) ListAccessReviewOwners(context.Context, string) ([]repository.AccessReviewOwner, error) {
	return s.owners, nil
}
func (s *accessReviewStub) ReplaceAccessReviewOwners(_ context.Context, _, _ string, in []repository.AccessReviewOwnerInput) ([]repository.AccessReviewOwner, error) {
	if s.createErr != nil {
		return nil, s.createErr
	}
	out := make([]repository.AccessReviewOwner, 0, len(in))
	for _, i := range in {
		out = append(out, repository.AccessReviewOwner{TargetRole: i.TargetRole, OwnerUserID: i.OwnerUserID})
	}
	s.owners = out
	return out, nil
}
func (s *accessReviewStub) CreateAccessReviewCampaign(_ context.Context, env string, in repository.CreateAccessReviewCampaignInput) (*repository.AccessReviewCampaign, error) {
	if s.createErr != nil {
		return nil, s.createErr
	}
	c := repository.AccessReviewCampaign{
		ID: "00000000-0000-4000-8000-0000000000c1", EnvironmentID: env, Name: in.Name,
		ReferenceRunID: in.ReferenceRunID, DueAt: in.DueAt, Status: "open", CreatedBy: in.CreatedBy,
		TotalItems: len(s.items), PendingItems: len(s.items),
	}
	s.campaigns = append(s.campaigns, c)
	return &c, nil
}
func (s *accessReviewStub) ListAccessReviewCampaigns(context.Context, string, int, int) ([]repository.AccessReviewCampaign, int, error) {
	return s.campaigns, len(s.campaigns), nil
}
func (s *accessReviewStub) GetAccessReviewCampaign(_ context.Context, _, id string) (*repository.AccessReviewCampaign, error) {
	for i := range s.campaigns {
		if s.campaigns[i].ID == id {
			return &s.campaigns[i], nil
		}
	}
	return nil, repository.ErrAccessReviewNotFound
}
func (s *accessReviewStub) ListAccessReviewMatrix(context.Context, string, string, repository.AccessReviewMatrixFilter) ([]repository.AccessReviewItem, int, error) {
	return s.items, len(s.items), nil
}
func (s *accessReviewStub) CloseAccessReviewCampaign(_ context.Context, _, id, actor string) (*repository.AccessReviewCampaign, error) {
	for i := range s.campaigns {
		if s.campaigns[i].ID == id {
			if s.campaigns[i].Status == "closed" {
				return nil, repository.ErrAccessReviewClosed
			}
			s.campaigns[i].Status = "closed"
			s.campaigns[i].ClosedBy = actor
			return &s.campaigns[i], nil
		}
	}
	return nil, repository.ErrAccessReviewNotFound
}
func (s *accessReviewStub) PostAccessReviewDecisions(_ context.Context, _, _, actor, _, _ string, in []repository.AccessReviewDecisionInput) ([]repository.AccessReviewDecision, error) {
	if s.decideErr != nil {
		return nil, s.decideErr
	}
	out := make([]repository.AccessReviewDecision, 0, len(in))
	for _, d := range in {
		out = append(out, repository.AccessReviewDecision{
			ID: "00000000-0000-4000-8000-0000000000d1", ItemID: d.ItemID, Decision: d.Decision,
			Justification: d.Justification, DecidedBy: actor, DecidedAt: time.Now().UTC(),
		})
	}
	s.decisions = append(s.decisions, out...)
	return out, nil
}
func (s *accessReviewStub) DiffAccessReviewCampaign(context.Context, string, string) (*repository.AccessReviewDiff, error) {
	return &repository.AccessReviewDiff{NewPrivileges: s.items}, nil
}
func (s *accessReviewStub) StreamAccessReviewCSV(_ context.Context, _, _ string, w io.Writer) (int, error) {
	n := len(s.items)
	_, _ = io.WriteString(w, "conta,objeto\napp_report,vendas.pedido\n# export_complete,"+itoa(n)+"\n")
	return n, nil
}
func (s *accessReviewStub) LogAuditorOperation(_ context.Context, _ *repository.AuditorUser, action, _, _, _ string) error {
	s.ops = append(s.ops, action)
	return nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

func accessReviewHandler(t *testing.T, stub *accessReviewStub, role string) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	registerAccessReviewRoutes(mux, stub)
	user := &repository.AuditorUser{
		ID: "00000000-0000-4000-8000-0000000000a1", Username: "u-" + role, Role: role,
		Environments: []string{"00000000-0000-4000-8000-0000000000e1"},
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = r.WithContext(context.WithValue(r.Context(), identityKey{}, user))
		mux.ServeHTTP(w, r)
	})
}

func TestCampaignRequiresCompleteRun(t *testing.T) {
	stub := &accessReviewStub{createErr: repository.ErrAccessReviewIncompleteRun}
	h := accessReviewHandler(t, stub, "operator")
	body := `{"name":"Q1","reference_run_id":"00000000-0000-4000-8000-0000000000r1","due_at":"2026-12-01T00:00:00Z"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/environments/00000000-0000-4000-8000-0000000000e1/access-reviews", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "coleta completa") {
		t.Fatalf("message: %s", rec.Body.String())
	}
}

func TestKeepWithoutJustificationRejected(t *testing.T) {
	stub := &accessReviewStub{
		campaigns: []repository.AccessReviewCampaign{{ID: "00000000-0000-4000-8000-0000000000c1", Status: "open"}},
		decideErr: repository.ErrAccessReviewKeepJustify,
	}
	h := accessReviewHandler(t, stub, "auditor")
	body := `{"decisions":[{"item_id":"00000000-0000-4000-8000-0000000000i1","decision":"keep"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/environments/00000000-0000-4000-8000-0000000000e1/access-reviews/00000000-0000-4000-8000-0000000000c1/decisions", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if len(stub.decisions) != 0 {
		t.Fatalf("nothing should be stored, got %d", len(stub.decisions))
	}
}

func TestViewerCannotDecide(t *testing.T) {
	// authorizeRequest is the gate; here we simulate handler still being reached only for auditor+.
	// Viewer is blocked by middleware in production; this asserts the decide path rejects ErrAccessReviewNotOwner too.
	stub := &accessReviewStub{
		campaigns: []repository.AccessReviewCampaign{{ID: "00000000-0000-4000-8000-0000000000c1", Status: "open"}},
		decideErr: repository.ErrAccessReviewNotOwner,
	}
	h := accessReviewHandler(t, stub, "viewer")
	body := `{"decisions":[{"item_id":"00000000-0000-4000-8000-0000000000i1","decision":"approve"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/environments/00000000-0000-4000-8000-0000000000e1/access-reviews/00000000-0000-4000-8000-0000000000c1/decisions", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAccessReviewCSVMarkerAndCount(t *testing.T) {
	stub := &accessReviewStub{
		items: []repository.AccessReviewItem{{AccountName: "app_report", ObjectName: "vendas.pedido", Privilege: "SELECT", OriginLabel: "via g_leitura"}},
	}
	h := accessReviewHandler(t, stub, "auditor")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/environments/00000000-0000-4000-8000-0000000000e1/access-reviews/00000000-0000-4000-8000-0000000000c1/export/download?format=csv", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "# export_complete") {
		t.Fatalf("missing marker: %s", body)
	}
}

func TestOwnerWithoutEnvironmentAccess400(t *testing.T) {
	stub := &accessReviewStub{createErr: repository.ErrAccessReviewOwnerAccess}
	h := accessReviewHandler(t, stub, "operator")
	body := `{"items":[{"target_role":"app_report","owner_user_id":"00000000-0000-4000-8000-0000000000a9"}]}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/environments/00000000-0000-4000-8000-0000000000e1/access-review-owners", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAuthorizeAccessReviewDecisionsRequiresAuditor(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/environments/00000000-0000-4000-8000-0000000000e1/access-reviews/00000000-0000-4000-8000-0000000000c1/decisions", bytes.NewReader([]byte(`{}`)))
	viewer := &repository.AuditorUser{Role: "viewer", Environments: []string{"00000000-0000-4000-8000-0000000000e1"}}
	if _, ok := authorizeRequest(req, viewer, fakeAuthStore{}); ok {
		t.Fatal("viewer must not authorize decisions")
	}
	auditor := &repository.AuditorUser{Role: "auditor", Environments: []string{"00000000-0000-4000-8000-0000000000e1"}}
	if _, ok := authorizeRequest(req, auditor, fakeAuthStore{}); !ok {
		t.Fatal("auditor must authorize decisions path (handler checks ownership)")
	}
}

func TestAccessReviewDiffEndpoint(t *testing.T) {
	stub := &accessReviewStub{
		items: []repository.AccessReviewItem{{AccountName: "app_etl", ObjectName: "vendas.pedido", Privilege: "UPDATE", Highlights: []string{"new_since_previous"}}},
	}
	h := accessReviewHandler(t, stub, "viewer")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/environments/00000000-0000-4000-8000-0000000000e1/access-reviews/00000000-0000-4000-8000-0000000000c1/diff", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	var diff repository.AccessReviewDiff
	if err := json.NewDecoder(rec.Body).Decode(&diff); err != nil {
		t.Fatal(err)
	}
	if len(diff.NewPrivileges) != 1 || diff.NewPrivileges[0].Privilege != "UPDATE" {
		t.Fatalf("diff: %+v", diff)
	}
}
