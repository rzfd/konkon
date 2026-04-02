package render

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/rzfd/metatech/konkon/internal/store"
)

const anthropicURL = "https://api.anthropic.com/v1/messages"

// FinalReport generates a structured incident final report in Markdown.
// If apiKey is set, it calls the Anthropic Claude API for narrative generation.
// Otherwise it falls back to a structured template.
func FinalReport(ctx context.Context, c *store.Case, steps []store.CaseStep, apiKey string) (string, error) {
	if apiKey == "" {
		return templateFinalReport(c, steps), nil
	}
	result, err := aiFinalReport(ctx, c, steps, apiKey)
	if err != nil {
		// fall back gracefully
		return templateFinalReport(c, steps), nil
	}
	return result, nil
}

func buildCaseText(c *store.Case, steps []store.CaseStep) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Case ID: %s\n", c.CaseID))
	b.WriteString(fmt.Sprintf("Judul: %s\n", c.Title))
	if c.Summary != "" {
		b.WriteString(fmt.Sprintf("Ringkasan: %s\n", c.Summary))
	}
	if c.Service != "" {
		b.WriteString(fmt.Sprintf("Layanan: %s\n", c.Service))
	}
	if c.Severity != "" {
		b.WriteString(fmt.Sprintf("Severity: %s\n", c.Severity))
	}
	b.WriteString(fmt.Sprintf("Status: %s\n", c.Status))
	if c.Reporter != "" {
		b.WriteString(fmt.Sprintf("Pelapor: %s\n", c.Reporter))
	}
	b.WriteString(fmt.Sprintf("Dibuat: %s\n", fmtTime(c.CreatedAt)))
	b.WriteString(fmt.Sprintf("Diperbarui: %s\n", fmtTime(c.UpdatedAt)))
	if c.ResolvedAt != nil {
		b.WriteString(fmt.Sprintf("Diselesaikan: %s\n", fmtTime(*c.ResolvedAt)))
	}
	if c.SOPSlug != "" {
		b.WriteString(fmt.Sprintf("SOP: %s — %s\n", c.SOPSlug, c.SOPTitle))
	}
	b.WriteString("\nLangkah-langkah checklist:\n")
	for _, st := range steps {
		status := "belum selesai"
		if st.DoneAt != nil {
			status = fmt.Sprintf("selesai %s", fmtTime(*st.DoneAt))
			if st.DoneBy != "" {
				status += fmt.Sprintf(" oleh %s", st.DoneBy)
			}
		}
		opt := ""
		if st.Optional {
			opt = " [opsional]"
		}
		b.WriteString(fmt.Sprintf("%d. %s%s — %s\n", st.StepNo, st.Title, opt, status))
		if strings.TrimSpace(st.Notes) != "" {
			b.WriteString(fmt.Sprintf("   Catatan: %s\n", st.Notes))
		}
		if strings.TrimSpace(st.EvidenceURL) != "" {
			b.WriteString(fmt.Sprintf("   Bukti: %s\n", st.EvidenceURL))
		}
	}
	return b.String()
}

