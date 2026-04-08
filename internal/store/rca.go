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
	ContributingFactors string   `json:"contributing_factors"` // also used for "Temuan Utama"
	CorrectiveActions   string   `json:"corrective_actions"`   // "Perbaikan yang diterapkan"
	PreventiveActions   string   `json:"preventive_actions"`   // "Pencegahan"

	// Extra narrative blocks used by the RCA_SAFARI_AUTH template.
	ActionItems  []string `json:"action_items"`  // short actionable items
	DetectionGap string   `json:"detection_gap"` // "Celah deteksi"
}

// Normalize trims empty items and caps list sizes to reasonable bounds.
func (r CaseRCA) Normalize() CaseRCA {
	if len(r.FiveWhys) > 12 {
		r.FiveWhys = r.FiveWhys[:12]
	}
	trimWhys := r.FiveWhys[:0]
	for _, w := range r.FiveWhys {
		if strings.TrimSpace(w) != "" {
			trimWhys = append(trimWhys, w)
		}
	}
	r.FiveWhys = trimWhys
	// Cap action items to a reasonable size and trim empties.
	if len(r.ActionItems) > 12 {
		r.ActionItems = r.ActionItems[:12]
	}
	trimmed := r.ActionItems[:0]
	for _, it := range r.ActionItems {
		if strings.TrimSpace(it) != "" {
			trimmed = append(trimmed, it)
		}
	}
	r.ActionItems = trimmed
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
	if len(r.ActionItems) > 0 {
		return true
	}
	if strings.TrimSpace(r.DetectionGap) != "" {
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
