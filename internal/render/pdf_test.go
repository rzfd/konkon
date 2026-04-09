package render

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rzfd/metatech/konkon/internal/store"
)

func countPDFPages(t *testing.T, pdf []byte) int {
	t.Helper()
	// Naive but effective for our generated PDFs:
	// "/Type /Pages" contains "/Type /Page" as a prefix, so subtract it.
	pages := bytes.Count(pdf, []byte("/Type /Page")) - bytes.Count(pdf, []byte("/Type /Pages"))
	if pages < 1 {
		t.Fatalf("expected at least 1 page, got %d", pages)
	}
	return pages
}

func TestPDF_ContainsRCAAndOmitsProgressBar(t *testing.T) {
	td := t.TempDir()

	// Create a small PNG so the "Lampiran" image path is valid.
	imgPath := filepath.Join(td, "a.png")
	f, err := os.Create(imgPath)
	if err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.Set(x, y, color.RGBA{R: 0x60, G: 0xA5, B: 0xFA, A: 0xFF})
		}
	}
	if err := png.Encode(f, img); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 4, 7, 6, 0, 0, 0, time.UTC)
	resolved := now.Add(-2 * time.Hour)

	longTimeline := strings.Repeat("timeline line — detail detail detail\n", 220)

	rca := store.CaseRCA{
		IncidentTimeline: longTimeline,
		FiveWhys:         []string{"a", "b", "c", "d", "e"},
		RootCause:        "root cause text",
		ContributingFactors: "factors text",
		CorrectiveActions:   "corrective text",
		PreventiveActions:   "preventive text",
	}.Normalize()
	rcaJSON, err := store.MarshalCaseRCAJSON(rca)
	if err != nil {
		t.Fatal(err)
	}

	c := &store.Case{
		CaseID:     "OPS-TEST-1",
		Title:      "Judul panjang untuk ngetes pagination",
		Status:     "resolved",
		Service:    "svc",
		Severity:   "P1",
		Reporter:   "tester",
		SOPSlug:    "incident-generic",
		SOPTitle:   "Respons insiden generik TechOps",
		SOPVersion: ptrInt(1),
		CreatedAt:  now.Add(-48 * time.Hour),
		UpdatedAt:  now,
		ResolvedAt: &resolved,
		SOPID:      ptrInt64(1),
		Summary:    "ringkasan",
		RCAJSON:    rcaJSON,
	}

	attachments := []store.CaseAttachment{
		{OriginalName: "a.png", FilePath: "a.png", Kind: "screenshot"},
		{OriginalName: "a2.png", FilePath: "a.png", Kind: "screenshot"},
	}

	steps := []store.CaseStep{
		{ID: 1, StepNo: 1, Title: "Langkah 1", DoneAt: &now},
		{ID: 2, StepNo: 2, Title: "Langkah 2", DoneAt: &now, DoneBy: "ops", EvidenceURL: "https://example.com"},
		{ID: 3, StepNo: 3, Title: "Langkah 3", Notes: "catatan"},
	}

	opts := DefaultPDFOptions()
	opts.IncludeChecklistProgress = false
	pdfBytes, err := PDFWithOptions(c, steps, attachments, nil, td, opts)
	if err != nil {
		t.Fatal(err)
	}

	// Make sure it paginates when content is long.
	if pages := countPDFPages(t, pdfBytes); pages < 2 {
		t.Fatalf("expected >= 2 pages, got %d", pages)
	}

	// Checklist and chronology must be removed from PDF layout.
	if bytes.Contains(pdfBytes, []byte("KRONOLOGI & CHECKLIST")) {
		t.Fatalf("pdf must not contain checklist section")
	}

	// RCA headings should exist.
	for _, needle := range []string{
		"ROOT CAUSE",
		"TIMELINE INSIDEN",
		"ANALISIS",
		"TEMUAN UTAMA",
		"PERBAIKAN YANG DITERAPKAN",
		"PENCEGAHAN & TINDAK LANJUT",
		"ACTION ITEMS",
		"CELAH DETEKSI",
	} {
		if !bytes.Contains(pdfBytes, []byte(needle)) {
			t.Fatalf("missing %q in pdf", needle)
		}
	}
}

func TestPDF_EmptyRCA_ShowsPlaceholders(t *testing.T) {
	td := t.TempDir()
	now := time.Date(2026, 4, 8, 12, 0, 0, 0, time.UTC)
	c := &store.Case{
		CaseID:    "OPS-EMPTY-1",
		Title:     "RCA placeholder smoke",
		Status:    "open",
		CreatedAt: now,
		UpdatedAt: now,
		RCAJSON:   "",
	}
	opts := DefaultPDFOptions()
	opts.IncludeChecklist = false
	pdfBytes, err := PDFWithOptions(c, nil, nil, nil, td, opts)
	if err != nil {
		t.Fatal(err)
	}
	for _, needle := range []string{
		"ROOT CAUSE",
		"TIMELINE INSIDEN",
		"ANALISIS",
		"Analisis 1:",
		"TEMUAN UTAMA",
		"Belum diisi",
	} {
		if !bytes.Contains(pdfBytes, []byte(needle)) {
			t.Fatalf("expected %q in pdf", needle)
		}
	}
}

func TestPDF_TemplateSections_ExcludeSessionEvidence(t *testing.T) {
	td := t.TempDir()
	now := time.Date(2026, 4, 8, 13, 0, 0, 0, time.UTC)
	rca := store.CaseRCA{
		RootCause:           "Status login membaca sumber sesi yang tidak konsisten.",
		IncidentTimeline:    "10:21 laporan pertama diterima\n10:30 investigasi dimulai",
		ContributingFactors: "Masalah terlihat saat akses HTTPS di iOS.",
		CorrectiveActions:   "Validasi diarahkan ke sesi login utama.",
		PreventiveActions:   "Tambah regression test iOS Safari.",
		ActionItems:         []string{"Rilis perbaikan ke production", "Review alur login lain"},
		DetectionGap:        "Perilaku lokal berbeda dengan produksi sehingga isu terlambat terdeteksi.",
	}.Normalize()
	rcaJSON, err := store.MarshalCaseRCAJSON(rca)
	if err != nil {
		t.Fatal(err)
	}
	c := &store.Case{
		CaseID:    "OPS-TEMPLATE-1",
		Title:     "Template section check",
		Status:    "open",
		CreatedAt: now,
		UpdatedAt: now,
		Summary:   "Ringkasan contoh.",
		RCAJSON:   rcaJSON,
	}
	opts := DefaultPDFOptions()
	opts.IncludeChecklist = false
	pdfBytes, err := PDFWithOptions(c, nil, nil, nil, td, opts)
	if err != nil {
		t.Fatal(err)
	}
	for _, needle := range []string{
		"RINGKASAN",
		"ROOT CAUSE",
		"TIMELINE INSIDEN",
		"TEMUAN UTAMA",
		"PERBAIKAN YANG DITERAPKAN",
		"PENCEGAHAN & TINDAK LANJUT",
		"ACTION ITEMS",
		"CELAH DETEKSI",
		"Rilis perbaikan ke production",
	} {
		if !bytes.Contains(pdfBytes, []byte(needle)) {
			t.Fatalf("expected %q in pdf", needle)
		}
	}
	if bytes.Contains(pdfBytes, []byte("BUKTI PERILAKU SESI")) {
		t.Fatalf("pdf must not contain session evidence section")
	}
}

func ptrInt(v int) *int       { return &v }
func ptrInt64(v int64) *int64 { return &v }

