package publicengine

import "insight-lab/internal/service"

// ModelRouting advertises the per-run model binding extension point: the
// stages a caller may bind and the models the operator allowed.
type ModelRouting struct {
	Stages        []string `json:"stages"`
	AllowedModels []string `json:"allowedModels"`
}

// WithAllowedModels advertises operator-allowed models for per-stage
// bindings. The JobManager enforces the same list.
func WithAllowedModels(models []string) Option {
	return func(e *Engine) { e.allowedModels = append([]string(nil), models...) }
}

func (e *Engine) modelRouting() *ModelRouting {
	allowed := append([]string(nil), e.allowedModels...)
	if allowed == nil {
		allowed = []string{}
	}
	return &ModelRouting{Stages: service.ModelStages(), AllowedModels: allowed}
}

// WithModelBacked reports, from live settings, whether analyses on this
// engine are model-backed (a model endpoint is configured). Deterministic
// analyses never form hypotheses, so research runs over them are refused.
func WithModelBacked(configured func() bool) Option {
	return func(e *Engine) { e.modelBacked = configured }
}
