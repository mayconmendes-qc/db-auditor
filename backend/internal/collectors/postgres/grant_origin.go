package postgres

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/osmendes/db-auditor/internal/config"
)

// MaxGrantOrigins bounds rows kept per database for grant_origin_snapshot.
const MaxGrantOrigins = 200000

// MaxColumnGrants bounds column ACL expansions per database.
const MaxColumnGrants = 50000

// GrantOriginFacts is one effective privilege with its grant path.
type GrantOriginFacts struct {
	DatabaseName string
	AccountName  string
	ObjectType   string // table|view|sequence|function|schema
	ObjectName   string // qualified (schema.obj) except schema (name only)
	Privilege    string
	OriginKind   string // direct|role|public|predefined
	ViaChain     []string
	InheritPath  *bool // set on PG 16+ when origin is role/predefined
	SetPath      *bool
}

// ColumnGrantFacts is one column privilege row (own matrix line, RF-03).
type ColumnGrantFacts struct {
	DatabaseName string
	SchemaName   string
	TableName    string
	ColumnName   string
	AccountName  string
	Privilege    string
	OriginKind   string
	ViaChain     []string
	InheritPath  *bool
	SetPath      *bool
}

// OriginLabelPT returns the Portuguese label for UI/CSV (direto / via X / PUBLIC / predefinida).
func OriginLabelPT(kind string, via []string) string {
	switch kind {
	case "direct":
		return "direto"
	case "public":
		return "PUBLIC"
	case "predefined":
		if len(via) > 0 {
			return "predefinida (" + strings.Join(via, ",") + ")"
		}
		return "predefinida"
	case "role":
		if len(via) == 0 {
			return "via role"
		}
		return "via " + strings.Join(via, " → ")
	default:
		return kind
	}
}

// Must stay byte-aligned with sql/collectors/postgres/grant_origins.sql (minus header comment).
const grantOriginsSQL = `
SELECT current_database(), r.rolname,
  CASE c.relkind WHEN 'v' THEN 'view' WHEN 'm' THEN 'view' WHEN 'S' THEN 'sequence' ELSE 'table' END,
  n.nspname || '.' || c.relname, p.priv
FROM pg_catalog.pg_class c
JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
CROSS JOIN pg_catalog.pg_roles r
CROSS JOIN LATERAL unnest(
  CASE WHEN c.relkind = 'S' THEN ARRAY['SELECT','USAGE','UPDATE']::text[]
  ELSE ARRAY['SELECT','INSERT','UPDATE','DELETE','TRUNCATE','REFERENCES','TRIGGER']::text[] END
) AS p(priv)
WHERE c.relkind IN ('r','p','f','v','m','S') AND r.rolcanlogin AND r.rolname !~ '^pg_'
  AND n.nspname NOT LIKE 'pg\_%' ESCAPE '\'
  AND n.nspname <> 'information_schema'
  AND CASE WHEN c.relkind = 'S' THEN has_sequence_privilege(r.oid, c.oid, p.priv)
    ELSE has_table_privilege(r.oid, c.oid, p.priv) END
UNION ALL
SELECT current_database(), r.rolname, 'function',
  n.nspname || '.' || p.proname || '(' || pg_catalog.pg_get_function_identity_arguments(p.oid) || ')',
  'EXECUTE'
FROM pg_catalog.pg_proc p
JOIN pg_catalog.pg_namespace n ON n.oid = p.pronamespace
CROSS JOIN pg_catalog.pg_roles r
WHERE r.rolcanlogin AND r.rolname !~ '^pg_'
  AND n.nspname NOT LIKE 'pg\_%' ESCAPE '\'
  AND n.nspname <> 'information_schema'
  AND has_function_privilege(r.oid, p.oid, 'EXECUTE')
UNION ALL
SELECT current_database(), r.rolname, 'schema', n.nspname, p.priv
FROM pg_catalog.pg_namespace n
CROSS JOIN pg_catalog.pg_roles r
CROSS JOIN LATERAL unnest(ARRAY['USAGE','CREATE']::text[]) AS p(priv)
WHERE r.rolcanlogin AND r.rolname !~ '^pg_'
  AND n.nspname NOT LIKE 'pg\_%' ESCAPE '\'
  AND n.nspname <> 'information_schema'
  AND has_schema_privilege(r.oid, n.oid, p.priv)
ORDER BY 1, 2, 3, 4, 5
`

