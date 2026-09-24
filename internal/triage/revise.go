package triage

import (
	"fmt"
	"strings"
)

// Move changes one variable's placement in a human revision. Revisions are
// the only way a stored plan changes, so every version is a reviewable diff
// and every move can be reversed by a later move.
type Move struct {
	Name            string   `json:"name"`
	FromBucket      Bucket   `json:"fromBucket,omitempty"`
	ToBucket        Bucket   `json:"toBucket"`
	Role            Role     `json:"role,omitempty"`
	Classifications []string `json:"classifications,omitempty"`
	Rationale       string   `json:"rationale"`
}

// Revise applies moves to base and returns the new decisions and the moves
// with FromBucket recorded. base is not modified.
func Revise(base []Decision, moves []Move) ([]Decision, []Move, error) {
	if len(moves) == 0 {
		return nil, nil, fmt.Errorf("%w: a revision needs at least one move", ErrInvalid)
	}
	out := append([]Decision(nil), base...)
	index := map[string]int{}
	for i, d := range out {
		index[d.Name] = i
	}
	applied := make([]Move, 0, len(moves))
	for i, m := range moves {
		pos, ok := index[strings.TrimSpace(m.Name)]
		if !ok {
			return nil, nil, fmt.Errorf("%w: moves[%d] references variable %q that is not in the plan", ErrInvalid, i, m.Name)
		}
		next := Decision{Name: out[pos].Name, Bucket: m.ToBucket, Role: m.Role, Classifications: m.Classifications, Rationale: m.Rationale, LinkedGapIDs: out[pos].LinkedGapIDs}
		if next.Role == "" {
			next.Role = out[pos].Role
		}
		if err := checkDecision(next); err != nil {
			return nil, nil, fmt.Errorf("%w: moves[%d] (%s): %v", ErrInvalid, i, m.Name, err)
		}
		m.Name, m.FromBucket = next.Name, out[pos].Bucket
		out[pos] = next
		applied = append(applied, m)
	}
	return out, applied, nil
}
