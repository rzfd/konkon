package validate

import (
	"strings"

	"github.com/rzfd/metatech/konkon/internal/store"
)

// CloseCase checks whether a case can move to resolved.
func CloseCase(c *store.Case, steps []store.CaseStep) []string {
	var errs []string
	if c == nil {
		return []string{"case missing"}
	}
	if c.SOPID == nil {
		return []string{"hubungkan case ke SOP terlebih dahulu (triage)"}
	}
	sev := strings.ToUpper(strings.TrimSpace(c.Severity))
	high := sev == "P1" || sev == "P2"

	var anyEvidence bool
	for _, st := range steps {
		if strings.TrimSpace(st.EvidenceURL) != "" {
			anyEvidence = true
		}
		if st.DoneAt == nil && !st.Optional {
			errs = append(errs, "langkah belum selesai: "+st.Title)
			continue
		}
		if st.RequiresEvidence && st.DoneAt != nil && strings.TrimSpace(st.EvidenceURL) == "" {
			errs = append(errs, "bukti wajib untuk: "+st.Title)
		}
	}
	if len(steps) == 0 && c.SOPID != nil {
		errs = append(errs, "checklist kosong")
	}
	if high && !anyEvidence {
		errs = append(errs, "P1/P2 memerlukan minimal satu evidence_url pada salah satu langkah")
	}
	return errs
}
