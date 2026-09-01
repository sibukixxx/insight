//go:build demo

package sampledata

import "testing"

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
	}
}
