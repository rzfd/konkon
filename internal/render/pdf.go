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

// ── colour palette ────────────────────────────────────────────────────────────
const (
	// Hero banner — dark navy
	heroR, heroG, heroB = 15, 23, 42 // slate-900

	// Primary accent — blue-600
	accentR, accentG, accentB = 37, 99, 235

	// Text shades
	textPrimaryR, textPrimaryG, textPrimaryB = 15, 23, 42   // slate-900
	textMidR, textMidG, textMidB             = 71, 85, 105  // slate-600
	textDimR, textDimG, textDimB             = 100, 116, 139 // slate-500

	// Neutral surfaces
	panelR, panelG, panelB    = 248, 250, 252 // slate-50
	panel2R, panel2G, panel2B = 241, 245, 249 // slate-100
	borderR, borderG, borderB = 226, 232, 240 // slate-200
	headerBgR, headerBgG, headerBgB = 248, 250, 252

	// Semantic
	greenR, greenG, greenB    = 22, 163, 74
	redR, redG, redB          = 220, 38, 38
	orangeR, orangeG, orangeB = 234, 88, 12
	yellowR, yellowG, yellowB = 202, 138, 4

	// Page layout — margins sedikit diperkecil agar kolom teks lebih lebar (laporan padat).
	pageW    = 210.0
	marginL  = 14.0
	marginR  = 14.0
	contentW = pageW - marginL - marginR

	// A4 height (mm) and bottom margin — must match SetAutoPageBreak below.
	pdfPageHeightMM          = 297.0
	pdfAutoPageBreakMarginMM = 16.0

	// ── Dense / “padat” typography (mirip template RCA naratif penuh) ───────────
	bannerBandH       = 32.0 // tinggi strip abu header halaman 1
	bannerAccentY     = 28.0 // posisi strip biru bawah banner
	bannerEyebrowY    = 5.5
	titleCaseIDFontPt = 10.0
	titleCaseIDLineH  = 5.0
	titleMainFontPt   = 14.0
	titleMainLineH    = 5.0
	afterTitleLn      = 3.0
	badgeAnchorY      = 10.0 // jarak vertikal blok badge dari baseline
	afterBadgeLn      = 2.0
	sectionHdrH       = 7.0
	sectionHdrFontPt  = 8.2
	sectionHdrTextY   = 1.35
	afterSectionHdrLn = 1.8
	metaRowPitchMM    = 7.2
	metaCardPadTop    = 4.0
	metaCardPadBottom = 8.0
	rcaBodyFontPt     = 8.5
	rcaBodyLineMM     = 4.2
	afterRcaBlockLn   = 2.0
	whyCardH          = 11.0
	whyBodyLineMM     = 3.8
	stepTitleLineMM   = 4.2
	stepMetaLineMM    = 3.6
	afterStepLn       = 1.0
	progressBarH      = 3.5
	afterProgressLn   = 1.8
	sopStripH         = 8.0
	// Tall phone screenshots scaled to full content width used to exceed page height, clip, and confuse Y — cap height.
	pdfMaxLampiranImageHMM = 165.0
	pdfMaxStepEvidenceHMM  = 72.0
)

// ── option types ──────────────────────────────────────────────────────────────

type PDFOptions struct {
	IncludeChecklist         bool
	IncludeChecklistProgress bool
	Compression              bool
}

func DefaultPDFOptions() PDFOptions {
	return PDFOptions{
		IncludeChecklist:         true,
		IncludeChecklistProgress: true,
		Compression:              false,
	}
}

// ── entry points ──────────────────────────────────────────────────────────────

func PDF(c *store.Case, steps []store.CaseStep, attachments []store.CaseAttachment, stepAtts map[int64][]store.CaseAttachment, uploadRoot string) ([]byte, error) {
	return PDFWithOptions(c, steps, attachments, stepAtts, uploadRoot, DefaultPDFOptions())
}

