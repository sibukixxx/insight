//go:build demo

package sampledata

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScenariosListsThreePacksInDisplayOrderWhenDemoBuild(t *testing.T) {
	scenarios, err := Scenarios()
	if err != nil {
		t.Fatalf("Scenarios: %v", err)
	}
	var ids []string
	for _, s := range scenarios {
		ids = append(ids, s.ID)
	}
	if got := strings.Join(ids, ","); got != "ja-shop-records,ja-official-population,ja-population-establishments" {
		t.Fatalf("scenario ids = %s", got)
	}
	kinds := map[string]string{"ja-shop-records": "synthetic", "ja-official-population": "official", "ja-population-establishments": "mixed"}
	for _, s := range scenarios {
		if s.DataKind != kinds[s.ID] {
			t.Errorf("%s dataKind = %q, want %q", s.ID, s.DataKind, kinds[s.ID])
		}
		if s.ImportKind != "documents" || s.ProjectName == "" || len(s.Limitations) == 0 {
			t.Errorf("%s: importKind=%q projectName=%q limitations=%d", s.ID, s.ImportKind, s.ProjectName, len(s.Limitations))
		}
	}
}

func TestScenarioInputMatchesManifestRowsAndDocumentsHeaderWhenRead(t *testing.T) {
	scenarios, err := Scenarios()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range scenarios {
		data, err := ScenarioInput(s.ID)
		if err != nil {
			t.Fatalf("%s: %v", s.ID, err)
		}
		records, err := csv.NewReader(bytes.NewReader(data)).ReadAll()
		if err != nil {
			t.Fatalf("%s: parse csv: %v", s.ID, err)
		}
		if strings.Join(records[0], ",") != "id,source,title,content" {
			t.Errorf("%s header = %v", s.ID, records[0])
		}
		if len(records)-1 != s.Rows {
			t.Errorf("%s rows = %d, manifest says %d", s.ID, len(records)-1, s.Rows)
		}
	}
}

// Every official value must be traceable to a pinned raw snapshot, and every
// synthetic row must say so in its own text, so a reader of one imported
// document can never mistake invented numbers for official statistics.
func TestScenarioSourcesKeepOfficialAndSyntheticRowsDistinguishableWhenLoaded(t *testing.T) {
	scenarios, err := Scenarios()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range scenarios {
		data, _ := ScenarioInput(s.ID)
		records, _ := csv.NewReader(bytes.NewReader(data)).ReadAll()
		content := map[string]string{}
		for _, r := range records[1:] {
			content[r[0]] = r[2] + " " + r[3]
		}
		var official, synthetic int
		for _, src := range s.Sources {
			switch src.Kind {
			case "official":
				official++
				if src.URL == "" || src.RetrievedAt == "" || src.License == "" || src.LicenseURL == "" || src.Unit == "" || src.RawFile == "" {
					t.Errorf("%s: official source lacks provenance: %+v", s.ID, src)
					continue
				}
				raw, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(src.RawFile)))
				if err != nil {
					t.Errorf("%s: raw snapshot: %v", s.ID, err)
					continue
				}
				sum := sha256.Sum256(raw)
				if hex.EncodeToString(sum[:]) != src.RawSHA256 {
					t.Errorf("%s: %s sha256 does not match manifest", s.ID, src.RawFile)
				}
			case "synthetic":
				synthetic++
				for _, id := range src.Rows {
					if !strings.Contains(content[id], "架空") {
						t.Errorf("%s: synthetic row %s does not say it is fictional", s.ID, id)
					}
				}
			default:
				t.Errorf("%s: unknown source kind %q", s.ID, src.Kind)
			}
		}
		want := map[string][2]bool{"synthetic": {false, true}, "official": {true, false}, "mixed": {true, true}}[s.DataKind]
		if (official > 0) != want[0] || (synthetic > 0) != want[1] {
			t.Errorf("%s (%s): %d official and %d synthetic sources", s.ID, s.DataKind, official, synthetic)
		}
	}
}

func TestScenarioInputReturnsErrUnknownScenarioWhenIDIsNotBundled(t *testing.T) {
	if _, err := ScenarioInput("../testdata/research_policy.json"); !errors.Is(err, ErrUnknownScenario) {
		t.Errorf("err = %v, want ErrUnknownScenario", err)
	}
}
