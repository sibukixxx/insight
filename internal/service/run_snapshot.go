package service

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"insight-lab/internal/buildinfo"
	"insight-lab/internal/domain"
	"insight-lab/internal/llm"
)

// promptVersion is the human-readable label for the current prompt set.
// Bump it together with any change to prompts.go, the step schemas or
// temperatures; the prompt fingerprint catches changes nobody labelled.
const promptVersion = "prompts/v1"

// openAICompatibleProvider names the only client implementation. The
// endpoint host distinguishes actual providers.
const openAICompatibleProvider = "openai_compatible"

// ExecutionConfig describes the instrument an analysis run used: engine
// build, deterministic rule versions, prompts, model and client parameters.
// Every field is part of the execution fingerprint. It never holds an API
// key, a full endpoint URL or a query string.
type ExecutionConfig struct {
	EngineVersion        string              `json:"engineVersion"`
	GitCommit            string              `json:"gitCommit"`
	GitDirty             string              `json:"gitDirty"`
	ExecutionMode        ExecutionMode       `json:"executionMode"`
	SemanticAnalysisMode domain.AnalysisMode `json:"semanticAnalysisMode,omitempty"`
	RuleVersions         map[string]string   `json:"ruleVersions"`
	PromptVersion        string              `json:"promptVersion,omitempty"`
	// PromptFingerprint (v2) covers prompts, response schemas, temperatures
	// and the client's fallback and retry wording.
	PromptFingerprint string `json:"promptFingerprint,omitempty"`
	// PromptFingerprintLegacy is the v1 fingerprint over prompts only, kept
	// so runs recorded before v2 can still be matched.
	PromptFingerprintLegacy string        `json:"promptFingerprintLegacy,omitempty"`
	LLM                     *LLMExecution `json:"llm,omitempty"`
}

// LLMExecution is the model side of a model-backed run.
type LLMExecution struct {
	ProviderHost string         `json:"providerHost"`
	Models       []ModelBinding `json:"models"`
	Parameters   LLMParameters  `json:"parameters"`
}

