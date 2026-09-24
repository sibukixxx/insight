package service

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"insight-lab/internal/domain"
)

// AxisState says whether one comparison axis (input or execution) differs
// between two analysis runs. UNKNOWN means at least one run did not record
// the axis; it is never read as SAME.
type AxisState string

const (
	AxisSame    AxisState = "SAME"
	AxisChanged AxisState = "CHANGED"
	AxisUnknown AxisState = "UNKNOWN"
)

// RunAttribution classifies which axis a result difference could be
// attributed to. It is a bookkeeping label, never a causal claim and never a
// quality judgement.
type RunAttribution string

const (
	AttributionSameConfiguration RunAttribution = "SAME_CONFIGURATION"
	AttributionExecutionChange   RunAttribution = "EXECUTION_CHANGE"
	AttributionInputChange       RunAttribution = "INPUT_CHANGE"
	AttributionConfounded        RunAttribution = "CONFOUNDED"
	AttributionUnavailable       RunAttribution = "ATTRIBUTION_UNAVAILABLE"
)

// RunComparisonNonCausalNote is always part of a comparison's explanation.
const RunComparisonNonCausalNote = "Differences between runs are not evidence of cause: a result delta records what changed, not why."

// RunComparison is a non-persistent value object comparing two analysis runs
// of one project (issue #83). It deliberately has no winner, score or
// better/worse field.
type RunComparison struct {
	From         RunRef            `json:"from"`
	To           RunRef            `json:"to"`
	Input        InputAxisDiff     `json:"input"`
	Execution    ExecutionAxisDiff `json:"execution"`
	Attribution  RunAttribution    `json:"attribution"`
	RepeatGroups []RepeatGroup     `json:"repeatGroups"`
	Metrics      []MetricDelta     `json:"metrics"`
	Insights     InsightResultDiff `json:"insights"`
	Explanation  []string          `json:"explanation"`
}

// RunRef identifies one compared run and what it recorded.
type RunRef struct {
	AnalysisID           string `json:"analysisId"`
	Status               string `json:"status"`
	CreatedAt            string `json:"createdAt"`
	InputFingerprint     string `json:"inputFingerprint,omitempty"`
	ExecutionFingerprint string `json:"executionFingerprint,omitempty"`
}

// FieldChange is one field-level difference. Empty From/To means the value
// was absent in that run.
type FieldChange struct {
	Field string `json:"field"`
	From  string `json:"from"`
	To    string `json:"to"`
}

type ExecutionAxisDiff struct {
	State   AxisState     `json:"state"`
	Changes []FieldChange `json:"changes"`
}

type InputAxisDiff struct {
	State                AxisState `json:"state"`
	DocumentsAdded       []string  `json:"documentsAdded"`
	DocumentsRemoved     []string  `json:"documentsRemoved"`
	DatasetHashesAdded   []string  `json:"datasetHashesAdded"`
	DatasetHashesRemoved []string  `json:"datasetHashesRemoved"`
	DatasetsAdded        []string  `json:"datasetsAdded"`
	DatasetsRemoved      []string  `json:"datasetsRemoved"`
}

// RunComparisonInput is one side of a comparison.
type RunComparisonInput struct {
	Analysis *domain.Analysis
	Insights []*domain.Insight // with Evidence populated
}

// CompareAnalysisRuns compares two runs. history is every run of the project
// and is used only to build repeat groups.
func CompareAnalysisRuns(from, to RunComparisonInput, history []*domain.Analysis) RunComparison {
	c := RunComparison{From: runRef(from.Analysis), To: runRef(to.Analysis)}
	c.Execution = compareExecution(from.Analysis, to.Analysis)
	c.Input = compareInput(from.Analysis, to.Analysis)
	c.Attribution = attribute(c.Input.State, c.Execution.State)
	c.Metrics = compareMetrics(from.Analysis.Metrics, to.Analysis.Metrics)
	c.Insights = matchInsights(from, to)
	c.RepeatGroups = repeatGroupsFor(history, from.Analysis, to.Analysis)
	c.Explanation = explainComparison(c)
	return c
}

func runRef(a *domain.Analysis) RunRef {
	return RunRef{AnalysisID: a.ID, Status: string(a.Status), CreatedAt: a.CreatedAt.UTC().Format(time.RFC3339Nano),
		InputFingerprint: a.InputFingerprint, ExecutionFingerprint: a.ExecutionFingerprint}
}

func axisFromFingerprints(a, b string) AxisState {
	switch {
	case a == "" || b == "":
		return AxisUnknown
	case a == b:
		return AxisSame
	}
	return AxisChanged
}

func attribute(input, execution AxisState) RunAttribution {
	switch {
	case input == AxisUnknown || execution == AxisUnknown:
		return AttributionUnavailable
	case input == AxisSame && execution == AxisSame:
		return AttributionSameConfiguration
	case input == AxisSame:
		return AttributionExecutionChange
	case execution == AxisSame:
		return AttributionInputChange
	}
	return AttributionConfounded
}

