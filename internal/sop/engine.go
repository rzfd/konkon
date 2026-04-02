package sop

import (
	"strings"

	"github.com/rzfd/metatech/konkon/internal/store"
)

// Intake is rule-matching input for a new case.
type Intake struct {
	Title    string
	Summary  string
	Service  string
	Severity string
}

// Pick returns the first matching SOP id by rule priority (rules must be ordered priority DESC).
func Pick(rules []store.SOPRule, in Intake) (sopID int64, matched bool) {
	for _, r := range rules {
		if matchRule(r, in) {
			return r.SOPID, true
		}
	}
	return 0, false
}

func matchRule(r store.SOPRule, in Intake) bool {
	if r.Service != "" {
		svc := strings.ToLower(strings.TrimSpace(in.Service))
		needle := strings.ToLower(strings.TrimSpace(r.Service))
		if svc == "" || !strings.Contains(svc, needle) {
			return false
		}
	}
	if r.Keyword != "" {
		hay := strings.ToLower(strings.TrimSpace(in.Title + " " + in.Summary))
		needle := strings.ToLower(strings.TrimSpace(r.Keyword))
		if !strings.Contains(hay, needle) {
			return false
		}
	}
	if r.SeverityMin != "" {
		cr, okc := severityRank(in.Severity)
		mr, okm := severityRank(r.SeverityMin)
		if !okc || !okm {
			return false
		}
		// Lower rank = worse; match if case is at least as severe as threshold.
		if cr > mr {
			return false
		}
	}
	return true
}

// severityRank maps P1 (worst) .. P4 (best) to 1..4.
func severityRank(s string) (int, bool) {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "P1":
		return 1, true
	case "P2":
		return 2, true
	case "P3":
		return 3, true
	case "P4":
		return 4, true
	default:
		return 0, false
	}
}
