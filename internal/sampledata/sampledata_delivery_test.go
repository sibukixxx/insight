//go:build !demo

package sampledata

import (
	"errors"
	"testing"
)

func TestLoadNotEmbedded(t *testing.T) {
	if Embedded {
		t.Fatal("Embedded should be false without the demo build tag")
	}
	if _, err := Load(); !errors.Is(err, ErrNotEmbedded) {
		t.Errorf("Load() error = %v, want ErrNotEmbedded", err)
	}
}
