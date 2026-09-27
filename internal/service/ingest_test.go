package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"insight-lab/internal/domain"
	"insight-lab/internal/repository"
	"insight-lab/internal/repository/sqlite"
)

type ingestFixture struct {
	db        *sqlite.DB
	repo      *sqlite.IngestRepository
	documents *sqlite.DocumentRepository
	dir       string
}

func newIngestFixture(t *testing.T) *ingestFixture {
	t.Helper()
	base := t.TempDir()
	db, err := sqlite.Open(filepath.Join(base, "ingest.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := sqlite.NewProjectRepository(db).Create(context.Background(), &domain.Project{ID: "p1", Name: "p", CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	return &ingestFixture{db: db, repo: sqlite.NewIngestRepository(db), documents: sqlite.NewDocumentRepository(db), dir: filepath.Join(base, "ingest")}
}

func (f *ingestFixture) manager(t *testing.T, repo repository.IngestRepository, limits IngestLimits) *IngestManager {
	t.Helper()
	if repo == nil {
		repo = f.repo
	}
	m, err := NewIngestManager(repo, f.dir, limits)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	return m
}

func startIngest(t *testing.T, m *IngestManager) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	m.Start(ctx)
	t.Cleanup(func() { cancel(); m.Wait() })
}

func waitIngest(t *testing.T, m *IngestManager, id string, want domain.IngestState) *domain.Ingest {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for {
		job, err := m.Get(context.Background(), "p1", id)
		if err != nil {
			t.Fatal(err)
		}
		if job.State.Terminal() || time.Now().After(deadline) {
			if job.State != want {
				t.Fatalf("ingest state = %s (%s), want %s", job.State, job.Failure, want)
			}
			return job
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func (f *ingestFixture) visibleDocuments(t *testing.T) []*domain.Document {
	t.Helper()
	docs, err := f.documents.ListByProject(context.Background(), "p1")
	if err != nil {
		t.Fatal(err)
	}
	return docs
}

func submitCSV(t *testing.T, m *IngestManager, kind, body string) (*domain.Ingest, bool) {
	t.Helper()
	job, created, err := m.Submit(context.Background(), IngestRequest{ProjectID: "p1", Kind: kind, FileName: "data.csv"}, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	return job, created
}

var ingestAnalysisCSV = analysisCSVHeader +
	"1000000000001,A,株式会社,13,サンプル都,229,サンプル市,2026-01-02,,,,,ASSIGNED,sample_registry,v4,2026-03-01T00:00:00Z\n" +
	"1000000000002,B,株式会社,13,サンプル都,229,サンプル市,2026-01-10,,,,,ASSIGNED,sample_registry,v4,2026-03-01T00:00:00Z\n" +
	"1000000000003,C,株式会社,13,サンプル都,229,サンプル市,2026-01-01,2026-02-03,,,,UPDATED,sample_registry,v4,2026-03-01T00:00:00Z\n" +
	",D,株式会社,13,サンプル都,229,サンプル市,2026-01-01,,,,,ASSIGNED,sample_registry,v4,2026-03-01T00:00:00Z\n"

const ingestDocsCSV = "id,source,title,content\nr1,interview,T1,first body\nr2,review,T2,second body\n"

func TestIngestDocumentsAreInvisibleUntilReadyAndCarryTheFileHash(t *testing.T) {
	f := newIngestFixture(t)
	m := f.manager(t, nil, DefaultIngestLimits())
	job, created := submitCSV(t, m, ImportKindDocuments, ingestDocsCSV)
	if !created || job.State != domain.IngestQueued || job.FileSHA256 != sha256Hex(ingestDocsCSV) || job.SizeBytes != int64(len(ingestDocsCSV)) {
		t.Fatalf("receipt = %+v created=%v", job, created)
	}
	if docs := f.visibleDocuments(t); len(docs) != 0 {
		t.Fatalf("documents visible before the ingest ran: %d", len(docs))
	}
	startIngest(t, m)
	done := waitIngest(t, m, job.ID, domain.IngestReady)
	if done.RowsRead != 2 || done.DocumentsCreated != 2 || done.RowsSkipped != 0 || done.BytesRead != job.SizeBytes {
		t.Fatalf("finished receipt = %+v", done)
	}
	docs := f.visibleDocuments(t)
	if len(docs) != 2 {
		t.Fatalf("visible documents = %d, want 2", len(docs))
	}
	for _, d := range docs {
		if d.Metadata[MetadataDatasetHash] != job.FileSHA256 || d.Metadata["csv_id"] == "" {
			t.Fatalf("document metadata = %v", d.Metadata)
		}
	}
	// Staging cleanup follows the READY transition.
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(5 * time.Millisecond) {
		_, err := os.Stat(filepath.Join(f.dir, job.ID))
		if errors.Is(err, os.ErrNotExist) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("staging directory kept after a READY ingest without rejected rows: %v", err)
		}
	}
}

func TestIngestSameBytesAndIdentityReturnTheSameReceiptWithoutDuplicateEvidence(t *testing.T) {
	f := newIngestFixture(t)
	m := f.manager(t, nil, DefaultIngestLimits())
	startIngest(t, m)
	first, _ := submitCSV(t, m, ImportKindDocuments, ingestDocsCSV)
	waitIngest(t, m, first.ID, domain.IngestReady)

	again, created := submitCSV(t, m, ImportKindDocuments, ingestDocsCSV)
	if created || again.ID != first.ID || again.State != domain.IngestReady {
		t.Fatalf("replay created=%v id=%s state=%s, want the READY receipt %s", created, again.ID, again.State, first.ID)
	}
	if docs := f.visibleDocuments(t); len(docs) != 2 {
		t.Fatalf("replay duplicated evidence: %d documents", len(docs))
	}

	keyed, created, err := m.Submit(context.Background(), IngestRequest{ProjectID: "p1", Kind: ImportKindDocuments, RequestKey: "second-import"}, strings.NewReader(ingestDocsCSV))
	if err != nil || !created || keyed.ID == first.ID {
		t.Fatalf("a different request key must be a new ingest: created=%v err=%v", created, err)
	}

	manifest := []byte(`{"sourceName":"e-Stat","datasetId":"1","retrievalMethod":"download","retrievedAt":"2026-09-01T09:00:00Z","schemaId":"x"}`)
	described, created, err := m.Submit(context.Background(), IngestRequest{ProjectID: "p1", Kind: ImportKindDocuments, Manifest: manifest}, strings.NewReader(ingestDocsCSV))
	if err != nil || !created || described.ID == first.ID || described.ManifestHash == "" {
		t.Fatalf("the same bytes with a manifest must be a new ingest: created=%v err=%v", created, err)
	}
}

func TestIngestRecordsEveryRejectedRowWhileTheReceiptKeepsACappedSample(t *testing.T) {
	f := newIngestFixture(t)
	limits := DefaultIngestLimits()
	limits.MaxErrorSample = 2
	m := f.manager(t, nil, limits)
	startIngest(t, m)
	body := "id,source,title,content\n" +
		"a,interview,T,ok\n" +
		"b,nonsense,T,body\n" +
		"c,review,T,\n" +
		"d,review,T,\xff\xfe latin-1\n" +
		"a,review,T,duplicate of row 1\n" +
		"e,review,T,\"unterminated\n"
	job, _ := submitCSV(t, m, ImportKindDocuments, body)
	done := waitIngest(t, m, job.ID, domain.IngestReady)
	if done.DocumentsCreated != 1 || done.RowsSkipped != 5 || done.ErrorCount != 5 || len(done.ErrorExamples) != 2 {
		t.Fatalf("receipt = %+v", done)
	}
	export, err := m.OpenErrors(done)
	if err != nil {
		t.Fatal(err)
	}
	defer export.Close()
	raw, _ := io.ReadAll(export)
	for _, want := range []string{"row,reason", "invalid source", "content is empty", invalidUTF8Reason, `duplicate id ""a"" in this file`, "extraneous or missing"} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("error export lacks %q:\n%s", want, raw)
		}
	}
	if docs := f.visibleDocuments(t); len(docs) != 1 {
		t.Fatalf("visible documents = %d, want 1", len(docs))
	}
}

func TestIngestFailsWithoutEvidenceWhenHeaderIsWrongOrNoRowIsImportable(t *testing.T) {
	for name, body := range map[string]string{
		"wrong header":    "a,b,c\n1,2,3\n",
		"no valid rows":   "id,source,title,content\nx,nonsense,T,body\n",
		"row over limit":  "id,source,title,content\nx,review,T," + strings.Repeat("y", 200) + "\n",
		"rows over limit": "id,source,title,content\n1,review,T,a\n2,review,T,b\n3,review,T,c\n",
	} {
		t.Run(name, func(t *testing.T) {
			f := newIngestFixture(t)
			limits := DefaultIngestLimits()
			limits.MaxRecordBytes, limits.MaxRows = 100, 2
			m := f.manager(t, nil, limits)
			startIngest(t, m)
			job, _ := submitCSV(t, m, ImportKindDocuments, body)
			done := waitIngest(t, m, job.ID, domain.IngestFailed)
			if done.Failure == "" {
				t.Fatal("failed ingest has no reason")
			}
			if docs := f.visibleDocuments(t); len(docs) != 0 {
				t.Fatalf("failed ingest left %d visible documents", len(docs))
			}
			if leftover, _ := f.repo.ListIngestDocuments(context.Background(), job.ID, 10); len(leftover) != 0 {
				t.Fatalf("failed ingest left %d staged documents", len(leftover))
			}
			retry, created := submitCSV(t, m, ImportKindDocuments, body)
			if !created || retry.ID == job.ID {
				t.Fatal("a failed ingest must not block uploading the same file again")
			}
		})
	}
}

func TestIngestAnalysisCSVAggregatesIntoDatasetDocuments(t *testing.T) {
	f := newIngestFixture(t)
	m := f.manager(t, nil, DefaultIngestLimits())
	startIngest(t, m)
	job, _ := submitCSV(t, m, ImportKindAnalysis, ingestAnalysisCSV)
	done := waitIngest(t, m, job.ID, domain.IngestReady)
	sync, err := ImportAnalysisCSV(context.Background(), &discardDocuments{}, "p1", strings.NewReader(ingestAnalysisCSV))
	if err != nil {
		t.Fatal(err)
	}
	if done.RowsRead != int64(sync.RecordsRead) || done.DocumentsCreated != int64(sync.Imported) || done.RowsSkipped != int64(sync.Skipped) {
		t.Fatalf("async receipt %+v disagrees with the synchronous importer %+v", done, sync)
	}
	for _, d := range f.visibleDocuments(t) {
		if d.Source != domain.SourceDataset || d.Metadata[MetadataDatasetHash] != job.FileSHA256 {
			t.Fatalf("analysis document = %+v", d)
		}
	}
}

func TestIngestCancelStopsQueuedAndRunningIngestsAndDiscardsTheirRows(t *testing.T) {
	f := newIngestFixture(t)
	gate := &gatedIngestRepo{IngestRepository: f.repo, entered: make(chan struct{}), release: make(chan struct{})}
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(gate.release) }) }
	limits := DefaultIngestLimits()
	limits.BatchRows = 1
	m := f.manager(t, gate, limits)

	queued, _ := submitCSV(t, m, ImportKindDocuments, "id,source,title,content\nq,review,T,queued\n")
	if got, err := m.Cancel(context.Background(), "p1", queued.ID); err != nil || got.State != domain.IngestCancelled {
		t.Fatalf("cancel queued: %+v %v", got, err)
	}

	running, _ := submitCSV(t, m, ImportKindDocuments, ingestDocsCSV)
	startIngest(t, m)
	t.Cleanup(release) // registered after startIngest, so it runs before the worker is awaited
	<-gate.entered // the first batch is written; the second is blocked
	if staged, _ := f.repo.ListIngestDocuments(context.Background(), running.ID, 10); len(staged) != 1 {
		t.Fatalf("staged documents mid-ingest = %d, want 1", len(staged))
	}
	if docs := f.visibleDocuments(t); len(docs) != 0 {
		t.Fatalf("a running ingest exposed %d documents", len(docs))
	}
	if _, err := f.documents.Get(context.Background(), "doc_missing"); !errors.Is(err, repository.ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := m.Cancel(context.Background(), "p1", running.ID); err != nil {
		t.Fatal(err)
	}
	release()
	done := waitIngest(t, m, running.ID, domain.IngestCancelled)
	if leftover, _ := f.repo.ListIngestDocuments(context.Background(), done.ID, 10); len(leftover) != 0 {
		t.Fatalf("cancelled ingest kept %d staged documents", len(leftover))
	}
	if docs := f.visibleDocuments(t); len(docs) != 0 {
		t.Fatalf("cancelled ingest left %d visible documents", len(docs))
	}
	if _, err := m.Cancel(context.Background(), "p1", done.ID); !errors.Is(err, ErrIngestNotCancellable) {
		t.Fatalf("cancelling a finished ingest: %v", err)
	}
}

