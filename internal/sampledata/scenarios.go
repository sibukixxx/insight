package sampledata

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path"
)

// ErrUnknownScenario is returned for a scenario id that is not bundled.
var ErrUnknownScenario = errors.New("unknown sample scenario")

// Scenario is one playable sample pack: a CSV the ordinary importer
// accepts plus the provenance a user needs to tell synthetic from official
// data. The files live in scenarios/<id>/ and are compiled in only by the
// "demo" build tag, like the policy demo.
type Scenario struct {
	ID          string            `json:"id"`
	ProjectName string            `json:"projectName"`
	DataKind    string            `json:"dataKind"`
	ImportKind  string            `json:"importKind"`
	InputFile   string            `json:"inputFile"`
	InputSHA256 string            `json:"inputSha256"`
	Rows        int               `json:"rows"`
	Sources     []ScenarioSource  `json:"sources"`
	Transform   ScenarioTransform `json:"transform"`
	Limitations []string          `json:"limitations"`
}

// ScenarioSource records where one part of a scenario's data came from.
// Official sources carry the retrieval and checksum fields; synthetic ones
// only a description and the rows they cover.
type ScenarioSource struct {
	Kind          string           `json:"kind"`
	Description   string           `json:"description,omitempty"`
	Rows          []string         `json:"rows,omitempty"`
	Publisher     string           `json:"publisher,omitempty"`
	Survey        string           `json:"survey,omitempty"`
	Dataset       string           `json:"dataset,omitempty"`
	Provider      string           `json:"provider,omitempty"`
	IndicatorCode string           `json:"indicatorCode,omitempty"`
	Regions       []ScenarioRegion `json:"regions,omitempty"`
	Periods       []string         `json:"periods,omitempty"`
	Unit          string           `json:"unit,omitempty"`
	URL           string           `json:"url,omitempty"`
	APIRequest    string           `json:"apiRequest,omitempty"`
	RetrievedAt   string           `json:"retrievedAt,omitempty"`
	RawFile       string           `json:"rawFile,omitempty"`
	RawSHA256     string           `json:"rawSha256,omitempty"`
	License       string           `json:"license,omitempty"`
	LicenseURL    string           `json:"licenseUrl,omitempty"`
	Attribution   string           `json:"attribution,omitempty"`
}

type ScenarioRegion struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

type ScenarioTransform struct {
	Script      string `json:"script"`
	Version     string `json:"version"`
	Description string `json:"description"`
}

type scenarioIndexEntry struct {
	ID          string `json:"id"`
	ProjectName string `json:"projectName"`
}

// Scenarios lists the bundled sample scenarios in display order. A
// delivery build has none.
func Scenarios() ([]Scenario, error) {
	files := scenarioFiles()
	if files == nil {
		return nil, nil
	}
	raw, err := fs.ReadFile(files, "scenarios/index.json")
	if err != nil {
		return nil, fmt.Errorf("read scenario index: %w", err)
	}
	var index []scenarioIndexEntry
	if err := json.Unmarshal(raw, &index); err != nil {
		return nil, fmt.Errorf("parse scenario index: %w", err)
	}
	out := make([]Scenario, 0, len(index))
	for _, entry := range index {
		s, err := loadScenario(files, entry)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

// FindScenario returns one bundled scenario.
func FindScenario(id string) (Scenario, error) {
	all, err := Scenarios()
	if err != nil {
		return Scenario{}, err
	}
	for _, s := range all {
		if s.ID == id {
			return s, nil
		}
	}
	return Scenario{}, ErrUnknownScenario
}

// ScenarioInput returns the exact bytes of a scenario's input CSV, checked
// against the checksum recorded in its manifest.
func ScenarioInput(id string) ([]byte, error) {
	s, err := FindScenario(id)
	if err != nil {
		return nil, err
	}
	return readScenarioInput(scenarioFiles(), s)
}

func loadScenario(files fs.FS, entry scenarioIndexEntry) (Scenario, error) {
	raw, err := fs.ReadFile(files, path.Join("scenarios", entry.ID, "scenario.json"))
	if err != nil {
		return Scenario{}, fmt.Errorf("read scenario %s: %w", entry.ID, err)
	}
	var s Scenario
	if err := json.Unmarshal(raw, &s); err != nil {
		return Scenario{}, fmt.Errorf("parse scenario %s: %w", entry.ID, err)
	}
	if s.ID != entry.ID {
		return Scenario{}, fmt.Errorf("scenario %s: manifest id is %q", entry.ID, s.ID)
	}
	s.ProjectName = entry.ProjectName
	if _, err := readScenarioInput(files, s); err != nil {
		return Scenario{}, err
	}
	return s, nil
}

func readScenarioInput(files fs.FS, s Scenario) ([]byte, error) {
	data, err := fs.ReadFile(files, path.Join("scenarios", s.ID, s.InputFile))
	if err != nil {
		return nil, fmt.Errorf("read scenario %s input: %w", s.ID, err)
	}
	sum := sha256.Sum256(data)
	if got := hex.EncodeToString(sum[:]); got != s.InputSHA256 {
		return nil, fmt.Errorf("scenario %s input sha256 is %s, manifest records %s", s.ID, got, s.InputSHA256)
	}
	return data, nil
}
