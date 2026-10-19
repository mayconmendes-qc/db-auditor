package repository

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/mail"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/osmendes/db-auditor/internal/analyzer"
	"github.com/osmendes/db-auditor/internal/collectors/postgres"
	"github.com/osmendes/db-auditor/internal/report"
)

var (
	ErrAccessReviewIncompleteRun = errors.New("campanha exige coleta completa")
	ErrAccessReviewClosed        = errors.New("campanha concluída")
	ErrAccessReviewOwnerAccess   = errors.New("responsável sem acesso ao ambiente")
	ErrAccessReviewKeepJustify   = errors.New("justificativa obrigatória para manter")
	ErrAccessReviewNotOwner      = errors.New("apenas o responsável da linha ou o operator pode decidir")
	ErrAccessReviewNotFound      = errors.New("campanha não encontrada")
	ErrAccessReviewBadContact    = errors.New("contato do responsável inválido")
	ErrAccessReviewValidation    = errors.New("dados da campanha inválidos")
)

var emailLoose = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

type AccessReviewOwner struct {
	ID           string `json:"id"`
	Environment  string `json:"environment_id"`
	TargetRole   string `json:"target_role"`
	OwnerUserID  string `json:"owner_user_id"`
	OwnerName    string `json:"owner_username"`
	OwnerRole    string `json:"owner_role"`
	ContactName  string `json:"contact_name,omitempty"`
	ContactEmail string `json:"contact_email,omitempty"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type AccessReviewOwnerInput struct {
	TargetRole   string `json:"target_role"`
	OwnerUserID  string `json:"owner_user_id"`
	ContactName  string `json:"contact_name"`
	ContactEmail string `json:"contact_email"`
}

type AccessReviewCampaign struct {
	ID              string          `json:"id"`
	EnvironmentID   string          `json:"environment_id"`
	Name            string          `json:"name"`
	Scope           json.RawMessage `json:"scope"`
	ReferenceRunID  string          `json:"reference_run_id"`
	DueAt           time.Time       `json:"due_at"`
	Status          string          `json:"status"`
	CreatedBy       string          `json:"created_by"`
	CreatedAt       time.Time       `json:"created_at"`
	ClosedAt        *time.Time      `json:"closed_at,omitempty"`
	ClosedBy        string          `json:"closed_by,omitempty"`
	TotalItems      int             `json:"total_items"`
	DecidedItems    int             `json:"decided_items"`
	PendingItems    int             `json:"pending_items"`
	DecidedPercent  float64         `json:"decided_percent"`
}

type AccessReviewItem struct {
	ID           string   `json:"id"`
	CampaignID   string   `json:"campaign_id"`
	DatabaseName string   `json:"database_name"`
	AccountName  string   `json:"account_name"`
	ObjectType   string   `json:"object_type"`
	ObjectName   string   `json:"object_name"`
	ColumnName   string   `json:"column_name,omitempty"`
	Privilege    string   `json:"privilege"`
	OriginKind   string   `json:"origin_kind"`
	ViaChain     []string `json:"via_chain"`
	OriginLabel  string   `json:"origin_label"`
	Highlights   []string `json:"highlights"`
	OwnerUserID  *string  `json:"owner_user_id,omitempty"`
	OwnerName    string   `json:"owner_username,omitempty"`
	// Latest decision fields (nullable when pending).
	Decision       *string    `json:"decision,omitempty"`
	Justification  string     `json:"justification,omitempty"`
	DecidedBy      string     `json:"decided_by,omitempty"`
	DecidedAt      *time.Time `json:"decided_at,omitempty"`
	Pending        bool       `json:"pending"`
	// Last login from log aggregate (ADR-0001 D10); source is always "log" when set.
	LastLoginAt     *time.Time `json:"last_login_at,omitempty"`
	LastLoginSource string     `json:"last_login_source,omitempty"`
}

type AccessReviewMatrixFilter struct {
	Account   string
	Object    string
	Privilege string
	Origin    string
	Highlight string
	Pending   *bool
	Owner     string
	MineUser  string // when set, filter owner_user_id = MineUser
	Limit     int
	Offset    int
}

type AccessReviewDecisionInput struct {
	ItemID        string     `json:"item_id"`
	Decision      string     `json:"decision"`
	RevokeDueAt   *time.Time `json:"revoke_due_at,omitempty"`
	Justification string     `json:"justification"`
	SupersedesID  string     `json:"supersedes_id,omitempty"`
}

type AccessReviewDecision struct {
	ID             string     `json:"id"`
	CampaignID     string     `json:"campaign_id"`
	ItemID         string     `json:"item_id"`
	Decision       string     `json:"decision"`
	RevokeDueAt    *time.Time `json:"revoke_due_at,omitempty"`
	Justification  string     `json:"justification"`
	DecidedBy      string     `json:"decided_by"`
	DecidedAt      time.Time  `json:"decided_at"`
	SupersedesID   *string    `json:"supersedes_id,omitempty"`
	FindingActionID *string   `json:"finding_action_id,omitempty"`
}

type AccessReviewDiff struct {
	PreviousCampaignID string              `json:"previous_campaign_id,omitempty"`
	NewPrivileges      []AccessReviewItem  `json:"new_privileges"`
	RemovedPrivileges  []AccessReviewItem  `json:"removed_privileges"`
	ChangedDecisions   []AccessReviewDiffDecision `json:"changed_decisions"`
	NewAccounts        []string            `json:"new_accounts"`
	RemovedAccounts    []string            `json:"removed_accounts"`
}

type AccessReviewDiffDecision struct {
	ItemKey    string  `json:"item_key"`
	Account    string  `json:"account_name"`
	Object     string  `json:"object_name"`
	Privilege  string  `json:"privilege"`
	Previous   *string `json:"previous_decision,omitempty"`
	Current    *string `json:"current_decision,omitempty"`
}

type CreateAccessReviewCampaignInput struct {
	Name           string          `json:"name"`
	ReferenceRunID string          `json:"reference_run_id"`
	DueAt          time.Time       `json:"due_at"`
	Scope          json.RawMessage `json:"scope"`
	CreatedBy      string
	CreatedByID    string
}

func validateContact(name, email string) error {
	if len(name) > 200 || len(email) > 254 {
		return ErrAccessReviewBadContact
	}
	email = strings.TrimSpace(email)
	if email == "" {
		return nil
	}
	if !emailLoose.MatchString(email) {
		return ErrAccessReviewBadContact
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return ErrAccessReviewBadContact
	}
	return nil
}

// AuditorUserHasEnvAccess reports whether the user is active, has auditor+ role,
// and is granted the environment (ADR-0001 D7).
func (s *Store) AuditorUserHasEnvAccess(ctx context.Context, userID, env string) (role string, ok bool, err error) {
	err = s.pool.QueryRow(ctx, `SELECT u.role FROM auditor_user u
JOIN auditor_user_environment e ON e.user_id=u.id
WHERE u.id=$1::uuid AND u.active AND e.environment_id=$2::uuid`, userID, env).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if role != "auditor" && role != "operator" {
		return role, false, nil
	}
	return role, true, nil
}

func (s *Store) ListAccessReviewOwners(ctx context.Context, env string) ([]AccessReviewOwner, error) {
	rows, err := s.pool.Query(ctx, `SELECT o.id::text,o.environment_id::text,o.target_role,o.owner_user_id::text,
u.username,u.role,o.contact_name,o.contact_email,o.updated_at
FROM access_review_owner o JOIN auditor_user u ON u.id=o.owner_user_id
WHERE o.environment_id=$1::uuid ORDER BY o.target_role`, env)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]AccessReviewOwner, 0)
	for rows.Next() {
		var item AccessReviewOwner
		if err := rows.Scan(&item.ID, &item.Environment, &item.TargetRole, &item.OwnerUserID,
			&item.OwnerName, &item.OwnerRole, &item.ContactName, &item.ContactEmail, &item.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) ReplaceAccessReviewOwners(ctx context.Context, env, actor string, inputs []AccessReviewOwnerInput) ([]AccessReviewOwner, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	seen := map[string]struct{}{}
	for _, in := range inputs {
		target := strings.TrimSpace(in.TargetRole)
		if target == "" || in.OwnerUserID == "" {
			return nil, ErrAccessReviewValidation
		}
		if err := validateContact(in.ContactName, in.ContactEmail); err != nil {
			return nil, err
		}
		if _, dup := seen[target]; dup {
			return nil, ErrAccessReviewValidation
		}
		seen[target] = struct{}{}
		role, ok, err := s.AuditorUserHasEnvAccess(ctx, in.OwnerUserID, env)
		if err != nil {
			return nil, err
		}
		if !ok {
			_ = role
			return nil, ErrAccessReviewOwnerAccess
		}
	}
	if _, err = tx.Exec(ctx, `DELETE FROM access_review_owner WHERE environment_id=$1::uuid`, env); err != nil {
		return nil, err
	}
	for _, in := range inputs {
		if _, err = tx.Exec(ctx, `INSERT INTO access_review_owner(environment_id,target_role,owner_user_id,contact_name,contact_email,updated_by)
VALUES($1::uuid,$2,$3::uuid,$4,$5,$6)`, env, strings.TrimSpace(in.TargetRole), in.OwnerUserID,
			strings.TrimSpace(in.ContactName), strings.TrimSpace(in.ContactEmail), actor); err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.ListAccessReviewOwners(ctx, env)
}

func (s *Store) CreateAccessReviewCampaign(ctx context.Context, env string, in CreateAccessReviewCampaignInput) (*AccessReviewCampaign, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" || in.ReferenceRunID == "" || in.DueAt.IsZero() {
		return nil, ErrAccessReviewValidation
	}
	comp, err := s.GetSnapshotCompleteness(ctx, env, in.ReferenceRunID)
	if err != nil {
		return nil, err
	}
	if comp == nil || comp.Completeness != "complete" || comp.AuditRunID != in.ReferenceRunID {
		return nil, ErrAccessReviewIncompleteRun
	}
	scope := in.Scope
	if len(scope) == 0 {
		scope = json.RawMessage(`{}`)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var campaignID string
	err = tx.QueryRow(ctx, `INSERT INTO access_review_campaign(environment_id,name,scope,reference_run_id,due_at,status,created_by,created_by_user_id)
VALUES($1::uuid,$2,$3::jsonb,$4::uuid,$5,'open',$6,$7::uuid) RETURNING id::text`,
		env, name, scope, in.ReferenceRunID, in.DueAt.UTC(), in.CreatedBy, nullUUID(in.CreatedByID)).Scan(&campaignID)
	if err != nil {
		return nil, err
	}

	owners, err := s.ownerMap(ctx, tx, env)
	if err != nil {
		return nil, err
	}
	prevHighlights, err := s.previousCampaignPrivilegeKeys(ctx, tx, env)
	if err != nil {
		return nil, err
	}
	powerful, inactive, err := s.highlightSources(ctx, tx, env, in.ReferenceRunID)
	if err != nil {
		return nil, err
	}

	origins, err := s.ListGrantOrigins(ctx, env, in.ReferenceRunID)
	if err != nil {
		return nil, err
	}
	columns, err := s.ListColumnGrants(ctx, env, in.ReferenceRunID)
	if err != nil {
		return nil, err
	}

	insertItem := func(db, account, objectType, objectName, column, privilege, originKind string, via []string, label string) error {
		highlights := computeHighlights(account, objectName, privilege, originKind, powerful, inactive, prevHighlights, db, objectType, column)
		ownerID := owners[account]
		_, err := tx.Exec(ctx, `INSERT INTO access_review_item(
campaign_id,environment_id,database_name,account_name,object_type,object_name,column_name,privilege,origin_kind,via_chain,origin_label,highlights,owner_user_id)
VALUES($1::uuid,$2::uuid,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13::uuid)
ON CONFLICT DO NOTHING`,
			campaignID, env, db, account, objectType, objectName, column, privilege, originKind,
			inheritedOrEmpty(via), label, highlights, nullUUID(ownerID))
		return err
	}

	for _, o := range origins {
		if err := insertItem(o.DatabaseName, o.AccountName, o.ObjectType, o.ObjectName, "", o.Privilege, o.OriginKind, o.ViaChain, o.OriginLabel); err != nil {
			return nil, err
		}
	}
	for _, c := range columns {
		obj := c.SchemaName + "." + c.TableName
		label := postgres.OriginLabelPT(c.OriginKind, c.ViaChain)
		if err := insertItem(c.DatabaseName, c.AccountName, "column", obj, c.ColumnName, c.Privilege, c.OriginKind, c.ViaChain, label); err != nil {
			return nil, err
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetAccessReviewCampaign(ctx, env, campaignID)
}

func (s *Store) ownerMap(ctx context.Context, tx pgx.Tx, env string) (map[string]string, error) {
	rows, err := tx.Query(ctx, `SELECT target_role,owner_user_id::text FROM access_review_owner WHERE environment_id=$1::uuid`, env)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var role, owner string
		if err := rows.Scan(&role, &owner); err != nil {
			return nil, err
		}
		out[role] = owner
	}
	return out, rows.Err()
}

func (s *Store) previousCampaignPrivilegeKeys(ctx context.Context, tx pgx.Tx, env string) (map[string]struct{}, error) {
	var prev string
	err := tx.QueryRow(ctx, `SELECT id::text FROM access_review_campaign
WHERE environment_id=$1::uuid AND status='closed' ORDER BY closed_at DESC NULLS LAST, created_at DESC LIMIT 1`, env).Scan(&prev)
	if errors.Is(err, pgx.ErrNoRows) {
		return map[string]struct{}{}, nil
	}
	if err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT database_name,account_name,object_type,object_name,column_name,privilege
FROM access_review_item WHERE campaign_id=$1::uuid`, prev)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]struct{}{}
	for rows.Next() {
		var db, account, ot, on, col, priv string
		if err := rows.Scan(&db, &account, &ot, &on, &col, &priv); err != nil {
			return nil, err
		}
		out[itemKey(db, account, ot, on, col, priv)] = struct{}{}
	}
	return out, rows.Err()
}

