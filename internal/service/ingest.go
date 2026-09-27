package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"insight-lab/internal/domain"
	"insight-lab/internal/input"
	"insight-lab/internal/repository"
)

// Large-CSV ingestion (#132). An upload is streamed to an immutable staged
// file under the ingest directory, hashed on the way, and queued as a
// durable ingest job. One background worker re-reads the staged file with
// the same importers as the small synchronous endpoints and writes the
// documents in bounded batches that stay invisible until the job is READY.
// Nothing about an ingest is held in memory beyond one batch, the capped
// error examples and, for the analysis kind, the per-group counts.

var (
	// ErrIngestDisabled means the server runs without an ingest directory.
	ErrIngestDisabled = errors.New("large ingestion is not enabled")
	// ErrIngestInvalid rejects the request before any job exists.
	ErrIngestInvalid = errors.New("invalid ingest request")
	// ErrIngestTooLarge means the upload exceeds the per-file limit.
	ErrIngestTooLarge = errors.New("the file exceeds the ingest size limit")
	// ErrIngestStorage means the staging area is full: the staged-bytes
	// quota is reached or the disk refused the write.
	ErrIngestStorage = errors.New("not enough ingest staging storage")
	// ErrIngestNotCancellable is returned for an ingest already finished.
	ErrIngestNotCancellable = errors.New("the ingest has already finished")
	// errIngestRecordTooLarge stops a scan whose line exceeds the limit.
	errIngestRecordTooLarge = errors.New("a CSV line exceeds the ingest record size limit")
)

// IngestLimits bounds every resource one ingest may use.
type IngestLimits struct {
	MaxUploadBytes int64 // per staged file
	MaxStagedBytes int64 // across QUEUED and VALIDATING ingests
	MaxRows        int64 // data rows read from one file
	MaxRecordBytes int   // one physical CSV line
	BatchRows      int   // documents per database transaction
	BatchBytes     int   // document content per database transaction
	MaxErrorSample int   // rejected rows kept on the receipt
	MaxGroups      int   // distinct groups of an analysis CSV
}

func DefaultIngestLimits() IngestLimits {
	return IngestLimits{
		MaxUploadBytes: 2 << 30,
		MaxStagedBytes: 8 << 30,
		MaxRows:        10_000_000,
		MaxRecordBytes: 8 << 20,
		BatchRows:      2000,
		BatchBytes:     8 << 20,
		MaxErrorSample: 50,
		MaxGroups:      input.DefaultMaxGroups,
	}
}

// IngestCapability is what a client may rely on; the Reference Web reads
// it instead of assuming the large-ingest path exists.
type IngestCapability struct {
	Enabled        bool     `json:"enabled"`
	Kinds          []string `json:"kinds,omitempty"`
	MaxUploadBytes int64    `json:"maxUploadBytes,omitempty"`
	MaxRows        int64    `json:"maxRows,omitempty"`
}

// IngestRequest is the metadata sent alongside the upload body.
type IngestRequest struct {
	ProjectID string
	Kind      string
	// FileName is the client's name for the file, kept for display only;
	// it never becomes part of a path.
	FileName string
	// RequestKey is an optional client idempotency key; the same bytes
	// under a different key are a separate ingest.
	RequestKey string
	// Manifest is the optional acquisition manifest JSON.
	Manifest []byte
}

const (
	ingestTmpDir     = "tmp"
	ingestUploadFile = "upload.csv"
	ingestErrorsFile = "errors.csv"
	ingestSniffBytes = 8 << 10
	ingestDiscardRow = 5000
	// IngestPreviewDocuments is how many documents the receipt samples.
	IngestPreviewDocuments = PreviewDocumentLimit
)

type IngestManager struct {
	repo   repository.IngestRepository
	dir    string
	limits IngestLimits

	wake chan struct{}
	wg   sync.WaitGroup

	mu      sync.Mutex
	cancels map[string]context.CancelFunc
}

// NewIngestManager stages uploads under dir, which it creates (0700).
func NewIngestManager(repo repository.IngestRepository, dir string, limits IngestLimits) (*IngestManager, error) {
	if dir == "" {
		return nil, ErrIngestDisabled
	}
	if err := os.MkdirAll(filepath.Join(dir, ingestTmpDir), 0o700); err != nil {
		return nil, fmt.Errorf("create ingest directory: %w", err)
	}
	return &IngestManager{
		repo: repo, dir: dir, limits: limits,
		wake: make(chan struct{}, 1), cancels: map[string]context.CancelFunc{},
	}, nil
}

