package service

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"insight-lab/internal/buildinfo"
	"insight-lab/internal/domain"
	"insight-lab/internal/llm"
)

var testBuild = buildinfo.Info{Version: "v0.9.0", Commit: "abc123", Dirty: "false"}

func modelSettings() Settings {
	return Settings{BaseURL: "https://api.example.com/v1", Model: "model-a", APIKey: "sk-secret-value"}
}

func mustExecution(t *testing.T, s Settings, build buildinfo.Info, at time.Time) ExecutionSnapshot {
	t.Helper()
	snapshot, err := BuildExecutionSnapshot(s, "", build, at)
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func TestExecutionFingerprintIsStableForTheSameConfigurationCapturedAtDifferentTimes(t *testing.T) {
	a := mustExecution(t, modelSettings(), testBuild, time.Unix(1, 0))
	b := mustExecution(t, modelSettings(), testBuild, time.Unix(99, 0))
	if a.ExecutionFingerprint == "" || a.ExecutionFingerprint != b.ExecutionFingerprint {
		t.Fatalf("fingerprints %q and %q must match", a.ExecutionFingerprint, b.ExecutionFingerprint)
	}
}

func TestExecutionFingerprintChangesWithModelHostCommitAndSemanticMode(t *testing.T) {
	base := mustExecution(t, modelSettings(), testBuild, time.Unix(1, 0))
	otherModel := modelSettings()
	otherModel.Model = "model-b"
	otherHost := modelSettings()
	otherHost.BaseURL = "https://other.example.com/v1"
	otherCommit := testBuild
	otherCommit.Commit = "def456"
	semantic, err := BuildExecutionSnapshot(modelSettings(), domain.AnalysisModeDatasetAnalysis, testBuild, time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	for name, changed := range map[string]ExecutionSnapshot{
		"model":         mustExecution(t, otherModel, testBuild, time.Unix(1, 0)),
		"provider host": mustExecution(t, otherHost, testBuild, time.Unix(1, 0)),
		"git commit":    mustExecution(t, modelSettings(), otherCommit, time.Unix(1, 0)),
		"semantic mode": semantic,
	} {
		if changed.ExecutionFingerprint == base.ExecutionFingerprint {
			t.Errorf("changing the %s must change the execution fingerprint", name)
		}
	}
}

func TestExecutionFingerprintIgnoresTheAPIKey(t *testing.T) {
	rotated := modelSettings()
	rotated.APIKey = "sk-rotated"
	if mustExecution(t, rotated, testBuild, time.Unix(1, 0)).ExecutionFingerprint != mustExecution(t, modelSettings(), testBuild, time.Unix(1, 0)).ExecutionFingerprint {
		t.Fatal("a credential is not part of the instrument and must not change the fingerprint")
	}
}

func TestExecutionSnapshotNeverContainsCredentialsOrTheFullProviderURL(t *testing.T) {
	s := Settings{BaseURL: "https://user:hunter2@api.example.com:8443/v1/secret-path?api_key=querysecret", Model: "model-a", APIKey: "sk-secret-value"}
	snapshot := mustExecution(t, s, testBuild, time.Unix(1, 0))
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"sk-secret-value", "hunter2", "user:", "querysecret", "api_key", "secret-path"} {
		if strings.Contains(string(encoded), secret) {
			t.Errorf("snapshot leaks %q: %s", secret, encoded)
		}
	}
	if snapshot.LLM == nil || snapshot.LLM.ProviderHost != "api.example.com:8443" {
		t.Fatalf("provider host = %+v, want api.example.com:8443", snapshot.LLM)
	}
}

func TestDeterministicExecutionSnapshotRecordsNoModel(t *testing.T) {
	snapshot := mustExecution(t, Settings{}, testBuild, time.Unix(1, 0))
	want := ExecutionConfig{
		EngineVersion: "v0.9.0", GitCommit: "abc123", GitDirty: "false",
		ExecutionMode: ExecutionModeDeterministic,
		RuleVersions:  map[string]string{"datasetPreanalysis": datasetPreAnalysisRuleVersion, "analyticalArtifact": analyticalArtifactRuleVersion, "grounding": groundingRuleVersion},
	}
	if got := snapshot.ExecutionConfig; !equalJSON(t, got, want) {
		t.Fatalf("deterministic snapshot = %+v, want %+v", got, want)
	}
}

func TestPromptFingerprintV2DetectsSchemaTemperatureFallbackAndRetryChanges(t *testing.T) {
	protocol := promptProtocol{Fallback: llm.JSONObjectFallbackInstruction, Retry: llm.SchemaRetryTemplate}
	base := mustPromptFingerprint(t, pipelineLLMSteps(), protocol)

	warmer := pipelineLLMSteps()
	warmer[0].Temperature += 0.1
	otherSchema := pipelineLLMSteps()
	otherSchema[1].Schema = func() llm.Schema { return llm.Schema{Name: "changed", Schema: map[string]any{"type": "object"}} }
	otherFallback := protocol
	otherFallback.Fallback += " "
	otherRetry := protocol
	otherRetry.Retry += " "

	for name, got := range map[string]string{
		"temperature": mustPromptFingerprint(t, warmer, protocol),
		"schema":      mustPromptFingerprint(t, otherSchema, protocol),
		"fallback":    mustPromptFingerprint(t, pipelineLLMSteps(), otherFallback),
		"retry":       mustPromptFingerprint(t, pipelineLLMSteps(), otherRetry),
	} {
		if got == base {
			t.Errorf("a %s change must change prompt fingerprint v2", name)
		}
	}
}

func mustPromptFingerprint(t *testing.T, steps []llmStep, protocol promptProtocol) string {
	t.Helper()
	fp, err := promptFingerprintV2(steps, protocol)
	if err != nil {
		t.Fatal(err)
	}
	return fp
}

func TestModelBackedSnapshotKeepsTheLegacyPromptFingerprint(t *testing.T) {
	snapshot := mustExecution(t, modelSettings(), testBuild, time.Unix(1, 0))
	if snapshot.PromptFingerprintLegacy != promptFingerprint() || snapshot.PromptFingerprint == snapshot.PromptFingerprintLegacy {
		t.Fatalf("legacy = %q, v2 = %q", snapshot.PromptFingerprintLegacy, snapshot.PromptFingerprint)
	}
}

func inputDocs() []*domain.Document {
	return []*domain.Document{
		{ID: "doc_1", Source: domain.SourceInterview, Content: "first", Metadata: map[string]string{"k": "v"}},
		{ID: "doc_2", Source: domain.SourceSurvey, Content: "second"},
	}
}

func TestInputFingerprintDependsOnContentNotDocumentIDsOrOrder(t *testing.T) {
	base := BuildInputSnapshot(inputDocs(), time.Unix(1, 0))
	reordered := inputDocs()
	reordered[0], reordered[1] = reordered[1], reordered[0]
	reordered[0].ID, reordered[1].ID = "doc_x", "doc_y"
	if got := BuildInputSnapshot(reordered, time.Unix(2, 0)); got.InputFingerprint != base.InputFingerprint || got.DocumentSetHash != base.DocumentSetHash {
		t.Fatal("identical content under different IDs or order must keep the input fingerprint")
	}

	contentChanged := inputDocs()
	contentChanged[1].Content = "second, edited"
	metadataChanged := inputDocs()
	metadataChanged[0].Metadata["k"] = "w"
	added := append(inputDocs(), &domain.Document{ID: "doc_3", Source: domain.SourceReview, Content: "third"})
	for name, docs := range map[string][]*domain.Document{"content": contentChanged, "metadata": metadataChanged, "document set": added} {
		if BuildInputSnapshot(docs, time.Unix(1, 0)).InputFingerprint == base.InputFingerprint {
			t.Errorf("a %s change must change the input fingerprint", name)
		}
	}
	if base.DocumentCount != 2 || len(base.Documents) != 2 || base.Documents[0].ID == "" || !strings.HasPrefix(base.Documents[0].ContentHash, "sha256:") {
		t.Fatalf("input snapshot is missing document entries: %+v", base)
	}
}

func equalJSON(t *testing.T, a, b any) bool {
	t.Helper()
	x, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	y, err := json.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	return string(x) == string(y)
}