func aiFinalReport(ctx context.Context, c *store.Case, steps []store.CaseStep, apiKey string) (string, error) {
	systemPrompt := `Anda adalah penulis laporan insiden teknis profesional untuk tim TechOps. Tugas Anda adalah membuat Final Report Insiden yang formal dan komprehensif berdasarkan data case yang diberikan.

Format laporan HARUS mengikuti struktur berikut (dalam Bahasa Indonesia):

# Final Report Insiden
## {CaseID} — {Judul Case}

### 1. Ringkasan Insiden
[Paragraf ringkas yang menjelaskan insiden: apa yang terjadi, layanan mana yang terdampak, severity, dan dampak utama]

### 2. Dampak
[Bullet point yang menjelaskan dampak operasional dari insiden ini]

### 3. Temuan Awal
[Deskripsi narasi dari temuan awal berdasarkan langkah-langkah investigasi, catatan, dan evidence yang tersedia]

### 4. Investigasi
[Narasi proses investigasi: apa yang dicek, sumber data yang digunakan, dan temuan teknis dari catatan langkah]

### 5. Tindakan Mitigasi
[Deskripsi langkah-langkah mitigasi yang dilakukan berdasarkan catatan di checklist]

### 6. Hasil Verifikasi
[Hasil setelah mitigasi dilakukan: apakah layanan kembali normal, monitoring, dll]

### 7. Status Akhir
[Status case saat ini dan catatan monitoring lanjutan yang diperlukan]

### 8. Dugaan Penyebab Sementara
[Analisis penyebab berdasarkan evidence yang ada — tandai sebagai dugaan jika belum dikonfirmasi]

### 9. Rekomendasi Tindak Lanjut
[Bullet point rekomendasi untuk pencegahan atau perbaikan ke depan]

PENTING:
- Tulis dalam Bahasa Indonesia yang formal dan profesional
- Buat narasi yang kohesif dan informatif berdasarkan data yang tersedia
- Gunakan catatan (Notes) dari setiap langkah sebagai bahan utama narasi
- Jangan mengarang informasi yang tidak ada dalam data case
- Gunakan format Markdown yang benar
- Langsung mulai dengan "# Final Report Insiden", jangan tambahkan teks pengantar`

	userMsg := fmt.Sprintf("Buatkan Final Report Insiden berdasarkan data case berikut:\n\n%s", buildCaseText(c, steps))

	type message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	type reqBody struct {
		Model     string    `json:"model"`
		MaxTokens int       `json:"max_tokens"`
		System    string    `json:"system"`
		Messages  []message `json:"messages"`
	}

	payload, err := json.Marshal(reqBody{
		Model:     "claude-haiku-4-5-20251001",
		MaxTokens: 2048,
		System:    systemPrompt,
		Messages:  []message{{Role: "user", Content: userMsg}},
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, anthropicURL, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var out struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(respBytes, &out); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}
	if out.Error != nil {
		return "", fmt.Errorf("anthropic error: %s", out.Error.Message)
	}
	if len(out.Content) == 0 {
		return "", fmt.Errorf("empty response")
	}
	return out.Content[0].Text, nil
}

func templateFinalReport(c *store.Case, steps []store.CaseStep) string {
	var b strings.Builder

	b.WriteString("# Final Report Insiden\n")
	b.WriteString(fmt.Sprintf("## %s — %s\n\n", c.CaseID, c.Title))

	// 1. Ringkasan
	b.WriteString("### 1. Ringkasan Insiden\n")
	svc := c.Service
	if svc == "" {
		svc = "layanan terkait"
	}
	sev := c.Severity
	if sev == "" {
		sev = "-"
	}
	b.WriteString(fmt.Sprintf("Telah terjadi insiden pada **%s** dengan nomor case **%s** dan severity **%s**.\n", svc, c.CaseID, sev))
	if c.Summary != "" {
		b.WriteString(c.Summary + "\n")
	}
	b.WriteString("\n")

	// 2. Dampak
	b.WriteString("### 2. Dampak\n")
	b.WriteString(fmt.Sprintf("- Insiden berdampak pada layanan **%s**.\n", svc))
	b.WriteString("- Memerlukan penanganan segera sesuai SOP yang berlaku.\n\n")

	// 3. Temuan Awal
	b.WriteString("### 3. Temuan Awal\n")
	var temuanAwal []string
	for _, st := range steps {
		if st.StepNo <= 2 && strings.TrimSpace(st.Notes) != "" {
			temuanAwal = append(temuanAwal, st.Notes)
		}
	}
	if len(temuanAwal) > 0 {
		b.WriteString(strings.Join(temuanAwal, " ") + "\n")
	} else {
		b.WriteString("Pengecekan awal dilakukan sesuai SOP yang berlaku.\n")
	}
	b.WriteString("\n")

	// 4. Investigasi
	b.WriteString("### 4. Investigasi\n")
	var inv []string
	for _, st := range steps {
		if strings.TrimSpace(st.Notes) != "" {
			inv = append(inv, fmt.Sprintf("- **%s**: %s", st.Title, st.Notes))
		}
	}
	if len(inv) > 0 {
		b.WriteString(strings.Join(inv, "\n") + "\n")
	} else {
		b.WriteString("Investigasi dilakukan mengikuti langkah-langkah checklist SOP.\n")
	}
	b.WriteString("\n")

	// 5. Tindakan Mitigasi
	b.WriteString("### 5. Tindakan Mitigasi\n")
	var mitig []string
	for _, st := range steps {
		if st.DoneAt != nil {
			line := fmt.Sprintf("- **%s**", st.Title)
			if st.DoneBy != "" {
				line += fmt.Sprintf(" (oleh %s)", st.DoneBy)
			}
			mitig = append(mitig, line)
		}
	}
	if len(mitig) > 0 {
		b.WriteString(strings.Join(mitig, "\n") + "\n")
	} else {
		b.WriteString("Tindakan mitigasi sedang dalam proses.\n")
	}
	b.WriteString("\n")

	// 6. Hasil Verifikasi
	b.WriteString("### 6. Hasil Verifikasi\n")
	if c.Status == "resolved" {
		b.WriteString("Setelah mitigasi dilakukan, layanan kembali berjalan normal. Monitoring lanjutan tetap dilakukan.\n")
	} else {
		b.WriteString("Verifikasi sedang berlangsung. Monitoring terus dilakukan untuk memastikan kestabilan layanan.\n")
	}
	b.WriteString("\n")

	// 7. Status Akhir
	b.WriteString("### 7. Status Akhir\n")
	statusLabel := map[string]string{
		"resolved":     "**Resolved**",
		"open":         "**Open** (masih dalam penanganan)",
		"needs_triage": "**Needs Triage** (menunggu triage)",
	}
	label, ok := statusLabel[c.Status]
	if !ok {
		label = fmt.Sprintf("**%s**", c.Status)
	}
	b.WriteString(fmt.Sprintf("Insiden saat ini berstatus %s. Monitoring lanjutan tetap diperlukan.\n\n", label))

	// 8. Dugaan Penyebab
	b.WriteString("### 8. Dugaan Penyebab Sementara\n")
	b.WriteString("Berdasarkan evidence yang tersedia, investigasi masih dalam proses untuk mengidentifikasi root cause secara definitif. Catatan ini bersifat indikatif.\n\n")

	// 9. Rekomendasi
	b.WriteString("### 9. Rekomendasi Tindak Lanjut\n")
	b.WriteString("- Lakukan review pasca-insiden untuk mengidentifikasi root cause lebih lanjut.\n")
	b.WriteString("- Pastikan langkah-langkah SOP telah diikuti secara lengkap.\n")
	b.WriteString("- Pertimbangkan penambahan monitoring atau alerting untuk mencegah insiden serupa.\n")
	b.WriteString("- Dokumentasikan temuan ke dalam backlog atau issue tracker jika diperlukan.\n")

	return b.String()
}
