package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"insight-lab/internal/buildinfo"
	"insight-lab/internal/domain"
	"insight-lab/internal/llm"
)

var profileTestProtocol = promptProtocol{Fallback: llm.JSONObjectFallbackInstruction, Retry: llm.SchemaRetryTemplate}

// GENERAL_RESEARCH must be byte-for-byte the #107/#108 prompt set so existing
// runs keep their prompt fingerprint and behavior.
func TestGeneralResearchPromptSetIsTheUnchangedDomainNeutralBaseline(t *testing.T) {
	for _, profile := range []domain.ReasoningProfile{"", domain.ReasoningProfileGeneralResearch} {
		steps := pipelineLLMStepsFor(profile)
		base := pipelineLLMSteps()
		for i := range base {
			if steps[i].SystemPrompt != base[i].SystemPrompt || steps[i].Name != base[i].Name {
				t.Fatalf("profile %q changed stage %s", profile, base[i].Name)
			}
		}
		if mustPromptFingerprint(t, steps, profileTestProtocol) != mustPromptFingerprint(t, base, profileTestProtocol) {
			t.Fatalf("profile %q changed the prompt fingerprint", profile)
		}
	}
}

// CUSTOMER_INSIGHT extends the shared invariants; it never replaces them with
// a separate customer-specific base prompt.
func TestCustomerInsightExtendsSharedInvariantsInsteadOfReplacingThem(t *testing.T) {
	general := pipelineLLMSteps()
	customer := pipelineLLMStepsFor(domain.ReasoningProfileCustomerInsight)
	if len(customer) != len(general) {
		t.Fatalf("customer profile must reuse the same %d stages, got %d", len(general), len(customer))
	}
	for i := range general {
		if customer[i].Name != general[i].Name || customer[i].Temperature != general[i].Temperature {
			t.Fatalf("stage identity changed for %s", general[i].Name)
		}
		if !strings.HasPrefix(customer[i].SystemPrompt, general[i].SystemPrompt) {
			t.Fatalf("stage %s dropped the shared grounding/evidence invariants", general[i].Name)
		}
		if !strings.Contains(customer[i].SystemPrompt, customerInsightMarker) {
			t.Fatalf("stage %s does not carry the explicit profile marker", general[i].Name)
		}
		if strings.Contains(general[i].SystemPrompt, customerInsightMarker) {
			t.Fatalf("generic stage %s leaks the customer profile", general[i].Name)
		}
	}
	hyp := strings.ToLower(customer[3].SystemPrompt)
	for _, want := range []string{"stated need", "latent need", "jobs to be done", "alternative", "not proof"} {
		if !strings.Contains(hyp, want) {
			t.Fatalf("customer hypothesis stage missing %q", want)
		}
	}
	if mustPromptFingerprint(t, customer, profileTestProtocol) == mustPromptFingerprint(t, general, profileTestProtocol) {
		t.Fatal("profiles must have distinct prompt fingerprints so runs are reproducible")
	}
}

func TestExecutionSnapshotRecordsProfileAndOnlyCustomerChangesFingerprint(t *testing.T) {
	at := time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)
	settings := Settings{BaseURL: "https://llm.example/v1", Model: "m"}
	legacy, err := BuildExecutionSnapshot(settings, "", buildinfo.Info{}, at)
	if err != nil {
		t.Fatal(err)
	}
	general, _ := BuildExecutionSnapshotFor(settings, "", "", buildinfo.Info{}, at)
	customer, _ := BuildExecutionSnapshotFor(settings, "", domain.ReasoningProfileCustomerInsight, buildinfo.Info{}, at)
	// GENERAL_RESEARCH adds nothing to the fingerprinted config, so the
	// profile-aware builder equals the builder callers used before #109.
	if general.ExecutionFingerprint != legacy.ExecutionFingerprint {
		t.Fatal("GENERAL_RESEARCH must not add anything to the execution fingerprint")
	}
	if customer.ExecutionFingerprint == general.ExecutionFingerprint {
		t.Fatal("changing the reasoning profile must change the execution fingerprint")
	}
	if general.ReasoningProfileResolution == nil || general.ReasoningProfileResolution.Requested != "" || general.ReasoningProfileResolution.Resolved != domain.ReasoningProfileGeneralResearch {
		t.Fatalf("resolution not recorded: %+v", general.ReasoningProfileResolution)
	}
	if customer.ReasoningProfileResolution.Resolved != domain.ReasoningProfileCustomerInsight || customer.ExecutionConfig.ReasoningProfile != domain.ReasoningProfileCustomerInsight {
		t.Fatalf("customer profile not part of the fingerprinted config: %+v", customer.ExecutionConfig)
	}
}

