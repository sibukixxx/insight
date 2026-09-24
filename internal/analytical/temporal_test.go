package analytical

import (
 "encoding/json"
 "math"
 "os"
 "reflect"
 "testing"

 "insight-lab/internal/domain"
)

func temporalFixture(t *testing.T) Artifact {
 t.Helper()
 b,err:=os.ReadFile("../../contracts/analytical-artifact/v1/fixtures/temporal-japan-eu.json")
 if err!=nil { t.Fatal(err) }
 a,err:=Import(b)
 if err!=nil { t.Fatal(err) }
 return *a
}

func temporalObservations(t *testing.T) []domain.Observation {
 t.Helper()
 cs,err:=ToCandidatesForAnalysis(temporalFixture(t),"analysis-1")
 if err!=nil { t.Fatal(err) }
 out:=make([]domain.Observation,len(cs))
 for i,c:=range cs { out[i]=c.Observation }
 return out
}

func hasWarning(d ObservationDelta,code string) bool {
 for _,w:=range d.Warnings { if w.Code==code { return true } }
 return false
}

func TestTemporalJapanEUThreeWindows(t *testing.T) {
 observations:=temporalObservations(t)
 ds:=CompareObservationSeries(observations)
 if len(ds)!=2 { t.Fatalf("deltas: %d",len(ds)) }
 expected:=[]struct{Absolute,Relative float64}{{15,.15},{6,6.0/115}}
 for i,d:=range ds {
  if !d.Valid||d.Absolute==nil||d.Relative==nil||*d.Absolute!=expected[i].Absolute||math.Abs(*d.Relative-expected[i].Relative)>1e-14||d.Direction!="increase" {
   t.Fatalf("delta %d: %+v",i,d)
  }
  if d.Previous.AnalysisID!="analysis-1"||d.Current.AnalysisID!="analysis-1" { t.Fatal("lost run ownership") }
  if d.SourceChanges||len(d.ChangedDimensions)>0||len(d.DataDefinitionChanges)>0 { t.Fatal("false definition change") }
  if d.Baseline.Spec.Reference!="queries/export_value_by_year.sql"||d.Comparison.Datasets[0].ID!="trade"||len(d.Comparison.Provenance)!=2 {
   t.Fatal("lost dataset/query/source provenance")
  }
 }
 if !hasWarning(ds[1],"QUALITY_FLAGS_PRESENT")||len(ds[1].Comparison.Result.Quality)!=1 { t.Fatal("lost provisional quality") }
 first,_:=json.Marshal(ds)
 second,_:=json.Marshal(CompareObservationSeries(temporalObservations(t)))
 if string(first)!=string(second) { t.Fatal("non-deterministic result") }
}

