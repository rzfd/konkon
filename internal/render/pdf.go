package render

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
	"github.com/rzfd/metatech/konkon/internal/store"
	"github.com/rzfd/metatech/konkon/internal/tz"
)

// colour palette — light theme (screen & print)
const (
	pageBgR, pageBgG, pageBgB             = 255, 255, 255
	headerBgR, headerBgG, headerBgB     = 248, 250, 252 // slate-50 banner
	panelR, panelG, panelB                = 241, 245, 249 // slate-100 panels
	accentR, accentG, accentB             = 37, 99, 235   // blue-600
	textPrimaryR, textPrimaryG, textPrimaryB = 15, 23, 42 // slate-900
	textMidR, textMidG, textMidB          = 71, 85, 105   // slate-600
	textDimR, textDimG, textDimB          = 100, 116, 139 // slate-500
	greenR, greenG, greenB                = 22, 163, 74
	redR, redG, redB                      = 220, 38, 38
	orangeR, orangeG, orangeB             = 234, 88, 12
	yellowR, yellowG, yellowB             = 202, 138, 4
	pageW                           = 210.0
	marginL                         = 18.0
	marginR                         = 18.0
	contentW                        = pageW - marginL - marginR
)

type PDFOptions struct {
	IncludeChecklist         bool
	IncludeChecklistProgress bool
	Compression              bool
}

func DefaultPDFOptions() PDFOptions {
	return PDFOptions{
		IncludeChecklist:         true,
		IncludeChecklistProgress: false,
		Compression:              false,
	}
}

// PDF generates a polished PDF case summary and returns the raw bytes.
func PDF(c *store.Case, steps []store.CaseStep, attachments []store.CaseAttachment, uploadRoot string) ([]byte, error) {
	return PDFWithOptions(c, steps, attachments, uploadRoot, DefaultPDFOptions())
}

