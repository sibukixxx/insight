package service

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"insight-lab/internal/domain"
	"insight-lab/internal/llm"
	"insight-lab/internal/repository/sqlite"
)

// fakeLLM drives the pipeline deterministically without any network
// access. It inspects the actual request payload for each step (in
// particular the real observation IDs the pipeline assigned after
// grounding) so the scripted responses stay valid regardless of how IDs
// were generated.
type fakeLLM struct {
	calls map[string]int
}

func newFakeLLM() *fakeLLM { return &fakeLLM{calls: map[string]int{}} }

const testAnalysisID = "ana_test"

type obsRefPayload struct {
	Observations []observationRef `json:"observations"`
}

type patternRefPayload struct {
	Patterns []patternRef `json:"patterns"`
}

func idsWhere(refs []observationRef, pred func(observationRef) bool) []string {
	var ids []string
	for _, r := range refs {
		if pred(r) {
			ids = append(ids, r.ID)
		}
	}
	return ids
}

func (f *fakeLLM) Generate(ctx context.Context, req llm.GenerateRequest) (*llm.GenerateResponse, error) {
	name := req.Schema.Name
	f.calls[name]++
	last := req.Messages[len(req.Messages)-1].Content

	var raw json.RawMessage
	switch name {
	case "observation_extraction":
		switch {
		case strings.Contains(last, "怖いんですよね"):
			raw = json.RawMessage(`{"observations":[
				{"quote":"設定を間違えたら怖いんですよね","behavior":"設定ミスを恐れている","topic":"anxiety"},
				{"quote":"絶対に安全だと確信しています","behavior":"fabricated - not in source","topic":"x"}
			]}`)
		case strings.Contains(last, "別に設定は難しくないです"):
			raw = json.RawMessage(`{"observations":[{"quote":"別に設定は難しくないです","behavior":"操作に困っていない","topic":"ease"}]}`)
		default:
			raw = json.RawMessage(`{"observations":[]}`)
		}

	case "pattern_detection":
		var p obsRefPayload
		_ = json.Unmarshal([]byte(last), &p)
		ids := idsWhere(p.Observations, func(observationRef) bool { return true })
		idsJSON, _ := json.Marshal(ids)
		raw = json.RawMessage(fmt.Sprintf(`{"patterns":[{"title":"確認への依存","description":"複数ドキュメントで確認行動が見られる","observationIds":%s}]}`, idsJSON))

	case "need_hypothesis":
		var p obsRefPayload
		_ = json.Unmarshal([]byte(last), &p)
		obsIDs := idsWhere(p.Observations, func(observationRef) bool { return true })
		obsIDsJSON, _ := json.Marshal(obsIDs)

		var pp patternRefPayload
		_ = json.Unmarshal([]byte(last), &pp)
		var patternIDs []string
		for _, pr := range pp.Patterns {
			patternIDs = append(patternIDs, pr.ID)
		}
		patternIDsJSON, _ := json.Marshal(patternIDs)

		raw = json.RawMessage(fmt.Sprintf(`{"hypotheses":[{
			"title":"確認への依存の裏にある恐怖",
			"statedNeed":"作業を早く終わらせたい",
			"latentNeed":"失敗して信頼を失うことを避けたい",
			"jtbd":"安心して提出できる状態にしたい",
			"rationale":"複数の発言で、確認作業そのものより「間違えたときの結果」への言及が繰り返されていたため、時間短縮ではなく失敗回避が本質的な動機だと判断した",
			"supportingObservationIds":%s,
			"basedOnPatternIds":%s
		}]}`, obsIDsJSON, patternIDsJSON))

	case "evidence_retrieval":
		var p obsRefPayload
		_ = json.Unmarshal([]byte(last), &p)
		support := idsWhere(p.Observations, func(o observationRef) bool { return o.Topic == "anxiety" })
		counter := idsWhere(p.Observations, func(o observationRef) bool { return o.Topic == "ease" })
		supportJSON, _ := json.Marshal(support)
		counterJSON, _ := json.Marshal(counter)
		raw = json.RawMessage(fmt.Sprintf(`{"supportingObservationIds":%s,"counterObservationIds":%s,"counterSearched":true}`, supportJSON, counterJSON))

	case "insight_writeup":
		raw = json.RawMessage(`{
			"title":"確認作業の裏にある「誤請求への恐怖」",
			"observationSummary":"複数ユーザーが送信前に手動で確認している",
			"interpretation":"時間短縮より、失敗を避けられる確信を求めている可能性がある",
			"alternativeInterpretation":"単に社内の承認プロセスが原因である可能性もある",
			"productOpportunity":"送信前の異常値検出と確認済み証跡の提供",
			"monetizationAngle":"請求書ミス防止チェックリストやテンプレートをnoteで販売できる可能性がある"
		}`)

	case "insight_dedupe":
		raw = json.RawMessage(`{"duplicateGroups":[]}`)

	default:
		return nil, fmt.Errorf("fakeLLM: unexpected schema %q", name)
	}

	if err := req.Schema.Validate(raw); err != nil {
		return nil, fmt.Errorf("fakeLLM produced invalid output for %s: %w", name, err)
	}
	return &llm.GenerateResponse{Content: raw, Mode: llm.ModeJSONSchema}, nil
}

