package publicengine

import (
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	"insight-lab/internal/input"
)

// wireTypes maps every schema $def that describes an object to the Go type
// the server encodes or decodes for it.
var wireTypes = map[string]any{
	"SubjectRef": SubjectRef{}, "EngineBuild": EngineBuild{}, "SchemaRef": SchemaRef{}, "EngineInfo": EngineInfo{},
	"CreateSubjectRequest": CreateSubjectRequest{}, "Subject": Subject{}, "EvidenceDocument": EvidenceDocument{},
	"AddEvidenceRequest": AddEvidenceRequest{}, "EvidenceItemReceipt": EvidenceItemReceipt{}, "EvidenceReceipt": EvidenceReceipt{},
	"StartAnalysisRequest": StartAnalysisRequest{}, "AnalysisProvenance": AnalysisProvenance{}, "AnalysisRun": AnalysisRun{},
	"Observation": Observation{}, "Finding": Finding{}, "AnalysisResults": AnalysisResults{},
	"CreateResearchRunRequest": CreateResearchRunRequest{}, "AddedEvidenceLink": AddedEvidenceLink{},
	"AppendIterationRequest": AppendIterationRequest{}, "ResearchResult": ResearchResult{},
	"ErrorBody": ErrorBody{}, "ErrorResponse": ErrorResponse{},
	"AnalysisList": AnalysisList{}, "RunComparisonResult": RunComparisonResult{}, "RunComparison": RunComparison{},
	"RunRef": RunRef{}, "FieldChange": FieldChange{}, "ExecutionAxisDiff": ExecutionAxisDiff{}, "InputAxisDiff": InputAxisDiff{},
	"MetricDelta": MetricDelta{}, "InsightMatch": InsightMatch{}, "InsightResultDiff": InsightResultDiff{},
	"MetricRange": MetricRange{}, "RepeatGroup": RepeatGroup{},
	"ResearchRunSummary": ResearchRunSummary{}, "ResearchRunList": ResearchRunList{},
	"ReEvaluationRequest": ReEvaluationRequest{}, "ReEvaluationTrigger": ReEvaluationTrigger{},
	"EvidenceChanges": EvidenceChanges{}, "ReEvaluationRecord": ReEvaluationRecord{}, "ReEvaluationResult": ReEvaluationResult{},
	"ObservationWindow": ObservationWindow{}, "ResearchTimeline": ResearchTimeline{}, "TimelineIteration": TimelineIteration{},
	"EvidenceEvent": EvidenceEvent{}, "TimelineObservationDelta": TimelineObservationDelta{}, "HypothesisEvent": HypothesisEvent{},
	"InsightVersion": InsightVersion{}, "InstrumentChange": InstrumentChange{},
	"TemporalOperationRequest": TemporalOperationRequest{}, "TemporalOperationResult": TemporalOperationResult{},
	"ExecutionProfileInfo": ExecutionProfileInfo{}, "ExecutionProfileResolution": ExecutionProfileResolution{},
	"RawArtifactRef": RawArtifactRef{}, "InputSource": InputSource{}, "InputSourceReceipt": InputSourceReceipt{},
	"PreparationSpec": input.PreparationSpec{},
}

func init() {
	for name, value := range scenarioWireTypes {
		wireTypes[name] = value
	}
}

type schemaDoc struct {
	Defs       map[string]schemaDef `json:"$defs"`
	ErrorCodes map[string]int       `json:"x-errorCodes"`
}

type schemaDef struct {
	Type       string                     `json:"type"`
	Properties map[string]json.RawMessage `json:"properties"`
	Required   []string                   `json:"required"`
}

func loadSchema(t *testing.T) schemaDoc {
	t.Helper()
	data, err := os.ReadFile("../../contracts/public-engine/v1/schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc schemaDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

func TestWireTypesMatchTheContractSchema(t *testing.T) {
	doc := loadSchema(t)
	for name, def := range doc.Defs {
		if def.Type != "object" || def.Properties == nil {
			continue
		}
		value, ok := wireTypes[name]
		if !ok {
			t.Errorf("schema $def %s has no Go wire type", name)
			continue
		}
		fields := jsonFields(reflect.TypeOf(value))
		var schemaKeys []string
		for key := range def.Properties {
			schemaKeys = append(schemaKeys, key)
		}
		sort.Strings(schemaKeys)
		if got, want := strings.Join(fields.names, ","), strings.Join(schemaKeys, ","); got != want {
			t.Errorf("%s: Go fields [%s] != schema properties [%s]", name, got, want)
		}
		for _, key := range def.Required {
			if fields.optional[key] {
				t.Errorf("%s.%s is required by the schema but omitempty in Go", name, key)
			}
		}
	}
	for name := range wireTypes {
		if _, ok := doc.Defs[name]; !ok {
			t.Errorf("Go wire type %s has no schema $def", name)
		}
	}
}

func TestErrorCodesMatchTheContractSchema(t *testing.T) {
	doc := loadSchema(t)
	if len(doc.ErrorCodes) != len(errorStatus) {
		t.Fatalf("schema lists %d error codes, engine maps %d", len(doc.ErrorCodes), len(errorStatus))
	}
	for code, status := range doc.ErrorCodes {
		if errorStatus[Code(code)] != status {
			t.Errorf("error code %s: engine status %d, schema %d", code, errorStatus[Code(code)], status)
		}
	}
}

type fieldSet struct {
	names    []string
	optional map[string]bool
}

func jsonFields(typ reflect.Type) fieldSet {
	out := fieldSet{optional: map[string]bool{}}
	for i := 0; i < typ.NumField(); i++ {
		tag := typ.Field(i).Tag.Get("json")
		name, opts, _ := strings.Cut(tag, ",")
		if name == "" || name == "-" {
			continue
		}
		out.names = append(out.names, name)
		out.optional[name] = strings.Contains(opts, "omitempty")
	}
	sort.Strings(out.names)
	return out
}
