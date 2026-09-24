// Command minimal is the smallest end-to-end use of the Go SDK: create an
// opaque subject, add evidence, run an analysis and print what it observed.
//
//	go run ./example/minimal -engine http://127.0.0.1:8787
//
// Research runs (hypotheses, gaps, data requirements) additionally need an
// engine with a model configured.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	insight "github.com/sibukixxx/insight/sdk/go"
)

func main() {
	engineURL := flag.String("engine", "http://127.0.0.1:8787", "Insight Lab base URL")
	flag.Parse()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	client := insight.NewClient(*engineURL)

	info, err := client.GetEngine(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("engine %s (commit %s), contract %s\n", info.Engine.Version, info.Engine.Commit, info.ContractVersion)

	subject, err := client.CreateSubject(ctx, insight.CreateSubjectRequest{
		IdempotencyKey: "example-subject",
		Subject:        insight.SubjectRef{Namespace: "example", ID: "city-inquiries"},
	})
	if err != nil {
		log.Fatal(err)
	}
	_, err = client.AddEvidence(ctx, subject.SubjectID, insight.AddEvidenceRequest{
		IdempotencyKey: "example-evidence",
		Documents: []insight.EvidenceDocument{
			{ExternalRef: "jan", Source: "dataset", Content: "Dataset observation: 2026-01 Example City inquiries = 120.",
				Metadata: map[string]string{"record_count": "120", "period": "2026-01", "event_type": "inquiry", "location": "Example City"}},
			{ExternalRef: "feb", Source: "dataset", Content: "Dataset observation: 2026-02 Example City inquiries = 150.",
				Metadata: map[string]string{"record_count": "150", "period": "2026-02", "event_type": "inquiry", "location": "Example City"}},
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	started, err := client.StartAnalysis(ctx, subject.SubjectID, insight.StartAnalysisRequest{IdempotencyKey: "example-analysis"})
	if err != nil {
		log.Fatal(err)
	}
	run, err := client.WaitForAnalysis(ctx, subject.SubjectID, started.AnalysisID)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("run %s %s (%s), input %s\n", run.AnalysisID, run.Status, run.ExecutionMode, run.InputFingerprint)
	results, err := client.GetAnalysisResults(ctx, subject.SubjectID, run.AnalysisID)
	if err != nil {
		log.Fatal(err)
	}
	for _, o := range results.Observations {
		fmt.Printf("- [%s] %s\n", o.ExternalRef, o.Quote)
	}
}
