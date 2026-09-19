package service

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"insight-lab/internal/domain"
)

// datasetPreAnalysisRuleVersion is stored in the analysis provenance so a
// reader of a report knows which deterministic rules produced the numbers.
// Bump it whenever the extraction or arithmetic below changes meaning.
const datasetPreAnalysisRuleVersion = "dataset-preanalysis/v1"

// DatasetPreAnalysis is everything the pipeline can establish from dataset
// documents without a model (Issue #16): grounded count observations,
// period-over-period arithmetic, and the provenance/comparability facts a
// reviewer needs to trust or distrust those numbers. The model, when
// configured, receives these results as input and never recomputes them.
type DatasetPreAnalysis struct {
	RuleVersion           string
	Observations          []*domain.Observation
	Comparisons           []DatasetComparison
	Manifests             []AcquisitionManifest
	DatasetHashes         []string
	CompatibilityWarnings []DatasetCompatibilityWarning
	Notes                 []string

	handled map[string]bool
}

// Handled reports whether docID was materialized deterministically, i.e.
// must not be sent to the model for observation extraction.
func (p DatasetPreAnalysis) Handled(docID string) bool { return p.handled[docID] }

// DatasetComparison is one consecutive-period step within a series of
// dataset documents that measure the same thing (same location, event
// type, provider, unit and population). RateOfChange is nil when the
// starting value is zero; the ratio is undefined, not infinite.
type DatasetComparison struct {
	Series             string   `json:"series"`
	FromDocumentID     string   `json:"fromDocumentId"`
	ToDocumentID       string   `json:"toDocumentId"`
	FromPeriod         string   `json:"fromPeriod"`
	ToPeriod           string   `json:"toPeriod"`
	FromValue          float64  `json:"fromValue"`
	ToValue            float64  `json:"toValue"`
	Delta              float64  `json:"delta"`
	RateOfChange       *float64 `json:"rateOfChange,omitempty"`
	BaselinePeriod     string   `json:"baselinePeriod"`
	BaselineValue      float64  `json:"baselineValue"`
	DeltaFromBaseline  float64  `json:"deltaFromBaseline"`
	ShareOfSeriesTotal float64  `json:"shareOfSeriesTotal"`
}

func RunDatasetPreAnalysis(docs []*domain.Document, now time.Time) DatasetPreAnalysis {
	pre := DatasetPreAnalysis{RuleVersion: datasetPreAnalysisRuleVersion, handled: map[string]bool{}}

	var obsNotes, cmpNotes []string
	pre.Observations, obsNotes = MaterializeDatasetObservations(docs, now)
	for _, o := range pre.Observations {
		pre.handled[o.DocumentID] = true
	}
	pre.Comparisons, cmpNotes = ComputeDatasetComparisons(docs)
	pre.Notes = append(append(pre.Notes, obsNotes...), cmpNotes...)

	hashes := map[string]bool{}
	seenDataset := map[string]bool{}
	for _, d := range docs {
		if h := d.Metadata[MetadataDatasetHash]; h != "" {
			hashes[h] = true
		}
		if m, ok := ManifestFromDocument(d); ok && !seenDataset[m.DatasetID] {
			seenDataset[m.DatasetID] = true
			pre.Manifests = append(pre.Manifests, m)
		}
	}
	for h := range hashes {
		pre.DatasetHashes = append(pre.DatasetHashes, h)
	}
	sort.Strings(pre.DatasetHashes)
	sort.Slice(pre.Manifests, func(i, j int) bool { return pre.Manifests[i].DatasetID < pre.Manifests[j].DatasetID })
	pre.CompatibilityWarnings = CheckDatasetCompatibility(pre.Manifests)
	return pre
}

