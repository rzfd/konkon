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
	b.WriteString("# Root Cause Analysis (RCA)\n\n")
	b.WriteString("_Konkon TechOps — ringkasan case_\n\n")
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
	b.WriteString("\n## Kronologi & checklist\n\n")
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

// HTML renders a high-contrast RCA layout aligned with the PDF export.
func HTML(c *store.Case, steps []store.CaseStep) string {
	var b strings.Builder
	title := htmlEscape(c.Title)
	caseID := htmlEscape(c.CaseID)
	b.WriteString("<!DOCTYPE html><html lang=\"id\"><head><meta charset=\"utf-8\"/><meta name=\"viewport\" content=\"width=device-width,initial-scale=1\"/>")
	b.WriteString("<title>" + caseID + " — RCA</title><style>")
	b.WriteString(`:root{--bg:#0a0e14;--banner:#121a26;--panel:#162030;--accent:#78c8ff;--text:#f8fafc;--muted:#94a3b8;--dim:#64748b;--ok:#48d597;--border:#1e3a5f;}
body{margin:0;background:var(--bg);color:var(--text);font:15px/1.5 system-ui,-apple-system,sans-serif;}
.wrap{max-width:820px;margin:0 auto;padding:28px 22px 48px;}
.banner{background:var(--banner);border-radius:12px;padding:20px 22px 18px;margin-bottom:22px;border:1px solid var(--border);position:relative;overflow:hidden;}
.banner::after{content:"";position:absolute;left:0;right:0;bottom:0;height:4px;background:var(--accent);}
.kicker{font-size:11px;font-weight:700;letter-spacing:.06em;color:var(--accent);text-transform:uppercase;margin:0 0 6px;}
.cid{font-size:13px;font-weight:700;font-family:ui-monospace,monospace;color:var(--accent);margin:0 0 4px;}
h1{font-size:1.45rem;font-weight:800;margin:0 0 12px;line-height:1.25;color:var(--text);}
.badges{display:flex;flex-wrap:wrap;gap:8px;align-items:center;}
.badge{display:inline-flex;align-items:center;padding:4px 10px;border-radius:6px;font-size:11px;font-weight:800;letter-spacing:.04em;text-transform:uppercase;}
.b-open{background:rgba(120,200,255,.15);color:var(--accent);border:1px solid rgba(120,200,255,.25);}
.b-res{background:rgba(72,213,151,.15);color:var(--ok);border:1px solid rgba(72,213,151,.25);}
.b-tri{background:rgba(250,204,21,.18);color:#fbbf24;border:1px solid rgba(250,204,21,.25);}
.b-def{background:rgba(100,116,139,.2);color:var(--muted);border:1px solid rgba(100,116,139,.25);}
.meta-h{font-size:11px;font-weight:700;letter-spacing:.1em;text-transform:uppercase;color:var(--muted);margin:0 0 12px;}
.panel{background:var(--panel);border:1px solid var(--border);border-radius:10px;padding:16px 18px;margin-bottom:20px;}
.grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(200px,1fr));gap:14px 22px;}
.meta dt{font-size:10px;font-weight:700;text-transform:uppercase;letter-spacing:.08em;color:var(--dim);margin:0 0 3px;}
.meta dd{margin:0;font-weight:600;color:var(--text);}
.sum-label{font-size:11px;font-weight:700;color:var(--accent);text-transform:uppercase;letter-spacing:.06em;margin:0 0 8px;}
.sum p{margin:0;color:var(--text);white-space:pre-wrap;}
.sec-h{display:flex;align-items:center;gap:10px;background:var(--banner);color:var(--text);font-size:12px;font-weight:800;letter-spacing:.06em;text-transform:uppercase;padding:8px 12px;border-radius:6px;margin:0 0 12px;border-left:4px solid var(--accent);}
.steps{list-style:none;padding:0;margin:0;}
.steps li{margin-bottom:10px;border-radius:8px;border:1px solid var(--border);overflow:hidden;background:var(--panel);}
.steps li:nth-child(even){background:#121b28;}
.sn{display:flex;gap:12px;padding:12px 14px;border-left:3px solid var(--dim);}
.steps li.done .sn{border-left-color:var(--ok);}
.num{flex-shrink:0;width:26px;height:26px;border-radius:50%;background:var(--dim);color:var(--text);font-size:11px;font-weight:800;display:flex;align-items:center;justify-content:center;}
.steps li.done .num{background:var(--ok);color:#0a0e14;}
.st-title{font-weight:700;font-size:14px;margin:0 0 4px;}
.st-meta{font-size:12px;font-style:italic;color:var(--ok);margin:0 0 4px;}
.st-note,.st-ev{font-size:12px;color:var(--muted);margin:0;}
.st-ev{color:var(--accent);font-style:italic;}
footer{margin-top:32px;padding-top:16px;border-top:1px solid var(--border);font-size:11px;color:var(--dim);display:flex;justify-content:space-between;flex-wrap:wrap;gap:8px;}`)
	b.WriteString("</style></head><body><div class=\"wrap\">")

	b.WriteString("<header class=\"banner\"><p class=\"kicker\">Konkon TechOps · Root Cause Analysis (RCA)</p>")
	b.WriteString("<p class=\"cid\">" + caseID + "</p>")
	b.WriteString("<h1>" + title + "</h1><div class=\"badges\">")
	b.WriteString(htmlStatusBadge(c.Status))
	if c.Severity != "" {
		b.WriteString("<span class=\"badge b-open\">" + htmlEscape(strings.ToUpper(c.Severity)) + "</span>")
	}
	if c.Service != "" {
		b.WriteString("<span class=\"badge b-def\">" + htmlEscape(c.Service) + "</span>")
	}
	b.WriteString("</div></header>")

	b.WriteString("<h2 class=\"meta-h\">Informasi insiden</h2><div class=\"panel\"><dl class=\"grid meta\">")
	writeMeta(&b, "ID case", c.CaseID)
	writeMeta(&b, "Reporter", nvlHTML(c.Reporter))
	writeMeta(&b, "Layanan", nvlHTML(c.Service))
	writeMeta(&b, "Severity", nvlHTML(c.Severity))
	writeMeta(&b, "Status", c.Status)
	if c.SOPSlug != "" {
		writeMeta(&b, "SOP", fmt.Sprintf("%s (v%d) — %s", c.SOPSlug, derefVer(c.SOPVersion), c.SOPTitle))
	}
	writeMeta(&b, "Dibuat", fmtTime(c.CreatedAt))
	writeMeta(&b, "Diperbarui", fmtTime(c.UpdatedAt))
	if c.ResolvedAt != nil {
		writeMeta(&b, "Selesai", fmtTime(*c.ResolvedAt))
	}
	b.WriteString("</dl></div>")

	if c.Summary != "" {
		b.WriteString("<div class=\"panel sum\"><p class=\"sum-label\">Ringkasan eksekutif</p><p>")
		b.WriteString(htmlEscape(c.Summary))
		b.WriteString("</p></div>")
	}

	b.WriteString("<h2 class=\"sec-h\">Kronologi & checklist</h2>")
	if len(steps) == 0 {
		b.WriteString("<p style=\"color:var(--muted)\">Belum ada langkah.</p>")
	} else {
		b.WriteString("<ol class=\"steps\">")
		for _, st := range steps {
			doneClass := ""
			if st.DoneAt != nil {
				doneClass = " done"
			}
			b.WriteString("<li class=\"" + strings.TrimSpace(doneClass) + "\"><div class=\"sn\">")
			b.WriteString("<span class=\"num\">" + htmlEscape(fmt.Sprintf("%d", st.StepNo)) + "</span><div>")
			b.WriteString("<p class=\"st-title\">" + htmlEscape(st.Title) + "</p>")
			if st.DoneAt != nil {
				line := "Selesai " + fmtTime(*st.DoneAt)
				if st.DoneBy != "" {
					line += " · " + st.DoneBy
				}
				b.WriteString("<p class=\"st-meta\">" + htmlEscape(line) + "</p>")
			} else {
				b.WriteString("<p class=\"st-meta\" style=\"color:var(--dim)\">Belum selesai</p>")
			}
			if strings.TrimSpace(st.Notes) != "" {
				b.WriteString("<p class=\"st-note\">Catatan: " + htmlEscape(strings.TrimSpace(st.Notes)) + "</p>")
			}
			if strings.TrimSpace(st.EvidenceURL) != "" {
				u := htmlEscape(strings.TrimSpace(st.EvidenceURL))
				b.WriteString("<p class=\"st-ev\">Bukti: <a href=\"" + u + "\" style=\"color:inherit\">" + u + "</a></p>")
			}
			b.WriteString("</div></div></li>")
		}
		b.WriteString("</ol>")
	}

	b.WriteString("<footer><span>Diekspor " + htmlEscape(time.Now().UTC().Format("2006-01-02 15:04 UTC")) + "</span>")
	b.WriteString("<span>Konkon TechOps</span></footer>")
	b.WriteString("</div></body></html>")
	return b.String()
}

func nvlHTML(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}

func writeMeta(b *strings.Builder, label, value string) {
	b.WriteString("<div><dt>" + htmlEscape(label) + "</dt><dd>" + htmlEscape(value) + "</dd></div>")
}

func htmlStatusBadge(status string) string {
	s := strings.ToLower(strings.TrimSpace(status))
	var cls string
	switch s {
	case "resolved":
		cls = "b-res"
	case "needs_triage":
		cls = "b-tri"
	case "open":
		cls = "b-open"
	default:
		cls = "b-def"
	}
	return "<span class=\"badge " + cls + "\">" + htmlEscape(status) + "</span>"
}

func htmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}