func (m *IngestManager) Capability() IngestCapability {
	if m == nil {
		return IngestCapability{}
	}
	return IngestCapability{
		Enabled: true, Kinds: []string{ImportKindDocuments, ImportKindAnalysis},
		MaxUploadBytes: m.limits.MaxUploadBytes, MaxRows: m.limits.MaxRows,
	}
}

// Recover makes the stored state consistent after a previous process
// stopped: interrupted jobs are requeued from their staged file (or failed
// when it is gone), leftover rows of unsuccessful jobs are discarded, and
// staging files without a job are removed. Call it before Start.
func (m *IngestManager) Recover(ctx context.Context) error {
	if err := removeDirContents(filepath.Join(m.dir, ingestTmpDir)); err != nil {
		return err
	}
	interrupted, err := m.repo.ListIngestsByState(ctx, domain.IngestValidating)
	if err != nil {
		return err
	}
	for _, job := range interrupted {
		if err := m.discardRows(ctx, job.ID); err != nil {
			return err
		}
		if _, statErr := os.Stat(m.uploadPath(job.ID)); statErr != nil {
			job.State, job.Stage, job.Failure = domain.IngestFailed, domain.IngestStageDone, "the staged upload was lost when the server restarted; upload the file again"
			if err := m.repo.FinishIngest(ctx, job); err != nil {
				return err
			}
			continue
		}
		if err := os.Remove(m.errorsPath(job.ID)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := m.repo.RequeueIngest(ctx, job.ID); err != nil {
			return err
		}
	}
	finished, err := m.repo.ListIngestsByState(ctx, domain.IngestReady, domain.IngestFailed, domain.IngestCancelled)
	if err != nil {
		return err
	}
	for _, job := range finished {
		if err := m.discardRows(ctx, job.ID); err != nil {
			return err
		}
		if err := m.removeStagedUpload(job.ID); err != nil {
			return err
		}
	}
	return m.removeOrphanDirs(ctx)
}

// Start runs the ingest worker until ctx is cancelled.
func (m *IngestManager) Start(ctx context.Context) {
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		for {
			m.drain(ctx)
			select {
			case <-ctx.Done():
				return
			case <-m.wake:
			}
		}
	}()
	m.signal()
}

// Wait blocks until the worker started by Start has returned.
func (m *IngestManager) Wait() { m.wg.Wait() }

func (m *IngestManager) signal() {
	select {
	case m.wake <- struct{}{}:
	default:
	}
}

func (m *IngestManager) drain(ctx context.Context) {
	for ctx.Err() == nil {
		job, err := m.repo.ClaimNextIngest(ctx)
		if errors.Is(err, repository.ErrNotFound) {
			return
		}
		if err != nil {
			if ctx.Err() == nil {
				slog.Error("claim ingest", "error", err)
			}
			return
		}
		m.run(ctx, job)
	}
}

