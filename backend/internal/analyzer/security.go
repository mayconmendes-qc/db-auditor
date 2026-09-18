package analyzer

import (
	"context"
	"fmt"
	"strings"
)

// SecurityAnalyzer flags SECURITY DEFINER functions and broad privileges for human review.
// It never recommends REVOKE or DROP automatically.
type SecurityAnalyzer struct{}

func (SecurityAnalyzer) Name() string { return "security" }

// Privileges considered broadly powerful when granted to PUBLIC or non-owner roles.
var excessivePrivileges = map[string]bool{
	"ALL":        true,
	"SUPERUSER":  true,
	"CREATE":     true,
	"TRUNCATE":   true,
	"REFERENCES": true,
	"TRIGGER":    true,
	"EXECUTE":    true, // on sensitive functions still review
	"INSERT":     true,
	"UPDATE":     true,
	"DELETE":     true,
}

func (SecurityAnalyzer) Analyze(_ context.Context, facts SnapshotFacts) ([]Finding, error) {
	out := make([]Finding, 0)

	// SECURITY DEFINER functions — list for review only
	for _, fn := range facts.Functions {
		if !fn.IsSecurityDefiner {
			continue
		}
		key := fmt.Sprintf("%s.%s.%s", fn.Database, fn.Schema, fn.FunctionName)
		if fn.IdentityArgs != "" {
			key = key + "(" + fn.IdentityArgs + ")"
		}
		title := fmt.Sprintf("SECURITY DEFINER function: %s", key)
		f := Finding{
			EnvironmentID: facts.EnvironmentID,
			AuditRunID:    facts.AuditRunID,
			FindingType:   "security.security_definer",
			Severity:      SeverityMedium,
			Status:        StatusOpen,
			Title:         title,
			Summary:       "Function runs with owner privileges (SECURITY DEFINER). Confirm search_path and privilege need; no automatic change is applied.",
			ObjectType:    "function",
			ObjectKey:     key,
			DatabaseName:  fn.Database,
			SchemaName:    fn.Schema,
			ObjectName:    fn.FunctionName,
			Evidence: map[string]any{
				"is_security_definer": true,
				"owner":               fn.Owner,
				"language":            fn.Language,
				"identity_arguments":  fn.IdentityArgs,
				"safety_note":         "Review only — never ALTER or DROP automatically",
			},
			DedupKey: DedupKey("security.security_definer", key, title),
		}
		out = append(out, f)
	}

	// Superuser / powerful roles
	for _, role := range facts.Roles {
		if !role.Superuser && !role.BypassRLS && !role.Replication {
			continue
		}
		key := fmt.Sprintf("%s.role:%s", role.Database, role.RoleName)
		title := fmt.Sprintf("Powerful role: %s", role.RoleName)
		f := Finding{
			EnvironmentID: facts.EnvironmentID,
			AuditRunID:    facts.AuditRunID,
			FindingType:   "security.powerful_role",
			Severity:      SeverityHigh,
			Status:        StatusOpen,
			Title:         title,
			Summary:       "Role has elevated attributes (superuser, bypass RLS, or replication). Confirm least-privilege need.",
			ObjectType:    "role",
			ObjectKey:     key,
			DatabaseName:  role.Database,
			ObjectName:    role.RoleName,
			Evidence: map[string]any{
				"superuser":   role.Superuser,
				"bypass_rls":  role.BypassRLS,
				"replication": role.Replication,
				"login":       role.Login,
				"safety_note": "Never ALTER ROLE or REVOKE automatically",
			},
			DedupKey: DedupKey("security.powerful_role", key, title),
		}
		out = append(out, f)
	}

	// Excessive grants (PUBLIC or ALL on application objects)
	for _, g := range facts.Grants {
		priv := strings.ToUpper(strings.TrimSpace(g.Privilege))
		grantee := strings.ToUpper(strings.TrimSpace(g.Grantee))
		isPublic := grantee == "PUBLIC" || grantee == ""
		isExcessive := excessivePrivileges[priv] || priv == "ALL"
		if !isPublic && !isExcessive {
			continue
		}
		// Skip routine SELECT-only grants to specific roles
		if !isPublic && priv == "SELECT" {
			continue
		}
		key := fmt.Sprintf("%s.%s.%s:%s:%s", g.Database, g.Schema, g.ObjectName, g.Grantee, priv)
		title := fmt.Sprintf("Privilege review: %s on %s to %s", priv, g.ObjectName, g.Grantee)
		sev := SeverityLow
		if isPublic && (priv == "ALL" || priv == "INSERT" || priv == "UPDATE" || priv == "DELETE") {
			sev = SeverityMedium
		}
		f := Finding{
			EnvironmentID: facts.EnvironmentID,
			AuditRunID:    facts.AuditRunID,
			FindingType:   "security.excessive_privilege",
			Severity:      sev,
			Status:        StatusOpen,
			Title:         title,
			Summary:       "Broad privilege detected for review. Confirm business need; auditor never issues REVOKE.",
			ObjectType:    g.ObjectType,
			ObjectKey:     key,
			DatabaseName:  g.Database,
			SchemaName:    g.Schema,
			ObjectName:    g.ObjectName,
			Evidence: map[string]any{
				"grantee":     g.Grantee,
				"privilege":   priv,
				"grantable":   g.Grantable,
				"object_type": g.ObjectType,
				"is_public":   isPublic,
				"safety_note": "Review only — never REVOKE automatically",
			},
			DedupKey: DedupKey("security.excessive_privilege", key, title),
		}
		out = append(out, f)
	}

	return out, nil
}
