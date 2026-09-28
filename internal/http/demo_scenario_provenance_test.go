//go:build demo

package httpapi_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

type wireScenarioSource struct {
	Kind        string   `json:"kind"`
	Survey      string   `json:"survey"`
	Dataset     string   `json:"dataset"`
	Periods     []string `json:"periods"`
	Unit        string   `json:"unit"`
	RetrievedAt string   `json:"retrievedAt"`
	RawFile     string   `json:"rawFile"`
	RawSHA256   string   `json:"rawSha256"`
	License     string   `json:"license"`
	Attribution string   `json:"attribution"`
	Rows        []string `json:"rows"`
}

type wireScenario struct {
	ID          string               `json:"id"`
	DataKind    string               `json:"dataKind"`
	InputSHA256 string               `json:"inputSha256"`
	Sources     []wireScenarioSource `json:"sources"`
	Limitations []string             `json:"limitations"`
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// The provenance the browser shows for the official and mixed samples comes
// from GET /api/demo/scenarios. What the API reports must match the pinned
// files on disk (independent of the manifest the handler reads), and the
// mixed sample must keep each survey's period separate rather than merging
// population and establishment counts into one time point (#151, #156).
func TestDemoScenariosHTTPReportsProvenanceMatchingPinnedFilesAndKeepsMixedPeriodsSeparate(t *testing.T) {
	router, _ := newDemoScenarioRouter(t)
	resp := serveDemo(router, http.MethodGet, "/api/demo/scenarios")
	var list []wireScenario
	if resp.Code != http.StatusOK || json.Unmarshal(resp.Body.Bytes(), &list) != nil {
		t.Fatalf("list: %d %s", resp.Code, resp.Body.String())
	}
	byID := map[string]wireScenario{}
	for _, s := range list {
		byID[s.ID] = s
	}

	for _, id := range []string{"ja-official-population", "ja-population-establishments"} {
		s, ok := byID[id]
		if !ok {
			t.Fatalf("scenario %s is not listed", id)
		}
		onDisk, err := os.ReadFile(filepath.Join("..", "sampledata", "scenarios", id, "input.csv"))
		if err != nil {
			t.Fatal(err)
		}
		served := serveDemo(router, http.MethodGet, "/api/demo/scenarios/"+id+"/input.csv")
		if got := sha256Hex(served.Body.Bytes()); served.Code != http.StatusOK || got != s.InputSHA256 || got != sha256Hex(onDisk) {
			t.Errorf("%s: served csv sha256 %s, listed %s, on disk %s", id, got, s.InputSHA256, sha256Hex(onDisk))
		}
		if len(s.Limitations) == 0 {
			t.Errorf("%s lists no limitations", id)
		}
		for _, src := range s.Sources {
			if src.Kind != "official" {
				continue
			}
			raw, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(src.RawFile)))
			if err != nil {
				t.Errorf("%s: %v", id, err)
				continue
			}
			if got := sha256Hex(raw); got != src.RawSHA256 {
				t.Errorf("%s: %s is %s on disk, API reports %s", id, src.RawFile, got, src.RawSHA256)
			}
			if src.RetrievedAt == "" || src.License == "" || src.Attribution == "" || src.Unit == "" || len(src.Periods) == 0 {
				t.Errorf("%s: official source %s lacks provenance on the wire: %+v", id, src.Dataset, src)
			}
		}
	}

	mixed := byID["ja-population-establishments"]
	periods := map[string]string{}
	var synthetic int
	for _, src := range mixed.Sources {
		switch src.Kind {
		case "official":
			periods[src.Survey+"/"+src.Unit+"/"+src.Dataset] = strings.Join(src.Periods, ",")
		case "synthetic":
			synthetic += len(src.Rows)
		}
	}
	var got []string
	for k, v := range periods {
		got = append(got, k+"="+v)
	}
	sort.Strings(got)
	want := []string{
		"国勢調査/人/総人口（総数）=2020",
		"経済センサス/事業所/事業所数（民営）=2021",
		"経済センサス/事業所/事業所数（民営）（～2016年）=2016",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("mixed sources must keep one survey period each:\n got %q\nwant %q", got, want)
	}
	if synthetic != 3 {
		t.Errorf("mixed sample lists %d synthetic rows, want 3", synthetic)
	}
	var warnsTimePoint bool
	for _, l := range mixed.Limitations {
		if strings.Contains(l, "同一時点の比率として組み合わせられない") {
			warnsTimePoint = true
		}
	}
	if !warnsTimePoint {
		t.Errorf("mixed sample does not warn that population and establishments are different time points: %q", mixed.Limitations)
	}
}