// Must stay byte-aligned with sql/collectors/postgres/column_grants.sql (minus header comment).
const columnGrantsSQL = `
SELECT current_database(), n.nspname, c.relname, a.attname,
  CASE WHEN acl.grantee = 0 THEN '' ELSE g.rolname END AS grantee,
  acl.privilege_type
FROM pg_catalog.pg_attribute a
JOIN pg_catalog.pg_class c ON c.oid = a.attrelid AND c.relkind IN ('r','p','f','v','m')
JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
CROSS JOIN LATERAL aclexplode(a.attacl) AS acl
LEFT JOIN pg_catalog.pg_roles g ON g.oid = acl.grantee
WHERE a.attnum > 0 AND NOT a.attisdropped AND a.attacl IS NOT NULL
  AND n.nspname NOT LIKE 'pg\_%' ESCAPE '\'
  AND n.nspname <> 'information_schema'
ORDER BY n.nspname, c.relname, a.attname, 5, acl.privilege_type
`

// Object ACL rows used to resolve origin of effective privileges.
const objectACLSQL = `
SELECT
  CASE c.relkind WHEN 'v' THEN 'view' WHEN 'm' THEN 'view' WHEN 'S' THEN 'sequence' ELSE 'table' END,
  n.nspname || '.' || c.relname,
  CASE WHEN a.grantee = 0 THEN '' ELSE g.rolname END,
  a.privilege_type,
  o.rolname AS owner
FROM pg_catalog.pg_class c
JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
JOIN pg_catalog.pg_roles o ON o.oid = c.relowner
CROSS JOIN LATERAL aclexplode(COALESCE(c.relacl, acldefault(
  CASE WHEN c.relkind = 'S' THEN 'S'::"char" ELSE 'r'::"char" END, c.relowner))) a
LEFT JOIN pg_catalog.pg_roles g ON g.oid = a.grantee
WHERE c.relkind IN ('r','p','f','v','m','S')
  AND n.nspname NOT LIKE 'pg\_%' ESCAPE '\'
  AND n.nspname <> 'information_schema'
UNION ALL
SELECT 'function',
  n.nspname || '.' || p.proname || '(' || pg_catalog.pg_get_function_identity_arguments(p.oid) || ')',
  CASE WHEN a.grantee = 0 THEN '' ELSE g.rolname END,
  a.privilege_type,
  o.rolname
FROM pg_catalog.pg_proc p
JOIN pg_catalog.pg_namespace n ON n.oid = p.pronamespace
JOIN pg_catalog.pg_roles o ON o.oid = p.proowner
CROSS JOIN LATERAL aclexplode(COALESCE(p.proacl, acldefault('f'::"char", p.proowner))) a
LEFT JOIN pg_catalog.pg_roles g ON g.oid = a.grantee
WHERE n.nspname NOT LIKE 'pg\_%' ESCAPE '\'
  AND n.nspname <> 'information_schema'
UNION ALL
SELECT 'schema', n.nspname,
  CASE WHEN a.grantee = 0 THEN '' ELSE g.rolname END,
  a.privilege_type,
  o.rolname
FROM pg_catalog.pg_namespace n
JOIN pg_catalog.pg_roles o ON o.oid = n.nspowner
CROSS JOIN LATERAL aclexplode(COALESCE(n.nspacl, acldefault('n'::"char", n.nspowner))) a
LEFT JOIN pg_catalog.pg_roles g ON g.oid = a.grantee
WHERE n.nspname NOT LIKE 'pg\_%' ESCAPE '\'
  AND n.nspname <> 'information_schema'
`

// Membership edges with version-aware reachability flags for the login role.
const membershipReachSQL = `
SELECT l.rolname AS login_role, g.rolname AS granted_role,
  pg_catalog.pg_has_role(l.oid, g.oid, $1) AS set_ok,
  pg_catalog.pg_has_role(l.oid, g.oid, $2) AS inherit_ok,
  pg_catalog.pg_has_role(l.oid, g.oid, $3) AS admin_ok
FROM pg_catalog.pg_roles l
CROSS JOIN pg_catalog.pg_roles g
WHERE l.rolcanlogin AND l.rolname !~ '^pg_' AND g.oid <> l.oid
  AND (pg_catalog.pg_has_role(l.oid, g.oid, $1)
    OR pg_catalog.pg_has_role(l.oid, g.oid, $2)
    OR pg_catalog.pg_has_role(l.oid, g.oid, $3))
`

