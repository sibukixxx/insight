package input

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"sort"
	"strings"
	"time"

	"insight-lab/internal/analytical"
)

const (
	PreparationProducer        = "insight-lab.input-preparation"
	PreparationProducerVersion = "1"
)

// PreparedArtifactID is deterministic in the raw bytes and the spec, so
// preparing the same input again (on any profile, after any restart) yields
// the same artifact identity.
func PreparedArtifactID(rawSHA256 string, spec PreparationSpec) string {
	sum := sha256.Sum256([]byte(strings.ToLower(rawSHA256) + "|" + string(spec.SpecJSON())))
	return "prepared-" + hex.EncodeToString(sum[:])[:24]
}

// BuildPreparedArtifact turns a merged aggregate into a sealed Analytical
// Artifact. registeredAt (when the reference was registered) is used as the
// generation time so the artifact hash does not depend on when or where the
// preparation ran.
func BuildPreparedArtifact(datasetID string, ref RawArtifactRef, spec PreparationSpec, agg *Aggregate, registeredAt time.Time) (analytical.Artifact, error) {
	keys := make([]string, 0, len(agg.Groups))
	for k := range agg.Groups {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var periods []string
	var results []analytical.Result
	for _, k := range keys {
		g := agg.Groups[k]
		for _, m := range spec.Metrics {
			p := g.Metrics[m.ID]
			res := analytical.Result{MetricID: m.ID, Dimensions: g.Dimensions}
			if spec.PeriodColumn != "" {
				res.Period = analytical.Period{Start: g.Period, End: g.Period, Basis: basis(spec)}
				periods = append(periods, g.Period)
			} else {
				res.Period = analytical.Period{Start: spec.Period.Start, End: spec.Period.End, Basis: spec.Period.Basis}
			}
			if v, ok := value(m, p); ok {
				res.Value, _ = json.Marshal(v)
			} else {
				res.Missing = true
				res.Quality = append(res.Quality, analytical.QualityFlag{Code: "NO_VALID_VALUES", Message: "no numeric values in group; unknown is not zero"})
			}
			if p.Invalid > 0 {
				res.Quality = append(res.Quality, analytical.QualityFlag{Code: "UNPARSEABLE_VALUES", Message: fmt.Sprintf("%d non-numeric values ignored", p.Invalid)})
			}
			results = append(results, res)
		}
	}
	if len(results) == 0 {
		return analytical.Artifact{}, fmt.Errorf("preparation produced no results")
	}
	period := analytical.Period{}
	if spec.PeriodColumn != "" {
		sort.Strings(periods)
		period = analytical.Period{Start: periods[0], End: periods[len(periods)-1], Basis: basis(spec)}
	} else {
		period = analytical.Period{Start: spec.Period.Start, End: spec.Period.End, Basis: spec.Period.Basis}
	}
	specSum := sha256.Sum256(spec.SpecJSON())
	metrics := make([]analytical.MetricDefinition, 0, len(spec.Metrics))
	for _, m := range spec.Metrics {
		metrics = append(metrics, analytical.MetricDefinition{ID: m.ID, Name: m.Name, Unit: m.Unit, Aggregation: m.Aggregation})
	}
	version := ref.Version
	if version == "" {
		version = "sha256:" + ref.SHA256[:12]
	}
	a := analytical.Artifact{
		ArtifactSchema: analytical.Schema, SchemaVersion: analytical.Version,
		ID: PreparedArtifactID(ref.SHA256, spec), Producer: PreparationProducer, ProducerVersion: PreparationProducerVersion,
		GeneratedAt: registeredAt.UTC(),
		Datasets:    []analytical.DatasetRef{{ID: datasetID, URI: ref.URI, Version: version, Hash: analytical.Hash{Algorithm: "sha256", Value: ref.SHA256}}},
		Spec:        analytical.SpecRef{Kind: spec.Kind, Reference: "inline:preparation", Hash: analytical.Hash{Algorithm: "sha256", Value: hex.EncodeToString(specSum[:])}},
		Dimensions:  spec.DimensionColumns,
		Period:      period,
		Population:  analytical.Population{Description: spec.Population.Description, Unit: spec.Population.Unit},
		Metrics:     metrics, Results: results,
		Computation: analytical.Computation{Engine: PreparationProducer, EngineVersion: PreparationProducerVersion, Deterministic: true, Timezone: "UTC"},
		Provenance:  []analytical.SourceProvenance{{DatasetID: datasetID, Source: ref.URI, RetrievedAt: registeredAt.UTC()}},
	}
	if agg.RowsWithoutTime > 0 {
		a.Quality = append(a.Quality, analytical.QualityFlag{Code: "ROWS_WITHOUT_PERIOD", Message: fmt.Sprintf("%d rows without a period value were not aggregated", agg.RowsWithoutTime)})
	}
	canonical, err := json.Marshal(a)
	if err != nil {
		return analytical.Artifact{}, err
	}
	sum := sha256.Sum256(canonical)
	a.ArtifactHash = analytical.Hash{Algorithm: "sha256", Value: hex.EncodeToString(sum[:])}
	return a, a.Validate()
}

func basis(s PreparationSpec) string {
	if s.Period != nil {
		return s.Period.Basis
	}
	return ""
}

func value(m PreparationMetric, p Partial) (float64, bool) {
	switch m.Aggregation {
	case "count":
		if m.Column == "" {
			return float64(p.Rows), true
		}
		return float64(p.N), true
	case "sum":
		f, _ := p.sum().Float64()
		return f, p.N > 0
	case "mean":
		if p.N == 0 {
			return 0, false
		}
		mean, _ := new(big.Rat).Quo(p.sum(), new(big.Rat).SetInt64(p.N)).Float64()
		return math.Round(mean*1e9) / 1e9, true
	case "min":
		return p.Min, p.N > 0
	case "max":
		return p.Max, p.N > 0
	}
	return 0, false
}
