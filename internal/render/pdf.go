package render

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
	"github.com/rzfd/metatech/konkon/internal/store"
)

// colour palette — high-contrast dark RCA layout (readable on screen & print)
const (
	pageBgR, pageBgG, pageBgB       = 10, 14, 20    // page fill
	headerBgR, headerBgG, headerBgB = 18, 24, 34    // title banner
	panelR, panelG, panelB          = 22, 30, 42    // metadata / summary panels
	accentR, accentG, accentB     = 120, 200, 255 // brighter sky for contrast on dark
	textBrightR, textBrightG, textBrightB = 248, 250, 252
	textMidR, textMidG, textMidB    = 186, 198, 212
	textDimR, textDimG, textDimB    = 130, 146, 165
	stepZebraAR, stepZebraAG, stepZebraAB = 16, 22, 32
	stepZebraBR, stepZebraBG, stepZebraBB = 24, 32, 46
	greenR, greenG, greenB          = 72, 212, 140
	redR, redG, redB                = 255, 90, 90
	orangeR, orangeG, orangeB       = 255, 150, 60
	yellowR, yellowG, yellowB       = 250, 204, 21
	pageW                           = 210.0
	marginL                         = 18.0
	marginR                         = 18.0
	contentW                        = pageW - marginL - marginR
)

