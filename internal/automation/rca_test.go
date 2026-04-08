package automation

import (
	"testing"
	"time"

	"github.com/rzfd/metatech/konkon/internal/store"
)

func TestHeuristicDraftBuildsTimelineAndActions(t *testing.T) {
	now := time.Date(2026, 4, 8, 10, 21, 0, 0, time.UTC)
	stepDoneAt := now.Add(9 * time.Minute)
	input := Input{
		Case: &store.Case{
			CaseID:    "OPS-20260408-001",
			Title:     "User login loop on Safari",
			Summary:   "User Safari diminta login ulang terus menerus setelah deploy.",
			Service:   "auth-gateway",
			Severity:  "high",
			Status:    "open",
			Reporter:  "nina",
			CreatedAt: now,
		},
		Steps: []store.CaseStep{
			{
				ID:               10,
				StepNo:           1,
				Title:            "Cek log autentikasi",
				RequiresEvidence: true,
			},
			{
				ID:               11,
				StepNo:           2,
				Title:            "Rollback config sesi",
				RequiresEvidence: true,
				DoneAt:           &stepDoneAt,
				DoneBy:           "nina",
				Notes:            "Rollback cookie config dan error login menurun.",
				EvidenceURL:      "https://grafana.local/auth",
			},
		},
		Audit: []store.CaseAudit{
			{
				Action:    "case_created",
				Detail:    "service=auth-gateway severity=high reporter=nina",
				CreatedAt: now,
			},
			{
				Action:    "sop_assigned",
				Detail:    "sop=auth-login v2",
				CreatedAt: now.Add(2 * time.Minute),
			},
		},
	}

	draft := heuristicDraft(input)
	if draft.Source != "heuristic" {
		t.Fatalf("expected heuristic source, got %q", draft.Source)
	}
	if draft.RCA.RootCause == "" {
		t.Fatalf("expected root cause draft to be populated")
	}
	if draft.RCA.IncidentTimeline == "" {
		t.Fatalf("expected incident timeline to be populated")
	}
	if len(draft.RCA.ActionItems) == 0 {
		t.Fatalf("expected action items to be populated")
	}
	if draft.RCA.DetectionGap == "" {
		t.Fatalf("expected detection gap to be populated")
	}
}

func TestExtractJSONObjectHandlesCodeFence(t *testing.T) {
	got := extractJSONObject("```json\n{\"root_cause\":\"test\"}\n```")
	if got != "{\"root_cause\":\"test\"}" {
		t.Fatalf("unexpected json extraction: %q", got)
	}
}