type aclKey struct {
	objectType string
	objectName string
	privilege  string
}

type aclGrant struct {
	grantee string // "" = PUBLIC
	owner   string
}

type reachEdge struct {
	granted string
	setOK   bool
	inherit bool
	adminOK bool
}

// CollectGrantOrigins resolves effective privilege origins for one database.
// Catalog read failures are returned as errors so ForEachUserDatabase records
// partial coverage without failing the whole run.
func CollectGrantOrigins(ctx context.Context, conn *pgx.Conn, scope config.Scope) ([]GrantOriginFacts, []ColumnGrantFacts, error) {
	var version int
	if err := conn.QueryRow(ctx, `SELECT current_setting('server_version_num')::int`).Scan(&version); err != nil {
		return nil, nil, fmt.Errorf("versão do servidor: %w", err)
	}
	setMode, inheritMode, adminMode := membershipModes(version)
	pg16 := version >= 160000

	eff, err := queryEffectivePrivileges(ctx, conn, scope)
	if err != nil {
		return nil, nil, err
	}
	acls, err := queryObjectACLs(ctx, conn, scope)
	if err != nil {
		return nil, nil, err
	}
	reach, err := queryMembershipReach(ctx, conn, setMode, inheritMode, adminMode)
	if err != nil {
		return nil, nil, err
	}

	var database string
	if err := conn.QueryRow(ctx, `SELECT current_database()`).Scan(&database); err != nil {
		return nil, nil, fmt.Errorf("database atual: %w", err)
	}
	// has_*_privilege only sees currently inherited privileges. CA-08 also
	// requires SET/MEMBER paths (NOINHERIT, INHERIT FALSE + SET TRUE).
	eff = expandReachablePrivileges(eff, acls, reach, database)

	out := make([]GrantOriginFacts, 0, len(eff))
	for _, e := range eff {
		kind, via, inherit, setOK := resolveOrigin(e.AccountName, e.ObjectType, e.ObjectName, e.Privilege, acls, reach, pg16)
		item := GrantOriginFacts{
			DatabaseName: e.DatabaseName,
			AccountName:  e.AccountName,
			ObjectType:   e.ObjectType,
			ObjectName:   e.ObjectName,
			Privilege:    e.Privilege,
			OriginKind:   kind,
			ViaChain:     via,
		}
		if pg16 && (kind == "role" || kind == "predefined") {
			item.InheritPath = boolPtr(inherit)
			item.SetPath = boolPtr(setOK)
		}
		out = append(out, item)
		if len(out) > MaxGrantOrigins {
			return nil, nil, fmt.Errorf("mais de %d origens de privilégio; reduza o escopo da coleta", MaxGrantOrigins)
		}
	}

	cols, err := collectColumnGrants(ctx, conn, scope, reach, pg16)
	if err != nil {
		return nil, nil, err
	}
	return out, cols, nil
}

type effRow struct {
	DatabaseName, AccountName, ObjectType, ObjectName, Privilege string
}

func queryEffectivePrivileges(ctx context.Context, conn *pgx.Conn, scope config.Scope) ([]effRow, error) {
	rows, err := conn.Query(ctx, grantOriginsSQL)
	if err != nil {
		return nil, fmt.Errorf("consultar privilégios efetivos: %w", err)
	}
	defer rows.Close()
	out := make([]effRow, 0)
	for rows.Next() {
		var r effRow
		if err := rows.Scan(&r.DatabaseName, &r.AccountName, &r.ObjectType, &r.ObjectName, &r.Privilege); err != nil {
			return nil, err
		}
		schema := schemaOf(r.ObjectType, r.ObjectName)
		if scope.AllowsSchema(schema) {
			out = append(out, r)
		}
	}
	return out, rows.Err()
}

func queryObjectACLs(ctx context.Context, conn *pgx.Conn, scope config.Scope) (map[aclKey][]aclGrant, error) {
	rows, err := conn.Query(ctx, objectACLSQL)
	if err != nil {
		return nil, fmt.Errorf("consultar ACLs de objeto: %w", err)
	}
	defer rows.Close()
	out := make(map[aclKey][]aclGrant)
	for rows.Next() {
		var objectType, objectName, grantee, privilege, owner string
		if err := rows.Scan(&objectType, &objectName, &grantee, &privilege, &owner); err != nil {
			return nil, err
		}
		if !scope.AllowsSchema(schemaOf(objectType, objectName)) {
			continue
		}
		key := aclKey{objectType, objectName, privilege}
		out[key] = append(out[key], aclGrant{grantee: grantee, owner: owner})
	}
	return out, rows.Err()
}

