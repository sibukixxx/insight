package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"insight-lab/internal/buildinfo"
	"insight-lab/internal/llm"
)

var routedSettings = Settings{BaseURL: "https://llm.example/v1", Model: "base-model", APIKey: "k"}

func TestResolveModelBindingsRejectsUnknownStagesAndDisallowedModels(t *testing.T) {
	allowed := []string{"small-model", "large-model"}
	tests := []struct {
		name     string
		settings Settings
		bindings map[string]string
		wantErr  error
	}{
		{"empty bindings keep settings model", routedSettings, nil, nil},
		{"allowed model on a known stage", routedSettings, map[string]string{"need_hypothesis": "large-model"}, nil},
		{"settings model is always allowed", routedSettings, map[string]string{"observation_extraction": "base-model"}, nil},
		{"unknown stage", routedSettings, map[string]string{"made_up_stage": "large-model"}, ErrInvalidModelBinding},
		{"model not allowed by the operator", routedSettings, map[string]string{"need_hypothesis": "other-model"}, ErrModelBindingUnavailable},
		{"no model configured at all", Settings{}, map[string]string{"need_hypothesis": "large-model"}, ErrModelBindingUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ResolveModelBindings(tt.settings, allowed, tt.bindings)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestExecutionSnapshotRecordsPerStageModelsAndChangesFingerprint(t *testing.T) {
	at := time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC)
	base, err := BuildExecutionSnapshot(routedSettings, "", buildinfo.Info{}, at)
	if err != nil {
		t.Fatal(err)
	}
	routed := routedSettings
	routed.StageModels = map[string]string{"need_hypothesis": "large-model"}
	snap, err := BuildExecutionSnapshot(routed, "", buildinfo.Info{}, at)
	if err != nil {
		t.Fatal(err)
	}
	if snap.ExecutionFingerprint == base.ExecutionFingerprint {
		t.Fatal("a different model binding must change the execution fingerprint")
	}
	got := map[string]string{}
	for _, b := range snap.LLM.Models {
		got[b.Stage] = b.Model
	}
	if got["need_hypothesis"] != "large-model" || got["observation_extraction"] != "base-model" || len(got) != len(pipelineLLMSteps()) {
		t.Fatalf("per-stage bindings = %v", got)
	}
}

type namedClient string

func (n namedClient) Generate(_ context.Context, req llm.GenerateRequest) (*llm.GenerateResponse, error) {
	return &llm.GenerateResponse{Content: []byte(`"` + string(n) + `"`)}, nil
}

func TestStageRouterSendsEachStageToItsBoundModel(t *testing.T) {
	routed := routedSettings
	routed.StageModels = map[string]string{"need_hypothesis": "large-model"}
	client := newStageRouter(routed, func(s Settings) llm.Client { return namedClient(s.Model) })
	for stage, want := range map[string]string{"need_hypothesis": `"large-model"`, "observation_extraction": `"base-model"`} {
		resp, err := client.Generate(context.Background(), llm.GenerateRequest{Schema: llm.Schema{Name: stage}})
		if err != nil || string(resp.Content) != want {
			t.Fatalf("%s routed to %s (%v), want %s", stage, resp.Content, err, want)
		}
	}
}
