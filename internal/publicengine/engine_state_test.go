package publicengine

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"insight-lab/internal/repository"
)

// Consumers persist references to Core-owned state. The engine-state
// identity lets them tell a restart from a fresh or restored database
// without inferring it from counts, timestamps or host details (#116).
func TestEngineInfoAdvertisesEngineStateWhenConfigured(t *testing.T) {
	e := newTestEngine(t)
	WithEngineState(&repository.EngineState{StateID: "0123456789abcdef0123456789abcdef", CreatedAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)})(e)
	state := e.Engine().State
	if state == nil {
		t.Fatal("engine state not advertised")
	}
	if state.StateID != "0123456789abcdef0123456789abcdef" || state.CreatedAt != "2026-01-02T03:04:05Z" {
		t.Fatalf("state = %+v", state)
	}
}

// Absent means "unknown", never "same": an engine without a state identity
// must omit the field instead of sending an empty identifier.
func TestEngineInfoOmitsEngineStateWhenUnknown(t *testing.T) {
	e := newTestEngine(t)
	if e.Engine().State != nil {
		t.Fatal("engine without a state identity advertised one")
	}
	body, err := json.Marshal(e.Engine())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), `"state"`) {
		t.Fatalf("unknown state serialized: %s", body)
	}
}
