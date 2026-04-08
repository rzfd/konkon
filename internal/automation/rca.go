package automation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/rzfd/metatech/konkon/internal/store"
	"github.com/rzfd/metatech/konkon/internal/tz"
)

const anthropicURL = "https://api.anthropic.com/v1/messages"

// Input bundles the evidence used to build an RCA draft.
type Input struct {
	Case            *store.Case
	Steps           []store.CaseStep
	Audit           []store.CaseAudit
	Attachments     []store.CaseAttachment
	StepAttachments map[int64][]store.CaseAttachment
}

// Draft is a generated RCA plus lightweight metadata for the UI.
type Draft struct {
	RCA        store.CaseRCA `json:"rca"`
	Source     string        `json:"source"`
	Confidence string        `json:"confidence"`
	Notes      []string      `json:"notes,omitempty"`
}

// Generator creates RCA drafts from stored incident evidence.
type Generator struct {
	apiKey string
	client *http.Client
}

// NewGenerator creates an RCA draft generator.
func NewGenerator(apiKey string) *Generator {
	return &Generator{
		apiKey: strings.TrimSpace(apiKey),
		client: http.DefaultClient,
	}
}

// Generate returns a draft RCA. It always falls back to a deterministic draft.
func (g *Generator) Generate(ctx context.Context, in Input) (Draft, error) {
	if in.Case == nil {
		return Draft{}, errors.New("missing case")
	}

	base := heuristicDraft(in)
	if g == nil || g.apiKey == "" {
		return base, nil
	}

	refined, err := g.generateWithAnthropic(ctx, in, base.RCA)
	if err != nil {
		base.Notes = append(base.Notes, "AI draft tidak tersedia, fallback ke draft heuristik.")
		return base, nil
	}

	return Draft{
		RCA:        mergeRCA(base.RCA, refined).Normalize(),
		Source:     "anthropic",
		Confidence: "medium",
		Notes:      base.Notes,
	}, nil
}

func heuristicDraft(in Input) Draft {
	c := in.Case
	service := strings.TrimSpace(c.Service)
	if service == "" {
		service = "layanan terkait"
	}

	hint, score := inferRootCauseHint(in)
	missingEvidence := collectMissingEvidence(in)
	openSteps := collectOpenSteps(in.Steps)

	rca := store.CaseRCA{
		IncidentTimeline:    buildTimeline(in),
		FiveWhys:            buildFiveWhys(service, hint, score, missingEvidence, openSteps),
		RootCause:           buildRootCause(service, hint, score),
		ContributingFactors: buildContributingFactors(in, hint, missingEvidence, openSteps),
		CorrectiveActions:   buildCorrectiveActions(in.Steps),
		PreventiveActions:   buildPreventiveActions(service, hint, missingEvidence, openSteps),
		ActionItems:         buildActionItems(service, hint, missingEvidence, openSteps),
		DetectionGap:        buildDetectionGap(in, missingEvidence),
	}.Normalize()

	confidence := "low"
	if score >= 2 || countDoneSteps(in.Steps) >= 2 {
		confidence = "medium"
	}

	return Draft{
		RCA:        rca,
		Source:     "heuristic",
		Confidence: confidence,
	}
}

