package app

import (
	"context"
	"fmt"
	"net/http"
	"time"

	httpapi "insight-lab/internal/http"
	"insight-lab/internal/http/handler"
	"insight-lab/internal/repository/sqlite"
	"insight-lab/internal/sampledata"
	"insight-lab/internal/service"
	"insight-lab/internal/usecase"
)

const analysisWorkers = 2

func Run(ctx context.Context, cfg *Config) error {
	db, err := sqlite.Open(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	projects := sqlite.NewProjectRepository(db)
	documents := sqlite.NewDocumentRepository(db)
	observations := sqlite.NewObservationRepository(db)
	patterns := sqlite.NewPatternRepository(db)
	analyses := sqlite.NewAnalysisRepository(db)
	insights := sqlite.NewInsightRepository(db)
	evidence := sqlite.NewEvidenceRepository(db)

	demoLoader := &service.DemoLoader{Projects: projects, Documents: documents}
	if cfg.Demo {
		if !sampledata.Embedded {
			return fmt.Errorf("this is a delivery build with no demo dataset embedded; build with `make build-demo` to get one")
		}
		if _, err := demoLoader.Ensure(ctx); err != nil {
			return fmt.Errorf("load demo dataset: %w", err)
		}
	}

	if n, err := analyses.FailInterrupted(ctx); err != nil {
		return fmt.Errorf("recover interrupted analyses: %w", err)
	} else if n > 0 {
		fmt.Printf("前回未完了だった解析を%d件、失敗として記録しました。\n", n)
	}

	settings := service.NewSettingsStore(service.Settings{APIKey: cfg.APIKey, Model: cfg.Model, BaseURL: cfg.BaseURL})
	pipeline := &service.Pipeline{
		Documents: documents, Observations: observations, Patterns: patterns,
		Insights: insights, Evidence: evidence,
	}
	jobManager := service.NewJobManager(analyses, pipeline, settings, service.DefaultLLMClientFactory)
	jobManager.Start(ctx, analysisWorkers)
	application := usecase.New(usecase.Repositories{
		Projects: projects, Documents: documents, Observations: observations, Patterns: patterns,
		Analyses: analyses, Insights: insights, Evidence: evidence,
	})

	router := httpapi.NewRouter(httpapi.Deps{
		App:  application,
		Demo: demoLoader, Settings: settings, JobManager: jobManager,
		NewLLMClient: service.DefaultLLMClientFactory,
		Build: handler.BuildInfo{
			DemoBuild:  sampledata.Embedded,
			ClientName: cfg.ClientName,
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
		fmt.Println("LLMは未設定です。設定画面（または --api-key/--model/--base-url）で接続先を指定してください。")
	}
	fmt.Printf("%s\n", displayURL)

	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	if !cfg.NoBrowser {
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
