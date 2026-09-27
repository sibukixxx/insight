package service

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"insight-lab/internal/domain"
)

// claimInspectionRuleVersion identifies the deterministic rules below. Bump
// it whenever linking, status derivation or flag meaning changes.
const claimInspectionRuleVersion = "claim-inspection/v1"

// metadataPublicExternalRef is the document metadata key under which the
// Public Engine records a consumer's externalRef.
const metadataPublicExternalRef = "public_external_ref"

// ClaimInspectionInput is the research state one iteration holds. Insights
// must carry their Evidence.
type ClaimInspectionInput struct {
	Claims       []domain.ResearchClaim
	Insights     []*domain.Insight
	Observations []*domain.Observation
	Documents    []*domain.Document
	ResearchGaps []domain.ResearchGap
	// Previous is the preceding iteration, when this is a re-inspection.
	Previous *domain.ResearchIteration
}

// Keyword hints. They are deliberately small and bilingual; a claim can
// avoid them with other wording, so an absent flag proves nothing.
var (
	causalWording  = regexp.MustCompile(`(?i)\b(caus(e|ed|es|ing)|because of|due to|led to|leads? to|lead to|result(ed|s)? in|drives?|drove|driven by|effect of|impact of|attributable to)\b|が原因|のせいで|によって|により|をもたらし|を引き起こ|効果があった|押し上げ`)
	generalWording = regexp.MustCompile(`(?i)\b(all|always|never|every|everyone|everybody|no one|nobody|universally|in every case)\b|すべて|全て|必ず|常に|誰もが|一切|例外なく`)
)

// InspectClaims inspects every claim against in. It returns an error only
// for an ambiguous request: one claim id reused with different content.
func InspectClaims(in ClaimInspectionInput) ([]domain.ClaimInspection, error) {
	byHash := map[string]string{}
	byID := map[string]string{}
	previous := map[string]domain.ClaimInspection{}
	if in.Previous != nil {
		for _, p := range in.Previous.ClaimInspections {
			previous[p.ClaimID] = p
		}
	}
	out := make([]domain.ClaimInspection, 0, len(in.Claims))
	for _, claim := range in.Claims {
		if err := claim.Validate(); err != nil {
			return nil, err
		}
		hash, err := claimContentHash(claim)
		if err != nil {
			return nil, err
		}
		if seen, ok := byID[claim.ID]; ok && seen != hash {
			return nil, fmt.Errorf("claim id %q is used for two different claims", claim.ID)
		}
		byID[claim.ID] = hash
		inspection := inspectClaim(in, claim)
		inspection.ContentHash = hash
		if first, ok := byHash[hash]; ok && first != claim.ID {
			inspection.DuplicateOf = first
		} else if !ok {
			byHash[hash] = claim.ID
		}
		if p, ok := previous[claim.ID]; ok {
			inspection.Previous = &domain.ClaimPreviousInspection{IterationID: in.Previous.ID, Status: p.Status, Changed: p.Status != inspection.Status}
		}
		out = append(out, inspection)
	}
	return out, nil
}

func claimContentHash(c domain.ResearchClaim) (string, error) {
	sorted := func(v []string) []string {
		out := append([]string(nil), v...)
		sort.Strings(out)
		return out
	}
	return Fingerprint(struct {
		Statement            string   `json:"statement"`
		SourceReference      string   `json:"sourceReference,omitempty"`
		EvidenceReferences   []string `json:"evidenceReferences,omitempty"`
		HypothesisReferences []string `json:"hypothesisReferences,omitempty"`
		Assumptions          []string `json:"assumptions,omitempty"`
	}{strings.TrimSpace(c.Statement), strings.TrimSpace(c.SourceReference), sorted(c.EvidenceReferences), sorted(c.HypothesisReferences), c.Assumptions})
}

