package domain

import (
	"fmt"
	"strings"
	"time"
)

// ObservationWindow is the as-of boundary of one longitudinal iteration
// (#71). AsOf is the latest instant whose evidence the iteration may use;
// Start/End optionally name the event-time window the question was
// re-observed over. It is recorded once and never rewritten.
type ObservationWindow struct {
	AsOf  time.Time `json:"asOf"`
	Start string    `json:"start,omitempty"`
	End   string    `json:"end,omitempty"`
	Basis string    `json:"basis,omitempty"`
	Note  string    `json:"note,omitempty"`
}

// Validate checks the window itself. Ordering against earlier iterations is
// checked by ValidateNextWindow.
func (w ObservationWindow) Validate() error {
	if w.AsOf.IsZero() {
		return fmt.Errorf("observation window asOf is required")
	}
	if (w.Start == "") != (w.End == "") {
		return fmt.Errorf("observation window start and end must be given together")
	}
	if w.Start != "" && strings.TrimSpace(w.Basis) == "" {
		return fmt.Errorf("observation window basis is required with start/end")
	}
	if w.Start > w.End {
		return fmt.Errorf("observation window start must not be after end")
	}
	return nil
}

// ValidateNextWindow enforces "no retroactive rewrite": a new iteration may
// not look at the world from an earlier as-of point than the latest windowed
// iteration already recorded.
func ValidateNextWindow(run ResearchRun, next *ObservationWindow) error {
	if next == nil {
		return nil
	}
	if err := next.Validate(); err != nil {
		return err
	}
	for i := len(run.Iterations) - 1; i >= 0; i-- {
		prev := run.Iterations[i].ObservationWindow
		if prev == nil {
			continue
		}
		if next.AsOf.Before(prev.AsOf) {
			return fmt.Errorf("observation window asOf %s is earlier than iteration %d asOf %s; historical iterations are not rewritten",
				next.AsOf.Format(time.RFC3339), run.Iterations[i].Sequence, prev.AsOf.Format(time.RFC3339))
		}
		return nil
	}
	return nil
}

// EvidenceChangeKind distinguishes how evidence moved between iterations.
type EvidenceChangeKind string

const (
	EvidenceAdded             EvidenceChangeKind = "ADDED"
	EvidenceRemoved           EvidenceChangeKind = "REMOVED"
	EvidenceContentChanged    EvidenceChangeKind = "CONTENT_CHANGED"
	EvidenceDefinitionChanged EvidenceChangeKind = "DEFINITION_CHANGED"
)

// ChangeAttribution explains what a hypothesis or insight change can be tied
// to. It never asserts a cause in the world.
type ChangeAttribution string

const (
	AttributedToEvidence   ChangeAttribution = "EVIDENCE_DELTA"
	AttributedToInstrument ChangeAttribution = "INSTRUMENT_CHANGE"
	AttributedToBoth       ChangeAttribution = "EVIDENCE_AND_INSTRUMENT"
	Unattributed           ChangeAttribution = "UNATTRIBUTED"
)

// FingerprintState compares two recorded fingerprints. Empty means "not
// recorded", which is UNKNOWN, never SAME.
type FingerprintState string

const (
	FingerprintSame    FingerprintState = "SAME"
	FingerprintChanged FingerprintState = "CHANGED"
	FingerprintUnknown FingerprintState = "UNKNOWN"
)

func CompareFingerprints(previous, current string) FingerprintState {
	switch {
	case previous == "" || current == "":
		return FingerprintUnknown
	case previous == current:
		return FingerprintSame
	}
	return FingerprintChanged
}