// gatedIngestRepo blocks the second batch write until release is closed.
type gatedIngestRepo struct {
	repository.IngestRepository
	calls   atomic.Int32
	entered chan struct{}
	release chan struct{}
}

func (g *gatedIngestRepo) AppendIngestBatch(ctx context.Context, id string, b repository.IngestBatch) ([]int, error) {
	if g.calls.Add(1) == 2 {
		close(g.entered)
		<-g.release
	}
	return g.IngestRepository.AppendIngestBatch(ctx, id, b)
}

func TestIngestRestartRequeuesAnInterruptedIngestWithoutDuplicatingRows(t *testing.T) {
	f := newIngestFixture(t)
	m := f.manager(t, nil, DefaultIngestLimits())
	job, _ := submitCSV(t, m, ImportKindDocuments, ingestDocsCSV)

	// Simulate a crash mid-ingest: claimed, one partial batch written.
	ctx := context.Background()
	if _, err := f.repo.ClaimNextIngest(ctx); err != nil {
		t.Fatal(err)
	}
	partial := &domain.Document{ID: "doc_partial", ProjectID: "p1", Source: domain.SourceReview, Content: "partial", CreatedAt: time.Now().UTC()}
	if _, err := f.repo.AppendIngestBatch(ctx, job.ID, repository.IngestBatch{Documents: []*domain.Document{partial}, RowKeys: []string{"r1"}}); err != nil {
		t.Fatal(err)
	}
	orphan := filepath.Join(f.dir, "ingest_orphan")
	if err := os.Mkdir(orphan, 0o700); err != nil {
		t.Fatal(err)
	}

	restarted := f.manager(t, nil, DefaultIngestLimits())
	if got, _ := restarted.Get(ctx, "p1", job.ID); got.State != domain.IngestQueued || got.DocumentsCreated != 0 {
		t.Fatalf("after recovery = %+v, want requeued with zero progress", got)
	}
	if _, err := os.Stat(orphan); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("recovery kept a staging directory without an ingest")
	}
	startIngest(t, restarted)
	done := waitIngest(t, restarted, job.ID, domain.IngestReady)
	if done.DocumentsCreated != 2 || done.RowsSkipped != 0 {
		t.Fatalf("requeued ingest = %+v", done)
	}
	docs := f.visibleDocuments(t)
	if len(docs) != 2 {
		t.Fatalf("visible documents = %d, want 2", len(docs))
	}
	for _, d := range docs {
		if d.ID == partial.ID {
			t.Fatal("the pre-crash partial row survived recovery")
		}
	}
}

