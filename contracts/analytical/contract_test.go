package analytical_test

import (
	"encoding/json"
	"os"
	"testing"

	contract "insight-lab/contracts/analytical"
)

func TestPublicContractImportExportPreservesOpaqueAndMissingValues(t *testing.T) {
	data, err := os.ReadFile("../analytical-artifact/v1/fixtures/external-consumer.json")
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := contract.Import(data)
	if err != nil {
		t.Fatal(err)
	}
	if artifact.ExternalSubject == nil || artifact.ExternalSubject.Namespace != "consumer.example" {
		t.Fatal("opaque external subject was not preserved")
	}
	if len(artifact.Results) != 2 || !artifact.Results[1].Missing || len(artifact.Results[1].Value) != 0 {
		t.Fatal("missing result was converted into a value")
	}
	exported, err := contract.Export(*artifact)
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]any
	if err := json.Unmarshal(exported, &wire); err != nil {
		t.Fatal(err)
	}
	results := wire["results"].([]any)
	missing := results[1].(map[string]any)
	if missing["missing"] != true {
		t.Fatal("missing marker was not preserved")
	}
	if _, exists := missing["value"]; exists {
		t.Fatal("unknown/missing was serialized as a value")
	}
}
