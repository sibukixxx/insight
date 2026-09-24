package publicengine

// Data Triage wire types (#92) join the schema drift check.
func init() {
	for name, value := range map[string]any{
		"DatasetRef": DatasetRef{}, "CreateDatasetProfileRequest": CreateDatasetProfileRequest{}, "ColumnProfile": ColumnProfile{},
		"DatasetProfile": DatasetProfile{}, "TriageGapRef": TriageGapRef{}, "TriageRequest": TriageRequest{},
		"PlanProposer": PlanProposer{}, "VariableDecision": VariableDecision{}, "PlanMove": PlanMove{},
		"SelectionPlan": SelectionPlan{}, "SelectionPlanList": SelectionPlanList{}, "ReviseSelectionPlanRequest": ReviseSelectionPlanRequest{},
	} {
		wireTypes[name] = value
	}
}
