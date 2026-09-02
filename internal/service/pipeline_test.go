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

// repetitionOnly returns the JSON array of pattern IDs that are not
// traces, so the low-quality hypothesis cites repetition alone.
func repetitionOnly(all, traces []string) string {
	isTrace := map[string]bool{}
	for _, id := range traces {
		isTrace[id] = true
	}
	var out []string
	for _, id := range all {
		if !isTrace[id] {
			out = append(out, id)
		}
	}
	b, _ := json.Marshal(out)
	return string(b)
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
		case strings.Contains(last, "営業メモ"):
			raw = json.RawMessage(`{"observations":[{"quote":"先方は検算をやめたくないと言っていた","behavior":"検算への固執（営業の記録）","topic":"anxiety"}]}`)
		case strings.Contains(last, "【質問者】"):
			// The transcript document: the model must only see role
			// labels on the analysis text, and even if it quotes the
			// interviewer verbatim, that quote must be rejected.
			if !strings.Contains(last, "【回答者】") {
				return nil, fmt.Errorf("fakeLLM: transcript analysis text lacks the customer label: %q", last)
			}
			raw = json.RawMessage(`{"observations":[
				{"quote":"検算はどのくらい時間がかかりますか","behavior":"interviewer question - must be rejected","topic":"x"},
				{"quote":"毎回三十分は検算に使っています","behavior":"検算に時間をかけている","topic":"anxiety"}
			]}`)
		default:
			raw = json.RawMessage(`{"observations":[]}`)
		}

	case "trace_detection":
		var p obsRefPayload
		_ = json.Unmarshal([]byte(last), &p)
		// The speaker's situation (from reserved metadata) must reach the
		// step that forms expectations, and the secondhand sales note must
		// be marked as such.
		var sawSituation, sawSecondhand bool
		for _, o := range p.Observations {
			if o.Situation == "経理担当 / 30名" {
				sawSituation = true
			}
			if o.Provenance == "secondhand" {
				sawSecondhand = true
			}
			if o.DocumentID == "" {
				return nil, fmt.Errorf("fakeLLM: observation %s has no documentId", o.ID)
			}
		}
		if !sawSituation || !sawSecondhand {
			return nil, fmt.Errorf("fakeLLM: trace detection payload lacks situation (%v) or secondhand provenance (%v)", sawSituation, sawSecondhand)
		}
		// The trace cites the anxiety observation plus one fabricated ID,
		// which buildTracePatterns must drop without dropping the trace.
		ids := idsWhere(p.Observations, func(o observationRef) bool { return o.Topic == "anxiety" })
		ids = append(ids, "obs_does_not_exist")
		idsJSON, _ := json.Marshal(ids)
		raw = json.RawMessage(fmt.Sprintf(`{"traces":[
			{"title":"急いでいるのに手動で確認する","expectation":"忙しいなら自動計算を信じて送るはず","actualBehavior":"最後は自分で確認している","deviationType":"excess_effort","observationIds":%s},
			{"title":"根拠のない痕跡","expectation":"x","actualBehavior":"y","deviationType":"other","observationIds":["obs_does_not_exist"]}
		]}`, idsJSON))

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
		var patternIDs, traceIDs []string
		for _, pr := range pp.Patterns {
			patternIDs = append(patternIDs, pr.ID)
			if pr.Kind == "deviation" {
				traceIDs = append(traceIDs, pr.ID)
			}
		}
		if len(traceIDs) == 0 {
			return nil, fmt.Errorf("fakeLLM: hypothesis step received no deviation pattern (traces must be passed to hypothesis generation)")
		}
		traceIDsJSON, _ := json.Marshal(traceIDs)

		// Two hypotheses: one proper abduction anchored to the trace, and
		// one "poor-quality insight" that restates the stated need with a
		// generic term and cites only the repetition pattern - the quality
		// gate, not the model, is expected to flag it.
		raw = json.RawMessage(fmt.Sprintf(`{"hypotheses":[{
			"title":"確認への依存の裏にある恐怖",
			"statedNeed":"作業を早く終わらせたい",
			"latentNeed":"失敗して信頼を失うことを避けたい",
			"jtbd":"間違いのない請求書を送れている状態でいたい",
			"expectation":"忙しいなら自動計算を信じて送るはず",
			"surprisingFact":"時間がかかると言いながら最後は自分で確認している",
			"rationale":"失敗による信頼喪失を避けたい欲求があるなら、時間をかけてでも確認する行動は当然になる",
			"supportingObservationIds":%s,
			"basedOnPatternIds":%s
		},{
			"title":"安心して使いたい",
			"statedNeed":"安心して使いたい",
			"latentNeed":"安心して使いたい",
			"jtbd":"安心",
			"expectation":"",
			"surprisingFact":"",
			"rationale":"",
			"supportingObservationIds":%s,
			"basedOnPatternIds":%s
		}]}`, obsIDsJSON, traceIDsJSON, obsIDsJSON, repetitionOnly(patternIDs, traceIDs)))

	case "evidence_retrieval":
		var p obsRefPayload
		_ = json.Unmarshal([]byte(last), &p)
		support := idsWhere(p.Observations, func(o observationRef) bool { return o.Topic == "anxiety" })
		counter := idsWhere(p.Observations, func(o observationRef) bool { return o.Topic == "ease" })
		supportJSON, _ := json.Marshal(support)
		counterJSON, _ := json.Marshal(counter)
		raw = json.RawMessage(fmt.Sprintf(`{"supportingObservationIds":%s,"counterObservationIds":%s,"counterSearched":true}`, supportJSON, counterJSON))

	case "insight_writeup":
		if strings.Contains(last, "安心して使いたい") {
			raw = json.RawMessage(`{
				"title":"安心して使いたい",
				"observationSummary":"確認している",
				"interpretation":"安心を求めている",
				"alternativeInterpretation":"習慣かもしれない",
				"productOpportunity":"",
				"monetizationAngle":""
			}`)
			break
		}
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

