package analytical

import (
 "bytes"
 "crypto/sha256"
 "encoding/hex"
 "encoding/json"
 "fmt"
 "math"
 "reflect"
 "sort"
 "strings"
 "time"

 "insight-lab/internal/domain"
)

// ObservationReference consumes AnalysisID from #82; no run identity is inferred.
type ObservationReference struct {
 ObservationID string `json:"observationId"`
 AnalysisID string `json:"analysisId,omitempty"`
 DocumentID string `json:"documentId"`
}

type ObservationDelta struct {
 Previous ObservationReference `json:"previous"`
 Current ObservationReference `json:"current"`
 Baseline *TemporalEvidence `json:"baseline,omitempty"`
 Comparison *TemporalEvidence `json:"comparison,omitempty"`
 Valid bool `json:"valid"`
 Absolute *float64 `json:"absoluteDelta,omitempty"`
 Relative *float64 `json:"relativeDelta,omitempty"`
 Unit string `json:"unit,omitempty"`
 Direction string `json:"direction"`
 ChangedDimensions []string `json:"changedDimensions,omitempty"`
 SourceChanges bool `json:"sourceChanges"`
 DataDefinitionChanges []string `json:"dataDefinitionChanges,omitempty"`
 Warnings []QualityFlag `json:"warnings,omitempty"`
 Limitations []string `json:"limitations"`
}

func temporalProjection(a Artifact, m MetricDefinition, r Result, index int) *TemporalEvidence {
 if r.Temporal == nil { return nil }
 if r.Period.Basis == "" { r.Period.Basis = a.Period.Basis }
 p := TemporalEvidence{
  ArtifactID:a.ID, ArtifactHash:a.ArtifactHash, ResultIndex:index,
  Metric:m, Result:r, Population:a.Population, Datasets:a.Datasets,
  Spec:a.Spec, Parameters:a.Parameters, Filters:a.Filters, Provenance:a.Provenance,
  Quality:a.Quality, GeneratedAt:a.GeneratedAt,
 }
 // Snapshots must not change if a caller later modifies the source artifact.
 data, _ := json.Marshal(p)
 var snapshot TemporalEvidence
 _ = json.Unmarshal(data, &snapshot)
 return &snapshot
}

// periodBounds accepts complete inclusive calendar windows. Unknown bases are
// rejected, never guessed from labels or compared by unequal second durations.
func periodBounds(p Period) (time.Time, time.Time, int, error) {
 var layout string
 switch p.Basis {
 case "calendar-year": layout = "2006"
 case "calendar-month": layout = "2006-01"
 case "calendar-day": layout = "2006-01-02"
 default: return time.Time{},time.Time{},0,fmt.Errorf("unsupported period basis %q",p.Basis)
 }
 s,e := time.Time{},time.Time{}
 var err error
 if s,err = time.Parse(layout,p.Start); err != nil { return s,e,0,err }
 if e,err = time.Parse(layout,p.End); err != nil { return s,e,0,err }
 if e.Before(s) { return s,e,0,fmt.Errorf("reversed period") }
 var count int
 switch p.Basis {
 case "calendar-year": count=e.Year()-s.Year()+1; e=e.AddDate(1,0,0)
 case "calendar-month": count=(e.Year()-s.Year())*12+int(e.Month())-int(s.Month())+1; e=e.AddDate(0,1,0)
 case "calendar-day": count=int(e.Sub(s).Hours()/24)+1; e=e.AddDate(0,0,1)
 }
 return s,e,count,nil
}

func validateTemporalResult(r Result, metrics []MetricDefinition, enclosing Period) error {
 t:=r.Temporal
 if t == nil { return nil }
 if t.ObservedAt.IsZero() || blank(t.Geography) { return fmt.Errorf("temporal observedAt and geography are required") }
 if t.Origin!="observed" && t.Origin!="derived" { return fmt.Errorf("origin must be observed or derived") }
 if t.ValueBasis!="nominal" && t.ValueBasis!="real" && t.ValueBasis!="not_applicable" {
  return fmt.Errorf("valueBasis must be nominal, real or not_applicable")
 }
 for _,m:=range metrics {
  if m.ID==r.MetricID && blank(m.Version) { return fmt.Errorf("temporal metric version is required") }
 }
 p:=r.Period
 if p.Basis=="" { p.Basis=enclosing.Basis }
 s,e,_,err:=periodBounds(p)
 if err!=nil { return err }
 as,ae,_,err:=periodBounds(enclosing)
 if err!=nil { return err }
 if s.Before(as)||e.After(ae) { return fmt.Errorf("result period outside artifact period") }
 return nil
}

