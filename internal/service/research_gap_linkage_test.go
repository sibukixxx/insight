package service

import (
	"testing"

	"insight-lab/internal/domain"
)

func TestCarryForwardResearchGapsWithLinksResolvesOnlyTargetedGap(t *testing.T) {
	previous := domain.ResearchIteration{
		ID: "it-1",
		ResearchGaps: []domain.ResearchGap{
			{ID: "gap-a", Need: "comparison series", Resolvable: true},
			{ID: "gap-b", Need: "pre-period series", Resolvable: true},
		},
	}
	current := domain.ResearchIteration{ID: "it-2"}
	got := CarryForwardResearchGapsWithLinks(previous, current, []string{"comparison.csv"}, []domain.AddedEvidenceLink{
		{Reference: "comparison.csv", GapIDs: []string{"gap-a"}},
	})

	var a, b *domain.ResearchGap
	for i := range got.ResearchGaps {
		switch got.ResearchGaps[i].ID {
		case "gap-a":
			a = &got.ResearchGaps[i]
		case "gap-b":
			b = &got.ResearchGaps[i]
		}
	}
	if a == nil || !a.Resolved || a.AddressedInIterationID != "it-2" {
		t.Fatalf("targeted gap must resolve: %+v", a)
	}
	if b == nil || b.Resolved {
		t.Fatalf("untargeted gap must remain unresolved: %+v", b)
	}
}

func TestCarryForwardResearchGapsWithLinksDoesNotResolveUnknownGap(t *testing.T) {
	previous := domain.ResearchIteration{
		ID: "it-1",
		ResearchGaps: []domain.ResearchGap{{ID: "gap-a", Need: "comparison series", Resolvable: true}},
	}
	got := CarryForwardResearchGapsWithLinks(previous, domain.ResearchIteration{ID: "it-2"}, []string{"other.csv"}, []domain.AddedEvidenceLink{
		{Reference: "other.csv", GapIDs: []string{"gap-unknown"}},
	})
	if len(got.ResearchGaps) != 1 || got.ResearchGaps[0].Resolved {
		t.Fatalf("unknown linkage must not resolve a real gap: %+v", got.ResearchGaps)
	}
}