// PDF generates a polished PDF case summary and returns the raw bytes.
func PDF(c *store.Case, steps []store.CaseStep, attachments []store.CaseAttachment, stepAtts map[int64][]store.CaseAttachment, uploadRoot string) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(marginL, 0, marginR)
	pdf.SetAutoPageBreak(true, 20)

	// UTF-8 -> cp1252 translator (handles •, —, accented chars, etc.)
	tr := pdf.UnicodeTranslatorFromDescriptor("")

	pdf.SetHeaderFunc(func() {
		pdf.SetFillColor(pageBgR, pageBgG, pageBgB)
		pdf.Rect(0, 0, pageW, 297, "F")
	})

	// ── Footer on every page ─────────────────────────────────────────────────
	pdf.SetFooterFunc(func() {
		pdf.SetY(-14)
		pdf.SetFont("Helvetica", "I", 7.5)
		pdf.SetTextColor(textDimR, textDimG, textDimB)
		pdf.SetDrawColor(textDimR, textDimG, textDimB)
		pdf.Line(marginL, pdf.GetY()-1, pageW-marginR, pdf.GetY()-1)
		pdf.CellFormat(contentW/2, 6,
			tr(fmt.Sprintf("Diekspor %s", time.Now().UTC().Format("2006-01-02 15:04 UTC"))),
			"", 0, "L", false, 0, "")
		pdf.CellFormat(contentW/2, 6,
			tr(fmt.Sprintf("Konkon TechOps  |  Hal. %d", pdf.PageNo())),
			"", 0, "R", false, 0, "")
	})

	pdf.AddPage()

	// ════════════════════════════════════════════════════════════════════════
	// HEADER BANNER
	// ════════════════════════════════════════════════════════════════════════
	pdf.SetFillColor(headerBgR, headerBgG, headerBgB)
	pdf.Rect(0, 0, pageW, 42, "F")

	// accent bar
	pdf.SetFillColor(accentR, accentG, accentB)
	pdf.Rect(0, 38, pageW, 4, "F")

	// document type (RCA)
	pdf.SetXY(marginL, 8)
	pdf.SetFont("Helvetica", "B", 7.5)
	pdf.SetTextColor(accentR, accentG, accentB)
	pdf.CellFormat(contentW, 5, "KONKON TECHOPS  |  ROOT CAUSE ANALYSIS (RCA)", "", 1, "L", false, 0, "")

	// Case ID
	pdf.SetX(marginL)
	pdf.SetFont("Helvetica", "B", 11)
	pdf.SetTextColor(accentR, accentG, accentB)
	pdf.CellFormat(contentW, 6, tr(c.CaseID), "", 1, "L", false, 0, "")

	// Case title
	pdf.SetX(marginL)
	pdf.SetFont("Helvetica", "B", 17)
	pdf.SetTextColor(255, 255, 255)
	pdf.MultiCell(contentW, 7, tr(c.Title), "", "L", false)

	pdf.Ln(6)

	// ════════════════════════════════════════════════════════════════════════
	// STATUS + SEVERITY BADGES
	// ════════════════════════════════════════════════════════════════════════
	y := pdf.GetY()
	x := marginL

	drawBadge(pdf, &x, y, "STATUS", tr(strings.ToUpper(c.Status)), statusColor(c.Status))
	if c.Severity != "" {
		drawBadge(pdf, &x, y, "SEVERITY", tr(strings.ToUpper(c.Severity)), severityColor(c.Severity))
	}
	if c.Service != "" {
		drawBadge(pdf, &x, y, "LAYANAN", tr(c.Service), [3]int{accentR, accentG, accentB})
	}

	pdf.SetY(y + 12)
	pdf.Ln(4)

	// ════════════════════════════════════════════════════════════════════════
	// METADATA CARD
	// ════════════════════════════════════════════════════════════════════════
	sectionHeader(pdf, "INFORMASI INSIDEN")

	// two-column grid
	type kv struct{ k, v string }
	left := []kv{
		{"ID Case", tr(c.CaseID)},
		{"Dibuat", fmtTimePDF(c.CreatedAt)},
		{"Diperbarui", fmtTimePDF(c.UpdatedAt)},
	}
	if c.ResolvedAt != nil {
		left = append(left, kv{"Selesai", fmtTimePDF(*c.ResolvedAt)})
	}
	right := []kv{
		{"Reporter", tr(nvl(c.Reporter, "-"))},
		{"Layanan", tr(nvl(c.Service, "-"))},
		{"Severity", tr(nvl(c.Severity, "-"))},
	}
	if c.SOPSlug != "" {
		right = append(right, kv{"SOP", tr(fmt.Sprintf("%s v%d", c.SOPSlug, derefVer(c.SOPVersion)))})
	}

	colW := contentW/2 - 3
	rows := max(len(left), len(right))
	cardY := pdf.GetY()

	// card background (dark panel)
	pdf.SetFillColor(panelR, panelG, panelB)
	pdf.SetDrawColor(accentR, accentG, accentB)
	pdf.SetLineWidth(0.2)
	pdf.RoundedRect(marginL, cardY, contentW, float64(rows)*9+10, 2, "1234", "FD")

	for i := 0; i < rows; i++ {
		rowY := cardY + 5 + float64(i)*9
		if i < len(left) {
			metaRow(pdf, marginL+4, rowY, colW, left[i].k, left[i].v)
		}
		if i < len(right) {
			metaRow(pdf, marginL+colW+6, rowY, colW, right[i].k, right[i].v)
		}
	}
	pdf.SetY(cardY + float64(rows)*9 + 14)

	// ════════════════════════════════════════════════════════════════════════
	// LAMPIRAN (ATTACHMENTS)
	// ════════════════════════════════════════════════════════════════════════
	if len(attachments) > 0 {
		sectionHeader(pdf, "LAMPIRAN")
		for _, att := range attachments {
			ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(att.FilePath), "."))
			if ext != "jpg" && ext != "jpeg" && ext != "png" && ext != "gif" {
				pdf.SetFont("Helvetica", "I", 8.5)
				pdf.SetTextColor(textMidR, textMidG, textMidB)
				pdf.SetX(marginL)
				pdf.CellFormat(contentW, 6, tr(att.OriginalName), "", 1, "L", false, 0, "")
				pdf.Ln(2)
				continue
			}
			fullPath := filepath.Join(uploadRoot, filepath.FromSlash(att.FilePath))
			pdf.SetFont("Helvetica", "I", 8)
			pdf.SetTextColor(textMidR, textMidG, textMidB)
			pdf.SetX(marginL)
			pdf.CellFormat(contentW, 5, tr(att.OriginalName), "", 1, "L", false, 0, "")
			pdf.Ln(1)
			imgY := pdf.GetY()
			tp := ext
			if tp == "jpg" {
				tp = "jpeg"
			}
			pdf.Image(fullPath, marginL, imgY, contentW, 0, false, strings.ToUpper(tp), 0, "")
			info := pdf.GetImageInfo(fullPath)
			if info != nil {
				scale := contentW / info.Width()
				pdf.SetY(imgY + info.Height()*scale)
			}
			pdf.Ln(4)
		}
	}

	// Summary block (if present)
	if c.Summary != "" {
		sy := pdf.GetY()
		pdf.SetFillColor(panelR, panelG, panelB)
		pdf.SetDrawColor(accentR, accentG, accentB)
		pdf.SetLineWidth(0.2)
		pdf.RoundedRect(marginL, sy, contentW, 0, 2, "1234", "FD")
		pdf.SetFont("Helvetica", "B", 8)
		pdf.SetTextColor(accentR, accentG, accentB)
		pdf.SetXY(marginL+4, sy+4)
		pdf.CellFormat(contentW-8, 5, "RINGKASAN EKSEKUTIF", "", 1, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 9)
		pdf.SetTextColor(textBrightR, textBrightG, textBrightB)
		pdf.SetX(marginL + 4)
		pdf.MultiCell(contentW-8, 5.5, tr(c.Summary), "", "L", false)
		pdf.Ln(4)
	}

	// SOP info row
	if c.SOPTitle != "" {
		sy := pdf.GetY()
		pdf.SetFillColor(18, 36, 54)
		pdf.SetDrawColor(accentR, accentG, accentB)
		pdf.RoundedRect(marginL, sy, contentW, 10, 2, "1234", "FD")
		pdf.SetXY(marginL+4, sy+2.5)
		pdf.SetFont("Helvetica", "B", 8)
		pdf.SetTextColor(accentR, accentG, accentB)
		pdf.CellFormat(22, 5, "SOP:", "", 0, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 8.5)
		pdf.SetTextColor(textBrightR, textBrightG, textBrightB)
		pdf.CellFormat(contentW-26, 5,
			tr(fmt.Sprintf("%s  (v%d)  -  %s", c.SOPSlug, derefVer(c.SOPVersion), c.SOPTitle)),
			"", 1, "L", false, 0, "")
		pdf.Ln(5)
	}

	// ════════════════════════════════════════════════════════════════════════
	// CHECKLIST
	// ════════════════════════════════════════════════════════════════════════
	if len(steps) > 0 {
		sectionHeader(pdf, "KRONOLOGI & CHECKLIST")

		done := 0
		for _, st := range steps {
			if st.DoneAt != nil {
				done++
			}
		}
		// progress bar
		barY := pdf.GetY()
		pdf.SetFillColor(stepZebraBR, stepZebraBG, stepZebraBB)
		pdf.Rect(marginL, barY, contentW, 5, "F")
		pct := float64(done) / float64(len(steps))
		pdf.SetFillColor(greenR, greenG, greenB)
		pdf.Rect(marginL, barY, contentW*pct, 5, "F")
		pdf.SetXY(marginL, barY+6)
		pdf.SetFont("Helvetica", "I", 7.5)
		pdf.SetTextColor(textMidR, textMidG, textMidB)
		pdf.CellFormat(contentW, 5,
			fmt.Sprintf("%d dari %d langkah selesai", done, len(steps)),
			"", 1, "R", false, 0, "")
		pdf.Ln(3)

		const pageBottom = 297.0 - 22.0 // A4 height minus footer zone
		pdf.SetAutoPageBreak(false, 0)

		for i, st := range steps {
			isDone := st.DoneAt != nil
			titleX := marginL + 16.0
			textW := contentW - titleX + marginL - 4

			// Estimate minimum height needed (title line + at least status line)
			minH := stepRowHeight(st)

			// If not enough room, start a new page
			if pdf.GetY()+minH > pageBottom {
				pdf.AddPage()
			}

			ry := pdf.GetY()

			// ── background fill ──────────────────────────────────────────
			if i%2 == 0 {
				pdf.SetFillColor(stepZebraAR, stepZebraAG, stepZebraAB)
			} else {
				pdf.SetFillColor(stepZebraBR, stepZebraBG, stepZebraBB)
			}
			pdf.Rect(marginL, ry, contentW, minH, "F")

			// ── left accent stripe ────────────────────────────────────────
			if isDone {
				pdf.SetFillColor(greenR, greenG, greenB)
			} else {
				pdf.SetFillColor(textDimR, textDimG, textDimB)
			}
			pdf.Rect(marginL, ry, 3, minH, "F")

			// ── step number circle ────────────────────────────────────────
			cx := marginL + 8.0
			cy := ry + 6.0
			if isDone {
				pdf.SetFillColor(greenR, greenG, greenB)
			} else {
				pdf.SetFillColor(textDimR, textDimG, textDimB)
			}
			pdf.Circle(cx, cy, 3.5, "F")
			pdf.SetFont("Helvetica", "B", 7)
			pdf.SetTextColor(255, 255, 255)
			numStr := fmt.Sprintf("%d", st.StepNo)
			numW := pdf.GetStringWidth(numStr)
			pdf.SetXY(cx-numW/2-0.3, cy-2.5)
			pdf.CellFormat(numW+0.6, 5, numStr, "", 0, "C", false, 0, "")

			// ── title ─────────────────────────────────────────────────────
			pdf.SetXY(titleX, ry+2)
			if isDone {
				pdf.SetFont("Helvetica", "B", 9)
				pdf.SetTextColor(textBrightR, textBrightG, textBrightB)
			} else {
				pdf.SetFont("Helvetica", "B", 9)
				pdf.SetTextColor(textMidR, textMidG, textMidB)
			}
			pdf.MultiCell(textW, 5.5, tr(st.Title), "", "L", false)

			// ── done info ─────────────────────────────────────────────────
			if isDone {
				pdf.SetX(titleX)
				pdf.SetFont("Helvetica", "I", 7.5)
				pdf.SetTextColor(greenR, greenG, greenB)
				info := fmt.Sprintf("Selesai %s", fmtTimePDF(*st.DoneAt))
				if st.DoneBy != "" {
					info += "  oleh " + st.DoneBy
				}
				pdf.CellFormat(textW, 4.5, tr(info), "", 1, "L", false, 0, "")
			}

			// ── notes ─────────────────────────────────────────────────────
			if strings.TrimSpace(st.Notes) != "" {
				pdf.SetX(titleX)
				pdf.SetFont("Helvetica", "", 7.5)
				pdf.SetTextColor(textMidR, textMidG, textMidB)
				pdf.MultiCell(textW, 4.5, tr("Catatan: "+strings.TrimSpace(st.Notes)), "", "L", false)
			}

			// ── evidence URL ──────────────────────────────────────────────
			if strings.TrimSpace(st.EvidenceURL) != "" {
				pdf.SetX(titleX)
				pdf.SetFont("Helvetica", "I", 7.5)
				pdf.SetTextColor(accentR, accentG, accentB)
				pdf.MultiCell(textW, 4.5, tr("Bukti: "+strings.TrimSpace(st.EvidenceURL)), "", "L", false)
			}

			pdf.Ln(3)

			// ── step-level uploaded images ────────────────────────────────
			if stepAtts != nil {
				for _, att := range stepAtts[st.ID] {
					ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(att.FilePath), "."))
					if ext != "jpg" && ext != "jpeg" && ext != "png" && ext != "gif" {
						continue
					}
					if pdf.GetY()+35 > pageBottom {
						pdf.AddPage()
					}
					fullPath := filepath.Join(uploadRoot, filepath.FromSlash(att.FilePath))
					pdf.SetFont("Helvetica", "I", 7.5)
					pdf.SetTextColor(accentR, accentG, accentB)
					pdf.SetX(titleX)
					pdf.CellFormat(textW, 4.5, tr("Bukti: "+att.OriginalName), "", 1, "L", false, 0, "")
					imgY := pdf.GetY()
					tp := ext
					if tp == "jpg" {
						tp = "jpeg"
					}
					imgW := contentW - 12
					pdf.Image(fullPath, marginL+6, imgY, imgW, 0, false, strings.ToUpper(tp), 0, "")
					info := pdf.GetImageInfo(fullPath)
					if info != nil {
						scale := imgW / info.Width()
						pdf.SetY(imgY + info.Height()*scale)
					}
					pdf.Ln(3)
				}
			}

			pdf.Ln(1)
		}

		pdf.SetAutoPageBreak(true, 20)
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ── helpers ──────────────────────────────────────────────────────────────────

