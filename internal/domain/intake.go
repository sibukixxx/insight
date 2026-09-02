package domain

// IntakeProfile is what a project learned about how its raw data looks:
// which transcript labels map to which speaker role, which terms to mask,
// and how a spreadsheet's columns map onto the document contract. It is
// filled in as the user resolves the intake preview and reused as the
// default on the next import, so the per-client setup work happens once.
type IntakeProfile struct {
	// SpeakerRoles maps a speaker label as it appears in transcripts
	// ("Q", "面接官", "田中") to a role.
	SpeakerRoles map[string]SpeakerRole `json:"speakerRoles,omitempty"`
	// MaskTerms are project-specific strings (names, company names) to
	// mask at intake in addition to the built-in patterns.
	MaskTerms []string `json:"maskTerms,omitempty"`
	// ColumnMapping is the last spreadsheet mapping used for this project.
	ColumnMapping *ColumnMapping `json:"columnMapping,omitempty"`
}

// ColumnMapping says which spreadsheet columns feed which document
// fields. Column names are matched case-insensitively against the header
// row. MetadataColumns maps a column name to the metadata key it becomes
// (a reserved key such as "role", or a free-form key).
type ColumnMapping struct {
	ContentColumn   string            `json:"contentColumn"`
	TitleColumn     string            `json:"titleColumn,omitempty"`
	IDColumn        string            `json:"idColumn,omitempty"`
	SourceColumn    string            `json:"sourceColumn,omitempty"`
	DefaultSource   SourceType        `json:"defaultSource,omitempty"`
	Provenance      Provenance        `json:"provenance,omitempty"`
	MetadataColumns map[string]string `json:"metadataColumns,omitempty"`
}

// MergeSpeakerRoles records the roles the user assigned to labels so the
// next transcript in the same project gets them as defaults.
func (p *IntakeProfile) MergeSpeakerRoles(roles map[string]SpeakerRole) {
	if len(roles) == 0 {
		return
	}
	if p.SpeakerRoles == nil {
		p.SpeakerRoles = map[string]SpeakerRole{}
	}
	for label, role := range roles {
		if label != "" && role.Valid() {
			p.SpeakerRoles[label] = role
		}
	}
}
