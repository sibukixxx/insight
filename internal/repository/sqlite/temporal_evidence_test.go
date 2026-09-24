package sqlite

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"insight-lab/internal/analytical"
	"insight-lab/internal/domain"
)

func TestTemporalEvidencePersistenceAndLegacy(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	project, document := seedProjectAndDocument(t, db)
	analysis := &domain.Analysis{ID: "temporal-run", ProjectID: project.ID, Status: domain.AnalysisCompleted, CreatedAt: time.Now().UTC()}
	if err := NewAnalysisRepository(db).Create(ctx, analysis); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile("../../../contracts/analytical-artifact/v1/fixtures/temporal-japan-eu.json")
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := analytical.Import(b)
	if err != nil {
		t.Fatal(err)
	}
	artifact.Parameters["cohortId"] = json.Number("9007199254740993")
	artifact.Filters["threshold"] = json.Number("9007199254740993")
	candidates, err := analytical.ToCandidatesForAnalysis(*artifact, analysis.ID)
	if err != nil {
		t.Fatal(err)
	}
	observations := NewObservationRepository(db)
	var batch []*domain.Observation
	for i := range candidates {
		candidates[i].Observation.DocumentID = document.ID
		batch = append(batch, &candidates[i].Observation)
	}
	if err := observations.CreateBatch(ctx, batch); err != nil {
		t.Fatal(err)
	}
	loaded := make([]domain.Observation, len(batch))
	for i, o := range batch {
		got, err := observations.Get(ctx, o.ID)
		if err != nil {
			t.Fatal(err)
		}
		before, _ := json.Marshal(o.Temporal)
		after, _ := json.Marshal(got.Temporal)
		if string(before) != string(after) || got.AnalysisID != analysis.ID {
			t.Fatal("temporal snapshot or ownership lost")
		}
		loaded[i] = *got
	}
	ds := analytical.CompareObservationSeries(loaded)
	if len(ds) != 2 || !ds[0].Valid || *ds[0].Absolute != 15 {
		t.Fatalf("%+v", ds)
	}
	for name, list := range map[string]func() ([]*domain.Observation, error){
		"document": func() ([]*domain.Observation, error) { return observations.ListByDocument(ctx, document.ID) },
		"project":  func() ([]*domain.Observation, error) { return observations.ListByProject(ctx, project.ID) },
		"ids": func() ([]*domain.Observation, error) {
			return observations.ListByIDs(ctx, []string{batch[0].ID, batch[1].ID, batch[2].ID})
		},
	} {
		got, err := list()
		if err != nil || len(got) != 3 {
			t.Fatalf("%s: %v %v", name, got, err)
		}
		for _, o := range got {
			if o.Temporal == nil {
				t.Fatalf("%s lost temporal metadata", name)
			}
		}
	}
	insight := &domain.Insight{ID: "temporal-insight", ProjectID: project.ID, Title: "Candidate", CreatedAt: time.Now().UTC()}
	if err := NewInsightRepository(db).Create(ctx, insight); err != nil {
		t.Fatal(err)
	}
	evidence := candidates[0].Evidence
	evidence.InsightID = insight.ID
	evidence.DocumentID = document.ID
	er := NewEvidenceRepository(db)
	if err := er.CreateBatch(ctx, []*domain.Evidence{&evidence}); err != nil {
		t.Fatal(err)
	}
	ev, err := er.ListByInsight(ctx, insight.ID)
	if err != nil || len(ev) != 1 {
		t.Fatalf("%v %v", ev, err)
	}
	before, _ := json.Marshal(evidence.Temporal)
	after, _ := json.Marshal(ev[0].Temporal)
	if string(before) != string(after) || ev[0].Type != domain.EvidenceNeutral {
		t.Fatal("evidence lost temporal provenance or neutrality")
	}

	// NULL is the representation of pre-migration metadata; never backfill guesses.
	if _, err := db.ExecContext(ctx, "UPDATE observations SET temporal_evidence = NULL WHERE id = ?", batch[0].ID); err != nil {
		t.Fatal(err)
	}
	legacy, err := observations.Get(ctx, batch[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if legacy.Temporal != nil {
		t.Fatal("legacy metadata invented")
	}
	if d := analytical.CompareObservations(*legacy, loaded[1]); d.Valid || d.Absolute != nil {
		t.Fatal("legacy value treated as zero")
	}
	if _, err := db.ExecContext(ctx, "UPDATE evidence SET temporal_evidence = NULL WHERE id = ?", evidence.ID); err != nil {
		t.Fatal(err)
	}
	ev, err = er.ListByInsight(ctx, insight.ID)
	if err != nil || ev[0].Temporal != nil {
		t.Fatal("legacy evidence incompatible")
	}
}
