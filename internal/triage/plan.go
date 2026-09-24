package triage

import (
	"fmt"
	"strings"
)

// Bucket places one variable in a Selection Plan. EXCLUDE never deletes
// source data; it only records that a human chose not to process a variable.
type Bucket string

const (
	BucketInclude     Bucket = "INCLUDE"
	BucketDefer       Bucket = "DEFER"
	BucketNeedsReview Bucket = "NEEDS_REVIEW"
	BucketExclude     Bucket = "EXCLUDE"
)

func (b Bucket) valid() bool {
	switch b {
	case BucketInclude, BucketDefer, BucketNeedsReview, BucketExclude:
		return true
	}
	return false
}

// Role is how deterministic processing would use a variable.
type Role string

const (
	RoleMetric     Role = "METRIC"
	RoleDimension  Role = "DIMENSION"
	RolePeriod     Role = "PERIOD"
	RoleIdentifier Role = "IDENTIFIER"
)

func (r Role) valid() bool {
	switch r {
	case "", RoleMetric, RoleDimension, RolePeriod, RoleIdentifier:
		return true
	}
	return false
}

// Classifications are candidate research roles. They are hypotheses about a
// variable, never causal findings.
var Classifications = []string{
	"POSSIBLE_OUTCOME", "POSSIBLE_EXPOSURE", "POSSIBLE_CONFOUNDER", "POSSIBLE_MEDIATOR",
	"POSSIBLE_COLLIDER", "COMPARISON_CANDIDATE", "DEFINITION_UNKNOWN",
}

func validClassification(c string) bool {
	for _, k := range Classifications {
		if k == c {
			return true
		}
	}
	return false
}

// ProposerKind records who produced a plan version.
type ProposerKind string

const (
	ProposerDeterministic ProposerKind = "DETERMINISTIC"
	ProposerModel         ProposerKind = "MODEL"
	ProposerHuman         ProposerKind = "HUMAN"
)

type Proposer struct {
	Kind  ProposerKind `json:"kind"`
	Actor string       `json:"actor,omitempty"`
	Model string       `json:"model,omitempty"`
}

// Decision is one variable's placement with its rationale.
type Decision struct {
	Name            string   `json:"name"`
	Bucket          Bucket   `json:"bucket"`
	Role            Role     `json:"role,omitempty"`
	Classifications []string `json:"classifications,omitempty"`
	Rationale       string   `json:"rationale"`
	LinkedGapIDs    []string `json:"linkedGapIds,omitempty"`
}

// GapRef is a ResearchGap or DataRequirement the selection should address.
type GapRef struct {
	GapID              string   `json:"gapId"`
	Need               string   `json:"need"`
	RequiredDimensions []string `json:"requiredDimensions,omitempty"`
}

// ProcessingBoundary is stated on every plan so consumers never mistake a
// selection for a computation.
const ProcessingBoundary = "A Selection Plan only selects variables. Numeric results come solely from deterministic processing of the source data; triage never computes evidence, never deletes source data and model-proposed relevance is not causal evidence."

const omittedRationale = "not classified by the triager; kept as NEEDS_REVIEW for later re-triage"

// Normalize enforces the audit invariant: every profiled column appears
// exactly once, in profile order. Unknown columns are rejected, omitted
// columns become NEEDS_REVIEW, the first placement of a duplicate wins and
// automated proposals can never EXCLUDE (downgraded to DEFER).
func Normalize(profile Profile, proposed []Decision, kind ProposerKind) ([]Decision, error) {
	known := map[string]bool{}
	for _, c := range profile.Columns {
		known[c.Name] = true
	}
	byName := map[string]Decision{}
	for i, d := range proposed {
		d.Name = strings.TrimSpace(d.Name)
		if !known[d.Name] {
			return nil, fmt.Errorf("%w: decisions[%d] references column %q that is not in the profile", ErrInvalid, i, d.Name)
		}
		if _, dup := byName[d.Name]; dup {
			continue
		}
		if err := checkDecision(d); err != nil {
			return nil, fmt.Errorf("%w: decisions[%d] (%s): %v", ErrInvalid, i, d.Name, err)
		}
		if d.Bucket == BucketExclude && kind != ProposerHuman {
			d.Bucket = BucketDefer
			d.Rationale += " [automated EXCLUDE downgraded to DEFER: only a human may exclude a variable]"
		}
		byName[d.Name] = d
	}
	out := make([]Decision, 0, len(profile.Columns))
	for _, c := range profile.Columns {
		d, ok := byName[c.Name]
		if !ok {
			d = Decision{Name: c.Name, Bucket: BucketNeedsReview, Classifications: []string{"DEFINITION_UNKNOWN"}, Rationale: omittedRationale}
		}
		out = append(out, d)
	}
	return out, nil
}

func checkDecision(d Decision) error {
	if !d.Bucket.valid() {
		return fmt.Errorf("unknown bucket %q", d.Bucket)
	}
	if !d.Role.valid() {
		return fmt.Errorf("unknown role %q", d.Role)
	}
	for _, c := range d.Classifications {
		if !validClassification(c) {
			return fmt.Errorf("unknown classification %q", c)
		}
	}
	if strings.TrimSpace(d.Rationale) == "" {
		return fmt.Errorf("rationale is required")
	}
	return nil
}