func buildTimeline(in Input) string {
	entries := make([]timelineEntry, 0, len(in.Audit)+len(in.Steps)+2)
	c := in.Case

	summary := cleanSnippet(c.Summary, 220)
	if summary == "" {
		summary = cleanSnippet(c.Title, 220)
	}
	if summary != "" {
		line := "Case dibuat"
		if reporter := strings.TrimSpace(c.Reporter); reporter != "" {
			line += " oleh " + reporter
		}
		line += " — " + summary
		entries = append(entries, timelineEntry{At: c.CreatedAt, Text: line})
	}

	audit := append([]store.CaseAudit(nil), in.Audit...)
	sort.SliceStable(audit, func(i, j int) bool {
		return audit[i].CreatedAt.Before(audit[j].CreatedAt)
	})
	for _, ev := range audit {
		if line := auditLine(ev); line != "" {
			entries = append(entries, timelineEntry{At: ev.CreatedAt, Text: line})
		}
	}

	for _, st := range in.Steps {
		if st.DoneAt == nil {
			continue
		}
		line := fmt.Sprintf("Langkah %d selesai: %s", st.StepNo, strings.TrimSpace(st.Title))
		if note := cleanSnippet(st.Notes, 180); note != "" {
			line += " — " + note
		}
		entries = append(entries, timelineEntry{At: *st.DoneAt, Text: line})
	}

	if c.ResolvedAt != nil {
		entries = append(entries, timelineEntry{At: *c.ResolvedAt, Text: "Case ditutup sebagai resolved"})
	}

	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].At.Equal(entries[j].At) {
			return entries[i].Text < entries[j].Text
		}
		return entries[i].At.Before(entries[j].At)
	})

	lines := make([]string, 0, len(entries))
	seen := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		text := strings.TrimSpace(entry.Text)
		if text == "" {
			continue
		}
		line := text
		if !entry.At.IsZero() {
			line = timelineStamp(entry.At) + " | " + text
		}
		if _, ok := seen[line]; ok {
			continue
		}
		seen[line] = struct{}{}
		lines = append(lines, line)
	}
	if len(lines) > 12 {
		lines = lines[:12]
	}
	return strings.Join(lines, "\n")
}

func timelineStamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.In(tz.Jakarta).Format("020106 15:04:05")
}

func buildFiveWhys(service, hint string, score int, missingEvidence, openSteps []string) []string {
	whys := make([]string, 5)
	if score <= 0 {
		whys[0] = fmt.Sprintf("Mengapa insiden perlu diinvestigasi? Karena %s mengalami gangguan yang memicu pembukaan case.", service)
		if len(missingEvidence) > 0 {
			whys[1] = "Mengapa akar masalah belum pasti? Karena evidence untuk beberapa langkah wajib masih belum lengkap."
		}
		if len(openSteps) > 0 {
			whys[2] = "Mengapa investigasi belum tuntas? Karena masih ada langkah checklist yang belum selesai."
		}
		return whys
	}

	switch hint {
	case "auth":
		whys[0] = fmt.Sprintf("Mengapa %s terdampak? Karena ada indikasi gangguan pada autentikasi atau sesi pengguna.", service)
		whys[1] = "Mengapa gejala auth/sesi muncul? Karena validasi state sesi atau konfigurasi login kemungkinan tidak konsisten."
	case "deploy":
		whys[0] = fmt.Sprintf("Mengapa %s terdampak? Karena indikasi terkuat mengarah ke perubahan deploy atau konfigurasi.", service)
		whys[1] = "Mengapa perubahan tersebut berdampak? Karena guardrail atau verifikasi pasca-rilis belum cukup cepat menangkap regresi."
	case "database":
		whys[0] = fmt.Sprintf("Mengapa %s terdampak? Karena evidence paling sering mengarah ke lapisan database atau query.", service)
		whys[1] = "Mengapa gangguan database berdampak luas? Karena layanan bergantung langsung pada query/koneksi tersebut."
	case "network":
		whys[0] = fmt.Sprintf("Mengapa %s terdampak? Karena terdapat indikasi timeout, jaringan, atau dependency yang tidak stabil.", service)
		whys[1] = "Mengapa timeout berdampak ke user? Karena retry, fallback, atau isolasi kegagalan belum cukup melindungi alur utama."
	case "resource":
		whys[0] = fmt.Sprintf("Mengapa %s terdampak? Karena ada indikasi bottleneck resource pada aplikasi atau node.", service)
		whys[1] = "Mengapa bottleneck terjadi? Karena kapasitas, proteksi lonjakan, atau tuning runtime belum memadai."
	case "dependency":
		whys[0] = fmt.Sprintf("Mengapa %s terdampak? Karena gejala paling kuat mengarah ke dependency eksternal/internal yang gagal.", service)
		whys[1] = "Mengapa kegagalan dependency memicu user impact? Karena fallback atau degradasi terkontrol belum cukup efektif."
	}
	if len(missingEvidence) > 0 {
		whys[2] = "Mengapa konfirmasi akhir belum kuat? Karena belum semua langkah wajib memiliki evidence yang bisa diverifikasi."
	}
	if len(openSteps) > 0 {
		whys[3] = "Mengapa tindak lanjut masih diperlukan? Karena ada langkah investigasi atau verifikasi yang masih terbuka."
	}
	return whys
}

