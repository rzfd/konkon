package store

import "time"

// Case is an operational case record.
type Case struct {
	CaseID      string
	Title       string
	Summary     string
	Service     string
	Severity    string
	Status      string
	SOPID       *int64
	SOPVersion  *int
	Reporter    string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	ResolvedAt  *time.Time
	SOPSlug     string
	SOPTitle    string
}

// CaseStep is one checklist step for a case.
type CaseStep struct {
	ID               int64
	CaseID           string
	StepNo           int
	Title            string
	RequiresEvidence bool
	DoneAt           *time.Time
	DoneBy           string
	Notes            string
	EvidenceURL      string
}

// CaseAttachment is an uploaded file linked to a case.
type CaseAttachment struct {
	ID           int64
	CaseID       string
	Kind         string
	FilePath     string
	OriginalName string
	CreatedAt    time.Time
}

// SOP is a standard operating procedure definition.
type SOP struct {
	ID         int64
	Slug       string
	Title      string
	Version    int
	Owner      string
	StepsJSON  string
	CreatedAt  time.Time
}

// SOPRule maps intake fields to an SOP.
type SOPRule struct {
	ID          int64
	SOPID       int64
	Priority    int
	Service     string
	Keyword     string
	SeverityMin string
}

// SOPStepDef is one step inside sop.steps_json.
type SOPStepDef struct {
	Title            string `json:"title"`
	RequiresEvidence bool   `json:"requires_evidence"`
}

// CaseAudit records an event on a case.
type CaseAudit struct {
	ID        int64
	CaseID    string
	Actor     string
	Action    string
	Detail    string
	CreatedAt time.Time
}

// CaseFilter holds optional filters and pagination for ListCases.
type CaseFilter struct {
	Status   string
	Severity string
	Service  string
	Search   string
	Page     int
	Limit    int
}
