package app

import (
	"context"
	"errors"
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

func TestDistributedRuntimeNeedsABrokerAndABuildWithItsAdapter(t *testing.T) {
	db := filepath.Join(t.TempDir(), "i.db")
	heavy, root := t.TempDir(), t.TempDir()
	if _, err := ParseConfig([]string{"-db", db, "-runtime", "distributed", "-heavy-dir", heavy, "-input-root", root}); err == nil {
		t.Fatal("distributed runtime without -broker was accepted")
	}
	cfg, err := ParseConfig([]string{"-db", db, "-runtime", "distributed", "-heavy-dir", heavy, "-input-root", root, "-broker", "nats://127.0.0.1:1"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if _, err := Open(ctx, cfg); !errors.Is(err, execution.ErrNoBrokerAdapter) {
		t.Fatalf("a default build must refuse the distributed runtime instead of running locally: %v", err)
	}
}
