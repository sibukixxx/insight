//go:build demo

package sampledata

import (
	"strings"
	"testing"
)

func TestLoadEmbedded(t *testing.T) {
	if !Embedded {
		t.Fatal("Embedded should be true in a demo-tagged build")
	}
	docs, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(docs) < 10 {
		t.Errorf("got %d documents, want at least 10", len(docs))
	}
	for _, d := range docs {
		if !d.Source.Valid() {
			t.Errorf("document %s has invalid source %q", d.ID, d.Source)
		}
		if d.Content == "" {
			t.Errorf("document %s has empty content", d.ID)
		}
		if d.Metadata["fixture"] != "synthetic" {
			t.Errorf("document %s is not explicitly marked synthetic", d.ID)
		}
	}
	joined := ""
	for _, d := range docs {
		joined += " " + d.Content
	}
	for _, required := range []string{"calendar year 2024 was 100", "calendar year 2025 was 130", "calendar year 2024 was 200", "calendar year 2025 was 240", "already increasing before", "system migration", "cannot tell"} {
		if !strings.Contains(joined, required) {
			t.Errorf("demo does not contain required research signal %q", required)
		}
	}
	for _, forbidden := range []string{"the program caused", "causal effect was", "CAUSALLY_SUPPORTED"} {
		if strings.Contains(strings.ToLower(joined), strings.ToLower(forbidden)) {
			t.Errorf("demo fixture contains a causal conclusion: %q", forbidden)
		}
	}
}
