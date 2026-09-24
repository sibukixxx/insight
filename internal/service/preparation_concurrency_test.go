package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"insight-lab/internal/domain"
	"insight-lab/internal/execution"
	"insight-lab/internal/input"
)

// countingResolver records how many raw artifacts are open at once.
type countingResolver struct {
	inner   input.Resolver
	current atomic.Int32
	peak    atomic.Int32
	mu      sync.Mutex
}

type countedReader struct {
	io.ReadCloser
	r *countingResolver
}

func (c countedReader) Close() error { c.r.current.Add(-1); return c.ReadCloser.Close() }

func (r *countingResolver) Open(ctx context.Context, uri string) (io.ReadCloser, error) {
	n := r.current.Add(1)
	r.mu.Lock()
	if n > r.peak.Load() {
		r.peak.Store(n)
	}
	r.mu.Unlock()
	time.Sleep(20 * time.Millisecond) // widen the overlap window
	rc, err := r.inner.Open(ctx, uri)
	if err != nil {
		r.current.Add(-1)
		return nil, err
	}
	return countedReader{rc, r}, nil
}

func rawDocs(t *testing.T, dir string, n int) []*domain.Document {
	t.Helper()
	var docs []*domain.Document
	for i := 0; i < n; i++ {
		content := fmt.Sprintf("year,region,amount\n2024,east,%d\n2025,west,%d\n", i+1, i+2)
		name := fmt.Sprintf("raw-%d.csv", i)
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256([]byte(content))
		docs = append(docs, &domain.Document{
			ID: fmt.Sprintf("doc-raw-%d", i), ProjectID: "p", Source: domain.SourceDataset, Content: "raw artifact descriptor",
			CreatedAt: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
			Metadata: map[string]string{
				input.MetadataKind: string(input.KindRawArtifact), input.MetadataRawURI: "file:" + name, input.MetadataRawMedia: "text/csv",
				input.MetadataRawSHA256: hex.EncodeToString(sum[:]), input.MetadataRawSize: strconv.Itoa(len(content)),
				input.MetadataPreparation: rawSpec, "public_external_ref": fmt.Sprintf("raw-%d", i),
			},
		})
	}
	return docs
}

func TestPrepareStandardBoundsConcurrencyAndKeepsDeterministicOrder(t *testing.T) {
	dir := t.TempDir()
	docs := rawDocs(t, dir, 6)
	now := time.Now()

	sequential := &Preparation{Resolver: input.Resolvers{"file": input.FileResolver{Root: dir}}, Concurrency: 1}
	want, err := sequential.Prepare(context.Background(), "p", docs, execution.ProfileStandard, now)
	if err != nil {
		t.Fatal(err)
	}

	counter := &countingResolver{inner: input.Resolvers{"file": input.FileResolver{Root: dir}}}
	concurrent := &Preparation{Resolver: counter, Concurrency: 3}
	got, err := concurrent.Prepare(context.Background(), "p", docs, execution.ProfileStandard, now)
	if err != nil {
		t.Fatal(err)
	}
	if peak := counter.peak.Load(); peak < 2 || peak > 3 {
		t.Fatalf("peak concurrent raw reads = %d, want between 2 and the bound 3", peak)
	}
	if len(got) != len(want) {
		t.Fatalf("prepared %d docs, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Metadata[MetadataPreparedFrom] != want[i].Metadata[MetadataPreparedFrom] ||
			got[i].Metadata[MetadataAnalyticalArtifactHash] != want[i].Metadata[MetadataAnalyticalArtifactHash] {
			t.Fatalf("doc %d differs between sequential and concurrent preparation", i)
		}
	}
}

func TestPrepareStandardReturnsFirstErrorWhenOneRawArtifactChanged(t *testing.T) {
	dir := t.TempDir()
	docs := rawDocs(t, dir, 4)
	if err := os.WriteFile(filepath.Join(dir, "raw-2.csv"), []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}
	p := &Preparation{Resolver: input.Resolvers{"file": input.FileResolver{Root: dir}}, Concurrency: 2}
	if _, err := p.Prepare(context.Background(), "p", docs, execution.ProfileStandard, time.Now()); err == nil {
		t.Fatal("a changed raw artifact must fail the whole preparation")
	}
}
