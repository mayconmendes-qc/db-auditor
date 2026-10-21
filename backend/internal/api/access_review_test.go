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
	owners     []repository.AccessReviewOwner
	campaigns  []repository.AccessReviewCampaign
	items      []repository.AccessReviewItem
	decisions  []repository.AccessReviewDecision
	createErr  error
	decideErr  error
	ops        []string
	lastFilter repository.AccessReviewMatrixFilter
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
func (s *accessReviewStub) ListAccessReviewMatrix(_ context.Context, _, _ string, f repository.AccessReviewMatrixFilter) ([]repository.AccessReviewItem, int, error) {
	s.lastFilter = f
	items := s.items
	if f.MineUser != "" {
		filtered := make([]repository.AccessReviewItem, 0, len(items))
		for _, it := range items {
			if it.OwnerUserID != nil && *it.OwnerUserID == f.MineUser {
				filtered = append(filtered, it)
			}
		}
		items = filtered
	}
	return items, len(items), nil
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
func (s *accessReviewStub) PostAccessReviewDecisions(_ context.Context, _, campaignID, actor, actorID, actorRole string, in []repository.AccessReviewDecisionInput) ([]repository.AccessReviewDecision, error) {
	if s.decideErr != nil {
		return nil, s.decideErr
	}
	for i := range s.campaigns {
		if s.campaigns[i].ID == campaignID && s.campaigns[i].Status == "closed" {
			return nil, repository.ErrAccessReviewClosed
		}
	}
	itemByID := make(map[string]repository.AccessReviewItem, len(s.items))
	for _, it := range s.items {
		itemByID[it.ID] = it
	}
	for _, d := range in {
		it, ok := itemByID[d.ItemID]
		if !ok {
			continue
		}
		allowed := false
		switch {
		case it.OwnerUserID == nil || *it.OwnerUserID == "":
			allowed = actorRole == "operator"
		case actorID != "" && *it.OwnerUserID == actorID:
			allowed = actorRole == "auditor" || actorRole == "operator"
		}
		if !allowed {
			return nil, repository.ErrAccessReviewNotOwner
		}
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
	return accessReviewHandlerWithUser(t, stub, &repository.AuditorUser{
		ID: "00000000-0000-4000-8000-0000000000a1", Username: "u-" + role, Role: role,
		Environments: []string{"00000000-0000-4000-8000-0000000000e1"},
	})
}

func accessReviewHandlerWithUser(t *testing.T, stub *accessReviewStub, user *repository.AuditorUser) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	registerAccessReviewRoutes(mux, stub)
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

const (
	arEnv        = "00000000-0000-4000-8000-0000000000e1"
	arCampaign   = "00000000-0000-4000-8000-0000000000c1"
	arOwnerID    = "00000000-0000-4000-8000-0000000000a1"
	arOtherID    = "00000000-0000-4000-8000-0000000000a2"
	arItemOwned  = "00000000-0000-4000-8000-0000000000i1"
	arItemOrphan = "00000000-0000-4000-8000-0000000000i2"
)

func TestOnlyOwnerOrOperatorDecides(t *testing.T) {
	owner := arOwnerID
	stub := &accessReviewStub{
		campaigns: []repository.AccessReviewCampaign{{ID: arCampaign, Status: "open"}},
		items: []repository.AccessReviewItem{
			{ID: arItemOwned, OwnerUserID: &owner, AccountName: "app_report", Privilege: "SELECT"},
			{ID: arItemOrphan, OwnerUserID: nil, AccountName: "app_etl", Privilege: "UPDATE"},
		},
	}
	decisionsURL := "/api/v1/environments/" + arEnv + "/access-reviews/" + arCampaign + "/decisions"

	t.Run("owner_decides_own_row", func(t *testing.T) {
		h := accessReviewHandlerWithUser(t, stub, &repository.AuditorUser{
			ID: arOwnerID, Username: "owner", Role: "auditor", Environments: []string{arEnv},
		})
		body := `{"decisions":[{"item_id":"` + arItemOwned + `","decision":"approve"}]}`
		req := httptest.NewRequest(http.MethodPost, decisionsURL, strings.NewReader(body))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("operator_decides_unowned_row", func(t *testing.T) {
		h := accessReviewHandlerWithUser(t, stub, &repository.AuditorUser{
			ID: "00000000-0000-4000-8000-0000000000a9", Username: "op", Role: "operator", Environments: []string{arEnv},
		})
		body := `{"decisions":[{"item_id":"` + arItemOrphan + `","decision":"approve"}]}`
		req := httptest.NewRequest(http.MethodPost, decisionsURL, strings.NewReader(body))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("other_auditor_forbidden", func(t *testing.T) {
		before := len(stub.decisions)
		h := accessReviewHandlerWithUser(t, stub, &repository.AuditorUser{
			ID: arOtherID, Username: "other", Role: "auditor", Environments: []string{arEnv},
		})
		body := `{"decisions":[{"item_id":"` + arItemOwned + `","decision":"approve"}]}`
		req := httptest.NewRequest(http.MethodPost, decisionsURL, strings.NewReader(body))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		if len(stub.decisions) != before {
			t.Fatalf("decision must not be stored on 403")
		}
	})
}

func TestMyPendingOnlyOwnRows(t *testing.T) {
	ownerA, ownerB := arOwnerID, arOtherID
	stub := &accessReviewStub{
		campaigns: []repository.AccessReviewCampaign{{ID: arCampaign, Status: "open"}},
		items: []repository.AccessReviewItem{
			{ID: arItemOwned, OwnerUserID: &ownerA, AccountName: "mine", Privilege: "SELECT", Pending: true},
			{ID: arItemOrphan, OwnerUserID: &ownerB, AccountName: "theirs", Privilege: "UPDATE", Pending: true},
		},
	}
	h := accessReviewHandlerWithUser(t, stub, &repository.AuditorUser{
		ID: arOwnerID, Username: "owner", Role: "auditor", Environments: []string{arEnv},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/environments/"+arEnv+"/access-reviews/"+arCampaign+"/matrix?mine=true&pending=true", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if stub.lastFilter.MineUser != arOwnerID {
		t.Fatalf("MineUser=%q want %s", stub.lastFilter.MineUser, arOwnerID)
	}
	var page struct {
		Items []repository.AccessReviewItem `json:"items"`
		Page  struct {
			Total int `json:"total"`
		} `json:"page"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&page); err != nil {
		t.Fatal(err)
	}
	if page.Page.Total != 1 || len(page.Items) != 1 || page.Items[0].AccountName != "mine" {
		t.Fatalf("expected only owned rows, got %+v", page)
	}
}

func TestAccessReviewDiffNewPrivilege(t *testing.T) {
	stub := &accessReviewStub{
		items: []repository.AccessReviewItem{{
			AccountName: "app_etl", ObjectName: "vendas.pedido", Privilege: "UPDATE",
			Highlights: []string{"new_since_previous"},
		}},
	}
	h := accessReviewHandler(t, stub, "auditor")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/environments/"+arEnv+"/access-reviews/"+arCampaign+"/diff", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var diff repository.AccessReviewDiff
	if err := json.NewDecoder(rec.Body).Decode(&diff); err != nil {
		t.Fatal(err)
	}
	if len(diff.NewPrivileges) != 1 {
		t.Fatalf("new_privileges=%d want 1", len(diff.NewPrivileges))
	}
	np := diff.NewPrivileges[0]
	if np.AccountName != "app_etl" || np.Privilege != "UPDATE" {
		t.Fatalf("new privilege: %+v", np)
	}
	found := false
	for _, h := range np.Highlights {
		if h == "new_since_previous" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("missing new_since_previous highlight: %+v", np.Highlights)
	}
}

func TestClosedCampaignImmutable(t *testing.T) {
	stub := &accessReviewStub{
		campaigns: []repository.AccessReviewCampaign{{ID: arCampaign, Status: "closed"}},
	}
	h := accessReviewHandler(t, stub, "operator")
	decisionsURL := "/api/v1/environments/" + arEnv + "/access-reviews/" + arCampaign + "/decisions"
	body := `{"decisions":[{"item_id":"` + arItemOwned + `","decision":"approve"}]}`
	req := httptest.NewRequest(http.MethodPost, decisionsURL, strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("decide on closed: status=%d body=%s", rec.Code, rec.Body.String())
	}
	if len(stub.decisions) != 0 {
		t.Fatalf("no decisions on closed campaign, got %d", len(stub.decisions))
	}

	closeURL := "/api/v1/environments/" + arEnv + "/access-reviews/" + arCampaign + "/close"
	req = httptest.NewRequest(http.MethodPost, closeURL, nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("close twice: status=%d body=%s", rec.Code, rec.Body.String())
	}
}