func PDFWithOptions(c *store.Case, steps []store.CaseStep, attachments []store.CaseAttachment, stepAtts map[int64][]store.CaseAttachment, uploadRoot string, opts PDFOptions) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(marginL, 0, marginR)
	pdf.SetAutoPageBreak(true, pdfAutoPageBreakMarginMM)
	pdf.SetCompression(opts.Compression)

	tr := pdf.UnicodeTranslatorFromDescriptor("")

	// No full-page header fill: a white rectangle over the whole MediaBox breaks or
	// hides body text in some PDF viewers (Chrome/Edge) after page breaks.
	pdf.SetHeaderFunc(nil)

	pdf.SetFooterFunc(func() {
		// Bottom accent strip
		pdf.SetFillColor(accentR, accentG, accentB)
		pdf.Rect(0, 290, pageW, 7, "F")
		pdf.SetY(-12)
		pdf.SetFont("Helvetica", "I", 7)
		pdf.SetTextColor(255, 255, 255)
		pdf.SetXY(0, 290.5)
		pdf.CellFormat(pageW/2, 5,
			tr(fmt.Sprintf("  Konkon TechOps  |  Diekspor %s", tz.FormatExportedAt(time.Now()))),
			"", 0, "L", false, 0, "")
		pdf.CellFormat(pageW/2, 5,
			tr(fmt.Sprintf("Halaman %d  ", pdf.PageNo())),
			"", 0, "R", false, 0, "")
	})

	pdf.AddPage()

	// HEADER BANNER (halaman 1 — ringkas agar ruang untuk narasi RCA)
	pdf.SetFillColor(headerBgR, headerBgG, headerBgB)
	pdf.Rect(0, 0, pageW, bannerBandH, "F")
	pdf.SetFillColor(accentR, accentG, accentB)
	pdf.Rect(0, bannerAccentY, pageW, 4, "F")
	pdf.SetXY(marginL, bannerEyebrowY)
	pdf.SetFont("Helvetica", "B", 7)
	pdf.SetTextColor(accentR, accentG, accentB)
	pdf.CellFormat(contentW, 4, "KONKON TECHOPS  |  ROOT CAUSE ANALYSIS (RCA)", "", 1, "L", false, 0, "")
	pdf.SetX(marginL)
	pdf.SetFont("Helvetica", "B", titleCaseIDFontPt)
	pdf.CellFormat(contentW, titleCaseIDLineH, tr(c.CaseID), "", 1, "L", false, 0, "")
	pdf.SetX(marginL)
	pdf.SetFont("Helvetica", "B", titleMainFontPt)
	pdf.SetTextColor(textPrimaryR, textPrimaryG, textPrimaryB)
	pdf.MultiCell(contentW, titleMainLineH, tr(c.Title), "", "L", false)
	pdf.Ln(afterTitleLn)

	// badges
	y := pdf.GetY()
	x := marginL
	drawBadge(pdf, &x, y, "STATUS", tr(strings.ToUpper(c.Status)), statusColor(c.Status))
	if c.Severity != "" {
		drawBadge(pdf, &x, y, "SEVERITY", tr(strings.ToUpper(c.Severity)), severityColor(c.Severity))
	}
	if c.Service != "" {
		drawBadge(pdf, &x, y, "LAYANAN", tr(c.Service), [3]int{accentR, accentG, accentB})
	}
	pdf.SetY(y + badgeAnchorY)
	pdf.Ln(afterBadgeLn)

	sectionHeader(pdf, "INFORMASI INSIDEN")
	type kv struct{ k, v string }
	left := []kv{{"ID Case", tr(c.CaseID)}, {"Dibuat", fmtTimePDF(c.CreatedAt)}, {"Diperbarui", fmtTimePDF(c.UpdatedAt)}}
	if c.ResolvedAt != nil {
		left = append(left, kv{"Selesai", fmtTimePDF(*c.ResolvedAt)})
	}
	right := []kv{{"Reporter", tr(nvl(c.Reporter, "-"))}, {"Layanan", tr(nvl(c.Service, "-"))}, {"Severity", tr(nvl(c.Severity, "-"))}}
	if c.SOPSlug != "" {
		right = append(right, kv{"SOP", tr(fmt.Sprintf("%s v%d", c.SOPSlug, derefVer(c.SOPVersion)))})
	}
	colW := contentW/2 - 3
	rows := max(len(left), len(right))
	cardY := pdf.GetY()
	cardInnerH := float64(rows)*metaRowPitchMM + metaCardPadTop + metaCardPadBottom
	pdf.SetFillColor(panelR, panelG, panelB)
	pdf.SetDrawColor(226, 232, 240)
	pdf.SetLineWidth(0.2)
	pdf.RoundedRect(marginL, cardY, contentW, cardInnerH, 2, "1234", "FD")
	for i := 0; i < rows; i++ {
		rowY := cardY + metaCardPadTop + float64(i)*metaRowPitchMM
		if i < len(left) {
			metaRow(pdf, marginL+4, rowY, colW, left[i].k, left[i].v)
		}
		if i < len(right) {
			metaRow(pdf, marginL+colW+6, rowY, colW, right[i].k, right[i].v)
		}
	}
	pdf.SetY(cardY + cardInnerH + 2)

	// ── GAMBAR DAMPAK (ikut template RCA_SAFARI_AUTH) ──────────────────────────
	if first := firstImageAttachment(attachments); first != nil {
		sectionHeader(pdf, "GAMBAR DAMPAK")
		fullPath := filepath.Join(uploadRoot, filepath.FromSlash(first.FilePath))
		ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(first.FilePath), "."))
		_ = drawLampiranImage(pdf, tr, fullPath, ext)
	}

	if c.Summary != "" {
		sectionHeader(pdf, "RINGKASAN")
		pdf.SetX(marginL)
		pdf.SetFont("Helvetica", "", rcaBodyFontPt)
		pdf.SetTextColor(textPrimaryR, textPrimaryG, textPrimaryB)
		pdf.MultiCell(contentW, rcaBodyLineMM, tr(c.Summary), "", "L", false)
		pdf.Ln(afterRcaBlockLn)
	}

	renderPDFRCASections(pdf, tr, store.ParseCaseRCAJSON(c.RCAJSON))

	if c.SOPTitle != "" {
		sy := pdf.GetY()
		pdf.SetFillColor(239, 246, 255)
		pdf.SetDrawColor(accentR, accentG, accentB)
		pdf.RoundedRect(marginL, sy, contentW, sopStripH, 2, "1234", "FD")
		pdf.SetXY(marginL+4, sy+1.8)
		pdf.SetFont("Helvetica", "B", 7.5)
		pdf.SetTextColor(accentR, accentG, accentB)
		pdf.CellFormat(18, 4.5, "SOP:", "", 0, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 8)
		pdf.SetTextColor(textPrimaryR, textPrimaryG, textPrimaryB)
		pdf.CellFormat(contentW-22, 4.5, tr(fmt.Sprintf("%s  (v%d)  -  %s", c.SOPSlug, derefVer(c.SOPVersion), c.SOPTitle)), "", 1, "L", false, 0, "")
		pdf.Ln(3)
	}

	_ = steps
	_ = stepAtts
	_ = opts

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ── Hero banner (page 1 only) ─────────────────────────────────────────────────

func drawHero(pdf *fpdf.Fpdf, tr func(string) string, c *store.Case) {
	heroH := 44.0

	// Dark navy background
	pdf.SetFillColor(heroR, heroG, heroB)
	pdf.Rect(0, 0, pageW, heroH, "F")

	// Blue accent bar — left edge
	pdf.SetFillColor(accentR, accentG, accentB)
	pdf.Rect(0, 0, 5, heroH, "F")

	// Eyebrow label
	pdf.SetFont("Helvetica", "B", 7.5)
	pdf.SetTextColor(100, 116, 139) // slate-500 on dark
	pdf.SetXY(marginL, 9)
	pdf.CellFormat(contentW-55, 4.5, "ROOT CAUSE ANALYSIS REPORT", "", 1, "L", false, 0, "")

	// Case title
	pdf.SetFont("Helvetica", "B", 16)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetXY(marginL, 15.5)
	pdf.MultiCell(contentW-55, 8, tr(c.Title), "", "L", false)

	// Case ID chip (bottom-left of banner)
	pdf.SetFont("Helvetica", "B", 7.5)
	pdf.SetTextColor(148, 163, 184) // slate-400
	pdf.SetXY(marginL, heroH-8)
	pdf.CellFormat(60, 5, tr("ID: "+c.CaseID), "", 0, "L", false, 0, "")

	// Status badge (top-right)
	drawHeroBadge(pdf, tr, pageW-marginR-28, 10, c.Status, statusColor(c.Status))
	// Severity badge
	if strings.TrimSpace(c.Severity) != "" {
		drawHeroBadge(pdf, tr, pageW-marginR-28, 20, c.Severity, severityColor(c.Severity))
	}

	pdf.SetY(heroH + 5)
}

