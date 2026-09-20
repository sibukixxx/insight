package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"insight-lab/internal/domain"
	httpapi "insight-lab/internal/http"
	"insight-lab/internal/repository/sqlite"
	"insight-lab/internal/usecase"
)

func TestResearchHTTPDogfoodPathPersistsHumanEvaluationAndReport(t *testing.T) {
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "http.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	projects, documents := sqlite.NewProjectRepository(db), sqlite.NewDocumentRepository(db)
	observations, patterns := sqlite.NewObservationRepository(db), sqlite.NewPatternRepository(db)
	analyses, insights, evidence := sqlite.NewAnalysisRepository(db), sqlite.NewInsightRepository(db), sqlite.NewEvidenceRepository(db)
	research := sqlite.NewResearchRepository(db)
	now := time.Now().UTC()
	ctx := context.Background()
	if err := projects.Create(ctx, &domain.Project{ID: "p1", Name: "Synthetic policy", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := analyses.Create(ctx, &domain.Analysis{ID: "a1", ProjectID: "p1", Status: domain.AnalysisCompleted, CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	analysisID := "a1"
	for n, title := range []string{"Treatment effect", "Common external trend", "Measurement change"} {
		if err := insights.Create(ctx, &domain.Insight{ID: "h" + string(rune('1'+n)), ProjectID: "p1", AnalysisID: &analysisID, Title: title, SurprisingFact: "treated and comparison outcomes increased", HypothesisSetID: "set1", MissingEvidence: []string{"pre-period trend", "comparison quality"}, ValidationStatus: domain.ValidationInsufficientEvidence, IdentificationStatus: domain.IdentificationNotIdentified, CreatedAt: now}); err != nil {
			t.Fatal(err)
		}
	}
	app := usecase.New(usecase.Repositories{Projects: projects, Documents: documents, Observations: observations, Patterns: patterns, Analyses: analyses, Insights: insights, Evidence: evidence, Research: research})
	router := httpapi.NewRouter(httpapi.Deps{App: app})

	create := httptest.NewRequest(http.MethodPost, "/api/projects/p1/research-runs", bytes.NewBufferString(`{"question":"Did treatment cause the increase?","inputReferences":["synthetic.csv"]}`))
	create.Header.Set("content-type", "application/json")
	created := httptest.NewRecorder()
	router.ServeHTTP(created, create)
	if created.Code != http.StatusCreated {
		t.Fatalf("create run: %d %s", created.Code, created.Body.String())
	}
	var run domain.ResearchRun
	if err := json.Unmarshal(created.Body.Bytes(), &run); err != nil {
		t.Fatal(err)
	}
	if len(run.Iterations) != 1 || len(run.Iterations[0].ResearchGaps) == 0 {
		t.Fatalf("research projection missing: %+v", run)
	}

	evaluationBody := `{"observationGrounding":4,"surpriseUsefulness":4,"hypothesisDiversity":5,"counterEvidenceQuality":3,"missingEvidenceQuality":4,"identificationHonesty":5,"nextDataUsefulness":4,"novelty":"NEW","overallUsefulness":4}`
	evalURL := "/api/research-runs/" + run.ID + "/iterations/" + run.Iterations[0].ID + "/evaluation"
	evalReq := httptest.NewRequest(http.MethodPut, evalURL, bytes.NewBufferString(evaluationBody))
	evalReq.Header.Set("content-type", "application/json")
	evalResp := httptest.NewRecorder()
	router.ServeHTTP(evalResp, evalReq)
	if evalResp.Code != http.StatusOK {
		t.Fatalf("save evaluation: %d %s", evalResp.Code, evalResp.Body.String())
	}

	iterateReq := httptest.NewRequest(http.MethodPost, "/api/research-runs/"+run.ID+"/iterations", bytes.NewBufferString(`{"inputReferences":["comparison.csv"],"addedEvidence":["untreated comparison outcomes"]}`))
	iterateReq.Header.Set("content-type", "application/json")
	iterateResp := httptest.NewRecorder()
	router.ServeHTTP(iterateResp, iterateReq)
	if iterateResp.Code != http.StatusCreated {
		t.Fatalf("append iteration: %d %s", iterateResp.Code, iterateResp.Body.String())
	}
	var second domain.ResearchIteration
	if err := json.Unmarshal(iterateResp.Body.Bytes(), &second); err != nil {
		t.Fatal(err)
	}
	if second.Sequence != 2 || len(second.AddedEvidence) != 1 || len(second.HypothesisChanges) != 3 {
		t.Fatalf("iteration history not auditable: %+v", second)
	}
	persisted, err := research.GetResearchRun(ctx, run.ID)
	if err != nil || len(persisted.Iterations) != 2 || persisted.Iterations[0].ID != run.Iterations[0].ID {
		t.Fatalf("prior iteration was not preserved: %+v %v", persisted, err)
	}

	reportReq := httptest.NewRequest(http.MethodGet, "/api/research-runs/"+run.ID+"/report.md", nil)
	reportResp := httptest.NewRecorder()
	router.ServeHTTP(reportResp, reportReq)
	if reportResp.Code != http.StatusOK {
		t.Fatalf("report: %d %s", reportResp.Code, reportResp.Body.String())
	}
	for _, want := range []string{"Research Gaps", "What We Cannot Conclude", "novelty `NEW`", "Decision Readiness"} {
		if !strings.Contains(reportResp.Body.String(), want) {
			t.Errorf("report missing %q", want)
		}
	}

	overrideURL := "/api/research-runs/" + run.ID + "/iterations/" + second.ID + "/override"
	overrideReq := httptest.NewRequest(http.MethodPut, overrideURL, bytes.NewBufferString(`{"stopReason":"EXTERNAL_BUDGET_BOUNDARY","note":"sprint ended"}`))
	overrideReq.Header.Set("content-type", "application/json")
	overrideResp := httptest.NewRecorder()
	router.ServeHTTP(overrideResp, overrideReq)
	if overrideResp.Code != http.StatusOK {
		t.Fatalf("apply override: %d %s", overrideResp.Code, overrideResp.Body.String())
	}
	var overridden domain.ResearchIteration
	if err := json.Unmarshal(overrideResp.Body.Bytes(), &overridden); err != nil {
		t.Fatal(err)
	}
	if overridden.Stop == nil || overridden.Stop.Reason != domain.StopExternalBudgetBoundary || overridden.Stop.Source != domain.StopSourceHuman {
		t.Fatalf("human override not recorded: %+v", overridden.Stop)
	}

	handoffReq := httptest.NewRequest(http.MethodGet, "/api/research-runs/"+run.ID+"/handoff", nil)
	handoffResp := httptest.NewRecorder()
	router.ServeHTTP(handoffResp, handoffReq)
	if handoffResp.Code != http.StatusOK {
		t.Fatalf("get handoff: %d %s", handoffResp.Code, handoffResp.Body.String())
	}
	var handoff domain.HumanHandoff
	if err := json.Unmarshal(handoffResp.Body.Bytes(), &handoff); err != nil {
		t.Fatal(err)
	}
	if handoff.ResearchRunID != run.ID || handoff.Stop == nil || handoff.Stop.Reason != domain.StopExternalBudgetBoundary {
		t.Fatalf("handoff must reflect the human stop decision: %+v", handoff)
	}

	artifactReq := httptest.NewRequest(http.MethodGet, "/api/research-runs/"+run.ID+"/artifact.json", nil)
	artifactResp := httptest.NewRecorder()
	router.ServeHTTP(artifactResp, artifactReq)
	if artifactResp.Code != http.StatusOK {
		t.Fatalf("get artifact: %d %s", artifactResp.Code, artifactResp.Body.String())
	}
	var artifact usecase.ResearchArtifact
	if err := json.Unmarshal(artifactResp.Body.Bytes(), &artifact); err != nil {
		t.Fatal(err)
	}
	if artifact.ArtifactSchema != usecase.ResearchArtifactSchema || artifact.SchemaVersion != usecase.ResearchArtifactVersion {
		t.Fatalf("artifact must declare its schema/version: %+v", artifact)
	}
	if artifact.ResearchRunID != run.ID || artifact.IterationID != second.ID {
		t.Fatalf("artifact must snapshot the run's latest iteration: %+v", artifact)
	}
	if artifact.StopDecision == nil || artifact.StopDecision.Reason != domain.StopExternalBudgetBoundary {
		t.Fatalf("artifact must reflect the human stop decision: %+v", artifact.StopDecision)
	}
	if artifact.ResearchStage != overridden.Stage {
		t.Fatalf("artifact must export the latest iteration's research stage losslessly over HTTP: artifact=%q iteration=%q", artifact.ResearchStage, overridden.Stage)
	}
}

func TestResearchHTTPPromotionReviewAndTransition(t *testing.T) {
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "http_promotion.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	projects, documents := sqlite.NewProjectRepository(db), sqlite.NewDocumentRepository(db)
	observations, patterns := sqlite.NewObservationRepository(db), sqlite.NewPatternRepository(db)
	analyses, insights, evidence := sqlite.NewAnalysisRepository(db), sqlite.NewInsightRepository(db), sqlite.NewEvidenceRepository(db)
	research := sqlite.NewResearchRepository(db)
	now := time.Now().UTC()
	ctx := context.Background()
	if err := projects.Create(ctx, &domain.Project{ID: "p1", Name: "Synthetic policy", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := analyses.Create(ctx, &domain.Analysis{ID: "a1", ProjectID: "p1", Status: domain.AnalysisCompleted, CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	analysisID := "a1"
	if err := insights.Create(ctx, &domain.Insight{ID: "h1", ProjectID: "p1", AnalysisID: &analysisID, Title: "Treatment effect", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	app := usecase.New(usecase.Repositories{Projects: projects, Documents: documents, Observations: observations, Patterns: patterns, Analyses: analyses, Insights: insights, Evidence: evidence, Research: research})
	router := httpapi.NewRouter(httpapi.Deps{App: app})

	create := httptest.NewRequest(http.MethodPost, "/api/projects/p1/research-runs", bytes.NewBufferString(`{"question":"Did treatment cause the increase?"}`))
	create.Header.Set("content-type", "application/json")
	created := httptest.NewRecorder()
	router.ServeHTTP(created, create)
	if created.Code != http.StatusCreated {
		t.Fatalf("create run: %d %s", created.Code, created.Body.String())
	}
	var run domain.ResearchRun
	if err := json.Unmarshal(created.Body.Bytes(), &run); err != nil {
		t.Fatal(err)
	}

	reviewURL := "/api/research-runs/" + run.ID + "/iterations/" + run.Iterations[0].ID + "/promotion-review"
	reviewReq := httptest.NewRequest(http.MethodPut, reviewURL, bytes.NewBufferString(`{"contribution":"CORRECTION","humanReviewCompleted":false}`))
	reviewReq.Header.Set("content-type", "application/json")
	reviewResp := httptest.NewRecorder()
	router.ServeHTTP(reviewResp, reviewReq)
	if reviewResp.Code != http.StatusOK {
		t.Fatalf("submit promotion review: %d %s", reviewResp.Code, reviewResp.Body.String())
	}
	var reviewed domain.ResearchIteration
	if err := json.Unmarshal(reviewResp.Body.Bytes(), &reviewed); err != nil {
		t.Fatal(err)
	}
	if reviewed.Promotion.State != domain.PromotionHumanReviewRequired {
		t.Fatalf("expected HUMAN_REVIEW_REQUIRED without human review, got %s (reasons: %v)", reviewed.Promotion.State, reviewed.Promotion.Reasons)
	}

	reportReq := httptest.NewRequest(http.MethodGet, "/api/research-runs/"+run.ID+"/report.md", nil)
	reportResp := httptest.NewRecorder()
	router.ServeHTTP(reportResp, reportReq)
	if reportResp.Code != http.StatusOK {
		t.Fatalf("report: %d %s", reportResp.Code, reportResp.Body.String())
	}
	for _, want := range []string{"## Promotion Status", "**State:** `HUMAN_REVIEW_REQUIRED`", "publication checklist is incomplete", "Unmet publication checklist items"} {
		if !strings.Contains(reportResp.Body.String(), want) {
			t.Errorf("report missing %q", want)
		}
	}

	transitionURL := "/api/research-runs/" + run.ID + "/iterations/" + run.Iterations[0].ID + "/promotion-transition"
	transitionReq := httptest.NewRequest(http.MethodPut, transitionURL, bytes.NewBufferString(`{"targetState":"REJECTED_FOR_PUBLICATION"}`))
	transitionReq.Header.Set("content-type", "application/json")
	transitionResp := httptest.NewRecorder()
	router.ServeHTTP(transitionResp, transitionReq)
	if transitionResp.Code != http.StatusOK {
		t.Fatalf("transition promotion state: %d %s", transitionResp.Code, transitionResp.Body.String())
	}
	var rejected domain.ResearchIteration
	if err := json.Unmarshal(transitionResp.Body.Bytes(), &rejected); err != nil {
		t.Fatal(err)
	}
	if rejected.Promotion.State != domain.PromotionRejectedForPublication {
		t.Fatalf("expected REJECTED_FOR_PUBLICATION, got %s", rejected.Promotion.State)
	}
}
