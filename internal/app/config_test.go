package app

import (
	"path/filepath"
	"testing"

	"insight-lab/internal/execution"
)

func TestParseConfigRuntimeIsLocalByDefaultAndProcessNeedsHeavyDirAndInputRoot(t *testing.T) {
	db := filepath.Join(t.TempDir(), "i.db")
	cfg, err := ParseConfig([]string{"-db", db})
	if err != nil || cfg.Runtime != execution.RuntimeLocal {
		t.Fatalf("default runtime = %q, %v", cfg.Runtime, err)
	}
	if _, err := ParseConfig([]string{"-db", db, "-runtime", "process", "-heavy-dir", t.TempDir()}); err == nil {
		t.Fatal("process runtime without -input-root was accepted")
	}
	if _, err := ParseConfig([]string{"-db", db, "-runtime", "remote"}); err == nil {
		t.Fatal("an unknown runtime was accepted")
	}
	cfg, err = ParseConfig([]string{"-db", db, "-runtime", "process", "-heavy-dir", t.TempDir(), "-input-root", t.TempDir()})
	if err != nil || cfg.Runtime != execution.RuntimeProcess {
		t.Fatalf("process runtime = %q, %v", cfg.Runtime, err)
	}
}