func drawHeroBadge(pdf *fpdf.Fpdf, _ func(string) string, x, y float64, label string, rgb [3]int) {
	w := 26.0
	pdf.SetFillColor(rgb[0], rgb[1], rgb[2])
	pdf.RoundedRect(x, y, w, 7.5, 2.5, "1234", "F")
	pdf.SetFont("Helvetica", "B", 7.5)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetXY(x, y+1.5)
	pdf.CellFormat(w, 4.5, strings.ToUpper(label), "", 0, "C", false, 0, "")
}

// ── Metadata cards (3-col grid) ───────────────────────────────────────────────

func drawMetaCards(pdf *fpdf.Fpdf, tr func(string) string, c *store.Case) {
	type card struct{ label, value string }
	cards := []card{
		{"CASE ID", c.CaseID},
		{"STATUS", strings.ToUpper(c.Status)},
		{"SEVERITY", strings.ToUpper(nvl(c.Severity, "—"))},
		{"SERVICE", nvl(c.Service, "—")},
		{"REPORTER", nvl(c.Reporter, "—")},
		{"UPDATED", fmtTimePDF(c.UpdatedAt)},
	}

	const ncols = 3
	const gap = 3.0
	cardW := (contentW - gap*float64(ncols-1)) / float64(ncols)
	cardH := 16.0
	rowGap := 3.0

	y := pdf.GetY()
	for i, card := range cards {
		col := i % ncols
		row := i / ncols
		x := marginL + float64(col)*(cardW+gap)
		cy := y + float64(row)*(cardH+rowGap)

		// Card bg + border
		pdf.SetFillColor(panelR, panelG, panelB)
		pdf.SetDrawColor(borderR, borderG, borderB)
		pdf.RoundedRect(x, cy, cardW, cardH, 2, "1234", "FD")

		// Left accent strip inside the card
		pdf.SetFillColor(accentR, accentG, accentB)
		pdf.Rect(x, cy+1.5, 2.5, cardH-3, "F")

		// Label
		pdf.SetFont("Helvetica", "B", 6.5)
		pdf.SetTextColor(textDimR, textDimG, textDimB)
		pdf.SetXY(x+6, cy+3)
		pdf.CellFormat(cardW-8, 4, card.label, "", 0, "L", false, 0, "")

		// Value
		pdf.SetFont("Helvetica", "B", 8.5)
		pdf.SetTextColor(textPrimaryR, textPrimaryG, textPrimaryB)
		pdf.SetXY(x+6, cy+8.5)
		pdf.CellFormat(cardW-8, 5, tr(card.value), "", 0, "L", false, 0, "")
	}

	rows := (len(cards) + ncols - 1) / ncols
	pdf.SetY(y + float64(rows)*(cardH+rowGap) + 4)
}

// ── Section header ────────────────────────────────────────────────────────────

func sectionHeader(pdf *fpdf.Fpdf, title string) {
	y := pdf.GetY()
	h := sectionHdrH

	// Full background (tanpa bilah aksen vertikal — menghindari “sumbu” biru yang mengganggu)
	pdf.SetFillColor(panel2R, panel2G, panel2B)
	pdf.Rect(marginL, y, contentW, h, "F")

	// Title text
	pdf.SetFont("Helvetica", "B", sectionHdrFontPt)
	pdf.SetTextColor(textPrimaryR, textPrimaryG, textPrimaryB)
	pdf.SetXY(marginL+6.5, y+sectionHdrTextY)
	pdf.CellFormat(contentW-7, 4.4, title, "", 1, "L", false, 0, "")

	pdf.Ln(afterSectionHdrLn)
}

// timelineDateLabelID formats a Jakarta-local date like "Sabtu, 04 April 2026".
func timelineDateLabelID(t time.Time) string {
	t = t.In(tz.Jakarta)
	wd := [...]string{"Minggu", "Senin", "Selasa", "Rabu", "Kamis", "Jumat", "Sabtu"}
	mo := [...]string{
		"", "Januari", "Februari", "Maret", "April", "Mei", "Juni",
		"Juli", "Agustus", "September", "Oktober", "November", "Desember",
	}
	return fmt.Sprintf("%s, %02d %s %d", wd[t.Weekday()], t.Day(), mo[t.Month()], t.Year())
}

func metaRow(pdf *fpdf.Fpdf, x, y, w float64, label, value string) {
	pdf.SetXY(x, y)
	pdf.SetFont("Helvetica", "B", 7)
	pdf.SetTextColor(textMidR, textMidG, textMidB)
	pdf.CellFormat(26, 4, strings.ToUpper(label), "", 0, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 8)
	pdf.SetTextColor(textPrimaryR, textPrimaryG, textPrimaryB)
	pdf.MultiCell(w-26, 4, value, "", "L", false)
}

func drawBadge(pdf *fpdf.Fpdf, x *float64, y float64, label, value string, rgb [3]int) {
	pdf.SetFont("Helvetica", "B", 6.5)
	lw := pdf.GetStringWidth(label) + 3
	pdf.SetFont("Helvetica", "B", 8.5)
	vw := pdf.GetStringWidth(value) + 5
	bw := lw + vw
	pdf.SetFillColor(226, 232, 240)
	pdf.SetTextColor(textMidR, textMidG, textMidB)
	pdf.SetXY(*x, y)
	pdf.SetFont("Helvetica", "B", 6.5)
	pdf.CellFormat(lw, 6.5, label, "", 0, "C", true, 0, "")
	pdf.SetFillColor(rgb[0], rgb[1], rgb[2])
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Helvetica", "B", 8.5)
	pdf.CellFormat(vw, 6.5, value, "", 0, "C", true, 0, "")
	*x += bw + 3
}

// ── Root cause highlighted box ────────────────────────────────────────────────

func drawRootCauseBox(pdf *fpdf.Fpdf, tr func(string) string, text string) {
	startY := pdf.GetY()
	body := tr(strings.TrimSpace(text))

	// Ukur tinggi teks dulu — jangan gambar Rect placeholder tinggi (bug lama: 999mm
	// menembus halaman dan terlihat sebagai garis biru vertikal panjang).
	pdf.SetXY(marginL+10, startY+4)
	pdf.SetFont("Helvetica", "", 9.2)
	pdf.SetTextColor(textPrimaryR, textPrimaryG, textPrimaryB)
	pdf.MultiCell(contentW-14, 5.6, body, "", "L", false)
	endY := pdf.GetY() + 2
	barH := endY - startY

	// Aksen kiri tipis saja, setinggi kotak (bukan pilar penuh halaman)
	pdf.SetFillColor(accentR, accentG, accentB)
	pdf.Rect(marginL, startY, 2.6, barH, "F")

	pdf.SetFillColor(239, 246, 255) // blue-50
	pdf.SetDrawColor(191, 219, 254) // blue-200
	pdf.RoundedRect(marginL+2.6, startY, contentW-2.6, barH, 2, "1234", "FD")

	pdf.SetXY(marginL+10, startY+4)
	pdf.SetFont("Helvetica", "", 9.2)
	pdf.SetTextColor(textPrimaryR, textPrimaryG, textPrimaryB)
	pdf.MultiCell(contentW-14, 5.6, body, "", "L", false)

	pdf.SetY(endY)
}