func TestCustomerSpecificQualityChecksApplyOnlyUnderCustomerInsight(t *testing.T) {
	in := QualityInput{
		StatedNeed: "lower price", LatentNeed: "lower price",
		Expectation: "baseline", SurprisingFact: "changed", Patterns: []*domain.Pattern{{Kind: domain.PatternDeviation}},
	}
	has := func(flags []domain.QualityFlag, code domain.QualityFlagCode) bool {
		for _, f := range flags {
			if f.Code == code {
				return true
			}
		}
		return false
	}
	if flags := AssessQuality(in); has(flags, domain.QualityStatedNeedEcho) {
		t.Fatalf("GENERAL_RESEARCH was penalized by a customer-needs rule: %+v", flags)
	}
	in.Profile = domain.ReasoningProfileCustomerInsight
	if flags := AssessQuality(in); !has(flags, domain.QualityStatedNeedEcho) {
		t.Fatalf("CUSTOMER_INSIGHT must keep the stated-need echo check: %+v", flags)
	}
}

func TestEveryStageReceivesTheSelectedProfileExtension(t *testing.T) {
	capture := &promptCaptureClient{}
	p := &Pipeline{LLM: capture, ReasoningProfile: domain.ReasoningProfileCustomerInsight}
	for _, step := range pipelineLLMSteps() {
		if _, err := p.generate(context.Background(), step, nil); err != nil {
			t.Fatal(err)
		}
	}
	for i, prompt := range capture.prompts {
		if !strings.Contains(prompt, customerInsightMarker) {
			t.Fatalf("stage %s ran without the selected profile", pipelineLLMSteps()[i].Name)
		}
	}
}

func TestCommercialProjectionsAreKeptOnlyUnderCustomerInsight(t *testing.T) {
	w := insightWriteup{ProductOpportunity: "a template", MonetizationAngle: "consulting"}
	if got := projectWriteup(w, domain.ReasoningProfileGeneralResearch); got.ProductOpportunity != "" || got.MonetizationAngle != "" {
		t.Fatalf("GENERAL_RESEARCH must leave commercial projections empty: %+v", got)
	}
	if got := projectWriteup(w, domain.ReasoningProfileCustomerInsight); got.ProductOpportunity != "a template" || got.MonetizationAngle != "consulting" {
		t.Fatalf("CUSTOMER_INSIGHT keeps the optional projections: %+v", got)
	}
}

// Same evidence under two profiles: the input stays SAME and the profile is
// attributed as an execution (instrument) change with its own field.
func TestCompareAttributesProfileChangeToExecutionWithSameInput(t *testing.T) {
	settings := Settings{BaseURL: "https://llm.example/v1", Model: "m"}
	analysis := func(id string, profile domain.ReasoningProfile) *domain.Analysis {
		snap, err := BuildExecutionSnapshotFor(settings, "", profile, buildinfo.Info{}, time.Time{})
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := json.Marshal(snap)
		a := &domain.Analysis{ID: id, Status: domain.AnalysisCompleted, ExecutionSnapshot: string(raw), ExecutionFingerprint: snap.ExecutionFingerprint}
		a.InputSnapshot, a.InputFingerprint = inputSnapshot(t, "same evidence")
		return a
	}
	a, b := analysis("a", ""), analysis("b", domain.ReasoningProfileCustomerInsight)
	if AnalysisReasoningProfile(a) != domain.ReasoningProfileGeneralResearch || AnalysisReasoningProfile(b) != domain.ReasoningProfileCustomerInsight {
		t.Fatalf("recorded profiles = %s, %s", AnalysisReasoningProfile(a), AnalysisReasoningProfile(b))
	}
	c := CompareAnalysisRuns(RunComparisonInput{Analysis: a}, RunComparisonInput{Analysis: b}, nil)
	if c.Input.State != AxisSame || c.Attribution != AttributionExecutionChange {
		t.Fatalf("input %s attribution %s", c.Input.State, c.Attribution)
	}
	var profileChange *FieldChange
	for i := range c.Execution.Changes {
		if c.Execution.Changes[i].Field == "reasoningProfile" {
			profileChange = &c.Execution.Changes[i]
		}
	}
	want := FieldChange{Field: "reasoningProfile", From: "GENERAL_RESEARCH", To: "CUSTOMER_INSIGHT"}
	if profileChange == nil || *profileChange != want {
		t.Fatalf("execution changes = %+v, want %+v", c.Execution.Changes, want)
	}
}
