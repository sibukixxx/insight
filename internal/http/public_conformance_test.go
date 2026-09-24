package httpapi_test

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"insight-lab/internal/buildinfo"
	"insight-lab/internal/execution"
	httpapi "insight-lab/internal/http"
	"insight-lab/internal/input"
	"insight-lab/internal/llm"
	"insight-lab/internal/llm/scripted"
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
	jobs := service.NewJobManager(analyses, pipeline, service.NewSettingsStore(settings), func(service.Settings) llm.Client { return scripted.Model{} })
	ctx, cancel := context.WithCancel(context.Background())
	jobs.Start(ctx, 2)
	app := usecase.New(usecase.Repositories{Projects: projects, Documents: documents, Observations: observations, Patterns: patterns, Analyses: analyses, Insights: insights, Evidence: evidence, Research: sqlite.NewResearchRepository(db), Scenarios: sqlite.NewScenarioRepository(db)})
	// Raw artifact fixtures read fixtures/data; HEAVY uses the local adapter.
	prep := &service.Preparation{Resolver: input.Resolvers{"file": input.FileResolver{Root: fixtureDir + "/data"}}, Heavy: execution.NewLocalRuntime(t.TempDir(), 2), Partitions: 3}
	jobs.ConfigureExecution(prep)
	jobs.AllowedModels = []string{"scripted-model-large"}
	engine := publicengine.New(app, sqlite.NewPublicRepository(db), documents, jobs, conformanceBuild, publicengine.WithInputResolver(prep.Resolver, 0), publicengine.WithAllowedModels(jobs.AllowedModels))
	engine.EnableTriage(sqlite.NewTriageRepository(db), nil)
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