// ── Timeline ──────────────────────────────────────────────────────────────────

func drawTimelineFromText(pdf *fpdf.Fpdf, tr func(string) string, raw string) {
	type ev struct {
		t      time.Time
		hasT   bool
		detail string
	}
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	items := make([]ev, 0, len(lines))
	for _, ln := range lines {
		s := strings.TrimSpace(ln)
		if s == "" {
			continue
		}
		ts := ""
		detail := s
		if parts := strings.SplitN(s, "|", 2); len(parts) == 2 {
			ts = strings.TrimSpace(parts[0])
			detail = strings.TrimSpace(parts[1])
		}
		t, err := time.ParseInLocation("020106 15:04:05", ts, tz.Jakarta)
		if err == nil {
			items = append(items, ev{t: t, hasT: true, detail: detail})
		} else {
			items = append(items, ev{detail: detail})
		}
	}
	if len(items) > 16 {
		items = items[:16]
	}
	if len(items) == 0 {
		pdf.SetX(marginL)
		pdf.SetFont("Helvetica", "I", 8.5)
		pdf.SetTextColor(textDimR, textDimG, textDimB)
		pdf.MultiCell(contentW, 4.2, "(Belum diisi)", "", "L", false)
		return
	}

	dateKey := func(t time.Time) string { return t.In(tz.Jakarta).Format("2006-01-02") }
	// Alternating date chips: warm sand vs cool blue (mirip referensi timeline).
	chipStyles := []struct {
		bg  [3]int
		fg  [3]int
		rul [3]int
	}{
		{bg: [3]int{217, 197, 154}, fg: [3]int{120, 77, 15}, rul: [3]int{180, 160, 120}},
		{bg: [3]int{219, 234, 254}, fg: [3]int{30, 64, 175}, rul: [3]int{147, 197, 253}},
	}

	dotPalette := [][3]int{
		{redR, redG, redB},
		{orangeR, orangeG, orangeB},
		{greenR, greenG, greenB},
		{accentR, accentG, accentB},
	}

	xLine := marginL + 4.0
	xTime := marginL + 8.5
	xText := marginL + 28.0
	textW := contentW - (xText - marginL)
	timeColW := xText - xTime - 1.5

	var lastTimedDateKey string
	dateChipN := -1
	timedSeen := 0
	noteRun := false

	for i, it := range items {
		if it.hasT {
			noteRun = false
			k := dateKey(it.t)
			if k != lastTimedDateKey {
				dateChipN++
				breakPageIfNotEnoughSpace(pdf, 14)
				yChip := pdf.GetY()
				st := chipStyles[dateChipN%len(chipStyles)]
				label := tr(timelineDateLabelID(it.t))
				pdf.SetFont("Helvetica", "B", 7.5)
				chipW := pdf.GetStringWidth(label) + 14
				if chipW < 42 {
					chipW = 42
				}
				pdf.SetFillColor(st.bg[0], st.bg[1], st.bg[2])
				pdf.RoundedRect(marginL, yChip, chipW, 7.2, 3.2, "1234", "F")
				pdf.SetTextColor(st.fg[0], st.fg[1], st.fg[2])
				pdf.SetXY(marginL+7, yChip+1.7)
				pdf.CellFormat(chipW-12, 4, label, "", 0, "L", false, 0, "")
				pdf.SetDrawColor(st.rul[0], st.rul[1], st.rul[2])
				pdf.SetLineWidth(0.25)
				pdf.Line(marginL+chipW+3, yChip+3.6, marginL+contentW, yChip+3.6)
				pdf.SetLineWidth(0.2)
				pdf.SetY(yChip + 9.2)
				lastTimedDateKey = k
			}
		} else {
			if !noteRun {
				breakPageIfNotEnoughSpace(pdf, 12)
				yN := pdf.GetY()
				pdf.SetFont("Helvetica", "B", 7)
				pdf.SetTextColor(textMidR, textMidG, textMidB)
				pdf.SetXY(marginL, yN)
				pdf.CellFormat(contentW, 4, tr("Entri tanpa stempel waktu"), "", 1, "L", false, 0, "")
				pdf.SetY(yN + 5.5)
				noteRun = true
			}
		}

		breakPageIfNotEnoughSpace(pdf, 20)
		y := pdf.GetY()
		rowTop := y

		if i > 0 {
			pdf.SetDrawColor(203, 213, 225)
			pdf.SetLineWidth(0.2)
			pdf.Line(xLine, y-1.2, xLine, y+2.8)
		}

		var dotRGB [3]int
		if it.hasT {
			dotRGB = dotPalette[timedSeen%len(dotPalette)]
			timedSeen++
		} else {
			dotRGB = [3]int{textDimR, textDimG, textDimB}
		}
		pdf.SetFillColor(dotRGB[0], dotRGB[1], dotRGB[2])
		pdf.Circle(xLine, y+2.8, 1.55, "F")
		pdf.SetDrawColor(255, 255, 255)
		pdf.SetLineWidth(0.15)
		pdf.Circle(xLine, y+2.8, 1.55, "D")

		if it.hasT {
			pdf.SetFont("Helvetica", "B", 6.8)
			pdf.SetTextColor(textDimR, textDimG, textDimB)
			pdf.SetXY(xTime, y+0.8)
			pdf.CellFormat(timeColW, 3.6, it.t.In(tz.Jakarta).Format("15:04:05"), "", 0, "L", false, 0, "")
		}

		// Event panel (light) — draw after measuring text height.
		title := it.detail
		desc := ""
		if p := strings.SplitN(it.detail, ".", 2); len(p) == 2 {
			title = strings.TrimSpace(p[0])
			desc = strings.TrimSpace(p[1])
		}
		innerPad := 3.5
		tx0 := xText
		pdf.SetXY(tx0+innerPad, rowTop+innerPad)
		pdf.SetFont("Helvetica", "B", 8.8)
		pdf.SetTextColor(textPrimaryR, textPrimaryG, textPrimaryB)
		pdf.MultiCell(textW-2*innerPad, 4.0, tr(title), "", "L", false)
		if desc != "" {
			pdf.SetX(tx0 + innerPad)
			pdf.SetFont("Helvetica", "", 8)
			pdf.SetTextColor(textMidR, textMidG, textMidB)
			pdf.MultiCell(textW-2*innerPad, 3.7, tr(desc), "", "L", false)
		}
		endY := pdf.GetY() + innerPad
		if endY < rowTop+11 {
			endY = rowTop + 11
		}
		pdf.SetFillColor(panelR, panelG, panelB)
		pdf.SetDrawColor(borderR, borderG, borderB)
		pdf.SetLineWidth(0.15)
		pdf.RoundedRect(tx0, rowTop, textW, endY-rowTop, 2.2, "1234", "FD")
		pdf.SetXY(tx0+innerPad, rowTop+innerPad)
		pdf.SetFont("Helvetica", "B", 8.8)
		pdf.SetTextColor(textPrimaryR, textPrimaryG, textPrimaryB)
		pdf.MultiCell(textW-2*innerPad, 4.0, tr(title), "", "L", false)
		if desc != "" {
			pdf.SetX(tx0 + innerPad)
			pdf.SetFont("Helvetica", "", 8)
			pdf.SetTextColor(textMidR, textMidG, textMidB)
			pdf.MultiCell(textW-2*innerPad, 3.7, tr(desc), "", "L", false)
		}
		pdf.SetY(endY + 1.8)
	}
}

