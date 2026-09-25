package app

import (
	"context"
	"fmt"
	"insight-lab/internal/execution"
	"insight-lab/internal/input"
	"net/http"
	"os"
	"time"

	"insight-lab/internal/buildinfo"
	httpapi "insight-lab/internal/http"
	"insight-lab/internal/http/handler"
	"insight-lab/internal/llm"
	"insight-lab/internal/publicengine"
	"insight-lab/internal/repository/sqlite"
	"insight-lab/internal/sampledata"
	"insight-lab/internal/service"
	"insight-lab/internal/usecase"
)

const analysisWorkers = 2

// Engine is the wired engine shared by the HTTP server and the headless
// CLI: the same repositories, use cases, job manager and Public Engine.
type Engine struct {
	DB         *sqlite.DB
	App        *usecase.Application
	Settings   *service.SettingsStore
	JobManager *service.JobManager
	Public     *publicengine.Engine
	Demo       *service.DemoLoader
}

// Close stops the analysis workers (after ctx is cancelled) and closes the DB.
func (e *Engine) Close() error {
	e.JobManager.Wait()
	return e.DB.Close()
}

// Open wires the engine and starts analysis workers bound to ctx.
func Open(ctx context.Context, cfg *Config) (*Engine, error) {
	db, err := sqlite.Open(cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	projects := sqlite.NewProjectRepository(db)
	documents := sqlite.NewDocumentRepository(db)
	observations := sqlite.NewObservationRepository(db)
	patterns := sqlite.NewPatternRepository(db)
	analyses := sqlite.NewAnalysisRepository(db)
	insights := sqlite.NewInsightRepository(db)
	evidence := sqlite.NewEvidenceRepository(db)
	research := sqlite.NewResearchRepository(db)

	demoLoader := &service.DemoLoader{Projects: projects, Documents: documents}
	if cfg.Demo {
		if !sampledata.Embedded {
			return nil, fmt.Errorf("this is a delivery build with no demo dataset embedded; build with `make build-demo` to get one")
		}
		if _, err := demoLoader.Ensure(ctx); err != nil {
			return nil, fmt.Errorf("load demo dataset: %w", err)
		}
	}

	if n, err := analyses.FailInterrupted(ctx); err != nil {
		return nil, fmt.Errorf("recover interrupted analyses: %w", err)
	} else if n > 0 {
		fmt.Fprintf(os.Stderr, "Marked %d unfinished analyses from the previous run as failed.\n", n)
	}

	settings := service.NewSettingsStore(service.Settings{APIKey: cfg.APIKey, Model: cfg.Model, BaseURL: cfg.BaseURL})
	pipeline := &service.Pipeline{
		Documents: documents, Observations: observations, Patterns: patterns,
		Insights: insights, Evidence: evidence,
	}
	jobManager := service.NewJobManager(analyses, pipeline, settings, service.DefaultLLMClientFactory)
	var engineOpts []publicengine.Option
	jobManager.AllowedModels = cfg.AllowedModels
	engineOpts = append(engineOpts, publicengine.WithAllowedModels(cfg.AllowedModels), publicengine.WithModelBacked(func() bool { return settings.Get().Configured() }))
	if cfg.InputRoot != "" {
		prep := &service.Preparation{Resolver: input.Resolvers{"file": input.FileResolver{Root: cfg.InputRoot}}, MaxRawBytes: publicengine.DefaultMaxRawArtifactBytes}
		if cfg.HeavyDir != "" {
			prep.Heavy = execution.NewLocalRuntime(cfg.HeavyDir, 4)
		}
		jobManager.ConfigureExecution(prep)
		engineOpts = append(engineOpts, publicengine.WithInputResolver(prep.Resolver, prep.MaxRawBytes))
	}
	jobManager.Start(ctx, analysisWorkers)
	application := usecase.New(usecase.Repositories{
		Projects: projects, Documents: documents, Observations: observations, Patterns: patterns,
		Analyses: analyses, Insights: insights, Evidence: evidence, Research: research,
		Scenarios: sqlite.NewScenarioRepository(db),
	})

	publicEngine := publicengine.New(application, sqlite.NewPublicRepository(db), documents, jobManager, buildinfo.Get(), engineOpts...)
	publicEngine.EnableTriage(sqlite.NewTriageRepository(db), func() (llm.Client, string, bool) {
		current := settings.Get()
		if !current.Configured() {
			return nil, "", false
		}
		return service.DefaultLLMClientFactory(current), current.Model, true
	})
	return &Engine{DB: db, App: application, Settings: settings, JobManager: jobManager, Public: publicEngine, Demo: demoLoader}, nil
}

// Run serves the API and, unless NoWeb, the embedded Reference Web.
func Run(ctx context.Context, cfg *Config) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	eng, err := Open(ctx, cfg)
	if err != nil {
		return err
	}
	defer func() { cancel(); eng.DB.Close() }()
	application, settings, jobManager, publicEngine, demoLoader := eng.App, eng.Settings, eng.JobManager, eng.Public, eng.Demo

	router := httpapi.NewRouter(httpapi.Deps{
		App:  application,
		Demo: demoLoader, Settings: settings, JobManager: jobManager,
		NewLLMClient: service.DefaultLLMClientFactory,
		PublicEngine: publicEngine,
		NoWeb:        cfg.NoWeb,
		Build: handler.BuildInfo{
			DemoBuild:  sampledata.Embedded,
			ClientName: cfg.ClientName,
			Engine:     buildinfo.Get(),
		},
	})

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	srv := &http.Server{
		Addr:              addr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	buildLabel := "delivery build"
	if sampledata.Embedded {
		buildLabel = "demo build"
	}
	displayURL := fmt.Sprintf("http://%s:%d", localHost(cfg.Host), cfg.Port)

	fmt.Printf("Insight Lab (%s) started.\n", buildLabel)
	if cfg.ClientName != "" {
		fmt.Printf("Confidential — prepared for %s\n", cfg.ClientName)
	}
	if !settings.Get().Configured() {
		fmt.Println("The LLM is not configured. Set the connection on the Settings page or use --api-key, --model, and --base-url.")
	}
	fmt.Printf("%s\n", displayURL)

	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	if !cfg.NoBrowser && !cfg.NoWeb {
		fmt.Println("Opening browser...")
		go openBrowser(displayURL)
	}

	select {
	case <-ctx.Done():
	case err := <-errCh:
		return err
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	jobManager.Wait()
	return nil
}

func localHost(bindHost string) string {
	if bindHost == "0.0.0.0" || bindHost == "" {
		return "127.0.0.1"
	}
	return bindHost
}