func TestIngestRestartFailsAnInterruptedIngestWhoseStagedFileIsGone(t *testing.T) {
	f := newIngestFixture(t)
	m := f.manager(t, nil, DefaultIngestLimits())
	job, _ := submitCSV(t, m, ImportKindDocuments, ingestDocsCSV)
	if _, err := f.repo.ClaimNextIngest(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(m.uploadPath(job.ID)); err != nil {
		t.Fatal(err)
	}
	restarted := f.manager(t, nil, DefaultIngestLimits())
	if got, _ := restarted.Get(context.Background(), "p1", job.ID); got.State != domain.IngestFailed || got.Failure == "" {
		t.Fatalf("after recovery = %+v, want FAILED with a reason", got)
	}
}

func TestIngestSubmitRejectsOversizedBinaryAndOverQuotaUploadsWithoutLeavingFiles(t *testing.T) {
	f := newIngestFixture(t)
	limits := DefaultIngestLimits()
	limits.MaxUploadBytes, limits.MaxStagedBytes = 64, 100
	m := f.manager(t, nil, limits)
	submit := func(body string) error {
		_, _, err := m.Submit(context.Background(), IngestRequest{ProjectID: "p1", Kind: ImportKindDocuments}, strings.NewReader(body))
		return err
	}
	if err := submit(strings.Repeat("x", 65)); !errors.Is(err, ErrIngestTooLarge) {
		t.Fatalf("oversized upload: %v", err)
	}
	if err := submit("\x00\x01\x02binary"); !errors.Is(err, ErrIngestInvalid) {
		t.Fatalf("binary upload: %v", err)
	}
	if err := submit(""); !errors.Is(err, ErrIngestInvalid) {
		t.Fatalf("empty upload: %v", err)
	}
	if _, _, err := m.Submit(context.Background(), IngestRequest{ProjectID: "p1", Kind: "pdf"}, strings.NewReader("x")); !errors.Is(err, ErrIngestInvalid) {
		t.Fatalf("unsupported kind: %v", err)
	}
	// 60 bytes are staged (worker not running); 41 more exceed the quota.
	if err := submit("id,source,title,content\nr,review,T," + strings.Repeat("c", 26) + "\n"); err != nil {
		t.Fatal(err)
	}
	if err := submit(strings.Repeat("y", 41)); !errors.Is(err, ErrIngestStorage) {
		t.Fatalf("over-quota upload: %v", err)
	}
	if entries, _ := os.ReadDir(filepath.Join(f.dir, ingestTmpDir)); len(entries) != 0 {
		t.Fatalf("rejected uploads left %d temp files", len(entries))
	}
}

func TestIngestFileNameIsDisplayOnlyAndNeverAPath(t *testing.T) {
	f := newIngestFixture(t)
	m := f.manager(t, nil, DefaultIngestLimits())
	job, _, err := m.Submit(context.Background(), IngestRequest{ProjectID: "p1", Kind: ImportKindDocuments, FileName: "../../etc/pass\x00wd.csv"}, strings.NewReader(ingestDocsCSV))
	if err != nil {
		t.Fatal(err)
	}
	if job.FileName != "passwd.csv" {
		t.Fatalf("file name = %q", job.FileName)
	}
	if _, err := os.Stat(filepath.Join(f.dir, job.ID, ingestUploadFile)); err != nil {
		t.Fatalf("upload not staged under the ingest id: %v", err)
	}
	if _, err := m.Get(context.Background(), "other-project", job.ID); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("another project could read the ingest: %v", err)
	}
}

