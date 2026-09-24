package publicengine

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"insight-lab/internal/llm"
	"insight-lab/internal/repository"
	"insight-lab/internal/triage"
)

// maxProfileCSVBytes bounds inline CSV sent for profiling.
const maxProfileCSVBytes = 8 << 20

// TriageModel returns a configured model client, or ok=false when none is
// configured. Model-backed triage is optional; deterministic triage always works.
type TriageModel func() (client llm.Client, model string, ok bool)

type triageDeps struct {
	repo  repository.TriageRepository
	model TriageModel
}

// EnableTriage turns on the Data Triage operations (#92).
func (e *Engine) EnableTriage(repo repository.TriageRepository, model TriageModel) {
	e.triage = &triageDeps{repo: repo, model: model}
}

func (e *Engine) triageEnabled() (*triageDeps, error) {
	if e.triage == nil {
		return nil, newError(CodeNotFound, "data triage is not enabled on this engine")
	}
	return e.triage, nil
}

// CreateDatasetProfile profiles CSV bytes, inline or from an existing
// evidence document of the subject. Only bounded metadata is stored.
func (e *Engine) CreateDatasetProfile(ctx context.Context, subjectID string, req CreateDatasetProfileRequest) (int, []byte, error) {
	deps, err := e.triageEnabled()
	if err != nil {
		return 0, nil, err
	}
	return e.idempotent(ctx, req.ContractVersion, req.IdempotencyKey, "createDatasetProfile:"+subjectID, req, func() (int, any, error) {
		subject, err := e.requireSubject(ctx, subjectID)
		if err != nil {
			return 0, nil, err
		}
		if strings.TrimSpace(req.Dataset.ID) == "" || strings.TrimSpace(req.Dataset.Version) == "" || len(req.Dataset.ID) > maxIdentifierLength || len(req.Dataset.Version) > maxIdentifierLength {
			return 0, nil, newError(CodeInvalidRequest, "dataset.id and dataset.version must be 1-%d characters", maxIdentifierLength)
		}
		data, err := e.profileInput(ctx, subject.ProjectID, req)
		if err != nil {
			return 0, nil, err
		}
		prof, err := triage.ProfileCSV(data)
		if err != nil {
			return 0, nil, newError(CodeInvalidRequest, "%v", err)
		}
		if req.Dataset.SHA256 != "" && !strings.EqualFold(req.Dataset.SHA256, prof.ContentSHA256) {
			return 0, nil, newError(CodeInvalidRequest, "dataset.sha256 %s does not match the profiled content %s", req.Dataset.SHA256, prof.ContentSHA256)
		}
		out := DatasetProfile{
			ContractVersion: ContractVersion, SubjectID: subjectID, ProfileID: newID("dprof"),
			Dataset: DatasetRef{ID: req.Dataset.ID, Version: req.Dataset.Version, SHA256: prof.ContentSHA256}, DocumentID: req.DocumentID,
			ContentSHA256: prof.ContentSHA256, RowCount: prof.RowCount, Columns: prof.Columns,
			ProfilerVersion: prof.ProfilerVersion, ProfileFingerprint: prof.Fingerprint(), CreatedAt: formatTime(e.now()),
		}
		body, err := json.Marshal(out)
		if err != nil {
			return 0, nil, err
		}
		if err := deps.repo.CreateProfile(ctx, &repository.StoredDatasetProfile{ProfileID: out.ProfileID, ProjectID: subject.ProjectID, Body: body, CreatedAt: e.now()}); err != nil {
			return 0, nil, err
		}
		return http.StatusCreated, out, nil
	})
}

func (e *Engine) profileInput(ctx context.Context, projectID string, req CreateDatasetProfileRequest) ([]byte, error) {
	switch {
	case req.CSV != "" && req.DocumentID != "":
		return nil, newError(CodeInvalidRequest, "send either csv or documentId, not both")
	case req.CSV != "":
		if len(req.CSV) > maxProfileCSVBytes {
			return nil, newError(CodeInvalidRequest, "inline csv is limited to %d bytes; add it as evidence and reference documentId", maxProfileCSVBytes)
		}
		return []byte(req.CSV), nil
	case req.DocumentID != "":
		doc, err := e.documents.Get(ctx, req.DocumentID)
		if errors.Is(err, repository.ErrNotFound) || (err == nil && doc.ProjectID != projectID) {
			return nil, newError(CodeNotFound, "document %q not found for this subject", req.DocumentID)
		}
		if err != nil {
			return nil, err
		}
		return []byte(doc.Content), nil
	}
	return nil, newError(CodeInvalidRequest, "csv or documentId is required")
}

// GetDatasetProfile returns a stored profile of the subject.
func (e *Engine) GetDatasetProfile(ctx context.Context, subjectID, profileID string) (DatasetProfile, error) {
	var out DatasetProfile
	deps, err := e.triageEnabled()
	if err != nil {
		return out, err
	}
	stored, err := deps.repo.GetProfile(ctx, profileID)
	if errors.Is(err, repository.ErrNotFound) || (err == nil && stored.ProjectID != subjectID) {
		return out, newError(CodeNotFound, "dataset profile %q not found", profileID)
	}
	if err != nil {
		return out, err
	}
	return out, json.Unmarshal(stored.Body, &out)
}

func profileOf(p DatasetProfile) triage.Profile {
	return triage.Profile{ContentSHA256: p.ContentSHA256, RowCount: p.RowCount, Columns: p.Columns, ProfilerVersion: p.ProfilerVersion}
}

// gapsFromRun collects unresolved ResearchGaps and DataRequirements of the
// latest iteration of a research run of the same subject.
func (e *Engine) gapsFromRun(ctx context.Context, subjectID, researchRunID string) ([]TriageGapRef, error) {
	run, err := e.publicResearchRun(ctx, researchRunID)
	if err != nil {
		return nil, err
	}
	if run.ProjectID != subjectID {
		return nil, newError(CodeNotFound, "research run %q not found for this subject", researchRunID)
	}
	iteration, ok := run.LatestIteration()
	if !ok {
		return nil, nil
	}
	var out []TriageGapRef
	for _, g := range iteration.ResearchGaps {
		if !g.Resolved {
			out = append(out, TriageGapRef{GapID: g.ID, Need: g.Need})
		}
	}
	for _, r := range iteration.DataRequirements {
		out = append(out, TriageGapRef{GapID: r.GapID, Need: r.Need, RequiredDimensions: r.RequiredDimensions})
	}
	return mergeGaps(nil, out), nil
}

// mergeGaps combines gap refs by gapId, keeping the first need and the union
// of required dimensions, in first-seen order.
func mergeGaps(a, b []TriageGapRef) []TriageGapRef {
	var out []TriageGapRef
	index := map[string]int{}
	for _, g := range append(append([]TriageGapRef(nil), a...), b...) {
		if i, ok := index[g.GapID]; ok {
			for _, d := range g.RequiredDimensions {
				if !contains(out[i].RequiredDimensions, d) {
					out[i].RequiredDimensions = append(out[i].RequiredDimensions, d)
				}
			}
			if out[i].Need == "" {
				out[i].Need = g.Need
			}
			continue
		}
		index[g.GapID] = len(out)
		out = append(out, g)
	}
	return out
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
