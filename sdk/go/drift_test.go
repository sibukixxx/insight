package insight

import (
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
)

var wireTypes = map[string]any{
	"SubjectRef": SubjectRef{}, "EngineBuild": EngineBuild{}, "SchemaRef": SchemaRef{}, "EngineInfo": EngineInfo{},
	"CreateSubjectRequest": CreateSubjectRequest{}, "Subject": Subject{}, "EvidenceDocument": EvidenceDocument{},
	"AddEvidenceRequest": AddEvidenceRequest{}, "EvidenceItemReceipt": EvidenceItemReceipt{}, "EvidenceReceipt": EvidenceReceipt{},
	"StartAnalysisRequest": StartAnalysisRequest{}, "AnalysisProvenance": AnalysisProvenance{}, "AnalysisRun": AnalysisRun{},
	"Observation": Observation{}, "Finding": Finding{}, "AnalysisResults": AnalysisResults{},
	"CreateResearchRunRequest": CreateResearchRunRequest{}, "AddedEvidenceLink": AddedEvidenceLink{},
	"AppendIterationRequest": AppendIterationRequest{}, "ResearchResult": ResearchResult{},
	"ErrorBody": ErrorBody{}, "ErrorResponse": ErrorResponse{},
}

type schemaDoc struct {
	Defs       map[string]schemaDef `json:"$defs"`
	ErrorCodes map[string]int       `json:"x-errorCodes"`
	Version    string               `json:"x-contractVersion"`
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

func TestSDKTypesMatchTheContractSchema(t *testing.T) {
	doc := loadSchema(t)
	if doc.Version != ContractVersion {
		t.Fatalf("SDK speaks contract %q, schema is %q", ContractVersion, doc.Version)
	}
	for name, def := range doc.Defs {
		if def.Type != "object" || def.Properties == nil {
			continue
		}
		value, ok := wireTypes[name]
		if !ok {
			t.Errorf("schema $def %s has no SDK type", name)
			continue
		}
		typ := reflect.TypeOf(value)
		var fields []string
		optional := map[string]bool{}
		for i := 0; i < typ.NumField(); i++ {
			name, opts, _ := strings.Cut(typ.Field(i).Tag.Get("json"), ",")
			fields = append(fields, name)
			optional[name] = strings.Contains(opts, "omitempty")
		}
		sort.Strings(fields)
		var keys []string
		for key := range def.Properties {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		if strings.Join(fields, ",") != strings.Join(keys, ",") {
			t.Errorf("%s: SDK fields %v != schema properties %v", name, fields, keys)
		}
		for _, key := range def.Required {
			if optional[key] && !strings.HasSuffix(name, "Request") {
				t.Errorf("%s.%s is required but omitempty", name, key)
			}
		}
	}
}

func TestSDKErrorCodesCoverTheContract(t *testing.T) {
	known := map[string]bool{CodeInvalidRequest: true, CodeUnsupportedContractVersion: true, CodeNotFound: true, CodeIdempotencyConflict: true,
		CodeIdentityConflict: true, CodeAnalysisNotCompleted: true, CodeAnalysisHasNoHypotheses: true, CodeMixedAnalysisRuns: true, CodeInternal: true}
	for code := range loadSchema(t).ErrorCodes {
		if !known[code] {
			t.Errorf("contract error code %s has no SDK constant", code)
		}
	}
}

func TestSDKModuleHasNoDependencies(t *testing.T) {
	data, err := os.ReadFile("go.mod")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "require") {
		t.Fatalf("the SDK must stay dependency-free and never require the Insight core module:\n%s", data)
	}
}