func buildRootCause(service, hint string, score int) string {
	if score <= 0 {
		return fmt.Sprintf("Akar masalah belum terkonfirmasi. Berdasarkan data case saat ini, investigasi perlu difokuskan pada korelasi gejala awal, perubahan terbaru, dan evidence observability pada %s.", service)
	}
	switch hint {
	case "auth":
		return fmt.Sprintf("Dugaan awal mengarah ke masalah autentikasi/sesi pada %s. Klaim ini masih perlu divalidasi dengan log aplikasi, perubahan konfigurasi login, dan bukti reproduksi.", service)
	case "deploy":
		return fmt.Sprintf("Dugaan awal mengarah ke perubahan deploy atau konfigurasi pada %s. Validasi akhir perlu memastikan perubahan mana yang paling dekat dengan awal gangguan.", service)
	case "database":
		return fmt.Sprintf("Dugaan awal mengarah ke gangguan database/query yang mempengaruhi %s. Konfirmasi akhir memerlukan korelasi log error, latency, dan perubahan schema/query.", service)
	case "network":
		return fmt.Sprintf("Dugaan awal mengarah ke timeout, konektivitas, atau dependency network yang berdampak ke %s. Perlu validasi tambahan dari trace, metric latency, dan status dependency.", service)
	case "resource":
		return fmt.Sprintf("Dugaan awal mengarah ke keterbatasan resource pada komponen %s. Konfirmasi akhir perlu melihat metrik CPU, memory, queue, dan saturation.", service)
	case "dependency":
		return fmt.Sprintf("Dugaan awal mengarah ke kegagalan dependency yang digunakan oleh %s. Validasi akhir perlu memastikan dependency mana yang gagal dan bagaimana efek propagasinya.", service)
	default:
		return fmt.Sprintf("Akar masalah sementara untuk %s belum dapat dipastikan dari evidence yang ada.", service)
	}
}

func buildContributingFactors(in Input, hint string, missingEvidence, openSteps []string) string {
	items := []string{}
	if summary := cleanSnippet(in.Case.Summary, 220); summary != "" {
		items = append(items, "Dampak awal yang tercatat: "+summary)
	}
	if hint != "" {
		items = append(items, "Catatan investigasi paling sering mengarah ke area "+hintLabel(hint)+".")
	}
	if len(openSteps) > 0 {
		items = append(items, "Masih ada langkah checklist terbuka: "+joinForSentence(openSteps, 3)+".")
	}
	if len(missingEvidence) > 0 {
		items = append(items, "Evidence untuk langkah wajib belum lengkap: "+joinForSentence(missingEvidence, 3)+".")
	}
	if len(in.Attachments) == 0 && totalStepAttachments(in.StepAttachments) == 0 {
		items = append(items, "Belum ada lampiran evidence terstruktur pada case ini.")
	}
	if len(items) == 0 {
		items = append(items, "Temuan utama masih terbatas pada data case, audit, dan checklist yang tersedia.")
	}
	return bullets(items)
}

func buildCorrectiveActions(steps []store.CaseStep) string {
	items := []string{}
	for _, st := range steps {
		if st.DoneAt == nil {
			continue
		}
		line := strings.TrimSpace(st.Title)
		if note := cleanSnippet(st.Notes, 180); note != "" {
			line += " — " + note
		}
		items = append(items, line)
		if len(items) >= 5 {
			break
		}
	}
	if len(items) == 0 {
		items = append(items, "Belum ada tindakan korektif yang tercatat sebagai selesai pada checklist.")
	}
	return bullets(items)
}

