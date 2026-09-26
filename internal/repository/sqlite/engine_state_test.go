package sqlite

import (
	"context"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"insight-lab/internal/repository"
)

func readEngineState(t *testing.T, path string) *repository.EngineState {
	t.Helper()
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	state, err := NewPublicRepository(db).EngineState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func TestEngineStateIsOpaqueAndSurvivesReopenWhenSameDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "engine.db")
	first := readEngineState(t, path)
	if !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(first.StateID) {
		t.Fatalf("stateId %q is not an opaque 128-bit hex identifier", first.StateID)
	}
	if first.CreatedAt.IsZero() {
		t.Fatal("createdAt is zero")
	}
	second := readEngineState(t, path)
	if second.StateID != first.StateID || !second.CreatedAt.Equal(first.CreatedAt) {
		t.Fatalf("state changed across reopen: %+v -> %+v", first, second)
	}
}

func TestEngineStateDiffersWhenDatabaseIsFresh(t *testing.T) {
	dir := t.TempDir()
	a := readEngineState(t, filepath.Join(dir, "a.db"))
	b := readEngineState(t, filepath.Join(dir, "b.db"))
	if a.StateID == b.StateID {
		t.Fatalf("two fresh databases share stateId %q", a.StateID)
	}
}

// A database initialized before the engine_state migration gets exactly one
// state whose createdAt is the database's original initialization time, not
// the upgrade time.
func TestEngineStateUsesOriginalInitTimeWhenDatabaseIsUpgraded(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	const initializedAt = "2026-01-02T03:04:05Z"
	for _, stmt := range []string{
		`DROP TABLE engine_state`,
		`DELETE FROM schema_migrations WHERE version = 20`,
		`UPDATE schema_migrations SET applied_at = '` + initializedAt + `'`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	db.Close()

	db, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var rows int
	if err := db.QueryRow(`SELECT COUNT(*) FROM engine_state`).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 1 {
		t.Fatalf("engine_state rows = %d, want 1", rows)
	}
	state, err := NewPublicRepository(db).EngineState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if want, _ := time.Parse(time.RFC3339, initializedAt); !state.CreatedAt.Equal(want) {
		t.Fatalf("createdAt = %s, want original init time %s", state.CreatedAt, want)
	}
}

func TestEngineStateRejectsSecondRowWhenInserted(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "engine.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`INSERT INTO engine_state (id, state_id, created_at) VALUES (2, 'x', '2026-01-01T00:00:00Z')`); err == nil {
		t.Fatal("engine_state accepted a second row")
	}
}