// ── RCA sections ──────────────────────────────────────────────────────────────

func renderPDFRCASections(pdf *fpdf.Fpdf, tr func(string) string, rca store.CaseRCA) {
	rca = rca.Normalize()
	// ROOT CAUSE (highlighted box)
	if strings.TrimSpace(rca.RootCause) != "" {
		sectionHeader(pdf, "ROOT CAUSE")
		drawRootCauseBox(pdf, tr, rca.RootCause)
		pdf.Ln(1)
	} else {
		pdfRCATextBlock(pdf, tr, "ROOT CAUSE", "")
	}

	// TIMELINE INSIDEN (visual timeline)
	sectionHeader(pdf, "TIMELINE INSIDEN")
	if strings.TrimSpace(rca.IncidentTimeline) == "" {
		pdf.SetX(marginL)
		pdf.SetFont("Helvetica", "I", rcaBodyFontPt)
		pdf.SetTextColor(textDimR, textDimG, textDimB)
		pdf.MultiCell(contentW, rcaBodyLineMM, tr("(Belum diisi)"), "", "L", false)
		pdf.Ln(afterRcaBlockLn)
	} else {
		drawTimelineFromText(pdf, tr, rca.IncidentTimeline)
		pdf.Ln(1)
	}

	// Rantai penyebab (5 whys) tetap ada, tapi formatnya bagian analisis (opsional).
	draw5WhysSection(pdf, tr, rca.FiveWhys)

	pdfRCATextBlock(pdf, tr, "TEMUAN UTAMA", rca.ContributingFactors)
	pdfRCATextBlock(pdf, tr, "PERBAIKAN YANG DITERAPKAN", rca.CorrectiveActions)
	pdfRCATextBlock(pdf, tr, "PENCEGAHAN & TINDAK LANJUT", rca.PreventiveActions)
	pdfActionItemsBlock(pdf, tr, "ACTION ITEMS", rca.ActionItems)
	pdfRCATextBlock(pdf, tr, "CELAH DETEKSI", rca.DetectionGap)
}

func pdfRCATextBlock(pdf *fpdf.Fpdf, tr func(string) string, title, body string) {
	body = strings.TrimSpace(body)
	sectionHeader(pdf, title)
	pdf.SetX(marginL)
	if body == "" {
		pdf.SetFont("Helvetica", "I", rcaBodyFontPt)
		pdf.SetTextColor(textDimR, textDimG, textDimB)
		pdf.MultiCell(contentW, rcaBodyLineMM, tr("(Belum diisi)"), "", "L", false)
		pdf.Ln(afterRcaBlockLn)
		return
	}
	innerW := contentW - 10.0
	startY := pdf.GetY()
	pdf.SetFont("Helvetica", "", rcaBodyFontPt)
	pdf.SetTextColor(textPrimaryR, textPrimaryG, textPrimaryB)
	pdf.SetXY(marginL+5, startY+3)
	pdf.MultiCell(innerW, rcaBodyLineMM, tr(body), "", "L", false)
	endY := pdf.GetY() + 2
	pdf.SetFillColor(panelR, panelG, panelB)
	pdf.SetDrawColor(borderR, borderG, borderB)
	pdf.SetLineWidth(0.15)
	pdf.RoundedRect(marginL, startY, contentW, endY-startY, 2.2, "1234", "FD")
	pdf.SetXY(marginL+5, startY+3)
	pdf.SetFont("Helvetica", "", rcaBodyFontPt)
	pdf.SetTextColor(textPrimaryR, textPrimaryG, textPrimaryB)
	pdf.MultiCell(innerW, rcaBodyLineMM, tr(body), "", "L", false)
	pdf.SetY(endY)
	pdf.Ln(afterRcaBlockLn)
}

