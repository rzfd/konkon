package sop

import (
	"testing"

	"github.com/rzfd/metatech/konkon/internal/store"
)

func TestPick_paymentService(t *testing.T) {
	rules := []store.SOPRule{
		{SOPID: 2, Priority: 100, Service: "payment"},
		{SOPID: 1, Priority: 10},
	}
	id, ok := Pick(rules, Intake{Title: "x", Service: "payment-api", Severity: "P3"})
	if !ok || id != 2 {
		t.Fatalf("got %v %v want 2 true", id, ok)
	}
}

func TestPick_keywordTimeout(t *testing.T) {
	rules := []store.SOPRule{
		{SOPID: 2, Priority: 90, Keyword: "timeout"},
		{SOPID: 1, Priority: 10},
	}
	id, ok := Pick(rules, Intake{Title: "Gateway timeout", Summary: "502"})
	if !ok || id != 2 {
		t.Fatalf("got %v %v want 2 true", id, ok)
	}
}

func TestPick_catchAll(t *testing.T) {
	rules := []store.SOPRule{{SOPID: 1, Priority: 10}}
	id, ok := Pick(rules, Intake{Title: "anything"})
	if !ok || id != 1 {
		t.Fatalf("got %v %v want 1 true", id, ok)
	}
}

func TestPick_severityMin(t *testing.T) {
	rules := []store.SOPRule{{SOPID: 9, Priority: 50, SeverityMin: "P2"}}
	id, ok := Pick(rules, Intake{Title: "t", Severity: "P3"})
	if ok {
		t.Fatalf("P3 should not match P2 min, got sop %d", id)
	}
	_, ok2 := Pick(rules, Intake{Title: "t", Severity: "P2"})
	if !ok2 {
		t.Fatal("P2 should match")
	}
}
