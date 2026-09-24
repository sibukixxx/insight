package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"insight-lab/internal/domain"
	"insight-lab/internal/execution"
	"insight-lab/internal/input"
)

const rawCSV = "year,region,amount\n2024,east,10\n2024,west,5\n2025,east,12\n2025,west,\n2025,west,7\n"

const rawSpec = `{"kind":"csv-aggregate/v1","metrics":[{"id":"amount_sum","name":"Amount","column":"amount","aggregation":"sum","unit":"units"}],"dimensionColumns":["region"],"periodColumn":"year","population":{"description":"sample rows"}}`

func rawRefDoc(t *testing.T, dir, content string) *domain.Document {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "raw.csv"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(content))
	return &domain.Document{
		ID: "doc-raw", ProjectID: "p", Source: domain.SourceDataset, Content: "raw artifact descriptor",
		CreatedAt: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		Metadata: map[string]string{
			input.MetadataKind: string(input.KindRawArtifact), input.MetadataRawURI: "file:raw.csv", input.MetadataRawMedia: "text/csv",
			input.MetadataRawSHA256: hex.EncodeToString(sum[:]), input.MetadataRawSize: strconv.Itoa(len(content)),
			input.MetadataPreparation: rawSpec, "public_external_ref": "raw-1",
		},
	}
}

func TestPrepareProducesIdenticalArtifactOnStandardAndHeavy(t *testing.T) {
	dir := t.TempDir()
	raw := rawRefDoc(t, dir, rawCSV)
	resolver := input.Resolvers{"file": input.FileResolver{Root: dir}}
	std := &Preparation{Resolver: resolver}
	heavy := &Preparation{Resolver: resolver, Heavy: execution.NewLocalRuntime(t.TempDir(), 2), Partitions: 3}
	now := time.Now()

	a, err := std.Prepare(context.Background(), "p", []*domain.Document{raw}, execution.ProfileStandard, now)
	if err != nil {
		t.Fatal(err)
	}
	b, err := heavy.Prepare(context.Background(), "p", []*domain.Document{raw}, execution.ProfileHeavy, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(a) != 1 || len(b) != 1 {
		t.Fatalf("prepared %d / %d documents", len(a), len(b))
	}
	if a[0].Content != b[0].Content || a[0].Metadata[MetadataAnalyticalArtifactHash] != b[0].Metadata[MetadataAnalyticalArtifactHash] {
		t.Fatalf("profiles produced different prepared artifacts:\n%s\n---\n%s", a[0].Content, b[0].Content)
	}
	if a[0].Metadata[MetadataPreparedFrom] != raw.ID {
		t.Fatal("prepared artifact is not linked to its raw reference")
	}
	art, ok := ArtifactFromDocument(a[0])
	if !ok || art.Datasets[0].Hash.Value != raw.Metadata[input.MetadataRawSHA256] {
		t.Fatalf("prepared artifact does not carry the verified raw hash: %+v", art)
	}
}

func TestPrepareSkipsAlreadyPreparedArtifacts(t *testing.T) {
	dir := t.TempDir()
	raw := rawRefDoc(t, dir, rawCSV)
	p := &Preparation{Resolver: input.Resolvers{"file": input.FileResolver{Root: dir}}}
	first, err := p.Prepare(context.Background(), "p", []*domain.Document{raw}, execution.ProfileStandard, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	again, err := p.Prepare(context.Background(), "p", []*domain.Document{raw, first[0]}, execution.ProfileStandard, time.Now())
	if err != nil || len(again) != 0 {
		t.Fatalf("re-preparation created %d documents (err %v)", len(again), err)
	}
}

func TestPrepareFailsWhenRawBytesChangedAfterRegistration(t *testing.T) {
	dir := t.TempDir()
	raw := rawRefDoc(t, dir, rawCSV)
	if err := os.WriteFile(filepath.Join(dir, "raw.csv"), []byte(rawCSV+"2025,east,1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	p := &Preparation{Resolver: input.Resolvers{"file": input.FileResolver{Root: dir}}}
	if _, err := p.Prepare(context.Background(), "p", []*domain.Document{raw}, execution.ProfileStandard, time.Now()); !errors.Is(err, input.ErrVerification) {
		t.Fatalf("want ErrVerification, got %v", err)
	}
}

func TestPrepareRefusesHeavyWithoutRuntimeAndLight(t *testing.T) {
	dir := t.TempDir()
	raw := rawRefDoc(t, dir, rawCSV)
	p := &Preparation{Resolver: input.Resolvers{"file": input.FileResolver{Root: dir}}}
	for _, profile := range []execution.Profile{execution.ProfileHeavy, execution.ProfileLight} {
		if _, err := p.Prepare(context.Background(), "p", []*domain.Document{raw}, profile, time.Now()); !errors.Is(err, execution.ErrProfileUnavailable) {
			t.Errorf("%s: want ErrProfileUnavailable, got %v", profile, err)
		}
	}
}
