// Package httpapi wires the HTTP router. It lives under internal/http for
// directory-naming consistency with the design doc, but is declared as
// httpapi (not http) so callers can import it alongside net/http without
// needing an alias.
package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"insight-lab/internal/http/handler"
	appmw "insight-lab/internal/http/middleware"
	"insight-lab/internal/http/public"
	"insight-lab/internal/llm"
	"insight-lab/internal/publicengine"
	"insight-lab/internal/service"
	"insight-lab/internal/usecase"
	"insight-lab/internal/web"
)

type Deps struct {
	App *usecase.Application

	Demo         *service.DemoLoader
	Settings     *service.SettingsStore
	JobManager   *service.JobManager
	NewLLMClient func(service.Settings) llm.Client
	// PublicEngine serves the Public Engine Contract under /api/public/v1.
	// It is not mounted when nil.
	PublicEngine *publicengine.Engine

	Build handler.BuildInfo
}

func NewRouter(deps Deps) http.Handler {
	r := chi.NewRouter()
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(appmw.RestrictOrigin)

	h := &handler.Handler{
		App:  deps.App,
		Demo: deps.Demo, Settings: deps.Settings, JobManager: deps.JobManager,
		NewLLMClient: deps.NewLLMClient, Build: deps.Build,
	}

	r.Route("/api", func(r chi.Router) {
		if deps.PublicEngine != nil {
			r.Mount("/public/v1", public.Router(deps.PublicEngine))
		}
		r.Get("/health", h.Health)
		r.Post("/demo", h.CreateDemoProject)

		r.Get("/settings", h.GetSettings)
		r.Put("/settings", h.UpdateSettings)
		r.Post("/settings/test", h.TestSettings)

		r.Route("/projects", func(r chi.Router) {
			r.Get("/", h.ListProjects)
			r.Post("/", h.CreateProject)

			r.Route("/{projectID}", func(r chi.Router) {
				r.Get("/", h.GetProject)
				r.Delete("/", h.DeleteProject)
				r.Get("/documents", h.ListDocuments)
				r.Post("/documents", h.CreateDocument)
				r.Post("/documents/import", h.ImportDocumentsCSV)
				r.Post("/documents/import/analysis", h.ImportAnalysisCSV)
				r.Post("/analysis", h.CreateAnalysis)
				r.Get("/analyses", h.ListAnalyses)
				r.Get("/analyses/compare", h.CompareProjectAnalyses)
				r.Get("/metrics-history", h.MetricsHistory)
				r.Get("/insights", h.ListInsights)
				r.Get("/patterns", h.ListPatterns)
				r.Get("/evaluation", h.GetEvaluation)
				r.Get("/report.md", h.ExportProjectReport)
				r.Get("/research-runs", h.ListResearchRuns)
				r.Post("/research-runs", h.CreateResearchRun)
			})
		})

		r.Get("/research-runs/{runID}", h.GetResearchRun)
		r.Route("/research-runs/{runID}", func(r chi.Router) {
			r.Post("/iterations", h.AppendResearchIteration)
			r.Get("/iterations/{iterationID}", h.GetResearchIteration)
			r.Get("/compare/{fromIterationID}/{toIterationID}", h.CompareResearchIterations)
			r.Get("/timeline", h.GetResearchTimeline)
			r.Get("/iterations/{iterationID}/evaluation", h.GetHumanEvaluation)
			r.Put("/iterations/{iterationID}/evaluation", h.SaveHumanEvaluation)
			r.Put("/iterations/{iterationID}/override", h.ApplyResearchHumanOverride)
			r.Put("/iterations/{iterationID}/promotion-review", h.SubmitPromotionReview)
			r.Put("/iterations/{iterationID}/promotion-transition", h.TransitionPromotionState)
			r.Get("/handoff", h.GetHumanHandoff)
			r.Get("/report.md", h.ExportResearchReport)
			r.Get("/artifact.json", h.GetResearchArtifact)
			r.Get("/approved-artifact.json", h.GetApprovedResearchArtifact)
			r.Get("/scenarios", h.GetScenarioAnalysis)
			r.Post("/scenario-sets", h.CreateScenarioSet)
			r.Post("/scenario-sets/scaffold", h.ScaffoldScenarioSet)
			r.Post("/scenario-sets/{scenarioSetID}/evaluations", h.EvaluateScenarios)
		})

		r.Get("/documents/{documentID}", h.GetDocument)

		r.Get("/analysis/{analysisID}", h.GetAnalysis)
		r.Get("/analysis/{analysisID}/events", h.AnalysisEvents)
		r.Get("/analysis/{analysisID}/compare/{otherID}", h.CompareAnalyses)

		r.Get("/insights/{insightID}", h.GetInsight)
		r.Get("/insights/{insightID}/evidence", h.GetInsightEvidence)
	})

	r.Handle("/*", web.Handler())
	return r
}
