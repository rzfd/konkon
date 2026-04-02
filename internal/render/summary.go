package render

import (
	"fmt"
	"strings"
	"time"

	"github.com/rzfd/metatech/konkon/internal/store"
)

// Markdown builds a case summary from checklist audit data.
func Markdown(c *store.Case, steps []store.CaseStep) string {
	var b strings.Builder
	b.WriteString("# Ringkasan case\n\n")
	b.WriteString(fmt.Sprintf("- **ID**: %s\n", c.CaseID))
	b.WriteString(fmt.Sprintf("- **Judul**: %s\n", c.Title))
	if c.Summary != "" {
		b.WriteString(fmt.Sprintf("- **Ringkasan**: %s\n", c.Summary))
	}
	if c.Service != "" {
		b.WriteString(fmt.Sprintf("- **Layanan**: %s\n", c.Service))
	}
	if c.Severity != "" {
		b.WriteString(fmt.Sprintf("- **Severity**: %s\n", c.Severity))
	}
	b.WriteString(fmt.Sprintf("- **Status**: %s\n", c.Status))
	if c.SOPSlug != "" {
		b.WriteString(fmt.Sprintf("- **SOP**: %s (v%d) — %s\n", c.SOPSlug, derefVer(c.SOPVersion), c.SOPTitle))
	}
	b.WriteString(fmt.Sprintf("- **Dibuat**: %s\n", fmtTime(c.CreatedAt)))
	b.WriteString(fmt.Sprintf("- **Diperbarui**: %s\n", fmtTime(c.UpdatedAt)))
	if c.ResolvedAt != nil {
		b.WriteString(fmt.Sprintf("- **Selesai**: %s\n", fmtTime(*c.ResolvedAt)))
	}
	b.WriteString("\n## Timeline checklist\n\n")
	for _, st := range steps {
		line := fmt.Sprintf("%d. **%s**", st.StepNo, st.Title)
		if st.DoneAt != nil {
			line += fmt.Sprintf(" — selesai %s", fmtTime(*st.DoneAt))
			if st.DoneBy != "" {
				line += fmt.Sprintf(" oleh `%s`", st.DoneBy)
			}
		} else {
			line += " — belum selesai"
		}
		b.WriteString(line + "\n")
		if strings.TrimSpace(st.Notes) != "" {
			b.WriteString(fmt.Sprintf("   - catatan: %s\n", st.Notes))
		}
		if strings.TrimSpace(st.EvidenceURL) != "" {
			b.WriteString(fmt.Sprintf("   - bukti: <%s>\n", strings.TrimSpace(st.EvidenceURL)))
		}
	}
	return b.String()
}

func derefVer(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}

func fmtTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.UTC().Format(time.RFC3339)
}

// HTML wraps markdown-like content in minimal HTML (escape + pre-like blocks).
func HTML(c *store.Case, steps []store.CaseStep) string {
	md := Markdown(c, steps)
	esc := htmlEscape(md)
	return "<!DOCTYPE html><html><head><meta charset=\"utf-8\"><title>" + htmlEscape(c.CaseID) + "</title></head><body><pre style=\"white-space:pre-wrap;font-family:system-ui,sans-serif\">" + esc + "</pre></body></html>"
}

func htmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}
