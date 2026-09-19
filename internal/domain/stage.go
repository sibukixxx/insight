package domain

import (
	"errors"
	"fmt"
)

// ResearchStage names what kind of work a research iteration is doing. It
// separates generating explanations from testing them, so that an
// explanation found while exploring a dataset is never reported as if it
// had been validated on that same dataset.
type ResearchStage string

const (
	// StageDiscovery: turning unsorted input into observations, patterns
	// and questions.
	StageDiscovery ResearchStage = "DISCOVERY"
	// StageExploratory: generating expectations, hypotheses and alternative
	// explanations from data that has already been observed.
	StageExploratory ResearchStage = "EXPLORATORY"
	// StageValidation: testing frozen expectations and falsification
	// criteria against additional or independent evidence.
	StageValidation ResearchStage = "VALIDATION"
	// StageSynthesis: combining results across iterations, datasets and
	// methods while stating the uncertainty that remains.
	StageSynthesis ResearchStage = "SYNTHESIS"
)

func (s ResearchStage) Valid() bool {
	switch s {
	case StageDiscovery, StageExploratory, StageValidation, StageSynthesis:
		return true
	}
	return false
}

// FindingKind is how a result produced in a stage must be labelled for a
// reader. Anything produced outside VALIDATION/SYNTHESIS, including results
// from an unknown stage, is an exploratory finding.
type FindingKind string

const (
	FindingExploratory      FindingKind = "EXPLORATORY_FINDING"
	FindingValidationResult FindingKind = "VALIDATION_RESULT"
	FindingSynthesis        FindingKind = "SYNTHESIS"
)

func (s ResearchStage) FindingKind() FindingKind {
	switch s {
	case StageValidation:
		return FindingValidationResult
	case StageSynthesis:
		return FindingSynthesis
	}
	return FindingExploratory
}

var (
	ErrStageTransitionNotAllowed        = errors.New("research stage: transition not allowed")
	ErrStageRequiresObservations        = errors.New("research stage: EXPLORATORY requires at least one observation")
	ErrStageRequiresFrozenTarget        = errors.New("research stage: VALIDATION requires at least one frozen expectation")
	ErrStageRequiresIndependentEvidence = errors.New("research stage: VALIDATION requires an evidence source independent of the generating iteration")
	ErrStageRequiresValidationResults   = errors.New("research stage: SYNTHESIS requires at least one completed validation")
)

// StageTransitionInput carries the facts a forward transition is checked
// against. The application supplies them; the stage never infers them.
type StageTransitionInput struct {
	// ObservationCount is the number of grounded observations recorded.
	ObservationCount int
	// Expectations are the candidate validation targets. At least one must
	// be frozen, and every frozen one must satisfy Expectation.Validate.
	Expectations []Expectation
	// IndependentEvidencePlanned states that evidence not used to generate
	// the frozen expectations has been identified for the validation run.
	IndependentEvidencePlanned bool
	// CompletedValidationCount is the number of validation iterations whose
	// results have been recorded.
	CompletedValidationCount int
}

// Transition checks whether moving from s to the target stage is permitted.
// Forward moves are one step at a time and require the stated conditions;
// stepping back is always allowed because it never promotes a claim.
func (s ResearchStage) Transition(to ResearchStage, in StageTransitionInput) error {
	if !s.Valid() || !to.Valid() || s == to {
		return fmt.Errorf("%w: %q -> %q", ErrStageTransitionNotAllowed, s, to)
	}
	if stageOrder[to] < stageOrder[s] {
		return nil
	}
	switch {
	case s == StageDiscovery && to == StageExploratory:
		if in.ObservationCount < 1 {
			return ErrStageRequiresObservations
		}
		return nil
	case s == StageExploratory && to == StageValidation:
		return validationEntryRequirements(in)
	case s == StageValidation && to == StageSynthesis:
		if in.CompletedValidationCount < 1 {
			return ErrStageRequiresValidationResults
		}
		return nil
	}
	return fmt.Errorf("%w: %q -> %q", ErrStageTransitionNotAllowed, s, to)
}

var stageOrder = map[ResearchStage]int{
	StageDiscovery:   0,
	StageExploratory: 1,
	StageValidation:  2,
	StageSynthesis:   3,
}

func validationEntryRequirements(in StageTransitionInput) error {
	frozen := 0
	for _, e := range in.Expectations {
		if !e.FrozenForValidation {
			continue
		}
		if err := e.Validate(); err != nil {
			return fmt.Errorf("frozen expectation %q: %w", e.ID, err)
		}
		frozen++
	}
	if frozen == 0 {
		return ErrStageRequiresFrozenTarget
	}
	if !in.IndependentEvidencePlanned {
		return ErrStageRequiresIndependentEvidence
	}
	return nil
}