func inspectClaim(in ClaimInspectionInput, claim domain.ResearchClaim) domain.ClaimInspection {
	ins := domain.ClaimInspection{ClaimID: claim.ID, RuleVersion: claimInspectionRuleVersion}
	flag := func(code domain.ClaimFlagCode, detail string) {
		ins.Flags = append(ins.Flags, domain.ClaimFlag{Code: code, Detail: detail})
	}
	for _, a := range claim.Assumptions {
		if a = strings.TrimSpace(a); a != "" {
			ins.UnverifiedAssumptions = append(ins.UnverifiedAssumptions, a)
		}
	}

	// Resolve citations to existing observations and documents only.
	observations := map[string]*domain.Observation{}
	for _, o := range in.Observations {
		observations[o.ID] = o
	}
	documents := map[string]*domain.Document{}
	for _, d := range in.Documents {
		documents[d.ID] = d
		if ref := d.Metadata[metadataPublicExternalRef]; ref != "" {
			if _, taken := documents[ref]; !taken {
				documents[ref] = d
			}
		}
	}
	citedObs, citedDocs := map[string]bool{}, map[string]bool{}
	var citedDocList []*domain.Document
	var notFound []string
	for _, ref := range claim.EvidenceReferences {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			continue
		}
		c := domain.ClaimCitation{Reference: ref, State: domain.ClaimCitationNotFound}
		if o, ok := observations[ref]; ok {
			c.State, c.ObservationID, c.DocumentID = domain.ClaimCitationResolved, o.ID, o.DocumentID
			citedObs[o.ID] = true
		} else if d, ok := documents[ref]; ok {
			c.State, c.DocumentID = domain.ClaimCitationResolved, d.ID
			if !citedDocs[d.ID] {
				citedDocs[d.ID] = true
				citedDocList = append(citedDocList, d)
			}
		} else {
			notFound = append(notFound, ref)
		}
		ins.Citations = append(ins.Citations, c)
	}
	if len(ins.Citations) == 0 {
		flag(domain.ClaimFlagNoSourceCitation, "the claim cites no evidence; it stays a claim, never an observation")
	}
	if len(notFound) > 0 {
		flag(domain.ClaimFlagCitationNotFound, strings.Join(notFound, ", "))
		ins.CannotConclude = append(ins.CannotConclude, "Cited material "+strings.Join(notFound, ", ")+" is not in the subject's evidence; nothing it is said to show can be concluded here.")
	}

	// Link hypotheses: explicitly named ones, else every hypothesis whose
	// evidence the claim cites.
	cites := func(e domain.Evidence) bool {
		return (e.ObservationID != nil && citedObs[*e.ObservationID]) || citedDocs[e.DocumentID]
	}
	usesCited := map[string]bool{}
	insights := map[string]*domain.Insight{}
	for _, h := range in.Insights {
		if h == nil {
			continue
		}
		insights[h.ID] = h
		for _, e := range h.Evidence {
			if cites(e) {
				usesCited[h.ID] = true
			}
		}
	}
	var basis []*domain.Insight
	inBasis := map[string]bool{}
	if len(claim.HypothesisReferences) > 0 {
		var missing []string
		for _, id := range claim.HypothesisReferences {
			if h, ok := insights[id]; ok && !inBasis[id] {
				basis = append(basis, h)
				inBasis[id] = true
			} else if !ok {
				missing = append(missing, id)
			}
		}
		if len(missing) > 0 {
			flag(domain.ClaimFlagHypothesisNotFound, strings.Join(missing, ", "))
		}
	} else {
		for _, h := range in.Insights {
			if h != nil && usesCited[h.ID] {
				basis = append(basis, h)
				inBasis[h.ID] = true
			}
		}
	}
	for _, h := range in.Insights {
		if h != nil && !inBasis[h.ID] && usesCited[h.ID] {
			ins.CompetingHypotheses = append(ins.CompetingHypotheses, hypothesisLink(h))
		}
	}
	for _, h := range basis {
		ins.Basis = append(ins.Basis, hypothesisLink(h))
		for _, e := range h.Evidence {
			if !cites(e) {
				continue
			}
			ce := domain.ClaimEvidence{EvidenceID: e.ID, InsightID: h.ID, DocumentID: e.DocumentID, Quote: e.Quote, Type: e.Type}
			if e.ObservationID != nil {
				ce.ObservationID = *e.ObservationID
			}
			switch e.Type {
			case domain.EvidenceSupport:
				ins.SupportingEvidence = append(ins.SupportingEvidence, ce)
			case domain.EvidenceCounter:
				ins.CounterEvidence = append(ins.CounterEvidence, ce)
			default:
				ins.NeutralEvidence = append(ins.NeutralEvidence, ce)
			}
		}
		ins.RequiredEvidence = appendUnique(ins.RequiredEvidence, h.MissingEvidence...)
	}
	for _, g := range in.ResearchGaps {
		for _, id := range g.AffectedHypothesisIDs {
			if inBasis[id] {
				ins.ResearchGapIDs = appendUnique(ins.ResearchGapIDs, g.ID)
			}
		}
	}

	// Wording hints.
	identified := len(basis) > 0
	for _, h := range basis {
		if h.CausalStatus != domain.CausallySupported || h.IdentificationStatus != domain.IdentificationIdentified {
			identified = false
		}
	}
	causal := causalWording.MatchString(claim.Statement)
	if causal && !identified {
		flag(domain.ClaimFlagCausalWithoutID, "causal wording, but no hypothesis it relies on is causally identified")
		ins.CannotConclude = append(ins.CannotConclude, "The cited evidence does not identify a causal effect; the claim's causal wording cannot be concluded from it.")
	}
	if generalWording.MatchString(claim.Statement) {
		flag(domain.ClaimFlagOvergeneralization, "universal wording; the evidence covers only the cited cases")
	}
	var manifests []AcquisitionManifest
	for _, d := range citedDocList {
		if m, ok := ManifestFromDocument(d); ok {
			manifests = append(manifests, m)
		}
	}
	for _, w := range CheckDatasetCompatibility(manifests) {
		flag(domain.ClaimFlagDefinitionMismatch, string(w.Code)+": "+w.Detail)
	}

	ins.Status = claimStatus(basis, ins.CompetingHypotheses, causal && !identified, flag)
	// A claim that cites none of the evidence its hypotheses rest on only
	// restates them; it cannot be reported as supported on its own.
	if ins.Status == domain.ClaimSupported && len(ins.SupportingEvidence)+len(ins.CounterEvidence)+len(ins.NeutralEvidence) == 0 {
		ins.Status = domain.ClaimInsufficient
	}
	if ins.Status == domain.ClaimUnknown && len(notFound) == 0 {
		ins.CannotConclude = append(ins.CannotConclude, "No inspected hypothesis uses the cited evidence; this research state neither supports nor contradicts the claim.")
	}
	return ins
}

