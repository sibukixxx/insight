package app

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func openTestEngine(t *testing.T, db string) (*Engine, context.CancelFunc, error) {
	t.Helper()
	cfg, err := ParseConfig([]string{"-db", db})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	eng, err := Open(ctx, cfg)
	if err != nil {
		cancel()
	}
	return eng, cancel, err
}

func TestOpenRefusesASecondEngineOnTheSameDatabaseUntilTheFirstCloses(t *testing.T) {
	db := filepath.Join(t.TempDir(), "insight.db")
	first, cancel, err := openTestEngine(t, db)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := openTestEngine(t, db); !errors.Is(err, ErrEngineInUse) {
		t.Fatalf("second engine on the same database: %v, want ErrEngineInUse", err)
	}
	other, cancelOther, err := openTestEngine(t, filepath.Join(t.TempDir(), "other.db"))
	if err != nil {
		t.Fatalf("an engine on another database must open: %v", err)
	}
	cancelOther()
	other.Close()

	cancel()
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	again, cancelAgain, err := openTestEngine(t, db)
	if err != nil {
		t.Fatalf("reopen after close: %v", err)
	}
	cancelAgain()
	again.Close()
}