// documentRowStream generates a documents CSV lazily; it is never whole in
// memory.
type documentRowStream struct {
	rows, next int
	content    string
	buf        []byte
}

func (s *documentRowStream) Read(p []byte) (int, error) {
	for len(s.buf) < len(p) && s.next <= s.rows {
		if s.next == 0 {
			s.buf = append(s.buf, "id,source,title,content\n"...)
		} else {
			s.buf = append(s.buf, fmt.Sprintf("r%d,dataset,Row %d,%s\n", s.next, s.next, s.content)...)
		}
		s.next++
	}
	if len(s.buf) == 0 {
		return 0, io.EOF
	}
	n := copy(p, s.buf)
	s.buf = s.buf[n:]
	return n, nil
}

func TestIngestLargeCSVStreamsWithBoundedMemory(t *testing.T) {
	if testing.Short() {
		t.Skip("large ingest")
	}
	f := newIngestFixture(t)
	m := f.manager(t, nil, DefaultIngestLimits())
	const rows = 48_000 // ~48 MiB, above the 32 MiB synchronous preview limit
	stream := &documentRowStream{rows: rows, content: strings.Repeat("z", 1000)}

	runtime.GC()
	var base runtime.MemStats
	runtime.ReadMemStats(&base)
	var peak atomic.Uint64
	stop := make(chan struct{})
	var sampler sync.WaitGroup
	sampler.Add(1)
	go func() {
		defer sampler.Done()
		var ms runtime.MemStats
		for {
			select {
			case <-stop:
				return
			case <-time.After(20 * time.Millisecond):
				runtime.ReadMemStats(&ms)
				if ms.HeapInuse > peak.Load() {
					peak.Store(ms.HeapInuse)
				}
			}
		}
	}()

	job, _, err := m.Submit(context.Background(), IngestRequest{ProjectID: "p1", Kind: ImportKindDocuments}, stream)
	if err != nil {
		t.Fatal(err)
	}
	if job.SizeBytes <= MaxImportPreviewBytes {
		t.Fatalf("fixture is only %d bytes", job.SizeBytes)
	}
	startIngest(t, m)
	done := waitIngest(t, m, job.ID, domain.IngestReady)
	close(stop)
	sampler.Wait()

	if done.DocumentsCreated != rows || done.BytesRead != job.SizeBytes {
		t.Fatalf("receipt = %+v", done)
	}
	if grew := int64(peak.Load()) - int64(base.HeapInuse); grew > 32<<20 {
		t.Fatalf("heap in use grew by %d MiB while ingesting %d MiB; ingestion must not hold the file", grew>>20, job.SizeBytes>>20)
	}
}