func itemKey(db, account, ot, on, col, priv string) string {
	return strings.Join([]string{db, account, ot, on, col, priv}, "\x00")
}

func (s *Store) highlightSources(ctx context.Context, tx pgx.Tx, env, run string) (powerful, inactive map[string]struct{}, err error) {
	powerful, inactive = map[string]struct{}{}, map[string]struct{}{}
	rows, err := tx.Query(ctx, `SELECT role_name,is_superuser,bypass_rls,replication,create_role,inherited_privileges,sampled_active,valid_until
FROM account_role_snapshot WHERE environment_id=$1::uuid AND audit_run_id=$2::uuid AND can_login`, env, run)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	now := time.Now().UTC()
	for rows.Next() {
		var name string
		var super, bypass, repl, create, sampled bool
		var inherited []string
		var validUntil *time.Time
		if err := rows.Scan(&name, &super, &bypass, &repl, &create, &inherited, &sampled, &validUntil); err != nil {
			return nil, nil, err
		}
		if super || bypass || repl || create || len(inherited) > 0 {
			powerful[name] = struct{}{}
		}
		expired := validUntil != nil && validUntil.Before(now)
		if !sampled || expired {
			inactive[name] = struct{}{}
		}
	}
	return powerful, inactive, rows.Err()
}

