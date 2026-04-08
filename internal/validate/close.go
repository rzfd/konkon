package validate

import (
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

	for _, st := range steps {
		if st.DoneAt == nil && !st.Optional {
			errs = append(errs, "langkah belum selesai: "+st.Title)
			continue
		}
	}
	if len(steps) == 0 && c.SOPID != nil {
		errs = append(errs, "checklist kosong")
	}
	return errs
}