// MaterializeDatasetObservations turns every dataset document that carries a
// numeric record_count into one Observation whose quote is the document's
// first sentence, grounded exactly like a model-proposed quote would be.
// Documents that are not countable are left for the model; a dataset
// document whose count is unparsable is reported in notes rather than
// silently guessed.
func MaterializeDatasetObservations(docs []*domain.Document, now time.Time) ([]*domain.Observation, []string) {
	var out []*domain.Observation
	var notes []string
	for _, d := range docs {
		if d.Source != domain.SourceDataset {
			continue
		}
		raw, ok := d.Metadata["record_count"]
		if !ok {
			continue
		}
		count, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
		if err != nil {
			notes = append(notes, fmt.Sprintf("document %s: record_count %q is not a number; skipped", d.ID, raw))
			continue
		}
		grounded, ok := Ground(d.Content, firstSentence(d.Content))
		if !ok {
			notes = append(notes, fmt.Sprintf("document %s: could not ground its first sentence; skipped", d.ID))
			continue
		}
		out = append(out, &domain.Observation{
			ID: newID("obs"), DocumentID: d.ID, Quote: grounded.Quote,
			StartOffset: grounded.StartOffset, EndOffset: grounded.EndOffset,
			Behavior:  fmt.Sprintf("record_count=%s (%s)", formatCount(count), strings.Join(nonEmpty(d.Metadata["event_type"], d.Metadata["period"], datasetLocation(d.Metadata)), ", ")),
			Topic:     firstNonEmpty(d.Metadata["event_type"], "dataset"),
			CreatedAt: now,
		})
	}
	return out, notes
}

// ComputeDatasetComparisons groups countable dataset documents into series
// and computes delta, rate, baseline delta and share for every consecutive
// pair of periods. Series are keyed on unit and population scope as well,
// so two datasets an AcquisitionManifest declares incomparable can never
// end up in the same subtraction - that is the calculation-level gate.
func ComputeDatasetComparisons(docs []*domain.Document) ([]DatasetComparison, []string) {
	type series struct {
		label  string
		points []datasetPoint
	}
	byKey := map[string]*series{}
	var keys []string
	for _, d := range docs {
		if d.Source != domain.SourceDataset {
			continue
		}
		value, err := strconv.ParseFloat(strings.TrimSpace(d.Metadata["record_count"]), 64)
		if err != nil || strings.TrimSpace(d.Metadata["period"]) == "" {
			continue
		}
		unit, population := "", ""
		if m, ok := ManifestFromDocument(d); ok {
			unit, population = m.Unit, m.PopulationScope
		}
		provider := strings.TrimSpace(d.Metadata["source_provider"] + " " + d.Metadata["source_version"])
		key := strings.Join([]string{d.Metadata["event_type"], datasetLocation(d.Metadata), provider, unit, population}, "\x00")
		s := byKey[key]
		if s == nil {
			s = &series{label: strings.Join(nonEmpty(d.Metadata["event_type"], datasetLocation(d.Metadata), provider), " / ")}
			byKey[key] = s
			keys = append(keys, key)
		}
		s.points = append(s.points, datasetPoint{docID: d.ID, period: d.Metadata["period"], value: value})
	}
	sort.Strings(keys)

	var out []DatasetComparison
	var notes []string
	for _, key := range keys {
		s := byKey[key]
		if len(s.points) < 2 {
			continue
		}
		sort.Slice(s.points, func(i, j int) bool { return s.points[i].period < s.points[j].period })
		if dup := duplicatePeriod(s.points); dup != "" {
			notes = append(notes, fmt.Sprintf("series %q has more than one document for period %s; comparisons skipped until the duplicate import is resolved", s.label, dup))
			continue
		}
		total := 0.0
		for _, p := range s.points {
			total += p.value
		}
		baseline := s.points[0]
		for i := 1; i < len(s.points); i++ {
			from, to := s.points[i-1], s.points[i]
			c := DatasetComparison{
				Series: s.label, FromDocumentID: from.docID, ToDocumentID: to.docID,
				FromPeriod: from.period, ToPeriod: to.period, FromValue: from.value, ToValue: to.value,
				Delta: to.value - from.value, BaselinePeriod: baseline.period, BaselineValue: baseline.value,
				DeltaFromBaseline: to.value - baseline.value,
			}
			if from.value != 0 {
				rate := (to.value - from.value) / from.value
				c.RateOfChange = &rate
			}
			if total != 0 {
				c.ShareOfSeriesTotal = to.value / total
			}
			out = append(out, c)
		}
	}
	return out, notes
}