// Submit stages body and queues an ingest. created is false when a live or
// READY ingest with the same identity already exists; that receipt is
// returned and the new bytes are discarded.
func (m *IngestManager) Submit(ctx context.Context, req IngestRequest, body io.Reader) (job *domain.Ingest, created bool, err error) {
	if m == nil {
		return nil, false, ErrIngestDisabled
	}
	if _, ok := FindImportFormat(req.Kind); !ok {
		return nil, false, fmt.Errorf("%w: unsupported import kind %q", ErrIngestInvalid, req.Kind)
	}
	if len(req.RequestKey) > 200 {
		return nil, false, fmt.Errorf("%w: the idempotency key is longer than 200 characters", ErrIngestInvalid)
	}
	manifestHash := ""
	if len(bytes.TrimSpace(req.Manifest)) > 0 {
		if _, err := ParseAcquisitionManifest(bytes.NewReader(req.Manifest)); err != nil {
			return nil, false, fmt.Errorf("%w: %v", ErrIngestInvalid, err)
		}
		sum := sha256.Sum256(req.Manifest)
		manifestHash = hex.EncodeToString(sum[:])
	}
	staged, err := m.repo.StagedBytes(ctx)
	if err != nil {
		return nil, false, err
	}
	room := m.limits.MaxStagedBytes - staged
	if room <= 0 {
		return nil, false, ErrIngestStorage
	}

	tmp, size, sum, err := m.stage(body, min(room, m.limits.MaxUploadBytes), room < m.limits.MaxUploadBytes)
	if err != nil {
		return nil, false, err
	}
	defer os.Remove(tmp) // a no-op once the file has been moved into place

	now := time.Now().UTC()
	job = &domain.Ingest{
		ID: newID("ingest"), ProjectID: req.ProjectID, Kind: req.Kind, RequestKey: req.RequestKey,
		FileName: displayFileName(req.FileName), FileSHA256: sum, SizeBytes: size,
		Manifest: string(bytes.TrimSpace(req.Manifest)), ManifestHash: manifestHash,
		State: domain.IngestQueued, Stage: domain.IngestStageStaged, CreatedAt: now, UpdatedAt: now,
	}
	if existing, err := m.repo.FindLiveIngest(ctx, job); err == nil {
		return existing, false, nil
	} else if !errors.Is(err, repository.ErrNotFound) {
		return nil, false, err
	}
	jobDir := filepath.Join(m.dir, job.ID)
	if err := os.Mkdir(jobDir, 0o700); err != nil {
		return nil, false, err
	}
	if err := os.Rename(tmp, m.uploadPath(job.ID)); err != nil {
		os.Remove(jobDir)
		return nil, false, err
	}
	if err := m.repo.CreateIngest(ctx, job); err != nil {
		os.Remove(m.uploadPath(job.ID))
		os.Remove(jobDir)
		if errors.Is(err, repository.ErrConflict) {
			// A concurrent identical request won the race.
			existing, findErr := m.repo.FindLiveIngest(ctx, job)
			if findErr != nil {
				return nil, false, findErr
			}
			return existing, false, nil
		}
		return nil, false, err
	}
	m.signal()
	return job, true, nil
}

// stage copies body into a new private file under the tmp directory,
// hashing it on the way. More than limit bytes fail with ErrIngestStorage
// when quotaBound (the staging quota, not the file limit, set the bound)
// and ErrIngestTooLarge otherwise.
func (m *IngestManager) stage(body io.Reader, limit int64, quotaBound bool) (path string, size int64, sum string, err error) {
	f, err := os.CreateTemp(filepath.Join(m.dir, ingestTmpDir), "upload-*.part")
	if err != nil {
		return "", 0, "", err
	}
	path = f.Name()
	fail := func(err error) (string, int64, string, error) {
		f.Close()
		os.Remove(path)
		return "", 0, "", err
	}
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(body, limit+1))
	if err != nil {
		if errors.Is(err, syscall.ENOSPC) {
			return fail(ErrIngestStorage)
		}
		var maxBytes *http.MaxBytesError
		if errors.As(err, &maxBytes) {
			return fail(ErrIngestTooLarge)
		}
		return fail(fmt.Errorf("receive upload: %w", err))
	}
	if n > limit {
		if quotaBound {
			return fail(ErrIngestStorage)
		}
		return fail(ErrIngestTooLarge)
	}
	if n == 0 {
		return fail(fmt.Errorf("%w: the file is empty", ErrIngestInvalid))
	}
	if err := f.Sync(); err != nil {
		if errors.Is(err, syscall.ENOSPC) {
			return fail(ErrIngestStorage)
		}
		return fail(err)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return fail(err)
	}
	head := make([]byte, ingestSniffBytes)
	k, _ := io.ReadFull(f, head)
	if err := sniffText(head[:k]); err != nil {
		return fail(err)
	}
	if err := f.Close(); err != nil {
		os.Remove(path)
		return "", 0, "", err
	}
	return path, n, hex.EncodeToString(h.Sum(nil)), nil
}

// sniffText rejects content that is clearly not a text CSV. Encoding is
// checked row by row later, where a failure can name the row.
func sniffText(head []byte) error {
	if bytes.IndexByte(head, 0) >= 0 || !strings.HasPrefix(http.DetectContentType(head), "text/") {
		return fmt.Errorf("%w: the file is not a text CSV", ErrIngestInvalid)
	}
	return nil
}

func (m *IngestManager) Get(ctx context.Context, projectID, id string) (*domain.Ingest, error) {
	job, err := m.repo.GetIngest(ctx, id)
	if err != nil {
		return nil, err
	}
	if job.ProjectID != projectID {
		return nil, repository.ErrNotFound
	}
	return job, nil
}