func sectionHeader(pdf *fpdf.Fpdf, title string) {
	pdf.SetFillColor(headerBgR, headerBgG, headerBgB)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Helvetica", "B", 8.5)
	y := pdf.GetY()
	pdf.Rect(marginL, y, contentW, 8, "F")
	// accent left bar
	pdf.SetFillColor(accentR, accentG, accentB)
	pdf.Rect(marginL, y, 3, 8, "F")
	pdf.SetXY(marginL+6, y+1.5)
	pdf.CellFormat(contentW-6, 5, title, "", 1, "L", false, 0, "")
	pdf.Ln(3)
}

func metaRow(pdf *fpdf.Fpdf, x, y, w float64, label, value string) {
	pdf.SetXY(x, y)
	pdf.SetFont("Helvetica", "B", 7.5)
	pdf.SetTextColor(textMidR, textMidG, textMidB)
	pdf.CellFormat(28, 4.5, strings.ToUpper(label), "", 0, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 8.5)
	pdf.SetTextColor(textBrightR, textBrightG, textBrightB)
	pdf.MultiCell(w-28, 4.5, value, "", "L", false)
}

func drawBadge(pdf *fpdf.Fpdf, x *float64, y float64, label, value string, rgb [3]int) {
	pdf.SetFont("Helvetica", "B", 7)
	lw := pdf.GetStringWidth(label) + 4
	pdf.SetFont("Helvetica", "B", 9)
	vw := pdf.GetStringWidth(value) + 6
	bw := lw + vw

	// label part (dark)
	pdf.SetFillColor(headerBgR+20, headerBgG+20, headerBgB+20)
	pdf.SetTextColor(textDimR, textDimG, textDimB)
	pdf.SetXY(*x, y)
	pdf.SetFont("Helvetica", "B", 7)
	pdf.CellFormat(lw, 8, label, "", 0, "C", true, 0, "")

	// value part (accent colour)
	pdf.SetFillColor(rgb[0], rgb[1], rgb[2])
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Helvetica", "B", 9)
	pdf.CellFormat(vw, 8, value, "", 0, "C", true, 0, "")

	*x += bw + 4
}

func stepRowHeight(st store.CaseStep) float64 {
	h := 12.0 // base: title + padding
	if st.DoneAt != nil {
		h += 5
	}
	if strings.TrimSpace(st.Notes) != "" {
		h += 5
	}
	if strings.TrimSpace(st.EvidenceURL) != "" {
		h += 5
	}
	return h
}

func statusColor(s string) [3]int {
	switch strings.ToLower(s) {
	case "resolved":
		return [3]int{greenR, greenG, greenB}
	case "open":
		return [3]int{accentR, accentG, accentB}
	default:
		return [3]int{textMidR, textMidG, textMidB}
	}
}

func severityColor(s string) [3]int {
	switch strings.ToUpper(s) {
	case "P1":
		return [3]int{redR, redG, redB}
	case "P2":
		return [3]int{orangeR, orangeG, orangeB}
	case "P3":
		return [3]int{yellowR, yellowG, yellowB}
	default:
		return [3]int{greenR, greenG, greenB}
	}
}

func nvl(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func fmtTimePDF(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.UTC().Format("2006-01-02 15:04 UTC")
}
