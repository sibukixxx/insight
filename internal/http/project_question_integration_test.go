package httpapi_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestProjectCreatedFromAQuestionAloneKeepsTheQuestion(t *testing.T) {
	rig := newExplorationRig(t, false) // saving a theme needs no model
	question := "地方都市で人口が減っているのに店舗が増える理由は？"
	rec := rig.do(t, http.MethodPost, "/api/projects", `{"researchQuestion":"`+question+`"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create from a question: %d %s", rec.Code, rec.Body.String())
	}
	var p struct {
		ID               string `json:"id"`
		Name             string `json:"name"`
		ResearchQuestion string `json:"researchQuestion"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &p)
	if p.Name == "" || p.ResearchQuestion != question {
		t.Fatalf("project = %+v", p)
	}
	rec = rig.do(t, http.MethodGet, "/api/projects/"+p.ID, "")
	p.ResearchQuestion = ""
	_ = json.Unmarshal(rec.Body.Bytes(), &p)
	if p.ResearchQuestion != question {
		t.Fatalf("question not persisted: %s", rec.Body.String())
	}
	if rec := rig.do(t, http.MethodPost, "/api/projects", `{}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("empty project: %d", rec.Code)
	}
}
