package validate

import (
	"strings"
	"testing"
	"time"

	"github.com/rzfd/metatech/konkon/internal/store"
)

func ptr(t time.Time) *time.Time { return &t }

func TestCloseCase_requiresSOP(t *testing.T) {
	c := &store.Case{CaseID: "x", Severity: "P4", SOPID: nil}
	errs := CloseCase(c, nil)
	if len(errs) == 0 {
		t.Fatal("expected error")
	}
}

func TestCloseCase_allStepsDone(t *testing.T) {
	sid := int64(1)
	v := 1
	c := &store.Case{CaseID: "x", Severity: "P4", SOPID: &sid, SOPVersion: &v}
	now := time.Now().UTC()
	steps := []store.CaseStep{
		{StepNo: 1, Title: "a", DoneAt: ptr(now), EvidenceURL: "https://x"},
		{StepNo: 2, Title: "b", RequiresEvidence: true, DoneAt: ptr(now), EvidenceURL: "https://y"},
	}
	errs := CloseCase(c, steps)
	if len(errs) != 0 {
		t.Fatalf("unexpected: %v", errs)
	}
}

func TestCloseCase_p1NeedsAnyEvidence(t *testing.T) {
	sid := int64(1)
	v := 1
	c := &store.Case{CaseID: "x", Severity: "P1", SOPID: &sid, SOPVersion: &v}
	now := time.Now().UTC()
	steps := []store.CaseStep{
		{StepNo: 1, Title: "a", DoneAt: ptr(now), RequiresEvidence: false},
	}
	errs := CloseCase(c, steps)
	found := false
	for _, e := range errs {
		if strings.Contains(strings.ToLower(e), "evidence") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected evidence error, got %v", errs)
	}
}