func buildPreventiveActions(service, hint string, missingEvidence, openSteps []string) string {
	items := []string{
		fmt.Sprintf("Tambahkan guardrail dan regression test pada area %s.", service),
		"Pastikan perubahan terbaru dapat ditelusuri cepat ke deploy, config, credential, atau dependency yang relevan.",
	}
	switch hint {
	case "auth":
		items[0] = fmt.Sprintf("Tambahkan regression test untuk alur login, sesi, dan token pada %s.", service)
	case "deploy":
		items[0] = fmt.Sprintf("Perketat verifikasi pasca-rilis dan rollback plan untuk %s.", service)
	case "database":
		items[0] = fmt.Sprintf("Tambahkan alert dan review performa query/database yang menopang %s.", service)
	case "network":
		items[0] = fmt.Sprintf("Perkuat timeout, retry, dan fallback untuk dependency yang digunakan %s.", service)
	case "resource":
		items[0] = fmt.Sprintf("Tambahkan kapasitas, autoscaling, atau proteksi saturation pada komponen %s.", service)
	case "dependency":
		items[0] = fmt.Sprintf("Perkuat fallback dan health check dependency yang menopang %s.", service)
	}
	if len(missingEvidence) > 0 || len(openSteps) > 0 {
		items = append(items, "Perbarui SOP agar evidence wajib terlampir dan verifikasi penutupan case lebih kuat.")
	}
	return bullets(items)
}

func buildActionItems(service, hint string, missingEvidence, openSteps []string) []string {
	items := make([]string, 0, 5)
	for _, title := range openSteps {
		items = append(items, "Selesaikan langkah checklist: "+title)
		if len(items) >= 5 {
			return items
		}
	}
	for _, title := range missingEvidence {
		items = append(items, "Lampirkan evidence untuk langkah: "+title)
		if len(items) >= 5 {
			return items
		}
	}
	switch hint {
	case "auth":
		items = append(items, "Tambahkan uji regresi autentikasi/sesi pada jalur kritis.")
	case "deploy":
		items = append(items, "Review perubahan deploy atau config yang paling dekat dengan awal insiden.")
	case "database":
		items = append(items, "Kumpulkan query, error log, dan latency database pada jendela insiden.")
	case "network":
		items = append(items, "Verifikasi status dependency, timeout, dan trace lintas layanan.")
	case "resource":
		items = append(items, "Kumpulkan metric CPU, memory, queue, dan saturation saat insiden.")
	case "dependency":
		items = append(items, "Pastikan dependency failure memiliki fallback dan alarm yang jelas.")
	default:
		items = append(items, "Korelasikan gejala awal dengan deploy, log, metric, dan dependency terkait.")
	}
	if len(items) < 5 {
		items = append(items, "Review kembali timeline dan pastikan semua perubahan signifikan tercatat.")
	}
	if len(items) > 5 {
		items = items[:5]
	}
	return items
}

func buildDetectionGap(in Input, missingEvidence []string) string {
	text := evidenceCorpus(in)
	manualSignals := strings.TrimSpace(in.Case.Reporter) != ""
	hasAlertSignal := containsAny(text, "alert", "monitor", "grafana", "datadog", "pagerduty", "alarm", "notified")

	switch {
	case manualSignals && !hasAlertSignal:
		return "Deteksi awal tampaknya berasal dari pelaporan manual, bukan alert otomatis yang tervalidasi. Perlu memastikan sinyal monitoring dapat menangkap gejala sebelum user impact meluas."
	case len(missingEvidence) > 0:
		return "Terdapat gap pada kualitas evidence investigasi. Beberapa langkah wajib belum memiliki bukti yang cukup untuk mempercepat konfirmasi akar masalah."
	default:
		return "Perlu memastikan alert dan dashboard utama cukup cepat menunjukkan gejala teknis sebelum eskalasi manual terjadi."
	}
}

