package domain

import "time"

// Pattern is the "repeats across multiple people" step a marketer does by
// hand when skimming interviews - noticing the same behavior, complaint,
// or workaround showing up more than once. It sits between Observation
// (a single grounded quote) and Insight (the final hypothesis, tested
// against evidence): visualizing this layer is what turns insight
// generation from a black box into an inspectable trail.
type Pattern struct {
	ID             string
	ProjectID      string
	AnalysisID     string
	Title          string
	Description    string
	ObservationIDs []string // populated on read via the pattern_observations join
	CreatedAt      time.Time
}