func (m *IngestManager) List(ctx context.Context, projectID string) ([]*domain.Ingest, error) {
	return m.repo.ListIngests(ctx, projectID)
}

// Preview samples the first documents the ingest produced, whatever its
// state; it is never an exhaustive view.
func (m *IngestManager) Preview(ctx context.Context, job *domain.Ingest) ([]*domain.Document, error) {
	return m.repo.ListIngestDocuments(ctx, job.ID, IngestPreviewDocuments)
}

// Cancel stops a queued or running ingest. A queued ingest is cancelled at
// once; a running one when its worker next checks, so the returned receipt
// may still be VALIDATING.
func (m *IngestManager) Cancel(ctx context.Context, projectID, id string) (*domain.Ingest, error) {
	job, err := m.Get(ctx, projectID, id)
	if err != nil {
		return nil, err
	}
	switch job.State {
	case domain.IngestQueued:
		job.State, job.Stage, job.Failure = domain.IngestCancelled, domain.IngestStageDone, ""
		if err := m.repo.FinishIngest(ctx, job); err != nil {
			if errors.Is(err, repository.ErrConflict) {
				// The worker claimed it meanwhile; cancel the running job.
				return m.Cancel(ctx, projectID, id)
			}
			return nil, err
		}
		if err := m.removeStagedUpload(job.ID); err != nil {
			slog.Error("remove staged upload", "ingest", job.ID, "error", err)
		}
		return job, nil
	case domain.IngestValidating:
		m.mu.Lock()
		cancel := m.cancels[job.ID]
		m.mu.Unlock()
		if cancel != nil {
			cancel()
		}
		return job, nil
	default:
		return nil, ErrIngestNotCancellable
	}
}

// OpenErrors opens the complete rejected-row export of the ingest. It
// returns os.ErrNotExist when no row has been rejected.
func (m *IngestManager) OpenErrors(job *domain.Ingest) (*os.File, error) {
	return os.Open(m.errorsPath(job.ID))
}

func (m *IngestManager) uploadPath(id string) string {
	return filepath.Join(m.dir, id, ingestUploadFile)
}

func (m *IngestManager) errorsPath(id string) string {
	return filepath.Join(m.dir, id, ingestErrorsFile)
}

// run processes one claimed job to a terminal state.
func (m *IngestManager) run(parent context.Context, job *domain.Ingest) {
	ctx, cancel := context.WithCancel(parent)
	m.mu.Lock()
	m.cancels[job.ID] = cancel
	m.mu.Unlock()
	defer func() {
		m.mu.Lock()
		delete(m.cancels, job.ID)
		m.mu.Unlock()
		cancel()
	}()

	scanErr := m.scan(ctx, job)
	if parent.Err() != nil {
		// Shutting down: leave the job VALIDATING so Recover requeues it.
		return
	}
	// Cleanup and the final state must be written even after a cancel.
	final := context.WithoutCancel(ctx)
	switch {
	case scanErr == nil:
		job.State, job.Stage = domain.IngestReady, domain.IngestStageDone
	case ctx.Err() != nil:
		job.State, job.Stage, job.Failure = domain.IngestCancelled, domain.IngestStageDone, ""
	default:
		job.State, job.Stage, job.Failure = domain.IngestFailed, domain.IngestStageDone, scanErr.Error()
	}
	if err := m.repo.FinishIngest(final, job); err != nil && !errors.Is(err, repository.ErrConflict) {
		slog.Error("finish ingest", "ingest", job.ID, "error", err)
		return
	}
	if err := m.discardRows(final, job.ID); err != nil {
		slog.Error("discard ingest rows", "ingest", job.ID, "error", err)
	}
	if err := m.removeStagedUpload(job.ID); err != nil {
		slog.Error("remove staged upload", "ingest", job.ID, "error", err)
	}
}