func inferRootCauseHint(in Input) (string, int) {
	text := evidenceCorpus(in)
	if text == "" {
		return "", 0
	}
	hints := []struct {
		Key     string
		Needles []string
	}{
		{Key: "auth", Needles: []string{"auth", "login", "session", "cookie", "token", "oauth", "sso"}},
		{Key: "deploy", Needles: []string{"deploy", "release", "rollback", "config", "feature flag", "migration", "hotfix"}},
		{Key: "database", Needles: []string{"database", "db ", "postgres", "sql", "query", "deadlock", "connection pool", "lock timeout"}},
		{Key: "network", Needles: []string{"timeout", "latency", "network", "dns", "socket", "tls", "connection reset", "unreachable"}},
		{Key: "resource", Needles: []string{"cpu", "memory", "oom", "resource", "saturation", "throttle", "disk full"}},
		{Key: "dependency", Needles: []string{"dependency", "third party", "upstream", "downstream", "redis", "kafka", "queue", "external api"}},
	}

	bestKey := ""
	bestScore := 0
	for _, hint := range hints {
		score := 0
		for _, needle := range hint.Needles {
			score += strings.Count(text, needle)
		}
		if score > bestScore {
			bestKey = hint.Key
			bestScore = score
		}
	}
	return bestKey, bestScore
}

func evidenceCorpus(in Input) string {
	parts := []string{
		strings.ToLower(strings.TrimSpace(in.Case.Title)),
		strings.ToLower(strings.TrimSpace(in.Case.Summary)),
	}
	for _, st := range in.Steps {
		parts = append(parts, strings.ToLower(strings.TrimSpace(st.Title)))
		parts = append(parts, strings.ToLower(strings.TrimSpace(st.Notes)))
		parts = append(parts, strings.ToLower(strings.TrimSpace(st.EvidenceURL)))
	}
	for _, ev := range in.Audit {
		parts = append(parts, strings.ToLower(strings.TrimSpace(ev.Action)))
		parts = append(parts, strings.ToLower(strings.TrimSpace(ev.Detail)))
	}
	return strings.Join(parts, "\n")
}

func collectMissingEvidence(in Input) []string {
	out := make([]string, 0, len(in.Steps))
	for _, st := range in.Steps {
		if !st.RequiresEvidence {
			continue
		}
		if strings.TrimSpace(st.EvidenceURL) != "" {
			continue
		}
		if len(in.StepAttachments[st.ID]) > 0 {
			continue
		}
		out = append(out, strings.TrimSpace(st.Title))
	}
	return out
}

func collectOpenSteps(steps []store.CaseStep) []string {
	out := make([]string, 0, len(steps))
	for _, st := range steps {
		if st.DoneAt == nil {
			out = append(out, strings.TrimSpace(st.Title))
		}
	}
	return out
}

func countDoneSteps(steps []store.CaseStep) int {
	n := 0
	for _, st := range steps {
		if st.DoneAt != nil {
			n++
		}
	}
	return n
}

func auditLine(ev store.CaseAudit) string {
	detail := cleanSnippet(ev.Detail, 180)
	switch ev.Action {
	case "case_created":
		if detail != "" {
			return "Case dicatat — " + detail
		}
		return "Case dicatat"
	case "sop_assigned":
		if detail != "" {
			return "SOP diterapkan — " + detail
		}
		return "SOP diterapkan"
	case "case_closed":
		return "Case ditutup"
	case "attachment_deleted":
		if detail != "" {
			return "Lampiran dihapus — " + detail
		}
		return "Lampiran dihapus"
	default:
		return ""
	}
}

func hintLabel(hint string) string {
	switch hint {
	case "auth":
		return "autentikasi/sesi"
	case "deploy":
		return "deploy/konfigurasi"
	case "database":
		return "database/query"
	case "network":
		return "jaringan/dependency"
	case "resource":
		return "resource aplikasi"
	case "dependency":
		return "dependency"
	default:
		return "investigasi teknis"
	}
}

func cleanSnippet(s string, max int) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	s = strings.Join(strings.Fields(s), " ")
	if max > 0 && len([]rune(s)) > max {
		r := []rune(s)
		s = strings.TrimSpace(string(r[:max])) + "…"
	}
	return s
}