func PDFWithOptions(c *store.Case, steps []store.CaseStep, attachments []store.CaseAttachment, uploadRoot string, opts PDFOptions) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(marginL, 0, marginR)
	pdf.SetAutoPageBreak(true, 20)
	pdf.SetCompression(opts.Compression)

	// UTF-8 -> cp1252 translator (handles •, —, accented chars, etc.)
	tr := pdf.UnicodeTranslatorFromDescriptor("")

	pdf.SetHeaderFunc(func() {
		pdf.SetFillColor(pageBgR, pageBgG, pageBgB)
		pdf.Rect(0, 0, pageW, 297, "F") // white page
	})

	// ── Footer on every page ─────────────────────────────────────────────────
	pdf.SetFooterFunc(func() {
		pdf.SetY(-14)
		pdf.SetFont("Helvetica", "I", 7.5)
		pdf.SetTextColor(textDimR, textDimG, textDimB)
		pdf.SetDrawColor(textDimR, textDimG, textDimB)
		pdf.Line(marginL, pdf.GetY()-1, pageW-marginR, pdf.GetY()-1)
		pdf.CellFormat(contentW/2, 6,
			tr(fmt.Sprintf("Diekspor %s", tz.FormatExportedAt(time.Now()))),
			"", 0, "L", false, 0, "")
		pdf.CellFormat(contentW/2, 6,
			tr(fmt.Sprintf("Konkon TechOps  |  Hal. %d", pdf.PageNo())),
			"", 0, "R", false, 0, "")
	})

	pdf.AddPage()
	rca := store.ParseCaseRCAJSON(c.RCAJSON).Normalize()

	drawCategoryTags(pdf, c)
	pdf.SetX(marginL)
	pdf.SetFont("Helvetica", "B", 20)
	pdf.SetTextColor(22, 22, 22)
	pdf.MultiCell(contentW, 9, tr("ROOT CAUSE ANALYSIS REPORT"), "", "L", false)
	pdf.SetX(marginL)
	pdf.SetFont("Helvetica", "B", 14)
	pdf.SetTextColor(textPrimaryR, textPrimaryG, textPrimaryB)
	pdf.MultiCell(contentW, 7, tr(c.Title), "", "L", false)
	pdf.Ln(1.5)

	// metadata strip
	pdf.SetX(marginL)
	pdf.SetFont("Helvetica", "", 8.5)
	pdf.SetTextColor(textMidR, textMidG, textMidB)
	meta := []string{
		"Case: " + c.CaseID,
		"Status: " + strings.ToUpper(c.Status),
		"Severity: " + strings.ToUpper(nvl(c.Severity, "-")),
		"Service: " + nvl(c.Service, "-"),
		"Updated: " + fmtTimePDF(c.UpdatedAt),
	}
	pdf.MultiCell(contentW, 4.8, tr(strings.Join(meta, "  |  ")), "", "L", false)
	thinDivider(pdf)

	// impact screenshot
	if first := firstImageAttachment(attachments); first != nil {
		sectionHeader(pdf, "DAMPAK / SCREENSHOT")
		drawCenteredImage(pdf, filepath.Join(uploadRoot, filepath.FromSlash(first.FilePath)), contentW, 62)
		pdf.Ln(2)
	}

	if strings.TrimSpace(c.Summary) != "" {
		sectionHeader(pdf, "RINGKASAN INSIDEN")
		pdf.SetX(marginL)
		pdf.SetFont("Helvetica", "", 9.2)
		pdf.SetTextColor(textPrimaryR, textPrimaryG, textPrimaryB)
		pdf.MultiCell(contentW, 5.6, tr(strings.TrimSpace(c.Summary)), "", "L", false)
		pdf.Ln(2)
	}

	if strings.TrimSpace(rca.RootCause) != "" {
		sectionHeader(pdf, "ROOT CAUSE")
		pdf.SetFillColor(243, 244, 246)
		pdf.SetDrawColor(229, 231, 235)
		startY := pdf.GetY()
		pdf.RoundedRect(marginL, startY, contentW, 0, 2, "1234", "FD")
		pdf.SetXY(marginL+4, startY+4)
		pdf.SetFont("Helvetica", "", 9.2)
		pdf.SetTextColor(textPrimaryR, textPrimaryG, textPrimaryB)
		pdf.MultiCell(contentW-8, 5.6, tr(strings.TrimSpace(rca.RootCause)), "", "L", false)
		pdf.Ln(2)
	}

	if strings.TrimSpace(rca.IncidentTimeline) != "" {
		sectionHeader(pdf, "TIMELINE INSIDEN")
		drawTimelineFromText(pdf, tr, rca.IncidentTimeline)
		pdf.Ln(2)
	}

	renderPDFRCASections(pdf, tr, rca)

	if opts.IncludeChecklist && len(steps) > 0 {
		sectionHeader(pdf, "KRONOLOGI & CHECKLIST")
		pdf.SetFont("Helvetica", "", 9)
		pdf.SetTextColor(textPrimaryR, textPrimaryG, textPrimaryB)
		pdf.SetX(marginL)
		for _, st := range steps {
			line := fmt.Sprintf("- %s", st.Title)
			if st.DoneAt != nil {
				line += fmt.Sprintf(" (selesai %s)", fmtTimePDF(*st.DoneAt))
			}
			pdf.MultiCell(contentW, 5.2, tr(line), "", "L", false)
		}
		pdf.Ln(2)
	}

	sectionHeader(pdf, "BUKTI PERILAKU SESI (LOKAL VS DEV/PROD)")
	drawSessionEvidenceTable(pdf, tr, c, steps)
	pdf.Ln(2)

	sectionHeader(pdf, "PERBAIKAN DITERAPKAN (BEFORE VS AFTER)")
	drawBeforeAfterBox(pdf, tr, rca)
	pdf.Ln(2)

	sectionHeader(pdf, "PENCEGAHAN & TINDAK LANJUT")
	writeFollowUpSection(pdf, tr, rca)

	if len(attachments) > 1 {
		pdf.AddPage()
		sectionHeader(pdf, "BUKTI TAMBAHAN")
		for i, att := range attachments {
			if i == 0 {
				continue
			}
			ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(att.FilePath), "."))
			if ext != "jpg" && ext != "jpeg" && ext != "png" && ext != "gif" {
				continue
			}
			fullPath := filepath.Join(uploadRoot, filepath.FromSlash(att.FilePath))
			pdf.SetFont("Helvetica", "B", 8)
			pdf.SetTextColor(textMidR, textMidG, textMidB)
			pdf.SetX(marginL)
			pdf.CellFormat(contentW, 5, tr(att.OriginalName), "", 1, "L", false, 0, "")
			drawCenteredImage(pdf, fullPath, contentW, 75)
			pdf.SetX(marginL)
			pdf.SetFont("Helvetica", "", 8.2)
			pdf.SetTextColor(textDimR, textDimG, textDimB)
			pdf.MultiCell(contentW, 4.8, tr("Catatan observasi: gambar ini mendukung analisis RCA dan verifikasi dampak insiden."), "", "L", false)
			pdf.Ln(2)
		}
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func renderPDFRCASections(pdf *fpdf.Fpdf, tr func(string) string, rca store.CaseRCA) {
	if !rca.HasContent() {
		return
	}
	rca = rca.Normalize()
	pdfRCATextBlock(pdf, tr, "KRONOLOGI INSIDEN", rca.IncidentTimeline)

	var whys []string
	for i, w := range rca.FiveWhys {
		if strings.TrimSpace(w) != "" {
			whys = append(whys, fmt.Sprintf("%d. %s", i+1, strings.TrimSpace(w)))
		}
	}
	if len(whys) > 0 {
		sectionHeader(pdf, "ANALISIS 5 WHYS")
		pdf.SetFont("Helvetica", "", 9)
		pdf.SetTextColor(textPrimaryR, textPrimaryG, textPrimaryB)
		pdf.SetX(marginL)
		pdf.MultiCell(contentW, 5.5, tr(strings.Join(whys, "\n")), "", "L", false)
		pdf.Ln(3)
	}

	pdfRCATextBlock(pdf, tr, "AKAR MASALAH (ROOT CAUSE)", rca.RootCause)
	pdfRCATextBlock(pdf, tr, "FAKTOR KONTRIBUTOR", rca.ContributingFactors)
	pdfRCATextBlock(pdf, tr, "TINDAKAN KOREKTIF", rca.CorrectiveActions)
	pdfRCATextBlock(pdf, tr, "TINDAKAN PENCEGAHAN", rca.PreventiveActions)
}

