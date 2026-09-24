// Package execution defines ExecutionProfile (Issue #91): LIGHT / STANDARD /
// HEAVY / AUTO as a resource/runtime strategy for the same Research Engine.
//
// It is a separate axis from AnalysisMode (how input is interpreted),
// ResearchStage (where research is) and ExecutionMode (deterministic or
// model-backed). A profile decides only how inputs are prepared and executed;
// it never changes hypothesis, validation or readiness semantics.
package execution

import (
	"errors"
	"fmt"

	"insight-lab/internal/input"
)

type Profile string

const (
	ProfileAuto     Profile = "AUTO"
	ProfileLight    Profile = "LIGHT"
	ProfileStandard Profile = "STANDARD"
	ProfileHeavy    Profile = "HEAVY"
)

// StrategyVersion is recorded with every resolution; bump it when the
// planner's rules change.
const StrategyVersion = "execution-planner/v1"

// ErrProfileUnavailable means the requested (or AUTO-selected) profile cannot
// run on this engine. The engine fails instead of silently downgrading.
var ErrProfileUnavailable = errors.New("execution profile unavailable")

// ErrInvalidProfile means the caller named an unknown profile.
var ErrInvalidProfile = errors.New("invalid execution profile")

func Parse(s string) (Profile, error) {
	switch p := Profile(s); p {
	case "":
		return ProfileAuto, nil
	case ProfileAuto, ProfileLight, ProfileStandard, ProfileHeavy:
		return p, nil
	}
	return "", fmt.Errorf("%w: %q (want LIGHT, STANDARD, HEAVY or AUTO)", ErrInvalidProfile, s)
}

// Capabilities describe what this engine instance can execute.
type Capabilities struct {
	// HeavyRuntime is true when a Heavy Execution Adapter is configured.
	HeavyRuntime bool
}

// ProfileInfo is advertised in EngineInfo.
type ProfileInfo struct {
	Profile     Profile `json:"profile"`
	Available   bool    `json:"available"`
	Description string  `json:"description"`
}

func (c Capabilities) Profiles() []ProfileInfo {
	return []ProfileInfo{
		{ProfileLight, true, "in-process; inline documents and prepared Analytical Artifacts only"},
		{ProfileStandard, true, "in-process; streams and prepares referenced raw artifacts with bounded memory"},
		{ProfileHeavy, c.HeavyRuntime, "prepares referenced raw artifacts through the configured Heavy Execution Adapter (partitioned, resumable)"},
		{ProfileAuto, true, "selects LIGHT, STANDARD or HEAVY from input shape; never downgrades"},
	}
}

// Resolution is recorded in the run's execution snapshot.
type Resolution struct {
	Requested       Profile     `json:"requested"`
	Resolved        Profile     `json:"resolved"`
	Reason          string      `json:"reason"`
	StrategyVersion string      `json:"strategyVersion"`
	Shape           input.Shape `json:"inputShape"`
}

// Planner holds the AUTO thresholds.
type Planner struct {
	LightMaxInlineBytes int64
	LightMaxDocuments   int
	StandardMaxRawBytes int64
	Capabilities        Capabilities
}

func DefaultPlanner(c Capabilities) Planner {
	return Planner{LightMaxInlineBytes: 8 << 20, LightMaxDocuments: 2000, StandardMaxRawBytes: 1 << 30, Capabilities: c}
}

// Resolve picks the concrete profile. It is deterministic in (requested,
// shape, capabilities).
func (p Planner) Resolve(requested Profile, shape input.Shape) (Resolution, error) {
	r := Resolution{Requested: requested, StrategyVersion: StrategyVersion, Shape: shape}
	unavailable := func(format string, args ...any) (Resolution, error) {
		return Resolution{}, fmt.Errorf("%w: %s", ErrProfileUnavailable, fmt.Sprintf(format, args...))
	}
	switch requested {
	case ProfileLight:
		if shape.RawToPrepare > 0 {
			return unavailable("LIGHT does not prepare referenced raw artifacts (%d pending); use STANDARD or HEAVY, or submit a prepared Analytical Artifact", shape.RawToPrepare)
		}
		r.Resolved, r.Reason = ProfileLight, "requested explicitly"
	case ProfileStandard:
		r.Resolved, r.Reason = ProfileStandard, "requested explicitly"
	case ProfileHeavy:
		if !p.Capabilities.HeavyRuntime {
			return unavailable("HEAVY requires a configured Heavy Execution Adapter")
		}
		r.Resolved, r.Reason = ProfileHeavy, "requested explicitly"
	case ProfileAuto:
		switch {
		case shape.RawToPrepare == 0 && shape.InlineBytes <= p.LightMaxInlineBytes && shape.Documents <= p.LightMaxDocuments:
			r.Resolved = ProfileLight
			r.Reason = fmt.Sprintf("AUTO: %d documents / %d inline bytes within LIGHT limits and no raw artifact to prepare", shape.Documents, shape.InlineBytes)
		case shape.RawToPrepareByte <= p.StandardMaxRawBytes:
			r.Resolved = ProfileStandard
			r.Reason = fmt.Sprintf("AUTO: %d raw artifacts (%d bytes) to prepare, %d inline bytes; within STANDARD limits", shape.RawToPrepare, shape.RawToPrepareByte, shape.InlineBytes)
		case p.Capabilities.HeavyRuntime:
			r.Resolved = ProfileHeavy
			r.Reason = fmt.Sprintf("AUTO: %d raw bytes to prepare exceed the STANDARD limit of %d", shape.RawToPrepareByte, p.StandardMaxRawBytes)
		default:
			return unavailable("AUTO selected HEAVY for %d raw bytes, but no Heavy Execution Adapter is configured", shape.RawToPrepareByte)
		}
	default:
		return Resolution{}, fmt.Errorf("%w: %q", ErrInvalidProfile, requested)
	}
	return r, nil
}
