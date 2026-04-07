package store

import (
	"encoding/json"
	"strings"
)

// CaseRCA holds optional Root Cause Analysis fields for export (PDF/HTML/MD) and UI.
type CaseRCA struct {
	IncidentTimeline    string   `json:"incident_timeline"`
	FiveWhys            []string `json:"five_whys"`
	RootCause           string   `json:"root_cause"`
	ContributingFactors string   `json:"contributing_factors"`
	CorrectiveActions   string   `json:"corrective_actions"`
	PreventiveActions   string   `json:"preventive_actions"`
}

// Normalize pads/truncates five_whys to exactly 5 entries.
func (r CaseRCA) Normalize() CaseRCA {
	for len(r.FiveWhys) < 5 {
		r.FiveWhys = append(r.FiveWhys, "")
	}
	if len(r.FiveWhys) > 5 {
		r.FiveWhys = r.FiveWhys[:5]
	}
	return r
}

// HasContent reports whether any RCA field is non-empty.
func (r CaseRCA) HasContent() bool {
	r = r.Normalize()
	if strings.TrimSpace(r.IncidentTimeline) != "" {
		return true
	}
	for _, w := range r.FiveWhys {
		if strings.TrimSpace(w) != "" {
			return true
		}
	}
	if strings.TrimSpace(r.RootCause) != "" {
		return true
	}
	if strings.TrimSpace(r.ContributingFactors) != "" {
		return true
	}
	if strings.TrimSpace(r.CorrectiveActions) != "" {
		return true
	}
	if strings.TrimSpace(r.PreventiveActions) != "" {
		return true
	}
	return false
}

// ParseCaseRCAJSON decodes rca_json column; invalid or empty JSON yields empty normalized RCA.
func ParseCaseRCAJSON(s string) CaseRCA {
	s = strings.TrimSpace(s)
	if s == "" {
		return CaseRCA{}.Normalize()
	}
	var r CaseRCA
	if err := json.Unmarshal([]byte(s), &r); err != nil {
		return CaseRCA{}.Normalize()
	}
	return r.Normalize()
}

// MarshalCaseRCAJSON encodes RCA for persistence.
func MarshalCaseRCAJSON(r CaseRCA) (string, error) {
	r = r.Normalize()
	b, err := json.Marshal(r)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