func newTestPipeline(t *testing.T) (*Pipeline, *sqlite.DB, *domain.Project) {
	t.Helper()
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "pipeline_test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	projects := sqlite.NewProjectRepository(db)
	documents := sqlite.NewDocumentRepository(db)
	observations := sqlite.NewObservationRepository(db)
	patterns := sqlite.NewPatternRepository(db)
	analyses := sqlite.NewAnalysisRepository(db)
	insights := sqlite.NewInsightRepository(db)
	evidence := sqlite.NewEvidenceRepository(db)

	ctx := context.Background()
	p := &domain.Project{ID: "proj_1", Name: "テスト", CreatedAt: time.Now().UTC()}
	if err := projects.Create(ctx, p); err != nil {
		t.Fatalf("create project: %v", err)
	}
	docs := []*domain.Document{
		{ID: "doc_1", ProjectID: p.ID, Source: domain.SourceInterview, Title: "Interview #1",
			Content: "設定を間違えたら怖いんですよね。最後は自分で確認します。", CreatedAt: time.Now().UTC()},
		{ID: "doc_2", ProjectID: p.ID, Source: domain.SourceReview, Title: "Review #1",
			Content: "別に設定は難しくないです。", CreatedAt: time.Now().UTC()},
	}
	if err := documents.CreateBatch(ctx, docs); err != nil {
		t.Fatalf("create documents: %v", err)
	}
	analysis := &domain.Analysis{ID: testAnalysisID, ProjectID: p.ID, Status: domain.AnalysisRunning, CreatedAt: time.Now().UTC()}
	if err := analyses.Create(ctx, analysis); err != nil {
		t.Fatalf("create analysis: %v", err)
	}

	pipeline := &Pipeline{
		Documents: documents, Observations: observations, Patterns: patterns,
		Insights: insights, Evidence: evidence,
		LLM: newFakeLLM(),
	}
	return pipeline, db, p
}