func compareExecution(a, b *domain.Analysis) ExecutionAxisDiff {
	out := ExecutionAxisDiff{State: axisFromFingerprints(a.ExecutionFingerprint, b.ExecutionFingerprint), Changes: []FieldChange{}}
	fa, okA := executionFields(a.ExecutionSnapshot)
	fb, okB := executionFields(b.ExecutionSnapshot)
	if !okA || !okB {
		out.State = AxisUnknown
		return out
	}
	out.Changes = diffFields(fa, fb)
	return out
}

// executionFields flattens the recorded instrument into comparable fields.
func executionFields(raw string) (map[string]string, bool) {
	var s ExecutionSnapshot
	if strings.TrimSpace(raw) == "" || json.Unmarshal([]byte(raw), &s) != nil {
		return nil, false
	}
	f := map[string]string{
		"engineVersion": s.EngineVersion, "gitCommit": s.GitCommit, "gitDirty": s.GitDirty,
		"executionMode": string(s.ExecutionMode), "semanticAnalysisMode": string(s.SemanticAnalysisMode),
		"promptVersion": s.PromptVersion, "promptFingerprint": s.PromptFingerprint,
	}
	for k, v := range s.RuleVersions {
		f["ruleVersions."+k] = v
	}
	if s.LLM != nil {
		f["provider"] = s.LLM.ProviderHost
		for _, m := range s.LLM.Models {
			f["models."+m.Stage] = m.Provider + "/" + m.Model
		}
		params, _ := json.Marshal(s.LLM.Parameters)
		f["parameters"] = string(params)
	}
	return f, true
}

func diffFields(a, b map[string]string) []FieldChange {
	keys := map[string]bool{}
	for k := range a {
		keys[k] = true
	}
	for k := range b {
		keys[k] = true
	}
	sorted := make([]string, 0, len(keys))
	for k := range keys {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)
	out := []FieldChange{}
	for _, k := range sorted {
		if a[k] != b[k] {
			out = append(out, FieldChange{Field: k, From: a[k], To: b[k]})
		}
	}
	return out
}

func compareInput(a, b *domain.Analysis) InputAxisDiff {
	out := InputAxisDiff{State: axisFromFingerprints(a.InputFingerprint, b.InputFingerprint),
		DocumentsAdded: []string{}, DocumentsRemoved: []string{}, DatasetHashesAdded: []string{},
		DatasetHashesRemoved: []string{}, DatasetsAdded: []string{}, DatasetsRemoved: []string{}}
	var sa, sb InputSnapshot
	if json.Unmarshal([]byte(a.InputSnapshot), &sa) != nil || json.Unmarshal([]byte(b.InputSnapshot), &sb) != nil {
		out.State = AxisUnknown
		return out
	}
	docKey := func(d InputDocument) string { return string(d.Source) + ":" + d.ContentHash + ":" + d.MetadataHash }
	out.DocumentsAdded, out.DocumentsRemoved = keyDiff(sa.Documents, sb.Documents, docKey)
	out.DatasetHashesAdded, out.DatasetHashesRemoved = keyDiff(sa.DatasetHashes, sb.DatasetHashes, func(s string) string { return s })
	dsKey := func(d DatasetProvenance) string {
		return fmt.Sprintf("%s@%s:%s", d.DatasetID, d.SchemaVersion, d.FileHash)
	}
	out.DatasetsAdded, out.DatasetsRemoved = keyDiff(sa.Datasets, sb.Datasets, dsKey)
	return out
}

func keyDiff[T any](from, to []T, key func(T) string) (added, removed []string) {
	in := func(list []T) map[string]bool {
		m := map[string]bool{}
		for _, v := range list {
			m[key(v)] = true
		}
		return m
	}
	fa, fb := in(from), in(to)
	added, removed = []string{}, []string{}
	for k := range fb {
		if !fa[k] {
			added = append(added, k)
		}
	}
	for k := range fa {
		if !fb[k] {
			removed = append(removed, k)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	return added, removed
}

func explainComparison(c RunComparison) []string {
	out := []string{RunComparisonNonCausalNote}
	switch c.Attribution {
	case AttributionSameConfiguration:
		out = append(out, "Input and execution are the same; remaining differences reflect run-to-run variability (e.g. model non-determinism).")
	case AttributionExecutionChange:
		out = append(out, "Only the execution (engine, rules, prompts, model or parameters) changed; the result delta is associated with an instrument change.")
	case AttributionInputChange:
		out = append(out, "Only the input evidence changed under the same execution.")
	case AttributionConfounded:
		out = append(out, "Both input and execution changed; the result delta cannot be attributed to either axis.")
	case AttributionUnavailable:
		out = append(out, "At least one run did not record an input or execution snapshot; attribution is unavailable, not assumed.")
	}
	for _, g := range c.RepeatGroups {
		if g.Runs < 2 {
			out = append(out, fmt.Sprintf("Only one run exists for configuration %s; run-to-run variation is unknown.", g.Key))
		}
	}
	return out
}
