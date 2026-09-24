package triage

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
)

// Input is the bounded information a triager may see: never raw rows.
type Input struct {
	Question      string
	HypothesisIDs []string
	Gaps          []GapRef
	Profile       Profile
}

// MaxDimensionCardinality bounds categorical columns treated as dimensions.
const MaxDimensionCardinality = 50

var periodNames = map[string]bool{"year": true, "month": true, "date": true, "period": true, "fy": true, "quarter": true, "week": true, "day": true}

// Deterministic is the rule-based triager. It needs no model and never
// proposes causal classifications; it only includes, defers or asks for
// review, so ResearchGap-driven re-triage can pick deferred variables up.
func Deterministic(in Input) []Decision {
	q := tokens(in.Question)
	gapTokens := map[string][]string{}
	for _, g := range in.Gaps {
		for t := range tokens(g.Need + " " + strings.Join(g.RequiredDimensions, " ")) {
			gapTokens[t] = append(gapTokens[t], g.GapID)
		}
	}
	var out []Decision
	for _, c := range in.Profile.Columns {
		name := tokens(c.Name)
		gaps := linkedGaps(name, gapTokens)
		d := Decision{Name: c.Name}
		switch {
		case c.Type == TypeEmpty:
			d.Bucket, d.Classifications, d.Rationale = BucketNeedsReview, []string{"DEFINITION_UNKNOWN"}, "column has no non-null values"
		case c.Type == TypeDate || (c.Type == TypeInteger && anyKey(name, periodNames)):
			d.Bucket, d.Role, d.Rationale = BucketInclude, RolePeriod, "date-like column usable as the period axis"
		case c.Type == TypeInteger || c.Type == TypeNumber:
			d.Role = RoleMetric
			if overlaps(q, name) || len(gaps) > 0 || len(q) == 0 {
				d.Bucket, d.Rationale = BucketInclude, "numeric column whose name matches the research question or a research gap"
			} else {
				d.Bucket, d.Rationale = BucketDefer, "numeric column without a question or gap match; deferred, not dropped"
			}
		case c.Type == TypeString && in.Profile.RowCount > MaxDimensionCardinality && c.DistinctCount >= in.Profile.RowCount:
			d.Bucket, d.Role, d.Rationale = BucketDefer, RoleIdentifier, "unique per row; likely an identifier"
		case (c.Type == TypeString || c.Type == TypeBoolean) && c.DistinctCount <= MaxDimensionCardinality && !c.DistinctCapped:
			d.Bucket, d.Role, d.Classifications = BucketInclude, RoleDimension, []string{"COMPARISON_CANDIDATE"}
			d.Rationale = fmt.Sprintf("low-cardinality categorical column (%d distinct values)", c.DistinctCount)
		default:
			d.Bucket, d.Rationale = BucketNeedsReview, "high-cardinality text needs human judgement"
		}
		if len(gaps) > 0 {
			d.LinkedGapIDs = gaps
			if d.Bucket != BucketInclude && c.Type != TypeEmpty {
				d.Bucket, d.Rationale = BucketInclude, "re-proposed because research gap(s) "+strings.Join(gaps, ", ")+" reference this variable"
			}
		}
		out = append(out, d)
	}
	return out
}

func linkedGaps(name map[string]bool, gapTokens map[string][]string) []string {
	seen := map[string]bool{}
	var out []string
	for t := range name {
		for g, ids := range gapTokens {
			if !sameStem(t, g) {
				continue
			}
			for _, id := range ids {
				if !seen[id] {
					seen[id] = true
					out = append(out, id)
				}
			}
		}
	}
	sort.Strings(out)
	return out
}

func tokens(s string) map[string]bool {
	out := map[string]bool{}
	for _, f := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
		if len([]rune(f)) >= 2 {
			out[f] = true
		}
	}
	return out
}

func overlaps(question, name map[string]bool) bool {
	for k := range name {
		if question[k] {
			return true
		}
		for q := range question {
			// Languages without word boundaries: allow substring matches.
			if len([]rune(k)) >= 2 && strings.Contains(q, k) {
				return true
			}
		}
	}
	return false
}

func anyKey(name map[string]bool, keys map[string]bool) bool {
	for k := range name {
		if keys[k] {
			return true
		}
	}
	return false
}

// sameStem matches equal tokens, or tokens of at least four characters where
// one is a prefix of the other ("import" / "imports").
func sameStem(a, b string) bool {
	if a == b {
		return true
	}
	if len(a) < 4 || len(b) < 4 {
		return false
	}
	return strings.HasPrefix(a, b) || strings.HasPrefix(b, a)
}