func pdfActionItemsBlock(pdf *fpdf.Fpdf, tr func(string) string, title string, items []string) {
	sectionHeader(pdf, title)
	pdf.SetX(marginL)
	if len(items) == 0 {
		pdf.SetFont("Helvetica", "I", rcaBodyFontPt)
		pdf.SetTextColor(textDimR, textDimG, textDimB)
		pdf.MultiCell(contentW, rcaBodyLineMM, tr("(Belum diisi)"), "", "L", false)
		pdf.Ln(afterRcaBlockLn)
		return
	}
	idx := 0
	for _, it := range items {
		it = strings.TrimSpace(it)
		if it == "" {
			continue
		}
		idx++
		breakPageIfNotEnoughSpace(pdf, 14)
		rowY := pdf.GetY()
		tx := marginL + 11.5
		tw := contentW - 12.0
		innerPadX := 3.5
		innerTop := 2.2
		minBoxH := 8.2
		badgeD := 5.2
		badgeX := marginL + 3.2

		pdf.SetFont("Helvetica", "", rcaBodyFontPt)
		pdf.SetTextColor(textPrimaryR, textPrimaryG, textPrimaryB)
		pdf.SetXY(tx+innerPadX, rowY+innerTop)
		pdf.MultiCell(tw-7, rcaBodyLineMM, tr(it), "", "L", false)
		endY := pdf.GetY() + 1.2
		boxH := endY - rowY
		if boxH < minBoxH {
			boxH = minBoxH
			endY = rowY + boxH
		}
		badgeY := rowY + (boxH-badgeD)/2
		pdf.SetFillColor(accentR, accentG, accentB)
		pdf.RoundedRect(badgeX, badgeY, badgeD, badgeD, 1.2, "1234", "F")
		pdf.SetFont("Helvetica", "B", 6.5)
		pdf.SetTextColor(255, 255, 255)
		num := fmt.Sprintf("%d", idx)
		nw := pdf.GetStringWidth(num)
		pdf.SetXY(badgeX+(badgeD-nw)/2, badgeY+1.1)
		pdf.CellFormat(nw, 3, num, "", 0, "C", false, 0, "")

		pdf.SetFillColor(239, 246, 255)
		pdf.SetDrawColor(191, 219, 254)
		pdf.SetLineWidth(0.12)
		pdf.RoundedRect(tx, rowY, tw, boxH, 2, "1234", "FD")
		pdf.SetXY(tx+innerPadX, rowY+innerTop)
		pdf.SetFont("Helvetica", "", rcaBodyFontPt)
		pdf.SetTextColor(textPrimaryR, textPrimaryG, textPrimaryB)
		pdf.MultiCell(tw-7, rcaBodyLineMM, tr(it), "", "L", false)
		pdf.SetY(endY + 1.6)
	}
	pdf.Ln(afterRcaBlockLn)
}

// draw5WhysSection renders a visual cascade of five-why cards.
func draw5WhysSection(pdf *fpdf.Fpdf, tr func(string) string, whys []string) {
	type colorSet struct{ bg, border, num [3]int }
	palette := []colorSet{
		{[3]int{239, 246, 255}, [3]int{191, 219, 254}, [3]int{37, 99, 235}},   // blue
		{[3]int{240, 249, 255}, [3]int{186, 230, 253}, [3]int{2, 132, 199}},   // sky
		{[3]int{236, 254, 255}, [3]int{165, 243, 252}, [3]int{8, 145, 178}},   // cyan
		{[3]int{240, 253, 250}, [3]int{153, 246, 228}, [3]int{13, 148, 136}},  // teal
		{[3]int{236, 253, 245}, [3]int{167, 243, 208}, [3]int{22, 163, 74}},   // green
	}

	nonEmpty := []int{}
	for i, w := range whys {
		if strings.TrimSpace(w) != "" {
			nonEmpty = append(nonEmpty, i)
		}
	}
	if len(nonEmpty) == 0 {
		sectionHeader(pdf, "ANALISIS")
		for i := 0; i < 5; i++ {
			pdf.SetX(marginL)
			pdf.SetFont("Helvetica", "I", 8)
			pdf.SetTextColor(textDimR, textDimG, textDimB)
			pdf.MultiCell(contentW, whyBodyLineMM, tr(fmt.Sprintf("Analisis %d: (belum diisi)", i+1)), "", "L", false)
		}
		pdf.Ln(1)
		return
	}

	sectionHeader(pdf, "ANALISIS")

	dotX := marginL + 6.5
	cardX := marginL + 16.0
	cardW := contentW - 16.0

	for idx, wi := range nonEmpty {
		w := strings.TrimSpace(whys[wi])
		c := palette[wi%len(palette)]

		y := pdf.GetY()
		cardH := whyCardH

		// Connecting line from previous card
		if idx > 0 {
			pdf.SetDrawColor(c.border[0], c.border[1], c.border[2])
			pdf.Line(dotX, y-1.5, dotX, y+2.5)
		}

		// Number circle
		pdf.SetFillColor(c.num[0], c.num[1], c.num[2])
		pdf.Circle(dotX, y+cardH/2, 4.2, "F")
		pdf.SetFont("Helvetica", "B", 7)
		pdf.SetTextColor(255, 255, 255)
		numStr := fmt.Sprintf("%d", wi+1)
		numW := pdf.GetStringWidth(numStr)
		pdf.SetXY(dotX-numW/2-0.5, y+cardH/2-2.2)
		pdf.CellFormat(numW+1, 4.2, numStr, "", 0, "C", false, 0, "")

		// "Why N" label inside circle region — small chip above card
		whyLabel := fmt.Sprintf("WHY %d", wi+1)
		_ = whyLabel

		// Card
		pdf.SetFillColor(c.bg[0], c.bg[1], c.bg[2])
		pdf.SetDrawColor(c.border[0], c.border[1], c.border[2])
		pdf.RoundedRect(cardX, y, cardW, cardH, 2.5, "1234", "FD")

		// "WHY N" chip inside card header
		chipW := 15.0
		pdf.SetFillColor(c.num[0], c.num[1], c.num[2])
		pdf.RoundedRect(cardX+3, y+2, chipW, 4.8, 1.2, "1234", "F")
		pdf.SetFont("Helvetica", "B", 6)
		pdf.SetTextColor(255, 255, 255)
		pdf.SetXY(cardX+3, y+2.6)
		pdf.CellFormat(chipW, 3.5, fmt.Sprintf("WHY %d", wi+1), "", 0, "C", false, 0, "")

		// Event text
		pdf.SetFont("Helvetica", "", 8)
		pdf.SetTextColor(textPrimaryR, textPrimaryG, textPrimaryB)
		pdf.SetXY(cardX+20, y+2.4)
		pdf.MultiCell(cardW-24, whyBodyLineMM, tr(w), "", "L", false)

		pdf.SetY(y + cardH + 1.2)
	}

	pdf.Ln(0.5)
}

// ── Checklist ─────────────────────────────────────────────────────────────────