// ModelBinding records which model served which pipeline stage. Today every
// stage uses one model ("all"); the list shape allows per-stage routing.
type ModelBinding struct {
	Stage    string `json:"stage"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

type LLMParameters struct {
	Temperatures     map[string]float64 `json:"temperatures"`
	ContextRuneLimit int                `json:"contextRuneLimit"`
	MaxChunkRunes    int                `json:"maxChunkRunes"`
	Client           llm.Policy         `json:"client"`
}

// ExecutionSnapshot is the ExecutionConfig captured when a run was enqueued,
// plus its fingerprint and audit fields that are not part of it.
type ExecutionSnapshot struct {
	ExecutionConfig
	ExecutionFingerprint string    `json:"executionFingerprint"`
	CapturedAt           time.Time `json:"capturedAt"`
	// SettingsChangedBeforeStart is true when the live settings differed from
	// this snapshot when the run started. The run still executed with the
	// snapshot's configuration.
	SettingsChangedBeforeStart bool `json:"settingsChangedBeforeStart,omitempty"`
}

// BuildExecutionSnapshot captures the configuration a run will execute with.
// A run without a configured model is deterministic and records no model.
func BuildExecutionSnapshot(settings Settings, semantic domain.AnalysisMode, build buildinfo.Info, at time.Time) (ExecutionSnapshot, error) {
	config := ExecutionConfig{
		EngineVersion: build.Version, GitCommit: build.Commit, GitDirty: build.Dirty,
		ExecutionMode: ExecutionModeDeterministic, SemanticAnalysisMode: semantic,
		RuleVersions: map[string]string{
			"datasetPreanalysis": datasetPreAnalysisRuleVersion, "analyticalArtifact": analyticalArtifactRuleVersion, "grounding": groundingRuleVersion,
		},
	}
	if settings.Configured() {
		promptFP, err := promptFingerprintV2(pipelineLLMSteps(), promptProtocol{Fallback: llm.JSONObjectFallbackInstruction, Retry: llm.SchemaRetryTemplate})
		if err != nil {
			return ExecutionSnapshot{}, err
		}
		config.ExecutionMode = ExecutionModeModelBacked
		config.RuleVersions["confidence"] = confidenceRuleVersion
		config.RuleVersions["quality"] = qualityRuleVersion
		config.RuleVersions["contextBudget"] = contextBudgetRuleVersion
		config.PromptVersion = promptVersion
		config.PromptFingerprint = promptFP
		config.PromptFingerprintLegacy = promptFingerprint()
		temperatures := map[string]float64{}
		for _, step := range pipelineLLMSteps() {
			temperatures[step.Name] = step.Temperature
		}
		config.LLM = &LLMExecution{
			ProviderHost: providerHost(settings.BaseURL),
			Models:       []ModelBinding{{Stage: "all", Provider: openAICompatibleProvider, Model: settings.Model}},
			Parameters: LLMParameters{
				Temperatures: temperatures, ContextRuneLimit: defaultContextRuneLimit, MaxChunkRunes: maxChunkRunes,
				Client: llm.OpenAIClientPolicy(),
			},
		}
	}
	fp, err := Fingerprint(config)
	if err != nil {
		return ExecutionSnapshot{}, err
	}
	return ExecutionSnapshot{ExecutionConfig: config, ExecutionFingerprint: fp, CapturedAt: at.UTC()}, nil
}

// providerHost keeps only host[:port] of the endpoint. User info, path and
// query are dropped because providers may carry credentials there.
func providerHost(baseURL string) string {
	u, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || u.Host == "" {
		return buildinfo.Unknown
	}
	return strings.ToLower(u.Host)
}

// promptProtocol is the client wording that reaches the model besides the
// step prompts.
type promptProtocol struct {
	Fallback string `json:"fallback"`
	Retry    string `json:"retry"`
}

// promptFingerprintV2 hashes everything that shapes what the model is told:
// each step's prompt, response schema and temperature, and the protocol
// wording used on fallback and retry.
func promptFingerprintV2(steps []llmStep, protocol promptProtocol) (string, error) {
	type stepIdentity struct {
		Name         string         `json:"name"`
		SystemPrompt string         `json:"systemPrompt"`
		SchemaName   string         `json:"schemaName"`
		Schema       map[string]any `json:"schema"`
		Temperature  float64        `json:"temperature"`
	}
	identities := make([]stepIdentity, 0, len(steps))
	for _, step := range steps {
		schema := step.Schema()
		identities = append(identities, stepIdentity{Name: step.Name, SystemPrompt: step.SystemPrompt, SchemaName: schema.Name, Schema: schema.Schema, Temperature: step.Temperature})
	}
	return Fingerprint(struct {
		Steps    []stepIdentity `json:"steps"`
		Protocol promptProtocol `json:"protocol"`
	}{identities, protocol})
}

// InputDocument identifies one document by content. ID is kept for tracing
// but is not part of any fingerprint.
type InputDocument struct {
	ID           string            `json:"id"`
	Source       domain.SourceType `json:"source"`
	ContentHash  string            `json:"contentHash"`
	MetadataHash string            `json:"metadataHash"`
}

// InputSnapshot is the evidence an analysis run read, captured when it
// started.
type InputSnapshot struct {
	Documents             []InputDocument               `json:"documents"`
	DocumentCount         int                           `json:"documentCount"`
	DocumentSetHash       string                        `json:"documentSetHash"`
	DatasetHashes         []string                      `json:"datasetHashes,omitempty"`
	Datasets              []DatasetProvenance           `json:"datasets,omitempty"`
	CompatibilityWarnings []DatasetCompatibilityWarning `json:"compatibilityWarnings,omitempty"`
	InputFingerprint      string                        `json:"inputFingerprint"`
	CapturedAt            time.Time                     `json:"capturedAt"`
}

type documentIdentity struct {
	Source       domain.SourceType `json:"source"`
	ContentHash  string            `json:"contentHash"`
	MetadataHash string            `json:"metadataHash"`
}

// BuildInputSnapshot fingerprints docs by content and metadata. The same
// evidence under different document IDs, or in a different order, yields
// the same fingerprint. Compatibility warnings are derived from the inputs
// and rules, so they are recorded but not fingerprinted.
func BuildInputSnapshot(docs []*domain.Document, at time.Time) InputSnapshot {
	pre := RunDatasetPreAnalysis(docs, at)
	snapshot := InputSnapshot{
		DocumentCount: len(docs), DatasetHashes: pre.DatasetHashes, Datasets: datasetProvenances(pre),
		CompatibilityWarnings: pre.CompatibilityWarnings, CapturedAt: at.UTC(),
	}
	identities := make([]documentIdentity, 0, len(docs))
	for _, d := range docs {
		metadataHash, _ := Fingerprint(d.Metadata) // a map[string]string always marshals
		sum := sha256.Sum256([]byte(d.Content))
		entry := InputDocument{ID: d.ID, Source: d.Source, ContentHash: "sha256:" + hex.EncodeToString(sum[:]), MetadataHash: metadataHash}
		snapshot.Documents = append(snapshot.Documents, entry)
		identities = append(identities, documentIdentity{entry.Source, entry.ContentHash, entry.MetadataHash})
	}
	sort.Slice(identities, func(i, j int) bool {
		return fmt.Sprint(identities[i]) < fmt.Sprint(identities[j])
	})
	snapshot.DocumentSetHash, _ = Fingerprint(identities)
	snapshot.InputFingerprint, _ = Fingerprint(struct {
		Documents     []documentIdentity  `json:"documents"`
		DatasetHashes []string            `json:"datasetHashes"`
		Datasets      []DatasetProvenance `json:"datasets"`
	}{identities, pre.DatasetHashes, snapshot.Datasets})
	return snapshot
}

func datasetProvenances(pre DatasetPreAnalysis) []DatasetProvenance {
	var out []DatasetProvenance
	for _, m := range pre.Manifests {
		out = append(out, DatasetProvenance{
			SourceName: m.SourceName, DatasetID: m.DatasetID, RetrievalMethod: m.RetrievalMethod, RetrievedAt: m.RetrievedAt,
			SchemaID: m.SchemaID, SchemaVersion: m.SchemaVersion, RecipeRef: m.RecipeRef, FileHash: m.FileHash,
		})
	}
	return out
}
