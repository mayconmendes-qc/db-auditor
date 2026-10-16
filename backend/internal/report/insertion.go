package report

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"
)

var insertionIdentifier = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// InsertionContract is confirmed by a named auditor/operator before collection.
// append_only=false still permits a present-row maximum, but not a historical
// last-insertion conclusion.
type InsertionContract struct {
	Column         string `json:"column"`
	Timezone       string `json:"timezone"`
	Validated      bool   `json:"validated"`
	FilledOnInsert bool   `json:"filled_on_insert"`
	AppendOnly     bool   `json:"append_only"`
}

func ParseInsertionContract(value string) (InsertionContract, error) {
	var contract InsertionContract
	decoder := json.NewDecoder(strings.NewReader(value))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&contract); err != nil {
		return contract, fmt.Errorf("invalid insertion contract: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return contract, fmt.Errorf("insertion contract has trailing data")
	}
	if !insertionIdentifier.MatchString(contract.Column) || !contract.Validated || !contract.FilledOnInsert || contract.Timezone == "" {
		return contract, fmt.Errorf("insertion contract requires validated column, timezone and insert-time filling")
	}
	if _, err := time.LoadLocation(contract.Timezone); err != nil {
		return contract, fmt.Errorf("invalid insertion timezone")
	}
	return contract, nil
}

// InsertionEvidence is the result of a bounded read in the audited run. A
// timestamp alone never proves historical insertion when deletes are possible.
type InsertionEvidence struct {
	ObjectID    string
	Column      string
	Latest      *time.Time
	CollectedAt time.Time
	Status      string // observed, empty, skipped_budget, permission_denied, failed, invalid_mapping
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
	if evidence.Latest.After(asOf) {
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