func drawChecklist(pdf *fpdf.Fpdf, tr func(string) string, steps []store.CaseStep, stepAtts map[int64][]store.CaseAttachment, uploadRoot string) {
	for _, st := range steps {
		y := pdf.GetY()
		done := st.DoneAt != nil
		boxSize := 4.5

		// Checkbox
		if done {
			pdf.SetFillColor(greenR, greenG, greenB)
			pdf.SetDrawColor(greenR, greenG, greenB)
			pdf.RoundedRect(marginL, y+0.5, boxSize, boxSize, 1, "1234", "FD")
			// Checkmark via a simple tick line
			pdf.SetDrawColor(255, 255, 255)
			pdf.Line(marginL+1, y+3, marginL+2, y+4.5)
			pdf.Line(marginL+2, y+4.5, marginL+3.8, y+1.5)
		} else {
			pdf.SetFillColor(255, 255, 255)
			pdf.SetDrawColor(borderR, borderG, borderB)
			pdf.RoundedRect(marginL, y+0.5, boxSize, boxSize, 1, "1234", "FD")
		}

		// Step title
		pdf.SetFont("Helvetica", "B", 8)
		if done {
			pdf.SetTextColor(textMidR, textMidG, textMidB)
		} else {
			pdf.SetTextColor(textPrimaryR, textPrimaryG, textPrimaryB)
		}
		pdf.SetXY(marginL+boxSize+3, y)
		pdf.MultiCell(contentW-boxSize-3, stepTitleLineMM, tr(st.Title), "", "L", false)

		// Meta line
		meta := []string{}
		if st.DoneAt != nil {
			meta = append(meta, "selesai "+fmtTimePDF(*st.DoneAt))
		}
		if st.DoneBy != "" {
			meta = append(meta, "oleh "+st.DoneBy)
		}
		if strings.TrimSpace(st.Notes) != "" {
			meta = append(meta, st.Notes)
		}
		if strings.TrimSpace(st.EvidenceURL) != "" {
			meta = append(meta, st.EvidenceURL)
		}
		if len(meta) > 0 {
			pdf.SetFont("Helvetica", "", 7)
			pdf.SetTextColor(textDimR, textDimG, textDimB)
			pdf.SetX(marginL + boxSize + 3)
			pdf.MultiCell(contentW-boxSize-3, stepMetaLineMM, tr(strings.Join(meta, "  ·  ")), "", "L", false)
		}

		// Step-level uploaded evidence images (if any)
		if stepAtts != nil {
			for _, att := range stepAtts[st.ID] {
				ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(att.FilePath), "."))
				if ext != "jpg" && ext != "jpeg" && ext != "png" && ext != "gif" {
					continue
				}
				pdf.SetFont("Helvetica", "I", 7)
				pdf.SetTextColor(accentR, accentG, accentB)
				pdf.SetX(marginL + boxSize + 3)
				pdf.MultiCell(contentW-boxSize-3, 3.5, tr("Bukti: "+att.OriginalName), "", "L", false)
				fullPath := filepath.Join(uploadRoot, filepath.FromSlash(att.FilePath))
				drawImageFit(pdf, fullPath, contentW-boxSize-3, pdfMaxStepEvidenceHMM, marginL+boxSize+3)
			}
		}
		pdf.Ln(afterStepLn)
	}
}

// ── Before / After ────────────────────────────────────────────────────────────

func drawBeforeAfterBox(pdf *fpdf.Fpdf, tr func(string) string, rca store.CaseRCA) {
	y := pdf.GetY()
	gap := 4.0
	colW := (contentW - gap) / 2
	boxH := 32.0
	headerH := 9.0

	// BEFORE — red-tinted
	pdf.SetFillColor(255, 241, 242) // red-50
	pdf.SetDrawColor(252, 165, 165) // red-300
	pdf.RoundedRect(marginL, y, colW, boxH, 3, "1234", "FD")
	pdf.SetFillColor(239, 68, 68) // red-500 header
	pdf.RoundedRect(marginL, y, colW, headerH, 3, "1234", "F")
	pdf.Rect(marginL, y+headerH/2, colW, headerH/2, "F") // flatten bottom corners of header
	pdf.SetFont("Helvetica", "B", 9)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetXY(marginL+5, y+2.5)
	pdf.CellFormat(colW-10, 5, "SEBELUM (BEFORE)", "", 0, "L", false, 0, "")

	before := "Perilaku autentikasi tidak konsisten lintas environment."
	if strings.TrimSpace(rca.ContributingFactors) != "" {
		before = strings.TrimSpace(rca.ContributingFactors)
	}
	pdf.SetFont("Helvetica", "", 8.5)
	pdf.SetTextColor(textPrimaryR, textPrimaryG, textPrimaryB)
	pdf.SetXY(marginL+5, y+headerH+3)
	pdf.MultiCell(colW-10, 5, tr(before), "", "L", false)

	// AFTER — green-tinted
	afterX := marginL + colW + gap
	pdf.SetFillColor(240, 253, 244) // green-50
	pdf.SetDrawColor(134, 239, 172) // green-300
	pdf.RoundedRect(afterX, y, colW, boxH, 3, "1234", "FD")
	pdf.SetFillColor(34, 197, 94) // green-500 header
	pdf.RoundedRect(afterX, y, colW, headerH, 3, "1234", "F")
	pdf.Rect(afterX, y+headerH/2, colW, headerH/2, "F")
	pdf.SetFont("Helvetica", "B", 9)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetXY(afterX+5, y+2.5)
	pdf.CellFormat(colW-10, 5, "SESUDAH (AFTER)", "", 0, "L", false, 0, "")

	after := "Pengaturan cookie/session diperkuat, validasi alur auth distabilkan."
	if strings.TrimSpace(rca.CorrectiveActions) != "" {
		after = strings.TrimSpace(rca.CorrectiveActions)
	}
	pdf.SetFont("Helvetica", "", 8.5)
	pdf.SetTextColor(textPrimaryR, textPrimaryG, textPrimaryB)
	pdf.SetXY(afterX+5, y+headerH+3)
	pdf.MultiCell(colW-10, 5, tr(after), "", "L", false)

	// Arrow in gap
	arrowX := marginL + colW + gap/2
	arrowY := y + boxH/2
	pdf.SetFillColor(accentR, accentG, accentB)
	pdf.Circle(arrowX, arrowY, 4.5, "F")
	pdf.SetFont("Helvetica", "B", 11)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetXY(arrowX-3, arrowY-3.5)
	pdf.CellFormat(6, 7, ">", "", 0, "C", false, 0, "")

	pdf.SetY(y + boxH + 3)
}

// ── Follow-up section ─────────────────────────────────────────────────────────