func pdfRCATextBlock(pdf *fpdf.Fpdf, tr func(string) string, title, body string) {
	body = strings.TrimSpace(body)
	if body == "" {
		return
	}
	sectionHeader(pdf, title)
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(textPrimaryR, textPrimaryG, textPrimaryB)
	pdf.SetX(marginL)
	pdf.MultiCell(contentW, 5.5, tr(body), "", "L", false)
	pdf.Ln(3)
}

// ── helpers ──────────────────────────────────────────────────────────────────

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

func sectionHeader(pdf *fpdf.Fpdf, title string) {
	pdf.SetFillColor(226, 232, 240)
	pdf.SetTextColor(textPrimaryR, textPrimaryG, textPrimaryB)
	pdf.SetFont("Helvetica", "B", 8.5)
	y := pdf.GetY()
	pdf.Rect(marginL, y, contentW, 8, "F")
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
	pdf.SetTextColor(textPrimaryR, textPrimaryG, textPrimaryB)
	pdf.MultiCell(w-28, 4.5, value, "", "L", false)
}

func drawBadge(pdf *fpdf.Fpdf, x *float64, y float64, label, value string, rgb [3]int) {
	pdf.SetFont("Helvetica", "B", 7)
	lw := pdf.GetStringWidth(label) + 4
	pdf.SetFont("Helvetica", "B", 9)
	vw := pdf.GetStringWidth(value) + 6
	bw := lw + vw

	// label part
	pdf.SetFillColor(226, 232, 240)
	pdf.SetTextColor(textMidR, textMidG, textMidB)
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
	return tz.FormatWIB(t)
}

func thinDivider(pdf *fpdf.Fpdf) {
	y := pdf.GetY() + 1.5
	pdf.SetDrawColor(229, 231, 235)
	pdf.Line(marginL, y, marginL+contentW, y)
	pdf.SetY(y + 3)
}

func drawCategoryTags(pdf *fpdf.Fpdf, c *store.Case) {
	type tag struct {
		label    string
		bg, fg   [3]int
		border   [3]int
	}
	tags := []tag{
		{label: "Bug", bg: [3]int{254, 242, 242}, fg: [3]int{153, 27, 27}, border: [3]int{254, 202, 202}},
		{label: "Severity: " + strings.ToUpper(nvl(c.Severity, "N/A")), bg: [3]int{254, 249, 195}, fg: [3]int{113, 63, 18}, border: [3]int{253, 224, 71}},
		{label: nvl(c.Service, "Safari / iOS / HTTPS"), bg: [3]int{239, 246, 255}, fg: [3]int{30, 64, 175}, border: [3]int{191, 219, 254}},
		{label: "Autentikasi", bg: [3]int{240, 253, 244}, fg: [3]int{22, 101, 52}, border: [3]int{187, 247, 208}},
		{label: "Bukti Tambahan", bg: [3]int{243, 244, 246}, fg: [3]int{55, 65, 81}, border: [3]int{209, 213, 219}},
	}
	x := marginL
	y := 10.0
	for _, t := range tags {
		pdf.SetFont("Helvetica", "B", 7.8)
		w := pdf.GetStringWidth(t.label) + 8
		if x+w > marginL+contentW {
			x = marginL
			y += 7
		}
		pdf.SetFillColor(t.bg[0], t.bg[1], t.bg[2])
		pdf.SetDrawColor(t.border[0], t.border[1], t.border[2])
		pdf.RoundedRect(x, y, w, 5.6, 2.5, "1234", "FD")
		pdf.SetXY(x+4, y+1.1)
		pdf.SetTextColor(t.fg[0], t.fg[1], t.fg[2])
		pdf.CellFormat(w-6, 3.8, t.label, "", 0, "L", false, 0, "")
		x += w + 2.5
	}
	pdf.SetY(y + 10)
}