func computeHighlights(account, objectName, privilege, originKind string, powerful, inactive, prev map[string]struct{}, db, objectType, column string) []string {
	out := make([]string, 0, 4)
	if _, ok := powerful[account]; ok {
		out = append(out, "powerful_role")
	}
	if _, ok := inactive[account]; ok {
		out = append(out, "inactive_or_expired")
	}
	writePriv := privilege == "INSERT" || privilege == "UPDATE" || privilege == "DELETE" || privilege == "TRUNCATE" || privilege == "CREATE"
	if writePriv {
		// Heuristic: writing outside public / own-named schema.
		schema := objectName
		if i := strings.IndexByte(objectName, '.'); i >= 0 {
			schema = objectName[:i]
		}
		if schema != "public" && !strings.Contains(strings.ToLower(schema), strings.ToLower(account)) {
			out = append(out, "cross_app_write")
		}
	}
	if originKind == "public" && !strings.HasPrefix(objectName, "public.") && objectName != "public" {
		out = append(out, "public_outside_public")
	}
	key := itemKey(db, account, objectType, objectName, column, privilege)
	if _, ok := prev[key]; !ok && len(prev) > 0 {
		out = append(out, "new_since_previous")
	}
	return out
}

func (s *Store) ListAccessReviewCampaigns(ctx context.Context, env string, limit, offset int) ([]AccessReviewCampaign, int, error) {
	if limit <= 0 {
		limit = 20
	}
	var total int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM access_review_campaign WHERE environment_id=$1::uuid`, env).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.pool.Query(ctx, `SELECT c.id::text,c.environment_id::text,c.name,c.scope,c.reference_run_id::text,c.due_at,c.status,c.created_by,c.created_at,c.closed_at,c.closed_by,
(SELECT count(*) FROM access_review_item i WHERE i.campaign_id=c.id),
(SELECT count(DISTINCT d.item_id) FROM access_review_decision d
  WHERE d.campaign_id=c.id AND NOT EXISTS (
    SELECT 1 FROM access_review_decision x WHERE x.supersedes_id=d.id))
FROM access_review_campaign c WHERE c.environment_id=$1::uuid
ORDER BY c.created_at DESC,c.id DESC LIMIT $2 OFFSET $3`, env, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]AccessReviewCampaign, 0)
	for rows.Next() {
		var c AccessReviewCampaign
		var decided int
		if err := rows.Scan(&c.ID, &c.EnvironmentID, &c.Name, &c.Scope, &c.ReferenceRunID, &c.DueAt, &c.Status,
			&c.CreatedBy, &c.CreatedAt, &c.ClosedAt, &c.ClosedBy, &c.TotalItems, &decided); err != nil {
			return nil, 0, err
		}
		c.DecidedItems = decided
		c.PendingItems = c.TotalItems - decided
		if c.PendingItems < 0 {
			c.PendingItems = 0
		}
		if c.TotalItems > 0 {
			c.DecidedPercent = float64(c.DecidedItems) * 100 / float64(c.TotalItems)
		}
		c.Status = effectiveCampaignStatus(c.Status, c.DueAt, c.PendingItems, c.TotalItems)
		out = append(out, c)
	}
	return out, total, rows.Err()
}

func effectiveCampaignStatus(status string, due time.Time, pending, total int) string {
	if status == "closed" {
		return status
	}
	if time.Now().UTC().After(due.UTC()) && status != "closed" {
		return "overdue"
	}
	if pending < total && pending > 0 && status == "open" {
		return "in_review"
	}
	return status
}

func (s *Store) GetAccessReviewCampaign(ctx context.Context, env, id string) (*AccessReviewCampaign, error) {
	items, _, err := s.ListAccessReviewCampaigns(ctx, env, 500, 0)
	if err != nil {
		return nil, err
	}
	for i := range items {
		if items[i].ID == id {
			return &items[i], nil
		}
	}
	return nil, ErrAccessReviewNotFound
}

func (s *Store) ListAccessReviewMatrix(ctx context.Context, env, campaignID string, f AccessReviewMatrixFilter) ([]AccessReviewItem, int, error) {
	if f.Limit <= 0 {
		f.Limit = 50
	}
	if _, err := s.GetAccessReviewCampaign(ctx, env, campaignID); err != nil {
		return nil, 0, err
	}
	where := []string{"i.campaign_id=$1::uuid", "i.environment_id=$2::uuid"}
	args := []any{campaignID, env}
	add := func(cond, val string) {
		if strings.TrimSpace(val) == "" {
			return
		}
		args = append(args, val)
		where = append(where, fmt.Sprintf(cond, len(args)))
	}
	add("i.account_name=$%d", f.Account)
	add("i.object_name ILIKE '%%'||$%d||'%%'", f.Object)
	add("i.privilege=$%d", f.Privilege)
	add("i.origin_kind=$%d", f.Origin)
	if strings.TrimSpace(f.Highlight) != "" {
		args = append(args, f.Highlight)
		where = append(where, fmt.Sprintf("$%d = ANY(i.highlights)", len(args)))
	}
	if f.MineUser != "" {
		args = append(args, f.MineUser)
		where = append(where, fmt.Sprintf("i.owner_user_id=$%d::uuid", len(args)))
	} else if f.Owner != "" {
		args = append(args, f.Owner)
		where = append(where, fmt.Sprintf("i.owner_user_id=$%d::uuid", len(args)))
	}
	latestDecision := `LEFT JOIN LATERAL (
  SELECT d.decision,d.justification,d.decided_by,d.decided_at FROM access_review_decision d
  WHERE d.item_id=i.id AND NOT EXISTS (SELECT 1 FROM access_review_decision x WHERE x.supersedes_id=d.id)
  ORDER BY d.decided_at DESC LIMIT 1
) ld ON true`
	if f.Pending != nil {
		if *f.Pending {
			where = append(where, "ld.decision IS NULL")
		} else {
			where = append(where, "ld.decision IS NOT NULL")
		}
	}
	whereSQL := strings.Join(where, " AND ")
	countQ := `SELECT count(*) FROM access_review_item i ` + latestDecision + ` WHERE ` + whereSQL
	var total int
	if err := s.pool.QueryRow(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, f.Limit, f.Offset)
	q := `SELECT i.id::text,i.campaign_id::text,i.database_name,i.account_name,i.object_type,i.object_name,i.column_name,
i.privilege,i.origin_kind,i.via_chain,i.origin_label,i.highlights,i.owner_user_id::text,COALESCE(u.username,''),
ld.decision,COALESCE(ld.justification,''),COALESCE(ld.decided_by,''),ld.decided_at,
ll.last_login_at,COALESCE(ll.source,'')
FROM access_review_item i
LEFT JOIN auditor_user u ON u.id=i.owner_user_id
LEFT JOIN account_last_login_observation ll ON ll.environment_id=i.environment_id AND ll.role_name=i.account_name
` + latestDecision + `
WHERE ` + whereSQL + `
ORDER BY i.account_name,i.object_name,i.column_name,i.privilege,i.id
LIMIT $` + fmt.Sprint(len(args)-1) + ` OFFSET $` + fmt.Sprint(len(args))
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]AccessReviewItem, 0)
	for rows.Next() {
		var item AccessReviewItem
		var ownerID *string
		var decision *string
		var decidedAt *time.Time
		var lastLogin *time.Time
		var lastSrc string
		if err := rows.Scan(&item.ID, &item.CampaignID, &item.DatabaseName, &item.AccountName, &item.ObjectType,
			&item.ObjectName, &item.ColumnName, &item.Privilege, &item.OriginKind, &item.ViaChain, &item.OriginLabel,
			&item.Highlights, &ownerID, &item.OwnerName, &decision, &item.Justification, &item.DecidedBy, &decidedAt,
			&lastLogin, &lastSrc); err != nil {
			return nil, 0, err
		}
		item.OwnerUserID = ownerID
		item.Decision = decision
		item.DecidedAt = decidedAt
		item.Pending = decision == nil
		item.LastLoginAt = lastLogin
		item.LastLoginSource = lastSrc
		out = append(out, item)
	}
	return out, total, rows.Err()
}

func (s *Store) CloseAccessReviewCampaign(ctx context.Context, env, id, actor string) (*AccessReviewCampaign, error) {
	cmd, err := s.pool.Exec(ctx, `UPDATE access_review_campaign SET status='closed',closed_at=now(),closed_by=$3
WHERE id=$1::uuid AND environment_id=$2::uuid AND status<>'closed'`, id, env, actor)
	if err != nil {
		return nil, err
	}
	if cmd.RowsAffected() == 0 {
		c, err := s.GetAccessReviewCampaign(ctx, env, id)
		if err != nil {
			return nil, err
		}
		if c.Status == "closed" {
			return nil, ErrAccessReviewClosed
		}
		return nil, ErrAccessReviewNotFound
	}
	return s.GetAccessReviewCampaign(ctx, env, id)
}

// PostAccessReviewDecisions appends decisions. Mixed-ownership batches fail entirely.
// actorRole is viewer|auditor|operator; actorID is the auditor_user id.
func (s *Store) PostAccessReviewDecisions(ctx context.Context, env, campaignID, actor, actorID, actorRole string, inputs []AccessReviewDecisionInput) ([]AccessReviewDecision, error) {
	if len(inputs) == 0 {
		return nil, ErrAccessReviewValidation
	}
	camp, err := s.GetAccessReviewCampaign(ctx, env, campaignID)
	if err != nil {
		return nil, err
	}
	if camp.Status == "closed" {
		return nil, ErrAccessReviewClosed
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	type rowMeta struct {
		ownerID *string
		exists  bool
	}
	metas := make([]rowMeta, len(inputs))
	for i, in := range inputs {
		dec := strings.ToLower(strings.TrimSpace(in.Decision))
		if dec != "approve" && dec != "revoke" && dec != "keep" {
			return nil, ErrAccessReviewValidation
		}
		if dec == "keep" && strings.TrimSpace(in.Justification) == "" {
			return nil, ErrAccessReviewKeepJustify
		}
		var ownerID *string
		err := tx.QueryRow(ctx, `SELECT owner_user_id::text FROM access_review_item
WHERE id=$1::uuid AND campaign_id=$2::uuid AND environment_id=$3::uuid`, in.ItemID, campaignID, env).Scan(&ownerID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrAccessReviewValidation
		}
		if err != nil {
			return nil, err
		}
		metas[i] = rowMeta{ownerID: ownerID, exists: true}
		allowed := false
		switch {
		case ownerID == nil || *ownerID == "":
			allowed = actorRole == "operator"
		case actorID != "" && *ownerID == actorID:
			allowed = actorRole == "auditor" || actorRole == "operator"
		}
		if !allowed {
			return nil, ErrAccessReviewNotOwner
		}
		inputs[i].Decision = dec
	}

	out := make([]AccessReviewDecision, 0, len(inputs))
	for _, in := range inputs {
		var supersedes any
		if in.SupersedesID != "" {
			supersedes = in.SupersedesID
		}
		var id string
		var at time.Time
		err = tx.QueryRow(ctx, `INSERT INTO access_review_decision(
campaign_id,item_id,decision,revoke_due_at,justification,decided_by,decided_by_user_id,supersedes_id)
VALUES($1::uuid,$2::uuid,$3,$4,$5,$6,$7::uuid,$8::uuid)
RETURNING id::text,decided_at`, campaignID, in.ItemID, in.Decision, in.RevokeDueAt, strings.TrimSpace(in.Justification),
			actor, nullUUID(actorID), supersedes).Scan(&id, &at)
		if err != nil {
			return nil, err
		}
		d := AccessReviewDecision{
			ID: id, CampaignID: campaignID, ItemID: in.ItemID, Decision: in.Decision,
			RevokeDueAt: in.RevokeDueAt, Justification: strings.TrimSpace(in.Justification),
			DecidedBy: actor, DecidedAt: at,
		}
		if in.SupersedesID != "" {
			d.SupersedesID = &in.SupersedesID
		}
		// Revoke → assisted action stub finding + finding_action (external execution).
		if in.Decision == "revoke" {
			actionID, err := s.createRevokeAssistedAction(ctx, tx, env, campaignID, in.ItemID, actor, in.RevokeDueAt)
			if err != nil {
				return nil, err
			}
			if actionID != "" {
				if _, err = tx.Exec(ctx, `UPDATE access_review_decision SET finding_action_id=$2::uuid WHERE id=$1::uuid`, id, actionID); err != nil {
					return nil, err
				}
				d.FindingActionID = &actionID
			}
		}
		out = append(out, d)
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) createRevokeAssistedAction(ctx context.Context, tx pgx.Tx, env, campaignID, itemID, actor string, due *time.Time) (string, error) {
	var db, account, ot, on, col, priv string
	if err := tx.QueryRow(ctx, `SELECT database_name,account_name,object_type,object_name,column_name,privilege
FROM access_review_item WHERE id=$1::uuid`, itemID).Scan(&db, &account, &ot, &on, &col, &priv); err != nil {
		return "", err
	}
	obj := on
	if col != "" {
		obj = on + "." + col
	}
	title := fmt.Sprintf("Revogar %s de %s em %s", priv, account, obj)
	evidence, _ := json.Marshal(map[string]any{
		"campaign_id": campaignID, "item_id": itemID, "account": account,
		"object": obj, "privilege": priv, "revoke_due_at": due,
		"safety_note": "Ação assistida: o auditor não executa REVOKE.",
	})
	var findingID string
	dedup := "access_review.revoke_pending|" + campaignID + "|" + itemID
	schemaName, objectName := on, on
	if i := strings.IndexByte(on, '.'); i >= 0 {
		schemaName, objectName = on[:i], on[i+1:]
	}
	err := tx.QueryRow(ctx, `INSERT INTO finding(
environment_id,finding_type,severity,status,title,summary,object_type,object_key,database_name,schema_name,object_name,evidence,dedup_key,confidence,category,rule_id,rule_version)
VALUES($1::uuid,'access_review.revoke_pending','medium','open',$2,$3,$4,$5,$6,$7,$8,$9::jsonb,$10,0.80,'access_review','access_review.revoke_pending','1.0.0')
ON CONFLICT (environment_id, dedup_key) DO UPDATE SET updated_at=now(),last_seen_at=now(),status='open'
RETURNING id::text`, env, title,
		"Decisão de revogação na revisão de acessos. Execute o REVOKE fora do auditor e registre a medição na próxima coleta.",
		ot, obj, db, schemaName, objectName, evidence, dedup).Scan(&findingID)
	if err != nil {
		// Soft-fail: decision must still persist even if assisted-action wiring fails.
		return "", nil
	}
	_, err = tx.Exec(ctx, `INSERT INTO finding_action(finding_id,status,owner_name,justification,result_note,updated_by)
VALUES($1::uuid,'suggested',$2,$3,'',$2)
ON CONFLICT(finding_id) DO UPDATE SET status='suggested',owner_name=EXCLUDED.owner_name,justification=EXCLUDED.justification,updated_by=EXCLUDED.updated_by,updated_at=now()`,
		findingID, actor, "Revogação decidida na campanha "+campaignID)
	if err != nil {
		return findingID, nil
	}
	return findingID, nil
}

func (s *Store) DiffAccessReviewCampaign(ctx context.Context, env, campaignID string) (*AccessReviewDiff, error) {
	camp, err := s.GetAccessReviewCampaign(ctx, env, campaignID)
	if err != nil {
		return nil, err
	}
	var prevID string
	err = s.pool.QueryRow(ctx, `SELECT id::text FROM access_review_campaign
WHERE environment_id=$1::uuid AND created_at < $2 AND id<>$3::uuid
ORDER BY created_at DESC LIMIT 1`, env, camp.CreatedAt, campaignID).Scan(&prevID)
	diff := &AccessReviewDiff{
		NewPrivileges: []AccessReviewItem{}, RemovedPrivileges: []AccessReviewItem{},
		ChangedDecisions: []AccessReviewDiffDecision{}, NewAccounts: []string{}, RemovedAccounts: []string{},
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return diff, nil
	}
	if err != nil {
		return nil, err
	}
	diff.PreviousCampaignID = prevID

	cur, err := s.loadItemMap(ctx, campaignID)
	if err != nil {
		return nil, err
	}
	prev, err := s.loadItemMap(ctx, prevID)
	if err != nil {
		return nil, err
	}
	curAccounts, prevAccounts := map[string]struct{}{}, map[string]struct{}{}
	for k, item := range cur {
		curAccounts[item.AccountName] = struct{}{}
		if _, ok := prev[k]; !ok {
			diff.NewPrivileges = append(diff.NewPrivileges, item)
		} else {
			p := prev[k]
			if (item.Decision == nil) != (p.Decision == nil) ||
				(item.Decision != nil && p.Decision != nil && *item.Decision != *p.Decision) {
				diff.ChangedDecisions = append(diff.ChangedDecisions, AccessReviewDiffDecision{
					ItemKey: k, Account: item.AccountName, Object: item.ObjectName, Privilege: item.Privilege,
					Previous: p.Decision, Current: item.Decision,
				})
			}
		}
	}
	for k, item := range prev {
		prevAccounts[item.AccountName] = struct{}{}
		if _, ok := cur[k]; !ok {
			diff.RemovedPrivileges = append(diff.RemovedPrivileges, item)
		}
	}
	for a := range curAccounts {
		if _, ok := prevAccounts[a]; !ok {
			diff.NewAccounts = append(diff.NewAccounts, a)
		}
	}
	for a := range prevAccounts {
		if _, ok := curAccounts[a]; !ok {
			diff.RemovedAccounts = append(diff.RemovedAccounts, a)
		}
	}
	return diff, nil
}

func (s *Store) loadItemMap(ctx context.Context, campaignID string) (map[string]AccessReviewItem, error) {
	rows, err := s.pool.Query(ctx, `SELECT i.id::text,i.campaign_id::text,i.database_name,i.account_name,i.object_type,i.object_name,i.column_name,
i.privilege,i.origin_kind,i.via_chain,i.origin_label,i.highlights,
ld.decision
FROM access_review_item i
LEFT JOIN LATERAL (
  SELECT d.decision FROM access_review_decision d WHERE d.item_id=i.id
  AND NOT EXISTS (SELECT 1 FROM access_review_decision x WHERE x.supersedes_id=d.id)
  ORDER BY d.decided_at DESC LIMIT 1
) ld ON true
WHERE i.campaign_id=$1::uuid`, campaignID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]AccessReviewItem{}
	for rows.Next() {
		var item AccessReviewItem
		var decision *string
		if err := rows.Scan(&item.ID, &item.CampaignID, &item.DatabaseName, &item.AccountName, &item.ObjectType,
			&item.ObjectName, &item.ColumnName, &item.Privilege, &item.OriginKind, &item.ViaChain, &item.OriginLabel,
			&item.Highlights, &decision); err != nil {
			return nil, err
		}
		item.Decision = decision
		item.Pending = decision == nil
		out[itemKey(item.DatabaseName, item.AccountName, item.ObjectType, item.ObjectName, item.ColumnName, item.Privilege)] = item
	}
	return out, rows.Err()
}

// StreamAccessReviewCSV writes the matrix with decisions and ends with # export_complete.
func (s *Store) StreamAccessReviewCSV(ctx context.Context, env, campaignID string, w io.Writer) (int, error) {
	owners, err := s.ListAccessReviewOwners(ctx, env)
	if err != nil {
		return 0, err
	}
	ownerContact := map[string]string{}
	for _, o := range owners {
		label := o.OwnerName
		if o.ContactName != "" || o.ContactEmail != "" {
			parts := []string{o.OwnerName}
			if o.ContactName != "" {
				parts = append(parts, o.ContactName)
			}
			if o.ContactEmail != "" {
				parts = append(parts, o.ContactEmail)
			}
			label = strings.Join(parts, " / ")
		}
		ownerContact[o.OwnerUserID] = label
	}

	cw := csv.NewWriter(w)
	header := []string{"conta", "objeto", "tipo", "privilegio", "origem", "destaque", "responsavel", "decisao", "justificativa", "decidido_por", "decidido_em"}
	if err := cw.Write(header); err != nil {
		return 0, err
	}
	exported := 0
	offset := 0
	const page = 500
	for {
		items, total, err := s.ListAccessReviewMatrix(ctx, env, campaignID, AccessReviewMatrixFilter{Limit: page, Offset: offset})
		if err != nil {
			return exported, err
		}
		for _, item := range items {
			owner := ""
			if item.OwnerUserID != nil {
				owner = ownerContact[*item.OwnerUserID]
				if owner == "" {
					owner = item.OwnerName
				}
			}
			obj := item.ObjectName
			if item.ColumnName != "" {
				obj = item.ObjectName + "." + item.ColumnName
			}
			dec, just, by, at := "", item.Justification, item.DecidedBy, ""
			if item.Decision != nil {
				dec = *item.Decision
			}
			if item.DecidedAt != nil {
				at = item.DecidedAt.UTC().Format(time.RFC3339)
			}
			if err := cw.Write([]string{
				item.AccountName, obj, item.ObjectType, item.Privilege, item.OriginLabel,
				strings.Join(item.Highlights, "|"), owner, dec, just, by, at,
			}); err != nil {
				return exported, err
			}
			exported++
		}
		offset += len(items)
		if offset >= total || len(items) == 0 {
			break
		}
		cw.Flush()
	}
	_ = cw.Write([]string{"# export_complete", fmt.Sprint(exported), "", "", "", "", "", "", "", "", ""})
	cw.Flush()
	return exported, cw.Error()
}

// CountAccessReviewPending returns pending lines for metrics.
func (s *Store) CountAccessReviewCampaignStats(ctx context.Context) (byStatus map[string]int, pending int, err error) {
	byStatus = map[string]int{}
	rows, err := s.pool.Query(ctx, `SELECT status, count(*) FROM access_review_campaign GROUP BY status`)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	for rows.Next() {
		var st string
		var n int
		if err := rows.Scan(&st, &n); err != nil {
			return nil, 0, err
		}
		byStatus[st] = n
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	err = s.pool.QueryRow(ctx, `SELECT count(*) FROM access_review_item i
WHERE NOT EXISTS (
  SELECT 1 FROM access_review_decision d WHERE d.item_id=i.id
  AND NOT EXISTS (SELECT 1 FROM access_review_decision x WHERE x.supersedes_id=d.id)
)`).Scan(&pending)
	return byStatus, pending, err
}

// LoadAccessReviewFacts attaches open/in_review/overdue campaigns for analyzer rules.
func (s *Store) LoadAccessReviewFacts(ctx context.Context, environmentID, auditRunID string) ([]analyzer.AccessReviewCampaignFact, error) {
	camps, _, err := s.ListAccessReviewCampaigns(ctx, environmentID, 200, 0)
	if err != nil {
		return nil, err
	}
	out := make([]analyzer.AccessReviewCampaignFact, 0)
	for _, c := range camps {
		if c.Status == "closed" {
			continue
		}
		fact := analyzer.AccessReviewCampaignFact{
			ID: c.ID, EnvironmentID: c.EnvironmentID, Status: c.Status, DueAt: c.DueAt,
			TotalItems: c.TotalItems, PendingItems: c.PendingItems,
		}
		items, _, err := s.ListAccessReviewMatrix(ctx, environmentID, c.ID, AccessReviewMatrixFilter{Limit: 5000, Offset: 0})
		if err != nil {
			return nil, err
		}
		for _, it := range items {
			dec := ""
			if it.Decision != nil {
				dec = *it.Decision
			}
			itemFact := analyzer.AccessReviewItemFact{
				ID: it.ID, AccountName: it.AccountName, ObjectType: it.ObjectType,
				ObjectName: it.ObjectName, ColumnName: it.ColumnName, Privilege: it.Privilege,
				OriginKind: it.OriginKind, Highlights: it.Highlights, Decision: dec,
				LastLoginAt: it.LastLoginAt, LastLoginSource: it.LastLoginSource,
			}
			if dec == "revoke" {
				var due *time.Time
				var actionID *string
				_ = s.pool.QueryRow(ctx, `SELECT revoke_due_at, finding_action_id::text FROM access_review_decision d
WHERE d.item_id=$1::uuid AND NOT EXISTS (SELECT 1 FROM access_review_decision x WHERE x.supersedes_id=d.id)
ORDER BY d.decided_at DESC LIMIT 1`, it.ID).Scan(&due, &actionID)
				itemFact.RevokeDueAt = due
				if actionID != nil {
					itemFact.FindingActionID = *actionID
				}
				var present int
				err := s.pool.QueryRow(ctx, `SELECT 1 FROM grant_origin_snapshot
WHERE environment_id=$1::uuid AND audit_run_id=$2::uuid AND account_name=$3 AND object_name=$4 AND privilege=$5
LIMIT 1`, environmentID, auditRunID, it.AccountName, it.ObjectName, it.Privilege).Scan(&present)
				itemFact.PrivilegeStillPresent = err == nil && present == 1
			}
			fact.Items = append(fact.Items, itemFact)
		}
		out = append(out, fact)
	}
	return out, nil
}

// BuildAccessReviewSummary prepares PDF evidence for a campaign.
func (s *Store) BuildAccessReviewSummary(ctx context.Context, env, campaignID string) (*report.AccessReviewSummary, error) {
	camp, err := s.GetAccessReviewCampaign(ctx, env, campaignID)
	if err != nil {
		return nil, err
	}
	owners, err := s.ListAccessReviewOwners(ctx, env)
	if err != nil {
		return nil, err
	}
	sum := &report.AccessReviewSummary{
		CampaignID: camp.ID, CampaignName: camp.Name, Status: camp.Status,
		DueAt: camp.DueAt.UTC().Format(time.RFC3339),
		TotalItems: camp.TotalItems, DecidedItems: camp.DecidedItems,
		PendingItems: camp.PendingItems, DecidedPercent: camp.DecidedPercent,
	}
	for _, o := range owners {
		contact := strings.TrimSpace(strings.TrimSpace(o.ContactName) + " " + strings.TrimSpace(o.ContactEmail))
		sum.Owners = append(sum.Owners, report.AccessReviewOwnerLine{
			TargetRole: o.TargetRole, OwnerName: o.OwnerName, Contact: strings.TrimSpace(contact),
		})
	}
	pending := true
	items, _, err := s.ListAccessReviewMatrix(ctx, env, campaignID, AccessReviewMatrixFilter{Pending: &pending, Limit: 20, Offset: 0})
	if err != nil {
		return nil, err
	}
	for _, it := range items {
		obj := it.ObjectName
		if it.ColumnName != "" {
			obj = it.ObjectName + "." + it.ColumnName
		}
		sum.PendingSamples = append(sum.PendingSamples, report.AccessReviewPendingLine{
			Account: it.AccountName, Object: obj, Privilege: it.Privilege, Origin: it.OriginLabel,
		})
	}
	return sum, nil
}
