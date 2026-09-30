package postgres

// Relation class labels persisted and exposed to the UI (US-051).
const (
	RelationClassTable            = "table"
	RelationClassPartitionedTable = "partitioned_table"
	RelationClassPartition        = "partition"
	RelationClassForeignTable     = "foreign_table"
	RelationClassUnknown          = "unknown"
)

// ClassifyRelation maps PostgreSQL relkind (+ partition flag) to a stable product class.
// relkind: r=ordinary, p=partitioned parent, f=foreign; partitions may be r/p with relispartition.
func ClassifyRelation(relkind string, isPartition bool) string {
	if isPartition {
		return RelationClassPartition
	}
	switch relkind {
	case "r":
		return RelationClassTable
	case "p":
		return RelationClassPartitionedTable
	case "f":
		return RelationClassForeignTable
	default:
		return RelationClassUnknown
	}
}
