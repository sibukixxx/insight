package domain

import "testing"

func TestCustomerSpansDefaultsToWholeContent(t *testing.T) {
	d := &Document{Content: "こんにちは。"}
	spans := d.CustomerSpans()
	if len(spans) != 1 || spans[0].Start != 0 || spans[0].End != 6 || spans[0].Role != RoleCustomer {
		t.Fatalf("CustomerSpans = %+v, want one span covering 6 runes", spans)
	}
}

func TestCustomerSpansFiltersAndSorts(t *testing.T) {
	d := &Document{Content: "0123456789", Spans: []Span{
		{Start: 6, End: 10, Role: RoleCustomer},
		{Start: 0, End: 3, Role: RoleInterviewer},
		{Start: 3, End: 6, Role: RoleCustomer},
		{Start: 8, End: 8, Role: RoleCustomer}, // empty, dropped
	}}
	spans := d.CustomerSpans()
	if len(spans) != 2 || spans[0].Start != 3 || spans[1].Start != 6 {
		t.Fatalf("CustomerSpans = %+v, want the two customer spans in order", spans)
	}
}

func TestSituationAndProvenanceDefaults(t *testing.T) {
	d := &Document{Metadata: map[string]string{MetaRole: "経理担当", MetaCompanySize: "30名", MetaVolume: "月150件発行", "note": "x"}}
	if got := d.Situation(); got != "経理担当 / 30名 / 月150件発行" {
		t.Errorf("Situation = %q", got)
	}
	if (&Document{}).Situation() != "" {
		t.Error("Situation should be empty without reserved keys")
	}
	if DefaultProvenance(SourceSales) != ProvenanceSecondhand || DefaultProvenance(SourceInterview) != ProvenanceFirsthand {
		t.Error("DefaultProvenance: sales logs are secondhand, interviews firsthand")
	}
}