func queryMembershipReach(ctx context.Context, conn *pgx.Conn, setMode, inheritMode, adminMode string) (map[string][]reachEdge, error) {
	rows, err := conn.Query(ctx, membershipReachSQL, setMode, inheritMode, adminMode)
	if err != nil {
		return nil, fmt.Errorf("consultar membership de roles: %w", err)
	}
	defer rows.Close()
	out := make(map[string][]reachEdge)
	for rows.Next() {
		var login, granted string
		var setOK, inheritOK, adminOK bool
		if err := rows.Scan(&login, &granted, &setOK, &inheritOK, &adminOK); err != nil {
			return nil, err
		}
		out[login] = append(out[login], reachEdge{granted: granted, setOK: setOK, inherit: inheritOK, adminOK: adminOK})
	}
	return out, rows.Err()
}

// expandReachablePrivileges adds privileges available via SET/MEMBER/ADMIN
// membership to an ACL grantee that has_*_privilege does not report.
func expandReachablePrivileges(eff []effRow, acls map[aclKey][]aclGrant, reach map[string][]reachEdge, database string) []effRow {
	seen := make(map[string]struct{}, len(eff))
	keyOf := func(account, objectType, objectName, privilege string) string {
		return account + "\x00" + objectType + "\x00" + objectName + "\x00" + privilege
	}
	for _, e := range eff {
		seen[keyOf(e.AccountName, e.ObjectType, e.ObjectName, e.Privilege)] = struct{}{}
	}
	out := append([]effRow(nil), eff...)
	for k, grants := range acls {
		for _, g := range grants {
			if g.grantee == "" {
				continue // PUBLIC already surfaces through has_*_privilege
			}
			for login, edges := range reach {
				for _, e := range edges {
					if e.granted != g.grantee || !(e.setOK || e.inherit || e.adminOK) {
						continue
					}
					sk := keyOf(login, k.objectType, k.objectName, k.privilege)
					if _, ok := seen[sk]; ok {
						continue
					}
					seen[sk] = struct{}{}
					out = append(out, effRow{
						DatabaseName: database,
						AccountName:  login,
						ObjectType:   k.objectType,
						ObjectName:   k.objectName,
						Privilege:    k.privilege,
					})
				}
			}
		}
	}
	return out
}

func resolveOrigin(account, objectType, objectName, privilege string, acls map[aclKey][]aclGrant, reach map[string][]reachEdge, pg16 bool) (kind string, via []string, inherit, setOK bool) {
	grants := acls[aclKey{objectType, objectName, privilege}]
	owner := ""
	direct, pub := false, false
	grantedRoles := make([]string, 0)
	for _, g := range grants {
		if owner == "" {
			owner = g.owner
		}
		switch {
		case g.grantee == account:
			direct = true
		case g.grantee == "":
			pub = true
		default:
			grantedRoles = append(grantedRoles, g.grantee)
		}
	}
	if owner == account || direct {
		return "direct", nil, false, false
	}
	if pub {
		return "public", nil, false, false
	}

	// Predefined roles that confer table data access.
	predef := ""
	switch privilege {
	case "SELECT":
		predef = "pg_read_all_data"
	case "INSERT", "UPDATE", "DELETE":
		predef = "pg_write_all_data"
	}
	edges := reach[account]
	if predef != "" {
		for _, e := range edges {
			if e.granted == predef && (e.setOK || e.inherit || e.adminOK) {
				return "predefined", []string{predef}, e.inherit, e.setOK
			}
		}
	}

	// Prefer a role that appears on the ACL and is reachable.
	sort.Strings(grantedRoles)
	for _, role := range grantedRoles {
		for _, e := range edges {
			if e.granted != role {
				continue
			}
			if e.setOK || e.inherit || e.adminOK {
				return "role", []string{role}, e.inherit, e.setOK
			}
		}
	}
	// Fallback: any reachable role (owner path / default ACL already covered).
	if owner != "" && owner != account {
		for _, e := range edges {
			if e.granted == owner && (e.setOK || e.inherit || e.adminOK) {
				return "role", []string{owner}, e.inherit, e.setOK
			}
		}
	}
	_ = pg16
	return "direct", nil, false, false
}