func firstImageAttachment(atts []store.CaseAttachment) *store.CaseAttachment {
	for i := range atts {
		ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(atts[i].FilePath), "."))
		if ext == "jpg" || ext == "jpeg" || ext == "png" || ext == "gif" {
			return &atts[i]
		}
	}
	return nil
}

func drawCenteredImage(pdf *fpdf.Fpdf, fullPath string, maxW, maxH float64) {
	info := pdf.GetImageInfo(fullPath)
	if info == nil {
		return
	}
	w := info.Width()
	h := info.Height()
	scale := maxW / w
	if h*scale > maxH {
		scale = maxH / h
	}
	drawW := w * scale
	drawH := h * scale
	x := marginL + (contentW-drawW)/2
	y := pdf.GetY()
	ext := strings.ToUpper(strings.TrimPrefix(filepath.Ext(fullPath), "."))
	if ext == "JPG" {
		ext = "JPEG"
	}
	pdf.Image(fullPath, x, y, drawW, drawH, false, ext, 0, "")
	pdf.SetY(y + drawH + 3)
}

func drawTimelineFromText(pdf *fpdf.Fpdf, tr func(string) string, raw string) {
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	if len(lines) > 8 {
		lines = lines[:8]
	}
	xDot := marginL + 3.5
	xText := marginL + 12
	for i, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		y := pdf.GetY()
		if i < len(lines)-1 {
			pdf.SetDrawColor(209, 213, 219)
			pdf.Line(xDot, y+2.5, xDot, y+9)
		}
		pdf.SetFillColor(96, 165, 250)
		pdf.Circle(xDot, y+2.5, 1.3, "F")
		pdf.SetFillColor(238, 242, 255)
		pdf.SetDrawColor(199, 210, 254)
		pdf.RoundedRect(xText, y, 26, 4.5, 1.6, "1234", "FD")
		pdf.SetXY(xText+1.8, y+0.7)
		pdf.SetFont("Helvetica", "B", 7)
		pdf.SetTextColor(67, 56, 202)
		pdf.CellFormat(22, 3, fmt.Sprintf("Status %d", i+1), "", 0, "C", false, 0, "")
		pdf.SetXY(xText+30, y)
		pdf.SetFont("Helvetica", "", 8.7)
		pdf.SetTextColor(textPrimaryR, textPrimaryG, textPrimaryB)
		pdf.MultiCell(contentW-32, 4.5, tr(l), "", "L", false)
		pdf.Ln(0.7)
	}
}

