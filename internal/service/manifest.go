package service

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"insight-lab/internal/domain"
)

// AcquisitionManifest records how an external dataset reached Insight Lab
// (Issue #16). It is the provenance half of a reproducible run: given the
// same file (FileHash) and the same manifest, a later run can be checked
// against an earlier one without re-fetching anything. The core never
// fetches data itself, so the manifest is supplied by whoever exported the
// file - a script, an adapter repository, or a person filling in a form.
//
// It deliberately has no field for credentials. Validate rejects
// credential-looking query parameters so an exporter cannot smuggle an
// API key into project metadata by accident.
type AcquisitionManifest struct {
	SourceName          string            `json:"sourceName"`
	SourceURL           string            `json:"sourceUrl,omitempty"`
	DatasetID           string            `json:"datasetId"` // dataset / table / stat identifier at the source
	RetrievalMethod     RetrievalMethod   `json:"retrievalMethod"`
	RetrievedAt         time.Time         `json:"retrievedAt"`
	QueryParameters     map[string]string `json:"queryParameters,omitempty"`
	Geography           string            `json:"geography,omitempty"`
	Period              string            `json:"period,omitempty"`
	Unit                string            `json:"unit,omitempty"`
	PopulationScope     string            `json:"populationScope,omitempty"` // population / denominator the counts refer to
	KnownCaveats        []string          `json:"knownCaveats,omitempty"`
	TransformationSteps []string          `json:"transformationSteps,omitempty"`
	FileHash            string            `json:"fileHash,omitempty"` // sha256 hex of the imported file; filled in by the importer
	SchemaID            string            `json:"schemaId"`
	SchemaVersion       string            `json:"schemaVersion,omitempty"`
	RecipeRef           string            `json:"recipeRef,omitempty"` // pointer to the acquisition recipe (script, doc, adapter version)
}

type RetrievalMethod string

const (
	RetrievalAPI          RetrievalMethod = "api"
	RetrievalDownload     RetrievalMethod = "download"
	RetrievalManualExport RetrievalMethod = "manual_export"
	RetrievalFixture      RetrievalMethod = "fixture" // checked-in public-data fixture
	RetrievalUserProvided RetrievalMethod = "user_provided"
)

var retrievalMethods = []RetrievalMethod{RetrievalAPI, RetrievalDownload, RetrievalManualExport, RetrievalFixture, RetrievalUserProvided}

func (m RetrievalMethod) Valid() bool {
	for _, known := range retrievalMethods {
		if m == known {
			return true
		}
	}
	return false
}

// Document metadata keys written by the importers. MetadataDatasetHash is
// set on every imported document, manifest or not, so a report can always
// be traced back to the exact file it came from.
const (
	MetadataDatasetHash         = "dataset_hash"
	MetadataAcquisitionManifest = "acquisition_manifest"
)

// credentialKeyPattern matches parameter names that conventionally carry a
// secret. The list is intentionally broad: a false positive costs the
// exporter a rename, a false negative leaks a key into SQLite.
var credentialKeyPattern = regexp.MustCompile(`(?i)(^id$|app_?id|api_?key|key$|token|secret|passw|credential|authorization|signature)`)

func (m AcquisitionManifest) Validate() error {
	if strings.TrimSpace(m.SourceName) == "" {
		return fmt.Errorf("manifest: sourceName is required")
	}
	if strings.TrimSpace(m.DatasetID) == "" {
		return fmt.Errorf("manifest: datasetId is required")
	}
	if strings.TrimSpace(string(m.RetrievalMethod)) == "" {
		return fmt.Errorf("manifest: retrievalMethod is required")
	}
	if !m.RetrievalMethod.Valid() {
		return fmt.Errorf("manifest: retrievalMethod must be one of %v", retrievalMethods)
	}
	if m.RetrievedAt.IsZero() {
		return fmt.Errorf("manifest: retrievedAt is required")
	}
	if strings.TrimSpace(m.SchemaID) == "" {
		return fmt.Errorf("manifest: schemaId is required")
	}
	for key := range m.QueryParameters {
		if credentialKeyPattern.MatchString(key) {
			return fmt.Errorf("manifest: queryParameters must not contain credentials (parameter %q)", key)
		}
	}
	if m.SourceURL != "" {
		parsed, err := url.Parse(m.SourceURL)
		if err != nil {
			return fmt.Errorf("manifest: sourceUrl is not a valid URL: %w", err)
		}
		for key := range parsed.Query() {
			if credentialKeyPattern.MatchString(key) {
				return fmt.Errorf("manifest: sourceUrl must not contain credentials (query parameter %q)", key)
			}
		}
	}
	return nil
}

// ParseAcquisitionManifest decodes a JSON manifest (e.g. a multipart form
// field alongside the CSV) and validates it.
func ParseAcquisitionManifest(r io.Reader) (*AcquisitionManifest, error) {
	var m AcquisitionManifest
	if err := json.NewDecoder(r).Decode(&m); err != nil {
		return nil, fmt.Errorf("decode manifest: %w", err)
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return &m, nil
}

// DocumentMetadata returns base plus the manifest serialized under
// MetadataAcquisitionManifest and the file hash under MetadataDatasetHash.
// The whole manifest is stored as one JSON value rather than flattened, so
// adding a manifest field never requires a metadata migration.
func (m AcquisitionManifest) DocumentMetadata(base map[string]string) map[string]string {
	out := make(map[string]string, len(base)+2)
	for k, v := range base {
		out[k] = v
	}
	if m.FileHash != "" {
		out[MetadataDatasetHash] = m.FileHash
	}
	if encoded, err := json.Marshal(m); err == nil {
		out[MetadataAcquisitionManifest] = string(encoded)
	}
	return out
}

// ManifestFromDocument recovers the manifest an importer attached to doc.
// ok is false for documents imported without one.
func ManifestFromDocument(doc *domain.Document) (AcquisitionManifest, bool) {
	if doc == nil {
		return AcquisitionManifest{}, false
	}
	raw, ok := doc.Metadata[MetadataAcquisitionManifest]
	if !ok || raw == "" {
		return AcquisitionManifest{}, false
	}
	var m AcquisitionManifest
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return AcquisitionManifest{}, false
	}
	return m, true
}