func TestTemporalIncompatibilitiesSuppressArithmetic(t *testing.T) {
 tests:=[]struct{
  name,warning string
  mutate func(*domain.Observation)
 }{
  {"population","INCOMPATIBLE_POPULATION",func(o *domain.Observation){o.Temporal.Population.Description="Different population"}},
  {"definition","INCOMPATIBLE_METRIC",func(o *domain.Observation){o.Temporal.Metric.Description="Net instead of gross"}},
  {"version","INCOMPATIBLE_METRIC",func(o *domain.Observation){o.Temporal.Metric.Version="v2"}},
  {"unit","INCOMPATIBLE_METRIC",func(o *domain.Observation){o.Temporal.Metric.Unit="EUR"}},
  {"basis","INCOMPATIBLE_PERIOD",func(o *domain.Observation){o.Temporal.Result.Period.Basis="fiscal-year"}},
  {"overlap","INCOMPATIBLE_PERIOD",func(o *domain.Observation){o.Temporal.Result.Period=Period{Start:"2022",End:"2023",Basis:"calendar-year"}}},
  {"length","INCOMPATIBLE_PERIOD",func(o *domain.Observation){o.Temporal.Result.Period.End="2024"}},
  {"invalid_date","INCOMPATIBLE_PERIOD",func(o *domain.Observation){o.Temporal.Result.Period.Start="unknown"}},
  {"dimension","INCOMPATIBLE_DIMENSIONS",func(o *domain.Observation){o.Temporal.Result.Dimensions["destination"]="US"}},
  {"geography","INCOMPATIBLE_GEOGRAPHY",func(o *domain.Observation){o.Temporal.Result.Temporal.Geography="JP→US"}},
  {"real","INCOMPATIBLE_VALUE_BASIS",func(o *domain.Observation){o.Temporal.Result.Temporal.ValueBasis="real"}},
  {"origin","INCOMPATIBLE_ORIGIN",func(o *domain.Observation){o.Temporal.Result.Temporal.Origin="derived"}},
  {"query","INCOMPATIBLE_SPEC",func(o *domain.Observation){o.Temporal.Spec.Hash.Value="bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}},
  {"filter","INCOMPATIBLE_FILTERS",func(o *domain.Observation){o.Temporal.Filters["hsChapter"]="10"}},
  {"ownership","UNKNOWN_OBSERVATION_OWNERSHIP",func(o *domain.Observation){o.AnalysisID=""}},
  {"provenance","INVALID_PROVENANCE",func(o *domain.Observation){o.Temporal.Provenance=nil}},
 }
 for _,tt:=range tests { t.Run(tt.name,func(t *testing.T){
  obs:=temporalObservations(t);tt.mutate(&obs[1])
  d:=CompareObservations(obs[0],obs[1])
  if d.Valid||d.Absolute!=nil||d.Relative!=nil||d.Direction!="unknown"||!hasWarning(d,tt.warning) { t.Fatalf("%+v",d) }
 }) }
}

func TestMissingIsNotZero(t *testing.T) {
 for _,raw:=range []string{"","null","\"0\"","true"} {
  obs:=temporalObservations(t);obs[1].Temporal.Result.Value=json.RawMessage(raw)
  d:=CompareObservations(obs[0],obs[1])
  if d.Valid||d.Absolute!=nil { t.Fatalf("non-number %q: %+v",raw,d) }
 }
 obs:=temporalObservations(t);obs[1].Temporal.Result.Missing=true
 if d:=CompareObservations(obs[0],obs[1]);d.Valid||d.Absolute!=nil { t.Fatal("missing coerced to value") }
 obs=temporalObservations(t);obs[0].Temporal.Result.Value=json.RawMessage("0")
 d:=CompareObservations(obs[0],obs[1])
 if !d.Valid||d.Absolute==nil||*d.Absolute!=115||d.Relative!=nil||!hasWarning(d,"ZERO_BASELINE") { t.Fatalf("%+v",d) }
 obs=temporalObservations(t);obs[0].Temporal.Result.Value=json.RawMessage("-100")
 d=CompareObservations(obs[0],obs[1])
 if !d.Valid||*d.Relative!=2.15 { t.Fatalf("negative baseline: %+v",d) }
}

func TestTemporalSourceChangeAndSnapshotIsolation(t *testing.T) {
 a:=temporalFixture(t)
 cs,err:=ToCandidatesForAnalysis(a,"analysis-1")
 if err!=nil { t.Fatal(err) }
 a.Datasets[0].Version="modified"
 a.Results[0].Dimensions["destination"]="US"
 if cs[0].Observation.Temporal.Datasets[0].Version=="modified"||cs[0].Observation.Temporal.Result.Dimensions["destination"]=="US" { t.Fatal("mutable artifact leaked") }
 obs:=temporalObservations(t);obs[1].Temporal.Datasets[0].Version="revised"
 d:=CompareObservations(obs[0],obs[1])
 if !d.Valid||!d.SourceChanges||!hasWarning(d,"SOURCE_CHANGED") { t.Fatalf("%+v",d) }
 if d.Baseline.Datasets[0].Version==d.Comparison.Datasets[0].Version { t.Fatal("old provenance overwritten") }
}

func TestTemporalLegacyCompatibilityAndRunIDs(t *testing.T) {
 a:=fixture(t)
 cs,err:=ToCandidates(a)
 if err!=nil { t.Fatal(err) }
 if cs[0].Observation.Temporal!=nil||cs[0].Evidence.Temporal!=nil { t.Fatal("invented legacy temporal metadata") }
 d:=CompareObservations(cs[0].Observation,cs[1].Observation)
 if d.Valid||!hasWarning(d,"MISSING_TEMPORAL_EVIDENCE") { t.Fatalf("%+v",d) }
 encoded,_:=json.Marshal(cs[0].Observation)
 var legacy map[string]any
 _=json.Unmarshal(encoded,&legacy)
 if _,ok:=legacy["temporal"];ok { t.Fatal("legacy wire shape changed") }
 a=temporalFixture(t)
 first,err:=ToCandidatesForAnalysis(a,"first")
 if err!=nil { t.Fatal(err) }
 second,err:=ToCandidatesForAnalysis(a,"second")
 if err!=nil { t.Fatal(err) }
 if first[0].Observation.ID==second[0].Observation.ID||second[0].Observation.AnalysisID!="second" { t.Fatal("run identity collision") }
 if *second[0].Evidence.ObservationID!=second[0].Observation.ID||second[0].Evidence.Type!=domain.EvidenceNeutral { t.Fatal("evidence link or neutrality lost") }
 if _,err:=ToCandidatesForAnalysis(a,"");err==nil { t.Fatal("unowned run accepted") }
 if len(CompareObservationSeries(nil))!=0 { t.Fatal("empty series") }
}

func TestTemporalImportValidation(t *testing.T) {
 cases:=[]func(*Artifact){
  func(a *Artifact){a.Metrics[0].Version=""},
  func(a *Artifact){a.Results[0].Temporal.Origin="cause"},
  func(a *Artifact){a.Results[0].Temporal.ValueBasis="unknown"},
  func(a *Artifact){a.Results[0].Period.Start="2021"},
  func(a *Artifact){a.Results[0].Missing=true},
 }
 for i,mutate:=range cases {
  a:=temporalFixture(t);mutate(&a)
  if err:=a.Validate();err==nil { t.Fatalf("case %d accepted",i) }
 }
 a:=temporalFixture(t)
 b,err:=Export(a);if err!=nil {t.Fatal(err)}
 got,err:=Import(b);if err!=nil {t.Fatal(err)}
 if !reflect.DeepEqual(a,*got) { t.Fatal("temporal round-trip differs") }
}

func TestTemporalDirectionAndCalendarMonths(t *testing.T) {
 obs:=temporalObservations(t)
 obs[0].Temporal.Result.Period=Period{Start:"2024-01",End:"2024-01",Basis:"calendar-month"}
 obs[1].Temporal.Result.Period=Period{Start:"2024-02",End:"2024-02",Basis:"calendar-month"}
 obs[1].Temporal.Result.Value=json.RawMessage("90")
 d:=CompareObservations(obs[0],obs[1])
 if !d.Valid||d.Direction!="decrease"||*d.Absolute!=-10 {t.Fatalf("%+v",d)}
 obs[1].Temporal.Result.Value=json.RawMessage("100")
 if d:=CompareObservations(obs[0],obs[1]);!d.Valid||d.Direction!="unchanged" {t.Fatalf("%+v",d)}
 obs[1].Temporal.Result.Value=json.RawMessage("1e400")
 if d:=CompareObservations(obs[0],obs[1]);d.Valid {t.Fatal("overflow accepted")}
}

func TestTemporalSnapshotRejectsUnserializableProvenance(t *testing.T) {
 a:=temporalFixture(t)
 a.Parameters["invalid"]=make(chan int)
 if _,err:=ToCandidatesForAnalysis(a,"analysis");err==nil {t.Fatal("snapshot silently discarded provenance")}
}
