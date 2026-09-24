package httpapi_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"insight-lab/internal/buildinfo"
	"insight-lab/internal/execution"
	httpapi "insight-lab/internal/http"
	"insight-lab/internal/input"
	"insight-lab/internal/llm"
	"insight-lab/internal/publicengine"
	"insight-lab/internal/publicengine/conformance"
	"insight-lab/internal/repository/sqlite"
	"insight-lab/internal/service"
	"insight-lab/internal/usecase"
)

const fixtureDir = "../../contracts/public-engine/v1/fixtures"

var conformanceBuild = buildinfo.Info{Version: "v0.0.0-conformance", Commit: "0000000", Dirty: "false"}

// newPublicServer starts a real engine (SQLite, JobManager, public router).
// modelBacked configures the scripted test model; otherwise the engine runs
// deterministically, exactly as without an API key.
func newPublicServer(t *testing.T, modelBacked bool) *httptest.Server {
	t.Helper()
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "public.db"))
	if err != nil {
		t.Fatal(err)
	}
	projects, documents := sqlite.NewProjectRepository(db), sqlite.NewDocumentRepository(db)
	observations, patterns := sqlite.NewObservationRepository(db), sqlite.NewPatternRepository(db)
	analyses, insights, evidence := sqlite.NewAnalysisRepository(db), sqlite.NewInsightRepository(db), sqlite.NewEvidenceRepository(db)
	settings := service.Settings{}
	if modelBacked {
		settings = service.Settings{BaseURL: "https://scripted.example.test/v1", Model: "scripted-model"}
	}
	pipeline := &service.Pipeline{Documents: documents, Observations: observations, Patterns: patterns, Insights: insights, Evidence: evidence}
	jobs := service.NewJobManager(analyses, pipeline, service.NewSettingsStore(settings), func(service.Settings) llm.Client { return scriptedModel{} })
	ctx, cancel := context.WithCancel(context.Background())
	jobs.Start(ctx, 2)
	app := usecase.New(usecase.Repositories{Projects: projects, Documents: documents, Observations: observations, Patterns: patterns, Analyses: analyses, Insights: insights, Evidence: evidence, Research: sqlite.NewResearchRepository(db)})
	// Raw artifact fixtures read fixtures/data; HEAVY uses the local adapter.
	prep := &service.Preparation{Resolver: input.Resolvers{"file": input.FileResolver{Root: fixtureDir + "/data"}}, Heavy: execution.NewLocalRuntime(t.TempDir(), 2), Partitions: 3}
	jobs.ConfigureExecution(prep)
	engine := publicengine.New(app, sqlite.NewPublicRepository(db), documents, jobs, conformanceBuild, publicengine.WithInputResolver(prep.Resolver, 0))
	server := httptest.NewServer(httpapi.NewRouter(httpapi.Deps{App: app, JobManager: jobs, PublicEngine: engine}))
	t.Cleanup(func() {
		server.Close()
		cancel()
		jobs.Wait()
		db.Close()
	})
	return server
}