// --- comparability ---

type CompatibilityCode string

const (
	CompatibilityUnitMismatch              CompatibilityCode = "unit_mismatch"
	CompatibilityPopulationMismatch        CompatibilityCode = "population_mismatch"
	CompatibilityPeriodGranularityMismatch CompatibilityCode = "period_granularity_mismatch"
	CompatibilitySchemaVersionMismatch     CompatibilityCode = "schema_version_mismatch"
)

// DatasetCompatibilityWarning says two datasets in the same project cannot
// be compared naively. It is a warning, not a verdict: harmonising 2021 and
// 2024 populations (Issue #12) is legitimate work, but it must be visible
// in the run rather than silently absorbed into a delta.
type DatasetCompatibilityWarning struct {
	Code       CompatibilityCode `json:"code"`
	Detail     string            `json:"detail"`
	DatasetIDs []string          `json:"datasetIds"`
}

// CheckDatasetCompatibility compares every pair of distinct datasets on
// unit, population scope, period granularity and schema id/version. Empty
// values are treated as unknown and never produce a warning - the absence
// of a declaration is a provenance gap, not evidence of incompatibility.
func CheckDatasetCompatibility(manifests []AcquisitionManifest) []DatasetCompatibilityWarning {
	byID := map[string]AcquisitionManifest{}
	var ids []string
	for _, m := range manifests {
		if _, seen := byID[m.DatasetID]; seen {
			continue
		}
		byID[m.DatasetID] = m
		ids = append(ids, m.DatasetID)
	}
	sort.Strings(ids)

	var warnings []DatasetCompatibilityWarning
	for i := 0; i < len(ids); i++ {
		for j := i + 1; j < len(ids); j++ {
			a, b := byID[ids[i]], byID[ids[j]]
			pair := []string{a.DatasetID, b.DatasetID}
			if mismatch(a.Unit, b.Unit) {
				warnings = append(warnings, DatasetCompatibilityWarning{Code: CompatibilityUnitMismatch, DatasetIDs: pair,
					Detail: fmt.Sprintf("unit %q (%s) differs from %q (%s); counts are not directly comparable", a.Unit, a.DatasetID, b.Unit, b.DatasetID)})
			}
			if mismatch(a.PopulationScope, b.PopulationScope) {
				warnings = append(warnings, DatasetCompatibilityWarning{Code: CompatibilityPopulationMismatch, DatasetIDs: pair,
					Detail: fmt.Sprintf("population scope %q (%s) differs from %q (%s); a delta would mix denominators", a.PopulationScope, a.DatasetID, b.PopulationScope, b.DatasetID)})
			}
			ga, gb := periodGranularity(a.Period), periodGranularity(b.Period)
			if ga != "unknown" && gb != "unknown" && ga != gb {
				warnings = append(warnings, DatasetCompatibilityWarning{Code: CompatibilityPeriodGranularityMismatch, DatasetIDs: pair,
					Detail: fmt.Sprintf("period %q (%s) is %s-level while %q (%s) is %s-level", a.Period, a.DatasetID, ga, b.Period, b.DatasetID, gb)})
			}
			if a.SchemaID == b.SchemaID && mismatch(a.SchemaVersion, b.SchemaVersion) {
				warnings = append(warnings, DatasetCompatibilityWarning{Code: CompatibilitySchemaVersionMismatch, DatasetIDs: pair,
					Detail: fmt.Sprintf("schema %s version %q (%s) differs from %q (%s); definitions may have changed between versions", a.SchemaID, a.SchemaVersion, a.DatasetID, b.SchemaVersion, b.DatasetID)})
			}
		}
	}
	return warnings
}

func mismatch(a, b string) bool {
	a, b = strings.TrimSpace(a), strings.TrimSpace(b)
	return a != "" && b != "" && !strings.EqualFold(a, b)
}

var (
	yearPattern  = regexp.MustCompile(`^\d{4}$`)
	monthPattern = regexp.MustCompile(`^\d{4}-\d{2}$`)
	dayPattern   = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
)

// periodGranularity classifies a period label by the first date token it
// contains ("2021", "2021..2024", "2026-01/2026-06"). Anything else is
// unknown and therefore never flagged.
func periodGranularity(period string) string {
	first := strings.TrimSpace(period)
	for _, sep := range []string{"..", "/", " to ", "〜", "~"} {
		if idx := strings.Index(first, sep); idx >= 0 {
			first = strings.TrimSpace(first[:idx])
		}
	}
	switch {
	case dayPattern.MatchString(first):
		return "day"
	case monthPattern.MatchString(first):
		return "month"
	case yearPattern.MatchString(first):
		return "year"
	default:
		return "unknown"
	}
}
