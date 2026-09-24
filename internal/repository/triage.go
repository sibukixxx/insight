package repository

import (
	"context"
	"time"
)

// StoredDatasetProfile is a persisted Dataset Profile (#92). Body is the
// profile JSON; the repository does not interpret it.
type StoredDatasetProfile struct {
	ProfileID string
	ProjectID string
	Body      []byte
	CreatedAt time.Time
}

// StoredSelectionPlan is one immutable version of a Selection Plan.
type StoredSelectionPlan struct {
	PlanID       string
	ProjectID    string
	ProfileID    string
	Version      int
	ParentPlanID string
	Body         []byte
	CreatedAt    time.Time
}

type TriageRepository interface {
	CreateProfile(ctx context.Context, p *StoredDatasetProfile) error
	GetProfile(ctx context.Context, profileID string) (*StoredDatasetProfile, error)
	// CreatePlan returns ErrConflict when the (profile, version) exists.
	CreatePlan(ctx context.Context, p *StoredSelectionPlan) error
	GetPlan(ctx context.Context, planID string) (*StoredSelectionPlan, error)
	ListPlans(ctx context.Context, profileID string) ([]*StoredSelectionPlan, error)
}