// TestPublicContractConformance runs every shared fixture over plain HTTP
// against live engines. Standalone SDK repositories run the same fixtures.
func TestPublicContractConformance(t *testing.T) {
	fixtures, err := conformance.Load(fixtureDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(fixtures) < 9 {
		t.Fatalf("expected the nine #59 conformance fixtures, found %d", len(fixtures))
	}
	servers := map[string]*httptest.Server{"deterministic": newPublicServer(t, false), "model_backed": newPublicServer(t, true)}

	for _, f := range fixtures {
		t.Run(f.Name, func(t *testing.T) {
			server, ok := servers[f.Engine]
			if !ok {
				t.Fatalf("fixture %s names unknown engine %q", f.Name, f.Engine)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			client := &conformance.Client{BaseURL: server.URL, PollInterval: 20 * time.Millisecond}
			if err := conformance.Run(ctx, client, f); err != nil {
				t.Fatal(err)
			}
		})
	}

}

// scriptedModel is a deterministic stand-in for an LLM, used only by this
// test. It answers each pipeline step from its input so every quote it
// returns is grounded in the evidence: each evidence line becomes an
// observation, all observations form one deviation, and one primary
// hypothesis with two alternatives names a missing comparison. Observations
// whose text says "contradicts" are returned as counter-evidence.
type scriptedModel struct{}

var scriptedMu sync.Mutex

func (scriptedModel) Generate(_ context.Context, req llm.GenerateRequest) (*llm.GenerateResponse, error) {
	scriptedMu.Lock()
	defer scriptedMu.Unlock()
	input := req.Messages[len(req.Messages)-1].Content
	var payload struct {
		Observations []struct {
			ID    string `json:"id"`
			Quote string `json:"quote"`
		} `json:"observations"`
		Patterns []struct {
			ID string `json:"id"`
		} `json:"patterns"`
	}
	_ = json.Unmarshal([]byte(input), &payload)
	var ids, supporting, counter []string
	for _, o := range payload.Observations {
		ids = append(ids, o.ID)
		if strings.Contains(strings.ToLower(o.Quote), "contradicts") {
			counter = append(counter, o.ID)
		} else {
			supporting = append(supporting, o.ID)
		}
	}
	var patternIDs []string
	for _, p := range payload.Patterns {
		patternIDs = append(patternIDs, p.ID)
	}

	var out any
	switch req.Schema.Name {
	case "observation_extraction":
		var observations []map[string]string
		for _, line := range strings.Split(input, "\n") {
			if line = strings.TrimSpace(line); line != "" {
				observations = append(observations, map[string]string{"quote": line, "behavior": "reports: " + line, "topic": "evidence"})
			}
		}
		out = map[string]any{"observations": nonNil(observations)}
	case "trace_detection":
		out = map[string]any{"traces": []map[string]any{{
			"title": "Outcome moved against the expectation", "expectation": "the outcome stays flat",
			"actualBehavior": "the outcome changed", "deviationType": "other", "observationIds": nonNil(ids),
		}}}
	case "pattern_detection":
		out = map[string]any{"patterns": []any{}}
	case "need_hypothesis":
		out = map[string]any{"hypotheses": []map[string]any{{
			"title": "Primary explanation", "statedNeed": "understand the change", "latentNeed": "the intervention changed behavior",
			"jtbd": "explain the outcome", "expectation": "the outcome stays flat", "surprisingFact": "the outcome changed",
			"rationale":                "if the intervention changed behavior, the change is expected",
			"supportingObservationIds": nonNil(ids), "basedOnPatternIds": nonNil(patternIDs),
			"expectationBasis":      "MODEL_PROPOSED_POST_HOC",
			"missingEvidence":       []string{"untreated comparison group outcomes over the same period"},
			"falsificationCriteria": []string{"the comparison group changed by the same amount"},
			"alternativeExplanations": []map[string]any{
				{"title": "Common trend", "explanation": "a shared external trend moved every group", "missingEvidence": []string{"pre-period trend for all groups"}},
				{"title": "Measurement change", "explanation": "the counting method changed", "missingEvidence": []string{"measurement definition history"}},
			},
		}}}
	case "evidence_retrieval":
		out = map[string]any{"supportingObservationIds": nonNil(supporting), "counterObservationIds": nonNil(counter), "counterSearched": true}
	case "insight_writeup":
		out = map[string]any{"title": "Scripted insight", "observationSummary": "grounded observations", "interpretation": "a candidate explanation",
			"alternativeInterpretation": "a shared trend", "productOpportunity": "", "monetizationAngle": ""}
	case "insight_dedupe":
		out = map[string]any{"duplicateGroups": []any{}}
	default:
		return nil, fmt.Errorf("scripted model has no answer for %s", req.Schema.Name)
	}
	raw, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}
	if req.Schema.Validate != nil {
		if err := req.Schema.Validate(raw); err != nil {
			return nil, fmt.Errorf("scripted %s answer is invalid: %w", req.Schema.Name, err)
		}
	}
	return &llm.GenerateResponse{Content: raw, Usage: llm.Usage{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2}}, nil
}

func nonNil[T any](v []T) []T {
	if v == nil {
		return []T{}
	}
	return v
}