// transcriptDoc is an interview with speaker attribution: the
// interviewer's question is context only, the answer is the customer's
// voice.
func transcriptDoc(projectID string) *domain.Document {
	q := "面接官: 検算はどのくらい時間がかかりますか？"
	a := "回答者: 毎回三十分は検算に使っています。"
	content := q + "\n" + a
	qLen := len([]rune(q))
	return &domain.Document{
		ID: "doc_3", ProjectID: projectID, Source: domain.SourceInterview, Title: "Interview #2 (transcript)",
		Content: content,
		Spans: []domain.Span{
			{Start: 0, End: qLen, Speaker: "面接官", Role: domain.RoleInterviewer},
			{Start: qLen + 1, End: len([]rune(content)), Speaker: "回答者", Role: domain.RoleCustomer},
		},
		Metadata:  map[string]string{domain.MetaRole: "経理担当", domain.MetaCompanySize: "30名"},
		CreatedAt: time.Now().UTC(),
	}
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
		transcriptDoc(p.ID),
		{ID: "doc_4", ProjectID: p.ID, Source: domain.SourceSales, Title: "営業メモ",
			Content: "営業メモ: 先方は検算をやめたくないと言っていた。", CreatedAt: time.Now().UTC()},
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

	// Five candidates: one fabricated (discarded by grounding), one a
	// verbatim quote of the interviewer (discarded as outside customer
	// speech), three genuine.
	if metrics.TotalObservationCandidates != 6 {
		t.Errorf("TotalObservationCandidates = %d, want 6", metrics.TotalObservationCandidates)
	}
	if metrics.GroundedObservations != 4 {
		t.Errorf("GroundedObservations = %d, want 4", metrics.GroundedObservations)
	}
	if metrics.QuotesOutsideCustomerSpeech != 1 {
		t.Errorf("QuotesOutsideCustomerSpeech = %d, want 1 (the interviewer's question)", metrics.QuotesOutsideCustomerSpeech)
	}
	wantRate := 2.0 / 6.0
	if diff := metrics.UnsupportedClaimRate - wantRate; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("UnsupportedClaimRate = %f, want %f", metrics.UnsupportedClaimRate, wantRate)
	}

	if metrics.FinalInsightCount != 2 {
		t.Fatalf("FinalInsightCount = %d, want 2", metrics.FinalInsightCount)
	}
	if metrics.EvidenceCoverage != 1.0 {
		t.Errorf("EvidenceCoverage = %f, want 1.0", metrics.EvidenceCoverage)
	}
	if metrics.CounterEvidenceCoverage != 1.0 {
		t.Errorf("CounterEvidenceCoverage = %f, want 1.0", metrics.CounterEvidenceCoverage)
	}
	// 1 trace (the one citing only a fabricated ID is dropped) + 1 repetition.
	if metrics.PatternCount != 2 || metrics.TraceCount != 1 {
		t.Errorf("PatternCount = %d, TraceCount = %d, want 2 and 1", metrics.PatternCount, metrics.TraceCount)
	}
	if metrics.TraceBackedInsightRate != 0.5 {
		t.Errorf("TraceBackedInsightRate = %f, want 0.5 (one of two insights cites a trace)", metrics.TraceBackedInsightRate)
	}
	if metrics.QualityFlaggedInsightRate != 0.5 {
		t.Errorf("QualityFlaggedInsightRate = %f, want 0.5", metrics.QualityFlaggedInsightRate)
	}
	for _, code := range []domain.QualityFlagCode{domain.QualityStatedNeedEcho, domain.QualityGenericTerm, domain.QualityNoTrace, domain.QualityAbductionIncomplete} {
		if metrics.QualityFlagCounts[string(code)] != 1 {
			t.Errorf("QualityFlagCounts[%s] = %d, want 1", code, metrics.QualityFlagCounts[string(code)])
		}
	}

	insights := sqlite.NewInsightRepository(db)
	list, err := insights.ListByProject(ctx, project.ID)
	if err != nil || len(list) != 2 {
		t.Fatalf("ListByProject = %v, %v", list, err)
	}
	var insight, poor *domain.Insight
	for _, i := range list {
		if i.Title == "安心して使いたい" {
			poor = i
		} else {
			insight = i
		}
	}
	if insight == nil || poor == nil {
		t.Fatalf("expected both the proper and the poor-quality insight, got %+v", list)
	}

	// The poor-quality insight is kept (the researcher decides), but every
	// app-side check must have fired on it.
	for _, code := range []domain.QualityFlagCode{domain.QualityStatedNeedEcho, domain.QualityGenericTerm, domain.QualityNoTrace, domain.QualityAbductionIncomplete} {
		if !poor.HasQualityFlag(code) {
			t.Errorf("poor insight missing quality flag %s: %+v", code, poor.QualityFlags)
		}
	}
	if len(insight.QualityFlags) != 0 {
		t.Errorf("proper insight should carry no quality flags, got %+v", insight.QualityFlags)
	}
	if insight.Expectation == "" || insight.SurprisingFact == "" {
		t.Errorf("abduction fields should round-trip: expectation=%q surprisingFact=%q", insight.Expectation, insight.SurprisingFact)
	}
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
	if !pattern.IsTrace() {
		t.Errorf("the proper insight should cite the deviation pattern, got kind %q", pattern.Kind)
	}
	if pattern.Expectation == "" || pattern.DeviationType != domain.DeviationExcessEffort {
		t.Errorf("trace pattern lost its expectation/deviation type: %+v", pattern)
	}
	if len(pattern.ObservationIDs) != 3 {
		t.Errorf("trace has %d linked observations, want 3 (the anxiety quotes; the fabricated ID must be dropped)", len(pattern.ObservationIDs))
	}

	poorPatterns, err := patterns.ListByInsight(ctx, poor.ID)
	if err != nil || len(poorPatterns) != 1 || poorPatterns[0].Kind != domain.PatternRepetition {
		t.Errorf("poor insight should cite exactly the repetition pattern: %v, %v", poorPatterns, err)
	}
	if len(poorPatterns) == 1 && len(poorPatterns[0].ObservationIDs) != 4 {
		t.Errorf("repetition pattern has %d linked observations, want 4 (all grounded observations, not the fabricated one)", len(poorPatterns[0].ObservationIDs))
	}

	// Traces are listed before repetitions so the "what surprised us"
	// layer reads first.
	projectPatterns, err := patterns.ListByProject(ctx, project.ID)
	if err != nil || len(projectPatterns) != 2 {
		t.Fatalf("ListByProject patterns = %v, %v", projectPatterns, err)
	}
	if !projectPatterns[0].IsTrace() || projectPatterns[1].IsTrace() {
		t.Errorf("patterns should be ordered deviation first: %q, %q", projectPatterns[0].Kind, projectPatterns[1].Kind)
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
			switch e.Quote {
			case "設定を間違えたら怖いんですよね", "毎回三十分は検算に使っています":
				if e.RelevanceScore < 0.5 {
					t.Errorf("firsthand evidence relevance = %f", e.RelevanceScore)
				}
			case "先方は検算をやめたくないと言っていた":
				// Secondhand: discounted relative to the same rank firsthand.
				if e.RelevanceScore > 0.7 {
					t.Errorf("secondhand evidence should be discounted, got %f", e.RelevanceScore)
				}
			default:
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
	if support != 3 || counter != 1 {
		t.Errorf("support=%d counter=%d, want 3 and 1", support, counter)
	}
	if insight.HasQualityFlag(domain.QualitySecondhandOnly) {
		t.Error("insight with firsthand support must not be flagged secondhand_only")
	}

	// The fabricated quote must never have made it into the DB at all.
	observations := sqlite.NewObservationRepository(db)
	allObs, err := observations.ListByProject(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListByProject observations: %v", err)
	}
	var transcriptObs int
	for _, o := range allObs {
		if strings.Contains(o.Quote, "絶対に安全") {
			t.Errorf("fabricated quote leaked into persisted observations: %q", o.Quote)
		}
		if strings.Contains(o.Quote, "時間がかかりますか") {
			t.Errorf("interviewer's question leaked into persisted observations: %q", o.Quote)
		}
		if o.DocumentID == "doc_3" {
			transcriptObs++
			if o.Quote != "毎回三十分は検算に使っています" {
				t.Errorf("unexpected transcript quote %q", o.Quote)
			}
		}
	}
	if transcriptObs != 1 {
		t.Errorf("transcript document yielded %d observations, want 1", transcriptObs)
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