func TestPipelineRunEndToEnd(t *testing.T) {
	pipeline, db, project := newTestPipeline(t)
	ctx := context.Background()

	var progressLog []string
	metrics, err := pipeline.Run(ctx, testAnalysisID, project.ID, func(step string, progress int, message string) {
		progressLog = append(progressLog, fmt.Sprintf("%s:%d", step, progress))
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// One fabricated quote out of three candidates must have been
	// discarded by grounding, not silently kept.
	if metrics.TotalObservationCandidates != 3 {
		t.Errorf("TotalObservationCandidates = %d, want 3", metrics.TotalObservationCandidates)
	}
	if metrics.GroundedObservations != 2 {
		t.Errorf("GroundedObservations = %d, want 2", metrics.GroundedObservations)
	}
	wantRate := 1.0 / 3.0
	if diff := metrics.UnsupportedClaimRate - wantRate; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("UnsupportedClaimRate = %f, want %f", metrics.UnsupportedClaimRate, wantRate)
	}

	if metrics.FinalInsightCount != 1 {
		t.Fatalf("FinalInsightCount = %d, want 1", metrics.FinalInsightCount)
	}
	if metrics.EvidenceCoverage != 1.0 {
		t.Errorf("EvidenceCoverage = %f, want 1.0", metrics.EvidenceCoverage)
	}
	if metrics.CounterEvidenceCoverage != 1.0 {
		t.Errorf("CounterEvidenceCoverage = %f, want 1.0", metrics.CounterEvidenceCoverage)
	}
	if metrics.PatternCount != 1 {
		t.Errorf("PatternCount = %d, want 1", metrics.PatternCount)
	}

	insights := sqlite.NewInsightRepository(db)
	list, err := insights.ListByProject(ctx, project.ID)
	if err != nil || len(list) != 1 {
		t.Fatalf("ListByProject = %v, %v", list, err)
	}
	insight := list[0]
	if insight.LatentNeed == "" || insight.AlternativeInterpretation == "" {
		t.Errorf("insight missing required narrative fields: %+v", insight)
	}
	if insight.MonetizationAngle == "" {
		t.Error("MonetizationAngle should round-trip through persistence")
	}
	if insight.Rationale == "" {
		t.Error("Rationale should round-trip through persistence (this is the previously-discarded 'why this hypothesis' reasoning)")
	}
	if insight.AnalysisID == nil || *insight.AnalysisID != testAnalysisID {
		t.Errorf("AnalysisID = %v, want %q", insight.AnalysisID, testAnalysisID)
	}
	if insight.Confidence <= 0 || insight.Confidence > 1 {
		t.Errorf("Confidence = %f, want in (0,1]", insight.Confidence)
	}

	// The reasoning trail: this insight's hypothesis must be traceable
	// back to the pattern it cited, and that pattern back to the
	// observation(s) it was built from - not just the final Evidence.
	patterns := sqlite.NewPatternRepository(db)
	insightPatterns, err := patterns.ListByInsight(ctx, insight.ID)
	if err != nil {
		t.Fatalf("ListByInsight patterns: %v", err)
	}
	if len(insightPatterns) != 1 {
		t.Fatalf("insight linked to %d patterns, want 1", len(insightPatterns))
	}
	pattern := insightPatterns[0]
	if pattern.Title == "" {
		t.Error("pattern missing title")
	}
	if len(pattern.ObservationIDs) != 2 {
		t.Errorf("pattern has %d linked observations, want 2 (both grounded observations, not the fabricated one)", len(pattern.ObservationIDs))
	}

	projectPatterns, err := patterns.ListByProject(ctx, project.ID)
	if err != nil || len(projectPatterns) != 1 {
		t.Fatalf("ListByProject patterns = %v, %v", projectPatterns, err)
	}

	evidenceRepo := sqlite.NewEvidenceRepository(db)
	ev, err := evidenceRepo.ListByInsight(ctx, insight.ID)
	if err != nil {
		t.Fatalf("ListByInsight: %v", err)
	}
	var support, counter int
	for _, e := range ev {
		switch e.Type {
		case domain.EvidenceSupport:
			support++
			if e.Quote != "設定を間違えたら怖いんですよね" {
				t.Errorf("unexpected supporting quote: %q", e.Quote)
			}
		case domain.EvidenceCounter:
			counter++
			if e.Quote != "別に設定は難しくないです" {
				t.Errorf("unexpected counter quote: %q", e.Quote)
			}
		}
		// Every evidence quote must be independently groundable in its
		// document - i.e. this test never trusts the fake LLM's quote,
		// only what grounding.go actually verified.
	}
	if support != 1 || counter != 1 {
		t.Errorf("support=%d counter=%d, want 1 and 1", support, counter)
	}

	// The fabricated quote must never have made it into the DB at all.
	observations := sqlite.NewObservationRepository(db)
	allObs, err := observations.ListByProject(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListByProject observations: %v", err)
	}
	for _, o := range allObs {
		if strings.Contains(o.Quote, "絶対に安全") {
			t.Errorf("fabricated quote leaked into persisted observations: %q", o.Quote)
		}
	}

	if len(progressLog) == 0 || progressLog[len(progressLog)-1] != "completed:100" {
		t.Errorf("progress log did not end with completed:100: %v", progressLog)
	}
}

func TestPipelineRunNoDocumentsFails(t *testing.T) {
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "empty.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	projects := sqlite.NewProjectRepository(db)
	ctx := context.Background()
	p := &domain.Project{ID: "proj_empty", Name: "空", CreatedAt: time.Now().UTC()}
	if err := projects.Create(ctx, p); err != nil {
		t.Fatalf("create project: %v", err)
	}

	pipeline := &Pipeline{
		Documents:    sqlite.NewDocumentRepository(db),
		Observations: sqlite.NewObservationRepository(db),
		Patterns:     sqlite.NewPatternRepository(db),
		Insights:     sqlite.NewInsightRepository(db),
		Evidence:     sqlite.NewEvidenceRepository(db),
		LLM:          newFakeLLM(),
	}
	if _, err := pipeline.Run(ctx, "ana_empty", p.ID, nil); err == nil {
		t.Fatal("expected an error for a project with no documents")
	}
}