func number(r Result) (float64,bool) {
 if r.Missing || len(r.Value)==0 || bytes.Equal(bytes.TrimSpace(r.Value),[]byte("null")) { return 0,false }
 var v any
 d:=json.NewDecoder(bytes.NewReader(r.Value)); d.UseNumber()
 if d.Decode(&v)!=nil { return 0,false }
 n,ok:=v.(json.Number)
 if !ok { return 0,false }
 x,err:=n.Float64()
 return x,err==nil&&!math.IsNaN(x)&&!math.IsInf(x,0)
}

func jsonEqual(a,b any) bool {
 x,xe:=json.Marshal(a); y,ye:=json.Marshal(b)
 return xe==nil && ye==nil && bytes.Equal(x,y)
}

func sortedSources(p *TemporalEvidence) []string {
 var sources []string
 for _,d:=range p.Datasets { b,_:=json.Marshal(d); sources=append(sources,"dataset:"+string(b)) }
 for _,s:=range p.Provenance { b,_:=json.Marshal(s); sources=append(sources,"source:"+string(b)) }
 sort.Strings(sources)
 return sources
}

// CompareObservations is pure and deterministic. It reports descriptive change,
// never a causal effect, anomaly explanation, insight or promotion decision.
// Relative delta is (current-previous)/abs(previous), a ratio, not a percentage.
func CompareObservations(previous,current domain.Observation) ObservationDelta {
 d:=ObservationDelta{
  Previous:ObservationReference{previous.ID,previous.AnalysisID,previous.DocumentID},
  Current:ObservationReference{current.ID,current.AnalysisID,current.DocumentID},
  Baseline:previous.Temporal, Comparison:current.Temporal, Direction:"unknown",
  Limitations:[]string{"Descriptive change is not a causal effect; correlation is not causation."},
 }
 warn:=func(code,message string) { d.Warnings=append(d.Warnings,QualityFlag{Code:code,Message:message}) }
 if blank(previous.ID)||blank(current.ID)||blank(previous.AnalysisID)||blank(current.AnalysisID) {
  warn("UNKNOWN_OBSERVATION_OWNERSHIP","Observation and Analysis identities must be recorded.")
 }
 p,c:=previous.Temporal,current.Temporal
 if p==nil||c==nil {
  warn("MISSING_TEMPORAL_EVIDENCE","Legacy or unrecorded temporal evidence is unknown, never zero.")
  return d
 }
 d.Unit=c.Metric.Unit
 for _,v:=range []*TemporalEvidence{p,c} {
  if err:=validateTemporalProvenance(v); err!=nil { warn("INVALID_PROVENANCE",err.Error()) }
 }
 definition:=func(name string,changed bool) {
  if changed { d.DataDefinitionChanges=append(d.DataDefinitionChanges,name); warn("INCOMPATIBLE_"+strings.ToUpper(name),"Changed "+name+" cannot be interpreted as real-world change.") }
 }
 definition("metric",p.Metric!=c.Metric || blank(p.Metric.Version)||blank(c.Metric.Version))
 definition("population",p.Population!=c.Population)
 definition("filters",!jsonEqual(p.Filters,c.Filters))
 definition("parameters",!jsonEqual(p.Parameters,c.Parameters))
 definition("spec",p.Spec!=c.Spec)
 pt,ct:=p.Result.Temporal,c.Result.Temporal
 if pt==nil||ct==nil {
  warn("MISSING_TEMPORAL_METADATA","Temporal metadata was not recorded.")
 } else {
  definition("geography",pt.Geography!=ct.Geography)
  definition("value_basis",pt.ValueBasis!=ct.ValueBasis)
  definition("origin",pt.Origin!=ct.Origin)
  for _,v:=range []*TemporalEvidence{p,c} {
   if err:=validateTemporalResult(v.Result,[]MetricDefinition{v.Metric},v.Result.Period);err!=nil {
    warn("INVALID_TEMPORAL_EVIDENCE",err.Error())
   }
  }
 }
 keys:=map[string]bool{}
 for k:=range p.Result.Dimensions { keys[k]=true }
 for k:=range c.Result.Dimensions { keys[k]=true }
 for k:=range keys {
  pv,pok:=p.Result.Dimensions[k]; cv,cok:=c.Result.Dimensions[k]
  if pok!=cok || pv!=cv { d.ChangedDimensions=append(d.ChangedDimensions,k) }
 }
 sort.Strings(d.ChangedDimensions)
 definition("dimensions",len(d.ChangedDimensions)>0)
 ps,pe,pn,perr:=periodBounds(p.Result.Period)
 cs,_,cn,cerr:=periodBounds(c.Result.Period)
 if perr!=nil||cerr!=nil||p.Result.Period.Basis!=c.Result.Period.Basis||pn!=cn||cs.Before(pe)||!cs.After(ps) {
  warn("INCOMPATIBLE_PERIOD","Periods must be ordered, non-overlapping, and use equal calendar basis and window counts.")
 }
 d.SourceChanges=!reflect.DeepEqual(sortedSources(p),sortedSources(c))
 if p.Result.MetricID!=p.Metric.ID||c.Result.MetricID!=c.Metric.ID {
  warn("INVALID_METRIC_REFERENCE","Result does not refer to its metric definition.")
 }
 pv,pok:=number(p.Result); cv,cok:=number(c.Result)
 if !pok||!cok {
  warn("MISSING_OR_NON_NUMERIC_VALUE","Missing, null and non-numeric values are not zero.")
 }
 // Incompatibilities suppress arithmetic completely.
 if len(d.Warnings)>0 { return d }
 absolute:=cv-pv
 if math.IsInf(absolute,0)||math.IsNaN(absolute) {
  warn("NON_FINITE_DELTA","The numeric delta cannot be represented."); return d
 }
 d.Valid=true; d.Absolute=&absolute; d.Direction="unchanged"
 if absolute>0 { d.Direction="increase" } else if absolute<0 { d.Direction="decrease" }
 if pv==0 {
  warn("ZERO_BASELINE","Relative delta is undefined for a zero baseline.")
 } else {
  relative:=absolute/math.Abs(pv)
  if math.IsInf(relative,0)||math.IsNaN(relative) {
   warn("NON_FINITE_RELATIVE_DELTA","Relative delta cannot be represented.")
  } else { d.Relative=&relative }
 }
 if d.SourceChanges { warn("SOURCE_CHANGED","Source or dataset provenance changed; verify comparability.") }
 if len(p.Quality)+len(c.Quality)+len(p.Result.Quality)+len(c.Result.Quality)>0 {
  warn("QUALITY_FLAGS_PRESENT","Review retained baseline and comparison quality flags.")
 }
 if ct!=nil && ct.ValueBasis=="nominal" {
  d.Limitations=append(d.Limitations,"Nominal change is not adjusted for inflation or exchange rates.")
 }
 return d
}

