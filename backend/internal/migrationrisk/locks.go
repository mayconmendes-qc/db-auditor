package migrationrisk

// Lock documentation references per PostgreSQL major (RF-03).
// Pages are the official Explicit Locking docs for each major.
var lockDocURLs = map[int]string{
	14: "https://www.postgresql.org/docs/14/explicit-locking.html",
	15: "https://www.postgresql.org/docs/15/explicit-locking.html",
	16: "https://www.postgresql.org/docs/16/explicit-locking.html",
	17: "https://www.postgresql.org/docs/17/explicit-locking.html",
	18: "https://www.postgresql.org/docs/18/explicit-locking.html",
}

// LockMode describes the lock taken by an operation on a given PG major.
type LockMode struct {
	Name string
	Doc  string
}

// LockFor returns the documented lock mode for an operation kind.
func LockFor(kind string, pgMajor int) LockMode {
	doc := lockDocURLs[pgMajor]
	if doc == "" {
		doc = lockDocURLs[16]
	}
	switch kind {
	case "create_index":
		return LockMode{Name: "SHARE", Doc: doc}
	case "add_column_default", "alter_column_type", "set_not_null",
		"add_check", "add_pk_unique", "drop_or_rename", "drop", "rename",
		"vacuum_full", "cluster", "refresh_matview":
		return LockMode{Name: "ACCESS EXCLUSIVE", Doc: doc}
	case "add_fk":
		return LockMode{Name: "SHARE ROW EXCLUSIVE", Doc: doc}
	default:
		return LockMode{Name: "", Doc: doc}
	}
}

// SupportsNotNullNotValid reports whether PG major accepts NOT NULL ... NOT VALID (PG 18+).
func SupportsNotNullNotValid(pgMajor int) bool {
	return pgMajor >= 18
}