func bullets(items []string) string {
	lines := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		lines = append(lines, "- "+item)
	}
	return strings.Join(lines, "\n")
}

func joinForSentence(items []string, limit int) string {
	filtered := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" {
			filtered = append(filtered, item)
		}
	}
	if len(filtered) == 0 {
		return ""
	}
	if limit > 0 && len(filtered) > limit {
		filtered = append(filtered[:limit], fmt.Sprintf("+%d lainnya", len(filtered)-limit))
	}
	return strings.Join(filtered, ", ")
}

func containsAny(s string, needles ...string) bool {
	s = strings.ToLower(s)
	for _, needle := range needles {
		if strings.Contains(s, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

func totalStepAttachments(stepAtts map[int64][]store.CaseAttachment) int {
	total := 0
	for _, list := range stepAtts {
		total += len(list)
	}
	return total
}

func mergeRCA(base, overlay store.CaseRCA) store.CaseRCA {
	out := base.Normalize()
	overlay = overlay.Normalize()
	if strings.TrimSpace(overlay.IncidentTimeline) != "" {
		out.IncidentTimeline = overlay.IncidentTimeline
	}
	for i := range out.FiveWhys {
		if i < len(overlay.FiveWhys) && strings.TrimSpace(overlay.FiveWhys[i]) != "" {
			out.FiveWhys[i] = overlay.FiveWhys[i]
		}
	}
	if strings.TrimSpace(overlay.RootCause) != "" {
		out.RootCause = overlay.RootCause
	}
	if strings.TrimSpace(overlay.ContributingFactors) != "" {
		out.ContributingFactors = overlay.ContributingFactors
	}
	if strings.TrimSpace(overlay.CorrectiveActions) != "" {
		out.CorrectiveActions = overlay.CorrectiveActions
	}
	if strings.TrimSpace(overlay.PreventiveActions) != "" {
		out.PreventiveActions = overlay.PreventiveActions
	}
	if len(overlay.ActionItems) > 0 {
		out.ActionItems = overlay.ActionItems
	}
	if strings.TrimSpace(overlay.DetectionGap) != "" {
		out.DetectionGap = overlay.DetectionGap
	}
	return out.Normalize()
}

func (g *Generator) generateWithAnthropic(ctx context.Context, in Input, seed store.CaseRCA) (store.CaseRCA, error) {
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

	payloadData, err := json.MarshalIndent(buildPromptPayload(in, seed), "", "  ")
	if err != nil {
		return store.CaseRCA{}, err
	}

	systemPrompt := `Anda membantu tim incident response menyusun draft Root Cause Analysis (RCA).
Keluarkan JSON SAJA tanpa markdown, tanpa penjelasan tambahan, dengan field:
incident_timeline (string),
five_whys ([]string, maksimal 12),
root_cause (string),
contributing_factors (string),
corrective_actions (string),
preventive_actions (string),
action_items ([]string, maksimal 12),
detection_gap (string).

Aturan:
- Gunakan Bahasa Indonesia formal, ringkas, dan operasional.
- Jangan mengarang fakta. Jika belum pasti, nyatakan sebagai dugaan sementara.
- Pakai evidence yang tersedia pada case, audit, langkah checklist, dan draft heuristik.
- corrective_actions dan preventive_actions boleh berbentuk bullet list dalam satu string.
- incident_timeline boleh multiline, satu baris satu kejadian penting.
- Format incident_timeline per baris: "ddmmyy hh:mm:ss | detail kejadian".`

	reqBytes, err := json.Marshal(reqBody{
		Model:     "claude-haiku-4-5-20251001",
		MaxTokens: 1600,
		System:    systemPrompt,
		Messages: []message{{
			Role:    "user",
			Content: "Susun draft RCA dari data berikut dan balas JSON saja.\n\n" + string(payloadData),
		}},
	})
	if err != nil {
		return store.CaseRCA{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, anthropicURL, bytes.NewReader(reqBytes))
	if err != nil {
		return store.CaseRCA{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", g.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := g.client.Do(req)
	if err != nil {
		return store.CaseRCA{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return store.CaseRCA{}, err
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
	if err := json.Unmarshal(body, &out); err != nil {
		return store.CaseRCA{}, fmt.Errorf("unmarshal anthropic response: %w", err)
	}
	if out.Error != nil {
		return store.CaseRCA{}, fmt.Errorf("anthropic error: %s", out.Error.Message)
	}
	if len(out.Content) == 0 {
		return store.CaseRCA{}, errors.New("empty anthropic response")
	}

	jsonText := extractJSONObject(out.Content[0].Text)
	if jsonText == "" {
		return store.CaseRCA{}, errors.New("anthropic response did not contain json object")
	}

	var rca store.CaseRCA
	if err := json.Unmarshal([]byte(jsonText), &rca); err != nil {
		return store.CaseRCA{}, fmt.Errorf("decode rca json: %w", err)
	}
	return rca.Normalize(), nil
}

func buildPromptPayload(in Input, seed store.CaseRCA) map[string]any {
	c := in.Case
	caseMap := map[string]any{
		"case_id":     c.CaseID,
		"title":       c.Title,
		"summary":     c.Summary,
		"service":     c.Service,
		"severity":    c.Severity,
		"status":      c.Status,
		"reporter":    c.Reporter,
		"created_at":  tz.FormatRFC3339(c.CreatedAt),
		"updated_at":  tz.FormatRFC3339(c.UpdatedAt),
		"resolved_at": "",
		"sop_slug":    c.SOPSlug,
		"sop_title":   c.SOPTitle,
	}
	if c.ResolvedAt != nil {
		caseMap["resolved_at"] = tz.FormatRFC3339(*c.ResolvedAt)
	}

	steps := make([]map[string]any, 0, len(in.Steps))
	for _, st := range in.Steps {
		row := map[string]any{
			"step_no":           st.StepNo,
			"title":             st.Title,
			"requires_evidence": st.RequiresEvidence,
			"optional":          st.Optional,
			"done_by":           st.DoneBy,
			"notes":             st.Notes,
			"evidence_url":      st.EvidenceURL,
			"attachment_count":  len(in.StepAttachments[st.ID]),
			"done_at":           "",
		}
		if st.DoneAt != nil {
			row["done_at"] = tz.FormatRFC3339(*st.DoneAt)
		}
		steps = append(steps, row)
	}

	audit := append([]store.CaseAudit(nil), in.Audit...)
	sort.SliceStable(audit, func(i, j int) bool {
		return audit[i].CreatedAt.Before(audit[j].CreatedAt)
	})
	audits := make([]map[string]any, 0, len(audit))
	for _, ev := range audit {
		audits = append(audits, map[string]any{
			"action":     ev.Action,
			"actor":      ev.Actor,
			"detail":     ev.Detail,
			"created_at": tz.FormatRFC3339(ev.CreatedAt),
		})
	}

	attachments := make([]map[string]any, 0, len(in.Attachments))
	for _, att := range in.Attachments {
		attachments = append(attachments, map[string]any{
			"kind":          att.Kind,
			"original_name": att.OriginalName,
			"created_at":    tz.FormatRFC3339(att.CreatedAt),
		})
	}

	return map[string]any{
		"case":            caseMap,
		"steps":           steps,
		"audit":           audits,
		"attachments":     attachments,
		"heuristic_seed":  seed.Normalize(),
		"generated_at":    time.Now().UTC().Format(time.RFC3339),
		"response_format": "json_only",
	}
}

func extractJSONObject(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```json")
		s = strings.TrimPrefix(s, "```")
		s = strings.TrimSuffix(s, "```")
		s = strings.TrimSpace(s)
	}
	start := strings.IndexByte(s, '{')
	end := strings.LastIndexByte(s, '}')
	if start == -1 || end == -1 || end < start {
		return ""
	}
	return s[start : end+1]
}

type timelineEntry struct {
	At   time.Time
	Text string
}