func collectColumnGrants(ctx context.Context, conn *pgx.Conn, scope config.Scope, reach map[string][]reachEdge, pg16 bool) ([]ColumnGrantFacts, error) {
	rows, err := conn.Query(ctx, columnGrantsSQL)
	if err != nil {
		return nil, fmt.Errorf("consultar privilégios por coluna: %w", err)
	}
	defer rows.Close()

	type rawCol struct {
		db, schema, table, column, grantee, privilege string
	}
	raw := make([]rawCol, 0)
	for rows.Next() {
		var r rawCol
		if err := rows.Scan(&r.db, &r.schema, &r.table, &r.column, &r.grantee, &r.privilege); err != nil {
			return nil, err
		}
		if scope.AllowsSchema(r.schema) {
			raw = append(raw, r)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	logins := make(map[string]struct{})
	for login := range reach {
		logins[login] = struct{}{}
	}
	// Also include logins with no membership edges.
	loginRows, err := conn.Query(ctx, `SELECT rolname FROM pg_catalog.pg_roles WHERE rolcanlogin AND rolname !~ '^pg_'`)
	if err != nil {
		return nil, err
	}
	defer loginRows.Close()
	for loginRows.Next() {
		var name string
		if err := loginRows.Scan(&name); err != nil {
			return nil, err
		}
		logins[name] = struct{}{}
	}
	if err := loginRows.Err(); err != nil {
		return nil, err
	}

	out := make([]ColumnGrantFacts, 0)
	seen := make(map[string]struct{})
	for _, r := range raw {
		targets := make([]struct {
			account string
			kind    string
			via     []string
			inherit bool
			setOK   bool
		}, 0)
		if r.grantee == "" {
			for login := range logins {
				targets = append(targets, struct {
					account string
					kind    string
					via     []string
					inherit bool
					setOK   bool
				}{login, "public", nil, false, false})
			}
		} else if _, isLogin := logins[r.grantee]; isLogin {
			targets = append(targets, struct {
				account string
				kind    string
				via     []string
				inherit bool
				setOK   bool
			}{r.grantee, "direct", nil, false, false})
		} else {
			for login, edges := range reach {
				for _, e := range edges {
					if e.granted == r.grantee && (e.setOK || e.inherit || e.adminOK) {
						kind := "role"
						via := []string{r.grantee}
						if isPredefinedRole(r.grantee) {
							kind = "predefined"
						}
						targets = append(targets, struct {
							account string
							kind    string
							via     []string
							inherit bool
							setOK   bool
						}{login, kind, via, e.inherit, e.setOK})
						break
					}
				}
			}
		}
		for _, t := range targets {
			key := strings.Join([]string{r.db, r.schema, r.table, r.column, t.account, r.privilege}, "\x00")
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			item := ColumnGrantFacts{
				DatabaseName: r.db,
				SchemaName:   r.schema,
				TableName:    r.table,
				ColumnName:   r.column,
				AccountName:  t.account,
				Privilege:    r.privilege,
				OriginKind:   t.kind,
				ViaChain:     t.via,
			}
			if pg16 && (t.kind == "role" || t.kind == "predefined") {
				item.InheritPath = boolPtr(t.inherit)
				item.SetPath = boolPtr(t.setOK)
			}
			out = append(out, item)
			if len(out) > MaxColumnGrants {
				return nil, fmt.Errorf("mais de %d privilégios por coluna; reduza o escopo da coleta", MaxColumnGrants)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.SchemaName != b.SchemaName {
			return a.SchemaName < b.SchemaName
		}
		if a.TableName != b.TableName {
			return a.TableName < b.TableName
		}
		if a.ColumnName != b.ColumnName {
			return a.ColumnName < b.ColumnName
		}
		if a.AccountName != b.AccountName {
			return a.AccountName < b.AccountName
		}
		return a.Privilege < b.Privilege
	})
	return out, nil
}

func schemaOf(objectType, objectName string) string {
	if objectType == "schema" {
		return objectName
	}
	if i := strings.IndexByte(objectName, '.'); i >= 0 {
		return objectName[:i]
	}
	return objectName
}

func isPredefinedRole(name string) bool {
	for _, p := range PowerfulPredefinedRoles {
		if p == name {
			return true
		}
	}
	return false
}

func boolPtr(v bool) *bool { return &v }
