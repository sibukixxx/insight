package repository

import (
	"context"

	"insight-lab/internal/domain"
)

// IngestBatch is one bounded batch of documents parsed from an ingest's
// staged file, written in a single transaction. RowKeys[i] is the in-file identity of Documents[i] (its csv_id);
// an empty key is never checked for duplicates.
type IngestBatch struct {
	Documents []*domain.Document
	RowKeys   []string
}

type IngestRepository interface {
	// CreateIngest returns ErrConflict when a QUEUED, VALIDATING or READY
	// ingest with the same identity exists.
	CreateIngest(ctx context.Context, job *domain.Ingest) error
	// FindLiveIngest returns the QUEUED, VALIDATING or READY ingest with the
	// identity of job, or ErrNotFound.
	FindLiveIngest(ctx context.Context, job *domain.Ingest) (*domain.Ingest, error)
	GetIngest(ctx context.Context, id string) (*domain.Ingest, error)
	ListIngests(ctx context.Context, projectID string) ([]*domain.Ingest, error)
	ListIngestsByState(ctx context.Context, states ...domain.IngestState) ([]*domain.Ingest, error)
	ListIngestIDs(ctx context.Context) ([]string, error)
	// StagedBytes sums SizeBytes over QUEUED and VALIDATING ingests.
	StagedBytes(ctx context.Context) (int64, error)
	// ClaimNextIngest moves the oldest QUEUED ingest to VALIDATING and
	// returns it, or ErrNotFound when none is queued.
	ClaimNextIngest(ctx context.Context) (*domain.Ingest, error)
	// AppendIngestBatch inserts the batch's documents as invisible rows of
	// the ingest in one transaction. It returns the
	// indexes of documents whose row key already occurred in this ingest;
	// those documents are not inserted.
	AppendIngestBatch(ctx context.Context, ingestID string, batch IngestBatch) ([]int, error)
	// SaveIngestProgress stores the progress counters of a VALIDATING job.
	SaveIngestProgress(ctx context.Context, job *domain.Ingest) error
	// FinishIngest moves job from QUEUED or VALIDATING to its State (a
	// terminal state) with its final counters. It returns ErrConflict when
	// the stored job is no longer QUEUED or VALIDATING.
	FinishIngest(ctx context.Context, job *domain.Ingest) error
	// RequeueIngest returns a VALIDATING job to QUEUED with zeroed progress,
	// for restart recovery after its documents have been discarded.
	RequeueIngest(ctx context.Context, id string) error
	// DiscardIngestRows deletes up to limit leftover rows of an ingest:
	// its invisible documents unless it is READY, then its duplicate-check
	// row keys. Callers repeat it until it returns 0, so no single
	// transaction holds the database for the whole ingest.
	DiscardIngestRows(ctx context.Context, id string, limit int) (int64, error)
	// ListIngestDocuments returns up to limit documents of the ingest in
	// insertion order, whatever its state.
	ListIngestDocuments(ctx context.Context, id string, limit int) ([]*domain.Document, error)
}