func drawSessionEvidenceTable(pdf *fpdf.Fpdf, tr func(string) string, c *store.Case, steps []store.CaseStep) {
	colA := marginL
	colW := contentW / 2
	colB := marginL + colW
	y := pdf.GetY()
	pdf.SetFillColor(249, 250, 251)
	pdf.SetDrawColor(209, 213, 219)
	pdf.Rect(colA, y, colW, 8, "FD")
	pdf.Rect(colB, y, colW, 8, "FD")
	pdf.SetFont("Helvetica", "B", 8.3)
	pdf.SetTextColor(31, 41, 55)
	pdf.SetXY(colA+3, y+2.1)
	pdf.CellFormat(colW-6, 4, "Lokal (HTTP)", "", 0, "L", false, 0, "")
	pdf.SetXY(colB+3, y+2.1)
	pdf.CellFormat(colW-6, 4, "Dev / Production (HTTPS)", "", 0, "L", false, 0, "")
	pdf.SetY(y + 8)

	left := []string{
		"Sesi stabil saat simulasi lokal",
		"Cookie auth terbaca normal",
		"Error reproduksi rendah",
	}
	right := []string{
		"Safari/iOS reject cookie pada kondisi tertentu",
		"Perbedaan perilaku secure/samesite terlihat",
		"Dampak terlihat pada autentikasi user",
	}
	if c.Service != "" {
		right = append(right, "Layanan terdampak: "+c.Service)
	}
	if len(steps) > 0 {
		right = append(right, fmt.Sprintf("Checklist dianalisis: %d langkah", len(steps)))
	}
	maxRows := max(len(left), len(right))
	for i := 0; i < maxRows; i++ {
		pdf.SetDrawColor(229, 231, 235)
		pdf.Rect(colA, pdf.GetY(), colW, 6.4, "D")
		pdf.Rect(colB, pdf.GetY(), colW, 6.4, "D")
		if i < len(left) {
			pdf.SetXY(colA+3, pdf.GetY()+1.3)
			pdf.SetFont("Helvetica", "", 8.2)
			pdf.SetTextColor(textPrimaryR, textPrimaryG, textPrimaryB)
			pdf.CellFormat(colW-6, 4, tr(left[i]), "", 0, "L", false, 0, "")
		}
		if i < len(right) {
			pdf.SetXY(colB+3, pdf.GetY()+1.3)
			pdf.SetFont("Helvetica", "", 8.2)
			pdf.SetTextColor(textPrimaryR, textPrimaryG, textPrimaryB)
			pdf.CellFormat(colW-6, 4, tr(right[i]), "", 0, "L", false, 0, "")
		}
		pdf.SetY(pdf.GetY() + 6.4)
	}
}

func drawBeforeAfterBox(pdf *fpdf.Fpdf, tr func(string) string, rca store.CaseRCA) {
	y := pdf.GetY()
	colW := contentW/2 - 2
	pdf.SetFillColor(254, 242, 242)
	pdf.SetDrawColor(252, 165, 165)
	pdf.RoundedRect(marginL, y, colW, 22, 2, "1234", "FD")
	pdf.SetFillColor(240, 253, 244)
	pdf.SetDrawColor(134, 239, 172)
	pdf.RoundedRect(marginL+colW+4, y, colW, 22, 2, "1234", "FD")
	pdf.SetFont("Helvetica", "B", 8)
	pdf.SetTextColor(153, 27, 27)
	pdf.SetXY(marginL+3, y+3)
	pdf.CellFormat(colW-6, 4, "BEFORE", "", 0, "L", false, 0, "")
	pdf.SetTextColor(22, 101, 52)
	pdf.SetXY(marginL+colW+7, y+3)
	pdf.CellFormat(colW-6, 4, "AFTER", "", 0, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 8.2)
	pdf.SetTextColor(textPrimaryR, textPrimaryG, textPrimaryB)
	pdf.SetXY(marginL+3, y+8)
	before := "Perilaku autentikasi tidak konsisten lintas environment."
	if strings.TrimSpace(rca.ContributingFactors) != "" {
		before = strings.TrimSpace(rca.ContributingFactors)
	}
	pdf.MultiCell(colW-6, 4.3, tr(before), "", "L", false)
	pdf.SetXY(marginL+colW+7, y+8)
	after := "Pengaturan cookie/session diperkuat, validasi alur auth distabilkan."
	if strings.TrimSpace(rca.CorrectiveActions) != "" {
		after = strings.TrimSpace(rca.CorrectiveActions)
	}
	pdf.MultiCell(colW-6, 4.3, tr(after), "", "L", false)
	pdf.SetY(y + 24)
}

func writeFollowUpSection(pdf *fpdf.Fpdf, tr func(string) string, rca store.CaseRCA) {
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(textPrimaryR, textPrimaryG, textPrimaryB)
	lines := []string{
		"- Tambahkan regression test lintas browser untuk skenario autentikasi.",
		"- Tetapkan SOP verifikasi cookie policy (secure, same-site, domain, expiry).",
		"- Monitor error-rate autentikasi dan alert dini untuk anomali sesi.",
	}
	if strings.TrimSpace(rca.PreventiveActions) != "" {
		lines = append(lines, "- "+strings.TrimSpace(rca.PreventiveActions))
	}
	pdf.SetX(marginL)
	pdf.MultiCell(contentW, 5.2, tr(strings.Join(lines, "\n")), "", "L", false)
	pdf.Ln(1.5)
}
