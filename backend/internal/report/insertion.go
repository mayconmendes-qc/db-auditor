package report

import "time"

// InsertionEvidence is the result of a bounded read in the audited run. A
// timestamp alone never proves historical insertion when deletes are possible.
type InsertionEvidence struct {
	ObjectID    string
	Column      string
	Latest      *time.Time
	CollectedAt time.Time
	Status      string // observed, empty, skipped_budget, permission_denied, failed
	Reason      string
	AppendOnly  bool
}

type InsertionState string

const (
	InsertionRecent        InsertionState = "recente"
	InsertionOld           InsertionState = "há mais de seis meses"
	InsertionIndeterminate InsertionState = "indeterminado"
	InsertionNoTables      InsertionState = "sem tabelas"
)

func ClassifyInsertion(asOf time.Time, evidence *InsertionEvidence) InsertionState {
	if evidence == nil || evidence.Status != "observed" || evidence.Latest == nil || !evidence.AppendOnly {
		return InsertionIndeterminate
	}
	if !evidence.Latest.Before(sixMonthsBefore(asOf)) {
		return InsertionRecent
	}
	return InsertionOld
}

func AggregateInsertion(states []InsertionState) InsertionState {
	if len(states) == 0 {
		return InsertionNoTables
	}
	allOld := true
	for _, state := range states {
		if state == InsertionRecent {
			return InsertionRecent
		}
		if state != InsertionOld {
			allOld = false
		}
	}
	if allOld {
		return InsertionOld
	}
	return InsertionIndeterminate
}