// removeStagedUpload deletes a finished ingest's upload and, unless it
// keeps an error export, its now empty staging directory.
func (m *IngestManager) removeStagedUpload(id string) error {
	if err := os.Remove(m.uploadPath(id)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if _, err := os.Stat(m.errorsPath(id)); errors.Is(err, os.ErrNotExist) {
		if err := os.Remove(filepath.Join(m.dir, id)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

// scan reads the staged file and writes its documents. A nil return means
// every row was read, the file still has its recorded hash and at least
// one document was written.
func (m *IngestManager) scan(ctx context.Context, job *domain.Ingest) error {
	f, err := os.Open(m.uploadPath(job.ID))
	if err != nil {
		return fmt.Errorf("open staged upload: %w", err)
	}
	defer f.Close()

	var manifest *AcquisitionManifest
	if job.Manifest != "" {
		if manifest, err = ParseAcquisitionManifest(strings.NewReader(job.Manifest)); err != nil {
			return err
		}
	}
	sink := &ingestSink{ctx: ctx, m: m, job: job, manifest: manifest}
	defer sink.close()

	h := sha256.New()
	counter := &progressReader{r: io.TeeReader(&contextReader{ctx: ctx, r: f}, h)}
	reader := csv.NewReader(stripBOM(&lineLimitReader{r: counter, max: m.limits.MaxRecordBytes}))
	reader.FieldsPerRecord = -1
	reader.ReuseRecord = true
	counter.onProgress = func(n int64) {
		job.BytesRead = n
		if err := m.repo.SaveIngestProgress(ctx, job); err != nil && ctx.Err() == nil {
			slog.Error("save ingest progress", "ingest", job.ID, "error", err)
		}
	}

	switch job.Kind {
	case ImportKindDocuments:
		if err := readDocumentsHeader(reader); err != nil {
			return err
		}
		err = scanDocumentRows(reader, job.ProjectID, sink.addDocument, sink.rejectRow)
	case ImportKindAnalysis:
		var columns map[string]int
		if columns, err = readAnalysisHeader(reader); err != nil {
			return err
		}
		var groups map[string]*analysisGroup
		var read int
		groups, read, err = aggregateAnalysisRows(reader, columns, analysisScanLimits{maxGroups: m.limits.MaxGroups, maxRows: m.limits.MaxRows}, sink.record)
		if err == nil {
			sink.rows = int64(read)
			for _, doc := range analysisDocuments(groups, job.ProjectID, job.FileSHA256, manifest) {
				if err = sink.add(doc, ""); err != nil {
					break
				}
			}
		}
	default:
		err = fmt.Errorf("unsupported import kind %q", job.Kind)
	}
	if err != nil {
		return err
	}
	job.Stage = domain.IngestStageFinalizing
	if err := sink.flush(); err != nil {
		return err
	}
	job.BytesRead = counter.n
	if got := hex.EncodeToString(h.Sum(nil)); got != job.FileSHA256 {
		return fmt.Errorf("the staged upload changed after it was received (sha256 %s, recorded %s)", got, job.FileSHA256)
	}
	if job.DocumentsCreated == 0 {
		return fmt.Errorf("the file has no importable rows; see the rejected rows")
	}
	return nil
}

// ingestSink batches documents into the repository and records rejected
// rows: every one in the error export, the first few on the receipt.
type ingestSink struct {
	ctx      context.Context
	m        *IngestManager
	job      *domain.Ingest
	manifest *AcquisitionManifest

	rows       int64 // data rows read, valid or not
	batch      repository.IngestBatch
	batchRows  []int
	batchBytes int

	errFile *os.File
	errCSV  *csv.Writer
}

func (s *ingestSink) close() {
	if s.errFile != nil {
		s.errCSV.Flush()
		s.errFile.Close()
	}
}

// countRow counts a data row and enforces the row limit.
func (s *ingestSink) countRow() error {
	s.rows++
	if s.rows > s.m.limits.MaxRows {
		return fmt.Errorf("the file has more than %d data rows", s.m.limits.MaxRows)
	}
	return nil
}

func (s *ingestSink) addDocument(doc *domain.Document) error {
	if err := s.countRow(); err != nil {
		return err
	}
	doc.Metadata = provenanceMetadata(doc.Metadata, s.job.FileSHA256, s.manifest)
	return s.add(doc, doc.Metadata["csv_id"])
}

func (s *ingestSink) add(doc *domain.Document, rowKey string) error {
	s.batch.Documents = append(s.batch.Documents, doc)
	s.batch.RowKeys = append(s.batch.RowKeys, rowKey)
	s.batchRows = append(s.batchRows, int(s.rows))
	s.batchBytes += len(doc.Content) + len(doc.Title)
	if len(s.batch.Documents) >= s.m.limits.BatchRows || s.batchBytes >= s.m.limits.BatchBytes {
		return s.flush()
	}
	return nil
}

func (s *ingestSink) flush() error {
	if len(s.batch.Documents) > 0 {
		if err := s.ctx.Err(); err != nil {
			return err
		}
		duplicates, err := s.m.repo.AppendIngestBatch(s.ctx, s.job.ID, s.batch)
		if err != nil {
			return fmt.Errorf("save documents: %w", err)
		}
		s.job.DocumentsCreated += int64(len(s.batch.Documents) - len(duplicates))
		for _, i := range duplicates {
			reason := fmt.Sprintf("duplicate id %q in this file", s.batch.RowKeys[i])
			if err := s.record(ImportRowError{Row: s.batchRows[i], Reason: reason}); err != nil {
				return err
			}
		}
		s.batch = repository.IngestBatch{}
		s.batchRows, s.batchBytes = s.batchRows[:0], 0
	}
	s.job.RowsRead = s.rows
	return s.m.repo.SaveIngestProgress(s.ctx, s.job)
}

// rejectRow counts and records a rejected documents row.
func (s *ingestSink) rejectRow(e ImportRowError) error {
	if err := s.countRow(); err != nil {
		return err
	}
	return s.record(e)
}

func (s *ingestSink) record(e ImportRowError) error {
	s.job.RowsSkipped++
	s.job.ErrorCount++
	if len(s.job.ErrorExamples) < s.m.limits.MaxErrorSample {
		s.job.ErrorExamples = append(s.job.ErrorExamples, domain.IngestRowError{Row: int64(e.Row), Reason: e.Reason})
	}
	if s.errFile == nil {
		f, err := os.OpenFile(s.m.errorsPath(s.job.ID), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
		if err != nil {
			return fmt.Errorf("create error export: %w", err)
		}
		s.errFile, s.errCSV = f, csv.NewWriter(f)
		if err := s.errCSV.Write([]string{"row", "reason"}); err != nil {
			return err
		}
	}
	return s.errCSV.Write([]string{fmt.Sprint(e.Row), e.Reason})
}

func (m *IngestManager) discardRows(ctx context.Context, id string) error {
	for {
		n, err := m.repo.DiscardIngestRows(ctx, id, ingestDiscardRow)
		if err != nil {
			return err
		}
		if n == 0 {
			return nil
		}
	}
}

func (m *IngestManager) removeOrphanDirs(ctx context.Context) error {
	ids, err := m.repo.ListIngestIDs(ctx)
	if err != nil {
		return err
	}
	known := make(map[string]bool, len(ids))
	for _, id := range ids {
		known[id] = true
	}
	entries, err := os.ReadDir(m.dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.Name() == ingestTmpDir || known[e.Name()] || !strings.HasPrefix(e.Name(), "ingest_") {
			continue
		}
		if err := os.RemoveAll(filepath.Join(m.dir, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

func removeDirContents(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := os.RemoveAll(filepath.Join(dir, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

// displayFileName keeps only a bounded base name of the client's file
// name. It is shown to the user and never used as a path.
func displayFileName(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	name = filepath.Base(strings.TrimSpace(name))
	if name == "." || name == "/" || name == ".." {
		return ""
	}
	name = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, name)
	if len(name) > 255 {
		name = name[:255]
	}
	return strings.ToValidUTF8(name, "")
}

// contextReader stops reading once ctx is done, so a cancel interrupts a
// scan between reads.
type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (c *contextReader) Read(p []byte) (int, error) {
	if err := c.ctx.Err(); err != nil {
		return 0, err
	}
	return c.r.Read(p)
}

// progressReader counts bytes and reports every ingestProgressBytes.
type progressReader struct {
	r          io.Reader
	n, last    int64
	onProgress func(int64)
}

const ingestProgressBytes = 4 << 20

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	p.n += int64(n)
	if p.onProgress != nil && p.n-p.last >= ingestProgressBytes {
		p.last = p.n
		p.onProgress(p.n)
	}
	return n, err
}

// lineLimitReader fails once a physical line grows beyond max bytes, which
// bounds what csv.Reader buffers for a single record.
type lineLimitReader struct {
	r    io.Reader
	max  int
	line int
}

func (l *lineLimitReader) Read(p []byte) (int, error) {
	n, err := l.r.Read(p)
	for _, b := range p[:n] {
		if b == '\n' {
			l.line = 0
			continue
		}
		l.line++
		if l.line > l.max {
			return 0, errIngestRecordTooLarge
		}
	}
	return n, err
}