func writeFollowUpSection(pdf *fpdf.Fpdf, tr func(string) string, rca store.CaseRCA) {
	defaultLines := []string{
		"Tambahkan regression test lintas browser untuk skenario autentikasi.",
		"Tetapkan SOP verifikasi cookie policy (secure, same-site, domain, expiry).",
		"Monitor error-rate autentikasi dan alert dini untuk anomali sesi.",
	}
	if strings.TrimSpace(rca.PreventiveActions) != "" {
		defaultLines = append(defaultLines, strings.TrimSpace(rca.PreventiveActions))
	}

	for i, line := range defaultLines {
		y := pdf.GetY()
		// Bullet number chip
		chipSize := 5.5
		pdf.SetFillColor(accentR, accentG, accentB)
		pdf.Circle(marginL+chipSize/2, y+chipSize/2+0.5, chipSize/2, "F")
		pdf.SetFont("Helvetica", "B", 7)
		pdf.SetTextColor(255, 255, 255)
		numStr := fmt.Sprintf("%d", i+1)
		nw := pdf.GetStringWidth(numStr)
		pdf.SetXY(marginL+chipSize/2-nw/2-0.5, y+0.3)
		pdf.CellFormat(nw+1, chipSize, numStr, "", 0, "C", false, 0, "")

		// Line text
		pdf.SetFont("Helvetica", "", 9)
		pdf.SetTextColor(textPrimaryR, textPrimaryG, textPrimaryB)
		pdf.SetXY(marginL+chipSize+3, y+0.5)
		pdf.MultiCell(contentW-chipSize-3, 5.2, tr(line), "", "L", false)
		pdf.Ln(1.5)
	}
	pdf.Ln(1)
}

// ── Session evidence table ────────────────────────────────────────────────────

func drawSessionEvidenceTable(pdf *fpdf.Fpdf, tr func(string) string, c *store.Case, steps []store.CaseStep) {
	colA := marginL
	colW := contentW / 2
	colB := marginL + colW

	// Header row
	y := pdf.GetY()
	pdf.SetFillColor(panel2R, panel2G, panel2B)
	pdf.SetDrawColor(borderR, borderG, borderB)
	pdf.Rect(colA, y, colW, 8, "FD")
	pdf.Rect(colB, y, colW, 8, "FD")

	// Left header accent
	pdf.SetFillColor(accentR, accentG, accentB)
	pdf.Rect(colA, y, 3, 8, "F")
	pdf.Rect(colB, y, 3, 8, "F")

	pdf.SetFont("Helvetica", "B", 8.5)
	pdf.SetTextColor(textPrimaryR, textPrimaryG, textPrimaryB)
	pdf.SetXY(colA+6, y+2)
	pdf.CellFormat(colW-8, 4.5, "Lokal (HTTP)", "", 0, "L", false, 0, "")
	pdf.SetXY(colB+6, y+2)
	pdf.CellFormat(colW-8, 4.5, "Dev / Production (HTTPS)", "", 0, "L", false, 0, "")
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
		rowY := pdf.GetY()
		alt := i%2 == 1
		if alt {
			pdf.SetFillColor(252, 252, 253)
		} else {
			pdf.SetFillColor(255, 255, 255)
		}
		pdf.SetDrawColor(borderR, borderG, borderB)
		pdf.Rect(colA, rowY, colW, 6.5, "FD")
		pdf.Rect(colB, rowY, colW, 6.5, "FD")

		pdf.SetFont("Helvetica", "", 8.2)
		pdf.SetTextColor(textPrimaryR, textPrimaryG, textPrimaryB)
		if i < len(left) {
			pdf.SetXY(colA+4, rowY+1.3)
			pdf.CellFormat(colW-6, 4, tr(left[i]), "", 0, "L", false, 0, "")
		}
		if i < len(right) {
			pdf.SetXY(colB+4, rowY+1.3)
			pdf.CellFormat(colW-6, 4, tr(right[i]), "", 0, "L", false, 0, "")
		}
		pdf.SetY(rowY + 6.5)
	}
}

// ── Utility helpers ───────────────────────────────────────────────────────────

func thinDivider(pdf *fpdf.Fpdf) {
	y := pdf.GetY() + 1.5
	pdf.SetDrawColor(borderR, borderG, borderB)
	pdf.Line(marginL, y, marginL+contentW, y)
	pdf.SetY(y + 3)
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

func fmtTimePDF(t time.Time) string {
	return tz.FormatWIB(t)
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

func pdfContentBottomY() float64 {
	return pdfPageHeightMM - pdfAutoPageBreakMarginMM
}

// breakPageIfNotEnoughSpace starts a new page when the cursor is too low to fit needMm.
func breakPageIfNotEnoughSpace(pdf *fpdf.Fpdf, needMm float64) {
	if pdf.GetY()+needMm > pdfContentBottomY() {
		pdf.AddPage()
	}
}

// drawLampiranImage scales case-level attachment images so they fit one page; avoids huge Y jumps and blank pages.
func drawLampiranImage(pdf *fpdf.Fpdf, tr func(string) string, fullPath, ext string) bool {
	tp := strings.ToLower(ext)
	if tp == "jpg" {
		tp = "jpeg"
	}
	info := pdf.RegisterImageOptions(fullPath, fpdf.ImageOptions{ImageType: tp, ReadDpi: true})
	if pdf.Error() != nil || info == nil {
		pdf.ClearError()
		return false
	}
	wu, hu := info.Width(), info.Height()
	if wu <= 0 || hu <= 0 {
		return false
	}
	scale := contentW / wu
	if hu*scale > pdfMaxLampiranImageHMM {
		scale = pdfMaxLampiranImageHMM / hu
	}
	drawW, drawH := wu*scale, hu*scale
	breakPageIfNotEnoughSpace(pdf, drawH+6)
	y := pdf.GetY()
	imgTP := strings.ToUpper(tp)
	if imgTP == "JPG" {
		imgTP = "JPEG"
	}
	pdf.Image(fullPath, marginL, y, drawW, drawH, false, imgTP, 0, "")
	pdf.SetY(y + drawH + 4)
	return true
}

// drawImageFit places an image scaled to maxW x maxH with top-left at x0.
func drawImageFit(pdf *fpdf.Fpdf, fullPath string, maxW, maxH, x0 float64) {
	tp := strings.ToLower(strings.TrimPrefix(filepath.Ext(fullPath), "."))
	if tp == "jpg" {
		tp = "jpeg"
	}
	info := pdf.RegisterImageOptions(fullPath, fpdf.ImageOptions{ImageType: tp, ReadDpi: true})
	if pdf.Error() != nil || info == nil {
		pdf.ClearError()
		return
	}
	w := info.Width()
	h := info.Height()
	if w <= 0 || h <= 0 {
		return
	}
	scale := maxW / w
	if h*scale > maxH {
		scale = maxH / h
	}
	drawW := w * scale
	drawH := h * scale
	breakPageIfNotEnoughSpace(pdf, drawH+4)
	y := pdf.GetY()
	imgTP := strings.ToUpper(tp)
	if imgTP == "JPG" {
		imgTP = "JPEG"
	}
	pdf.Image(fullPath, x0, y, drawW, drawH, false, imgTP, 0, "")
	pdf.SetY(y + drawH + 3)
}