// CompareObservationSeries compares adjacent caller-ordered observations. It does
// not silently sort, select a latest run or discard incompatible windows.
func CompareObservationSeries(observations []domain.Observation) []ObservationDelta {
 out:=make([]ObservationDelta,0)
 for i:=1;i<len(observations);i++ { out=append(out,CompareObservations(observations[i-1],observations[i])) }
 return out
}

// ToCandidatesForAnalysis binds candidates to an existing #82 Analysis identity.
// It never invents an Analysis or infers ownership from artifact timestamps.
func ToCandidatesForAnalysis(artifact Artifact, analysisID string) ([]Candidate,error) {
 if blank(analysisID) { return nil,fmt.Errorf("analysis ID is required") }
 candidates,err:=ToCandidates(artifact)
 if err!=nil { return nil,err }
 for i:=range candidates {
  c:=&candidates[i]
  key,_:=json.Marshal([]string{analysisID,c.Observation.ID})
  hash:=sha256.Sum256(key)
  id:="analytical:"+hex.EncodeToString(hash[:])
  c.Observation.ID=id; c.Observation.AnalysisID=analysisID
  c.Evidence.ID=id; c.Evidence.ObservationID=&c.Observation.ID
 }
 return candidates,nil
}

func validateTemporalProvenance(p *TemporalEvidence) error {
 if blank(p.ArtifactID)||p.ResultIndex<0||p.GeneratedAt.IsZero() { return fmt.Errorf("artifact identity, result index and generation time are required") }
 if err:=validateHash("artifactHash",p.ArtifactHash);err!=nil { return err }
 if blank(p.Spec.Kind)||blank(p.Spec.Reference) { return fmt.Errorf("query/spec reference is required") }
 if err:=validateHash("spec.hash",p.Spec.Hash);err!=nil { return err }
 if len(p.Datasets)==0||len(p.Provenance)==0 { return fmt.Errorf("datasets and source provenance are required") }
 ids:=map[string]bool{}
 for _,d:=range p.Datasets {
  if blank(d.ID)||blank(d.Version)||ids[d.ID] { return fmt.Errorf("invalid dataset identity") }
  if err:=validateHash("dataset.hash",d.Hash);err!=nil { return err }
  ids[d.ID]=true
 }
 covered:=map[string]bool{}
 for _,source:=range p.Provenance {
  if !ids[source.DatasetID]||blank(source.Source)||source.RetrievedAt.IsZero() { return fmt.Errorf("invalid source provenance") }
  covered[source.DatasetID]=true
 }
 for id:=range ids { if !covered[id] { return fmt.Errorf("dataset has no source provenance") } }
 return nil
}