// claimStatus derives the status from existing validation states and never
// reports more than they allow.
func claimStatus(basis []*domain.Insight, competing []domain.ClaimHypothesisLink, causalOverreach bool, flag func(domain.ClaimFlagCode, string)) domain.ClaimStatus {
	if len(basis) == 0 {
		return domain.ClaimUnknown
	}
	supported, contradicted := 0, 0
	for _, h := range basis {
		switch h.ValidationStatus {
		case domain.ValidationSupported, domain.ValidationPartiallySupported:
			supported++
		case domain.ValidationContradicted:
			contradicted++
		}
	}
	switch {
	case contradicted == len(basis):
		return domain.ClaimContradicted
	case supported > 0 && contradicted > 0:
		flag(domain.ClaimFlagConflictingEvidence, "hypotheses the claim relies on are both supported and contradicted")
		return domain.ClaimInsufficient
	case supported < len(basis):
		return domain.ClaimInsufficient
	}
	var open []string
	for _, c := range competing {
		if c.ValidationStatus != domain.ValidationContradicted {
			open = append(open, c.InsightID)
		}
	}
	if len(open) > 0 {
		flag(domain.ClaimFlagCompetingExplanationOpen, strings.Join(open, ", "))
		return domain.ClaimInsufficient
	}
	if causalOverreach {
		return domain.ClaimInsufficient
	}
	return domain.ClaimSupported
}

func hypothesisLink(h *domain.Insight) domain.ClaimHypothesisLink {
	return domain.ClaimHypothesisLink{InsightID: h.ID, Title: h.Title, Role: h.HypothesisRole, CausalStatus: h.CausalStatus,
		ValidationStatus: h.ValidationStatus, IdentificationStatus: h.IdentificationStatus}
}
