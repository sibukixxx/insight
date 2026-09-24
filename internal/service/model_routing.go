package service

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"

	"insight-lab/internal/llm"
)

// Per-run model bindings are the minimal provider-neutral extension point for
// downstream routing policy (#65 moved routing policy to consumers): a caller
// may choose, per pipeline stage, among models the engine operator allowed on
// the configured endpoint. The engine never picks models by itself, and
// bindings never change research semantics — they are execution config and
// part of the execution fingerprint.
var (
	ErrInvalidModelBinding     = errors.New("invalid model binding")
	ErrModelBindingUnavailable = errors.New("model binding unavailable")
)

// ModelStages lists the stage names bindings may use: the pipeline's LLM
// step schema names, in pipeline order.
func ModelStages() []string {
	var out []string
	for _, step := range pipelineLLMSteps() {
		out = append(out, step.Schema().Name)
	}
	return out
}

// ResolveModelBindings validates caller bindings against the operator's
// allowed models (the configured model is always allowed).
func ResolveModelBindings(settings Settings, allowed []string, bindings map[string]string) (map[string]string, error) {
	if len(bindings) == 0 {
		return nil, nil
	}
	if !settings.Configured() {
		return nil, fmt.Errorf("%w: no model endpoint is configured", ErrModelBindingUnavailable)
	}
	stages := ModelStages()
	out := make(map[string]string, len(bindings))
	for stage, model := range bindings {
		if !slices.Contains(stages, stage) {
			return nil, fmt.Errorf("%w: unknown stage %q (known: %v)", ErrInvalidModelBinding, stage, stages)
		}
		if model != settings.Model && !slices.Contains(allowed, model) {
			return nil, fmt.Errorf("%w: model %q is not allowed by this engine", ErrModelBindingUnavailable, model)
		}
		out[stage] = model
	}
	return out, nil
}

// stageModelBindings returns the per-stage bindings recorded in the
// execution snapshot, in pipeline order.
func stageModelBindings(settings Settings) []ModelBinding {
	if len(settings.StageModels) == 0 {
		return []ModelBinding{{Stage: "all", Provider: openAICompatibleProvider, Model: settings.Model}}
	}
	var out []ModelBinding
	for _, stage := range ModelStages() {
		model := settings.Model
		if m, ok := settings.StageModels[stage]; ok {
			model = m
		}
		out = append(out, ModelBinding{Stage: stage, Provider: openAICompatibleProvider, Model: model})
	}
	return out
}

// stageRouter sends each generation to the client of its stage's model.
type stageRouter struct {
	byStage map[string]llm.Client
	def     llm.Client
}

func newStageRouter(settings Settings, newClient func(Settings) llm.Client) llm.Client {
	def := newClient(settings)
	if len(settings.StageModels) == 0 {
		return def
	}
	byModel := map[string]llm.Client{settings.Model: def}
	models := make([]string, 0, len(settings.StageModels))
	for _, m := range settings.StageModels {
		models = append(models, m)
	}
	sort.Strings(models)
	for _, m := range models {
		if _, ok := byModel[m]; !ok {
			s := settings
			s.Model, s.StageModels = m, nil
			byModel[m] = newClient(s)
		}
	}
	r := &stageRouter{byStage: map[string]llm.Client{}, def: def}
	for stage, m := range settings.StageModels {
		r.byStage[stage] = byModel[m]
	}
	return r
}

func (r *stageRouter) Generate(ctx context.Context, req llm.GenerateRequest) (*llm.GenerateResponse, error) {
	if c, ok := r.byStage[req.Schema.Name]; ok {
		return c.Generate(ctx, req)
	}
	return r.def.Generate(ctx, req)
}
