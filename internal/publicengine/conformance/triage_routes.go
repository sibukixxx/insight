package conformance

import "net/http"

// Data Triage operations (#92).
func init() {
	RegisterRoute("createDatasetProfile", http.MethodPost, "/api/public/v1/subjects/{subjectId}/dataset-profiles")
	RegisterRoute("getDatasetProfile", http.MethodGet, "/api/public/v1/subjects/{subjectId}/dataset-profiles/{profileId}")
	RegisterRoute("triage", http.MethodPost, "/api/public/v1/subjects/{subjectId}/dataset-profiles/{profileId}/triage")
	RegisterRoute("listSelectionPlans", http.MethodGet, "/api/public/v1/subjects/{subjectId}/dataset-profiles/{profileId}/selection-plans")
	RegisterRoute("getSelectionPlan", http.MethodGet, "/api/public/v1/selection-plans/{planId}")
	RegisterRoute("reviseSelectionPlan", http.MethodPost, "/api/public/v1/selection-plans/{planId}/revisions")
}
