package domain

import "time"

// IngestState is the lifecycle of a large-file ingestion (#132). It is
// separate from an Analysis Run: an ingest only makes Evidence available;
// it never analyses it.
type IngestState string

const (
	IngestQueued     IngestState = "QUEUED"
	IngestValidating IngestState = "VALIDATING"
	IngestReady      IngestState = "READY"
	IngestFailed     IngestState = "FAILED"
	IngestCancelled  IngestState = "CANCELLED"
)

// Terminal reports whether the ingest will not change state again.
func (s IngestState) Terminal() bool {
	return s == IngestReady || s == IngestFailed || s == IngestCancelled
}

// Ingest stages within the VALIDATING state.
const (
	IngestStageStaged     = "STAGED"
	IngestStageParsing    = "PARSING"
	IngestStageFinalizing = "FINALIZING"
	IngestStageDone       = "DONE"
)

// IngestRowError is one rejected row. Row is 1-based, header excluded.
type IngestRowError struct {
	Row    int64  `json:"row"`
	Reason string `json:"reason"`
}

// Ingest is the durable receipt of one staged upload. The identity of a
// request is (ProjectID, Kind, FileSHA256, ManifestHash, RequestKey): the
// same bytes submitted again with the same identity resolve to the same
// live or READY receipt instead of importing the Evidence twice.
type Ingest struct {
	ID           string
	ProjectID    string
	Kind         string
	RequestKey   string
	FileName     string
	FileSHA256   string
	SizeBytes    int64
	Manifest     string // acquisition manifest JSON as submitted; empty when none
	ManifestHash string

	State IngestState
	Stage string

	BytesRead        int64
	RowsRead         int64
	RowsSkipped      int64
	DocumentsCreated int64
	ErrorCount       int64
	// ErrorExamples holds at most a bounded number of rejected rows; the
	// complete list is the ingest's error export.
	ErrorExamples []IngestRowError
	Failure       string

	CreatedAt  time.Time
	UpdatedAt  time.Time
	FinishedAt *time.Time
}