// buildComparisonPatterns persists comparisons as repetition patterns that
// cite the two dataset observations they were computed from, so the
// hypothesis step sees the arithmetic as a given rather than redoing it.
func buildComparisonPatterns(projectID, analysisID string, comparisons []DatasetComparison, observations []*domain.Observation, now time.Time) []*domain.Pattern {
	obsByDoc := map[string]string{}
	for _, o := range observations {
		obsByDoc[o.DocumentID] = o.ID
	}
	var out []*domain.Pattern
	for _, c := range comparisons {
		fromObs, toObs := obsByDoc[c.FromDocumentID], obsByDoc[c.ToDocumentID]
		if fromObs == "" || toObs == "" {
			continue
		}
		rate := "rate undefined (start value is 0)"
		if c.RateOfChange != nil {
			rate = fmt.Sprintf("rate %+.1f%%", *c.RateOfChange*100)
		}
		out = append(out, &domain.Pattern{
			ID: newID("pat"), ProjectID: projectID, AnalysisID: analysisID, Kind: domain.PatternRepetition,
			Title: fmt.Sprintf("Deterministic comparison: %s, %s → %s", c.Series, c.FromPeriod, c.ToPeriod),
			Description: fmt.Sprintf("record_count %s → %s (delta %s, %s); baseline %s = %s (delta from baseline %s); %s share of series total %.1f%%. Computed by rule %s from record_count metadata. This is arithmetic on exported records and is not an estimate of startups, policy effect, or cause.",
				formatCount(c.FromValue), formatCount(c.ToValue), formatSigned(c.Delta), rate,
				c.BaselinePeriod, formatCount(c.BaselineValue), formatSigned(c.DeltaFromBaseline),
				c.ToPeriod, c.ShareOfSeriesTotal*100, datasetPreAnalysisRuleVersion),
			ObservationIDs: []string{fromObs, toObs}, CreatedAt: now,
		})
	}
	return out
}

// firstSentence returns content up to and including the first sentence
// terminator: "。" anywhere, or "." followed by whitespace or end of text
// (so "v4.1;" is not a boundary). Falls back to the whole text.
func firstSentence(content string) string {
	runes := []rune(strings.TrimSpace(content))
	for i, r := range runes {
		switch {
		case r == '。':
			return string(runes[:i+1])
		case r == '.' && (i+1 == len(runes) || runes[i+1] == ' ' || runes[i+1] == '\n' || runes[i+1] == '\r' || runes[i+1] == '\t'):
			return string(runes[:i+1])
		}
	}
	return string(runes)
}

func datasetLocation(meta map[string]string) string {
	location := strings.TrimSpace(meta["prefecture_name"] + " " + meta["city_name"])
	return firstNonEmpty(location, strings.TrimSpace(meta["location"]), strings.TrimSpace(meta["geography"]))
}

type datasetPoint struct {
	docID  string
	period string
	value  float64
}

func duplicatePeriod(points []datasetPoint) string {
	for i := 1; i < len(points); i++ {
		if points[i].period == points[i-1].period {
			return points[i].period
		}
	}
	return ""
}

func formatCount(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }
func formatSigned(v float64) string {
	if v >= 0 {
		return "+" + formatCount(v)
	}
	return formatCount(v)
}

func nonEmpty(values ...string) []string {
	var out []string
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			out = append(out, v)
		}
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
