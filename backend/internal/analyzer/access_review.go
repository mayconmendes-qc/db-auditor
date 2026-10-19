package analyzer

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// AccessReviewAnalyzer emits findings from campaign decisions and frozen matrix
// highlights (spec 07 rules). Thresholds are initial values to calibrate.
type AccessReviewAnalyzer struct{}

func (AccessReviewAnalyzer) Name() string { return "access_review" }

const (
	accessReviewInactiveDays     = 90  // valor inicial, calibrar
	accessReviewOverduePendingPct = 10 // valor inicial, calibrar
)

// AccessReviewCampaignFact is loaded from the store for analysis.
type AccessReviewCampaignFact struct {
	ID            string
	EnvironmentID string
	Status        string
	DueAt         time.Time
	TotalItems    int
	PendingItems  int
	Items         []AccessReviewItemFact
}

type AccessReviewItemFact struct {
	ID           string
	AccountName  string
	ObjectType   string
	ObjectName   string
	ColumnName   string
	Privilege    string
	OriginKind   string
	Highlights   []string
	Decision     string // empty = pending
	RevokeDueAt  *time.Time
	FindingActionID string
	PrivilegeStillPresent bool // set when a later complete run still has the privilege
}

func (AccessReviewAnalyzer) Analyze(_ context.Context, facts SnapshotFacts) ([]Finding, error) {
	out := make([]Finding, 0)
	for _, camp := range facts.AccessReviewCampaigns {
		if camp.EnvironmentID == "" {
			camp.EnvironmentID = facts.EnvironmentID
		}
		pendingPct := 0.0
		if camp.TotalItems > 0 {
			pendingPct = float64(camp.PendingItems) * 100 / float64(camp.TotalItems)
		}
		if (camp.Status == "overdue" || time.Now().UTC().After(camp.DueAt.UTC())) &&
			camp.Status != "closed" && pendingPct >= accessReviewOverduePendingPct {
			key := fmt.Sprintf("campaign:%s", camp.ID)
			title := "Campanha de revisão vencida com pendências"
			out = append(out, Finding{
				EnvironmentID: camp.EnvironmentID, AuditRunID: facts.AuditRunID,
				FindingType: "access_review.campaign_overdue_pending", Severity: SeverityMedium, Status: StatusOpen,
				Title: title, Summary: fmt.Sprintf("Campanha com %.0f%% de linhas pendentes após o prazo (limiar %d%%).", pendingPct, accessReviewOverduePendingPct),
				ObjectType: "access_review_campaign", ObjectKey: key, ObjectName: camp.ID,
				Evidence: map[string]any{"campaign_id": camp.ID, "pending_percent": pendingPct, "threshold_percent": accessReviewOverduePendingPct},
				DedupKey: DedupKey("access_review.campaign_overdue_pending", key, title),
			})
		}
		for _, item := range camp.Items {
			obj := item.ObjectName
			if item.ColumnName != "" {
				obj = item.ObjectName + "." + item.ColumnName
			}
			key := fmt.Sprintf("%s|%s|%s|%s", item.AccountName, obj, item.Privilege, item.ID)

			if item.Decision == "revoke" && item.RevokeDueAt != nil && time.Now().UTC().After(item.RevokeDueAt.UTC()) && item.PrivilegeStillPresent {
				title := fmt.Sprintf("Revogação não aplicada: %s %s em %s", item.AccountName, item.Privilege, obj)
				ev := map[string]any{
					"campaign_id": camp.ID, "item_id": item.ID, "account": item.AccountName,
					"object": obj, "privilege": item.Privilege, "revoke_due_at": item.RevokeDueAt.UTC().Format(time.RFC3339),
				}
				if item.FindingActionID != "" {
					ev["finding_action_id"] = item.FindingActionID
				}
				out = append(out, Finding{
					EnvironmentID: camp.EnvironmentID, AuditRunID: facts.AuditRunID,
					FindingType: "access_review.revocation_not_applied", Severity: SeverityHigh, Status: StatusOpen,
					Title: title, Summary: "Decisão de revogar com prazo vencido e privilégio ainda presente na coleta completa seguinte.",
					ObjectType: item.ObjectType, ObjectKey: key, ObjectName: obj, DatabaseName: "", SchemaName: "",
					Evidence: ev, DedupKey: DedupKey("access_review.revocation_not_applied", key, title),
				})
			}

			powerful := containsHighlight(item.Highlights, "powerful_role")
			if powerful && item.Decision != "approve" && (item.Decision == "" || item.Decision == "keep" || item.Decision == "revoke") {
				// Superuser/BYPASSRLS without approve on the active campaign.
				if camp.Status != "closed" {
					title := fmt.Sprintf("Papel poderoso sem aprovação: %s", item.AccountName)
					out = append(out, Finding{
						EnvironmentID: camp.EnvironmentID, AuditRunID: facts.AuditRunID,
						FindingType: "access_review.powerful_without_approve", Severity: SeverityHigh, Status: StatusOpen,
						Title: title, Summary: "Conta com poderes elevados sem decisão \"aprovar\" na campanha vigente.",
						ObjectType: "role", ObjectKey: key, ObjectName: item.AccountName,
						Evidence: map[string]any{"campaign_id": camp.ID, "decision": item.Decision, "highlights": item.Highlights},
						DedupKey: DedupKey("access_review.powerful_without_approve", key, title),
					})
				}
			}

			if containsHighlight(item.Highlights, "inactive_or_expired") && isWritePrivilege(item.Privilege) {
				title := fmt.Sprintf("Conta inativa com escrita: %s em %s", item.AccountName, obj)
				out = append(out, Finding{
					EnvironmentID: camp.EnvironmentID, AuditRunID: facts.AuditRunID,
					FindingType: "access_review.inactive_with_write", Severity: SeverityMedium, Status: StatusOpen,
					Title: title, Summary: fmt.Sprintf("Conta sem sessão observada ou validade vencida (≥ %d dias) com privilégio de escrita.", accessReviewInactiveDays),
					ObjectType: item.ObjectType, ObjectKey: key, ObjectName: obj,
					Evidence: map[string]any{"campaign_id": camp.ID, "inactive_days_threshold": accessReviewInactiveDays, "privilege": item.Privilege},
					DedupKey: DedupKey("access_review.inactive_with_write", key, title),
				})
			}

			if item.OriginKind == "public" && !strings.HasPrefix(item.ObjectName, "public.") && item.ObjectName != "public" {
				title := fmt.Sprintf("Privilégio via PUBLIC fora de public: %s", obj)
				out = append(out, Finding{
					EnvironmentID: camp.EnvironmentID, AuditRunID: facts.AuditRunID,
					FindingType: "access_review.public_outside_public", Severity: SeverityMedium, Status: StatusOpen,
					Title: title, Summary: "Privilégio concedido via PUBLIC em objeto fora do schema public.",
					ObjectType: item.ObjectType, ObjectKey: key, ObjectName: obj,
					Evidence: map[string]any{"campaign_id": camp.ID, "account": item.AccountName, "privilege": item.Privilege},
					DedupKey: DedupKey("access_review.public_outside_public", key, title),
				})
			}

			if containsHighlight(item.Highlights, "new_since_previous") && item.Decision == "" {
				title := fmt.Sprintf("Privilégio novo sem decisão: %s %s em %s", item.AccountName, item.Privilege, obj)
				out = append(out, Finding{
					EnvironmentID: camp.EnvironmentID, AuditRunID: facts.AuditRunID,
					FindingType: "access_review.new_privilege_undecided", Severity: SeverityMedium, Status: StatusOpen,
					Title: title, Summary: "Privilégio novo desde a campanha anterior ainda sem decisão.",
					ObjectType: item.ObjectType, ObjectKey: key, ObjectName: obj,
					Evidence: map[string]any{"campaign_id": camp.ID, "account": item.AccountName, "privilege": item.Privilege},
					DedupKey: DedupKey("access_review.new_privilege_undecided", key, title),
				})
			}
		}
	}
	return out, nil
}

func containsHighlight(list []string, want string) bool {
	for _, h := range list {
		if h == want {
			return true
		}
	}
	return false
}

func isWritePrivilege(p string) bool {
	switch p {
	case "INSERT", "UPDATE", "DELETE", "TRUNCATE", "CREATE":
		return true
	default:
		return false
	}
}
