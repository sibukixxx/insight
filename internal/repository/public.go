package repository

import (
	"context"
	"errors"
	"time"

	"insight-lab/internal/domain"
)

// ErrConflict means a unique identity already exists.
var ErrConflict = errors.New("conflict")

// PublicSubject maps an opaque external subject of the Public Engine
// Contract to the project that holds its evidence. Namespace, ExternalID and
// Type are stored and echoed, never interpreted.
type PublicSubject struct {
	ProjectID  string
	Namespace  string
	ExternalID string
	Type       string
	Metadata   map[string]string
	CreatedAt  time.Time
}

// EngineState identifies one persisted engine state (database). StateID is
// opaque and generated once when the state is initialized.
type EngineState struct {
	StateID   string
	CreatedAt time.Time
}

// IdempotentResponse is the first successful response to an idempotency key,
// replayed verbatim for every retry with the same request.
type IdempotentResponse struct {
	Key         string
	RequestHash string
	StatusCode  int
	Body        []byte
	CreatedAt   time.Time
}

type PublicRepository interface {
	// CreateSubject creates the project and its subject mapping atomically.
	// It returns ErrConflict when the (namespace, external ID) pair exists.
	CreateSubject(ctx context.Context, project *domain.Project, subject *PublicSubject) error
	GetSubject(ctx context.Context, projectID string) (*PublicSubject, error)
	FindSubject(ctx context.Context, namespace, externalID string) (*PublicSubject, error)
	GetResponse(ctx context.Context, key string) (*IdempotentResponse, error)
	// SaveResponse stores a response; it returns ErrConflict when the key
	// already has one.
	SaveResponse(ctx context.Context, response *IdempotentResponse) error
}
