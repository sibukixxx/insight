package publicengine

import "testing"

// Consumers must be able to tell, before starting work, whether this engine
// forms hypotheses (a model is configured). A deterministic engine refuses
// research runs with ANALYSIS_HAS_NO_HYPOTHESES; advertising it avoids a
// failed round trip and lets consumers explain the requirement.
func TestEngineInfoAdvertisesWhetherAnalysesAreModelBacked(t *testing.T) {
	e := newTestEngine(t)
	if e.Engine().ModelBacked {
		t.Fatal("an engine without a model probe must not claim model-backed analysis")
	}
	configured := false
	WithModelBacked(func() bool { return configured })(e)
	if e.Engine().ModelBacked {
		t.Fatal("unconfigured model reported as model-backed")
	}
	configured = true
	if !e.Engine().ModelBacked {
		t.Fatal("configured model not reported; the value must follow live settings")
	}
}
