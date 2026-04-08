package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/rzfd/metatech/konkon/internal/render"
	"github.com/rzfd/metatech/konkon/internal/sop"
	"github.com/rzfd/metatech/konkon/internal/store"
	"github.com/rzfd/metatech/konkon/internal/tz"
	"github.com/rzfd/metatech/konkon/internal/validate"
)

// ── Rate limiter ────────────────────────────────────────────────────────────

type rlClient struct {
	count   int
	resetAt time.Time
}

type rateLimiter struct {
	mu      sync.Mutex
	clients map[string]*rlClient
}

func newRateLimiter() *rateLimiter {
	rl := &rateLimiter{clients: make(map[string]*rlClient)}
	go func() {
		t := time.NewTicker(5 * time.Minute)
		for range t.C {
			rl.mu.Lock()
			now := time.Now()
			for ip, c := range rl.clients {
				if now.After(c.resetAt) {
					delete(rl.clients, ip)
				}
			}
			rl.mu.Unlock()
		}
	}()
	return rl
}

func (rl *rateLimiter) allow(ip string, limit int, window time.Duration) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	c, ok := rl.clients[ip]
	if !ok || now.After(c.resetAt) {
		rl.clients[ip] = &rlClient{count: 1, resetAt: now.Add(window)}
		return true
	}
	if c.count >= limit {
		return false
	}
	c.count++
	return true
}

// ── Server ──────────────────────────────────────────────────────────────────

// Server wires HTTP routes to the store.
type Server struct {
	log          *slog.Logger
	store        *store.Store
	uploadRoot   string
	anthropicKey string
	rl           *rateLimiter
}

// New creates an API server.
func New(logger *slog.Logger, st *store.Store, uploadRoot string, anthropicKey string) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	return &Server{log: logger, store: st, uploadRoot: uploadRoot, anthropicKey: anthropicKey, rl: newRateLimiter()}
}

// limited wraps a handler with IP-based rate limiting (60 req/min).
func (s *Server) limited(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
		if idx := strings.LastIndex(ip, ":"); idx != -1 {
			ip = ip[:idx]
		}
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			ip = strings.TrimSpace(strings.Split(xff, ",")[0])
		}
		if !s.rl.allow(ip, 60, time.Minute) {
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		h(w, r)
	}
}

// Register attaches routes to mux (Go 1.22+ patterns).
func (s *Server) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /health", s.handleHealth)

	// SOP
	mux.HandleFunc("GET /api/sops", s.handleListSOPs)
	mux.HandleFunc("GET /api/sops/{slug}", s.handleGetSOP)
	mux.HandleFunc("POST /api/sops", s.limited(s.handleCreateSOP))
	mux.HandleFunc("PATCH /api/sops/{slug}", s.limited(s.handleUpdateSOP))
	mux.HandleFunc("DELETE /api/sops/{slug}", s.limited(s.handleDeleteSOP))

	// Cases
	mux.HandleFunc("GET /api/cases", s.handleListCases)
	mux.HandleFunc("POST /api/cases", s.limited(s.handleCreateCase))
	mux.HandleFunc("GET /api/cases/{id}", s.handleGetCase)
	mux.HandleFunc("GET /api/cases/{id}/steps", s.handleListSteps)
	mux.HandleFunc("PATCH /api/cases/{id}/sop", s.limited(s.handlePatchSOP))
	mux.HandleFunc("PATCH /api/cases/{caseId}/steps/{stepId}", s.limited(s.handlePatchStep))
	mux.HandleFunc("POST /api/cases/{id}/close", s.limited(s.handleCloseCase))
	mux.HandleFunc("GET /api/cases/{id}/summary", s.handleSummary)
	mux.HandleFunc("PATCH /api/cases/{id}/rca", s.limited(s.handlePatchRCA))
	mux.HandleFunc("GET /api/cases/{id}/report", s.handleFinalReport)
	mux.HandleFunc("GET /api/cases/{id}/attachments", s.handleListAttachments)
	mux.HandleFunc("GET /api/cases/{id}/audit", s.handleListAudit)
	mux.HandleFunc("GET /api/cases/{caseId}/attachments/{attId}/raw", s.handleAttachmentRaw)
	mux.HandleFunc("DELETE /api/cases/{caseId}/attachments/{attId}", s.limited(s.handleDeleteAttachment))
	mux.HandleFunc("POST /api/cases/{id}/steps/{stepId}/attachment", s.limited(s.handleUploadStepAttachment))
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"ok":true}`))
}

// ── SOP handlers ────────────────────────────────────────────────────────────

func (s *Server) handleListSOPs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	list, err := s.store.ListSOPs(ctx)
	if err != nil {
		s.log.Error("list sops", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	type row struct {
		ID      int64  `json:"id"`
		Slug    string `json:"slug"`
		Title   string `json:"title"`
		Version int    `json:"version"`
		Owner   string `json:"owner"`
	}
	out := make([]row, 0, len(list))
	for _, x := range list {
		out = append(out, row{ID: x.ID, Slug: x.Slug, Title: x.Title, Version: x.Version, Owner: x.Owner})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleGetSOP(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	ctx := r.Context()
	st, err := s.store.GetSOPBySlug(ctx, slug)
	if err != nil || st == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	defs, _ := store.ParseSOPSteps(st.StepsJSON)
	writeJSON(w, http.StatusOK, map[string]any{
		"id":         st.ID,
		"slug":       st.Slug,
		"title":      st.Title,
		"version":    st.Version,
		"owner":      st.Owner,
		"steps_json": st.StepsJSON,
		"steps":      defs,
	})
}

type sopBody struct {
	Slug  string             `json:"slug"`
	Title string             `json:"title"`
	Owner string             `json:"owner"`
	Steps []store.SOPStepDef `json:"steps"`
}

func (s *Server) handleCreateSOP(w http.ResponseWriter, r *http.Request) {
	var body sopBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	body.Slug = strings.TrimSpace(body.Slug)
	body.Title = strings.TrimSpace(body.Title)
	if body.Slug == "" || body.Title == "" {
		http.Error(w, "slug dan title wajib diisi", http.StatusBadRequest)
		return
	}
	if len(body.Steps) == 0 {
		http.Error(w, "minimal satu langkah diperlukan", http.StatusBadRequest)
		return
	}
	stepsJSON, err := json.Marshal(body.Steps)
	if err != nil {
		http.Error(w, "invalid steps", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	st, err := s.store.CreateSOP(ctx, body.Slug, body.Title, body.Owner, string(stepsJSON))
	if err != nil {
		s.log.Error("create sop", "err", err)
		if strings.Contains(err.Error(), "UNIQUE") {
			http.Error(w, "slug sudah digunakan", http.StatusConflict)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"slug": st.Slug, "title": st.Title, "version": st.Version})
}

func (s *Server) handleUpdateSOP(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	var body sopBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	body.Title = strings.TrimSpace(body.Title)
	if body.Title == "" {
		http.Error(w, "title wajib diisi", http.StatusBadRequest)
		return
	}
	if len(body.Steps) == 0 {
		http.Error(w, "minimal satu langkah diperlukan", http.StatusBadRequest)
		return
	}
	stepsJSON, err := json.Marshal(body.Steps)
	if err != nil {
		http.Error(w, "invalid steps", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	st, err := s.store.UpdateSOP(ctx, slug, body.Title, body.Owner, string(stepsJSON))
	if err != nil || st == nil {
		s.log.Error("update sop", "err", err)
		http.Error(w, "not found or internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"slug": st.Slug, "title": st.Title, "version": st.Version})
}

func (s *Server) handleDeleteSOP(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	ctx := r.Context()
	if err := s.store.DeleteSOP(ctx, slug); err != nil {
		s.log.Error("delete sop", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── Case handlers ───────────────────────────────────────────────────────────

func (s *Server) handleListCases(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := store.CaseFilter{
		Status:   q.Get("status"),
		Severity: q.Get("severity"),
		Service:  q.Get("service"),
		Search:   q.Get("search"),
	}
	if p, err := strconv.Atoi(q.Get("page")); err == nil && p > 0 {
		f.Page = p
	}
	if l, err := strconv.Atoi(q.Get("limit")); err == nil && l > 0 && l <= 200 {
		f.Limit = l
	}
	ctx := r.Context()
	list, err := s.store.ListCases(ctx, f)
	if err != nil {
		s.log.Error("list cases", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, casesToJSON(list))
}

func (s *Server) handleGetCase(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ctx := r.Context()
	c, err := s.store.GetCase(ctx, id)
	if err != nil || c == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, caseToJSON(*c))
}

func (s *Server) handleListSteps(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ctx := r.Context()
	steps, err := s.store.ListSteps(ctx, id)
	if err != nil {
		s.log.Error("list steps", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	stepAtts, _ := s.store.ListStepAttachmentsForCase(ctx, id)
	writeJSON(w, http.StatusOK, stepsWithAttachmentsToJSON(steps, stepAtts, id))
}

func (s *Server) handleCreateCase(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	const maxMem = 32 << 20
	if err := r.ParseMultipartForm(maxMem); err != nil {
		http.Error(w, "bad multipart form", http.StatusBadRequest)
		return
	}
	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		http.Error(w, "title required", http.StatusBadRequest)
		return
	}
	summary := strings.TrimSpace(r.FormValue("summary"))
	svc := strings.TrimSpace(r.FormValue("service"))
	sev := strings.TrimSpace(r.FormValue("severity"))
	reporter := strings.TrimSpace(r.FormValue("reporter"))

	rules, err := s.store.ListSOPRules(ctx)
	if err != nil {
		s.log.Error("rules", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	sopID, matched := sop.Pick(rules, sop.Intake{Title: title, Summary: summary, Service: svc, Severity: sev})

	caseID, err := s.store.NextCaseID(ctx)
	if err != nil {
		s.log.Error("case id", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	var st *store.SOP
	if matched {
		st, err = s.store.GetSOPByID(ctx, sopID)
		if err != nil || st == nil {
			matched = false
		}
	}

	status := "needs_triage"
	var sid *int64
	var sv *int
	if matched && st != nil {
		status = "open"
		sid = &st.ID
		v := st.Version
		sv = &v
	}

	c := store.Case{
		CaseID:     caseID,
		Title:      title,
		Summary:    summary,
		Service:    svc,
		Severity:   sev,
		Status:     status,
		SOPID:      sid,
		SOPVersion: sv,
		Reporter:   reporter,
	}
	if err := s.store.CreateCase(ctx, c); err != nil {
		s.log.Error("create case", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if matched && st != nil {
		defs, err := store.ParseSOPSteps(st.StepsJSON)
		if err != nil {
			s.log.Error("parse steps", "err", err)
		} else if err := s.store.InsertSteps(ctx, caseID, defs); err != nil {
			s.log.Error("insert steps", "err", err)
		}
	}

	// audit
	detail := fmt.Sprintf("service=%s severity=%s reporter=%s", svc, sev, reporter)
	_ = s.store.LogAudit(ctx, caseID, reporter, "case_created", detail)

	var screenshotWarning string
	var fileHeaders []*multipart.FileHeader
	if r.MultipartForm != nil {
		fileHeaders = append(fileHeaders, r.MultipartForm.File["screenshots"]...)
		if len(r.MultipartForm.File["screenshot"]) > 0 {
			fileHeaders = append(fileHeaders, r.MultipartForm.File["screenshot"]...)
		}
	}
	const maxIntakeImages = 24
	if len(fileHeaders) > maxIntakeImages {
		fileHeaders = fileHeaders[:maxIntakeImages]
		if screenshotWarning == "" {
			screenshotWarning = fmt.Sprintf("maksimal %d gambar per case; sisanya diabaikan", maxIntakeImages)
		}
	}
	dir := filepath.Join(s.uploadRoot, caseID)
	if len(fileHeaders) > 0 {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			s.log.Error("mkdir upload", "err", err)
			screenshotWarning = "gagal menyimpan lampiran gambar (mkdir)"
		} else {
			for _, hdr := range fileHeaders {
				if !isImageFileHeader(hdr) {
					continue
				}
				f, err := hdr.Open()
				if err != nil {
					s.log.Error("open upload", "err", err)
					if screenshotWarning == "" {
						screenshotWarning = "beberapa gambar gagal dibaca"
					}
					continue
				}
				safe := filepath.Base(hdr.Filename)
				if safe == "." || safe == "/" {
					safe = "screenshot"
				}
				dest := filepath.Join(dir, fmt.Sprintf("%d_%s", time.Now().UnixNano(), safe))
				dst, err := os.Create(dest)
				if err != nil {
					_ = f.Close()
					s.log.Error("create upload", "err", err)
					if screenshotWarning == "" {
						screenshotWarning = "gagal menyimpan satu atau lebih gambar"
					}
					continue
				}
				_, copyErr := io.Copy(dst, f)
				_ = dst.Close()
				_ = f.Close()
				if copyErr != nil {
					s.log.Error("copy upload", "err", copyErr)
					if screenshotWarning == "" {
						screenshotWarning = "gagal menyimpan satu atau lebih gambar"
					}
					continue
				}
				rel, _ := filepath.Rel(s.uploadRoot, dest)
				if err := s.store.AddAttachment(ctx, store.CaseAttachment{
					CaseID:       caseID,
					Kind:         "screenshot",
					FilePath:     rel,
					OriginalName: hdr.Filename,
				}); err != nil && screenshotWarning == "" {
					screenshotWarning = "gagal mencatat lampiran di database"
				}
			}
		}
	}

	out, err := s.store.GetCase(ctx, caseID)
	if err != nil || out == nil {
		writeJSON(w, http.StatusCreated, map[string]any{"case_id": caseID})
		return
	}
	resp := caseToJSON(*out)
	if screenshotWarning != "" {
		resp["screenshot_warning"] = screenshotWarning
	}
	writeJSON(w, http.StatusCreated, resp)
}

type patchSOPBody struct {
	Slug string `json:"slug"`
}

func (s *Server) handlePatchSOP(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body patchSOPBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Slug) == "" {
		http.Error(w, "json {slug} required", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	st, err := s.store.GetSOPBySlug(ctx, strings.TrimSpace(body.Slug))
	if err != nil || st == nil {
		http.Error(w, "sop not found", http.StatusNotFound)
		return
	}
	defs, err := store.ParseSOPSteps(st.StepsJSON)
	if err != nil {
		http.Error(w, "invalid sop steps", http.StatusInternalServerError)
		return
	}
	if err := s.store.DeleteStepsForCase(ctx, id); err != nil {
		s.log.Error("delete steps", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if err := s.store.InsertSteps(ctx, id, defs); err != nil {
		s.log.Error("insert steps", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	v := st.Version
	if err := s.store.UpdateCaseSOP(ctx, id, st.ID, v, "open"); err != nil {
		s.log.Error("update case sop", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	_ = s.store.LogAudit(ctx, id, "", "sop_assigned", fmt.Sprintf("sop=%s v%d", st.Slug, v))
	c, err := s.store.GetCase(ctx, id)
	if err != nil || c == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, caseToJSON(*c))
}

type patchStepBody struct {
	Done        *bool   `json:"done"`
	DoneBy      *string `json:"done_by"`
	Notes       *string `json:"notes"`
	EvidenceURL *string `json:"evidence_url"`
}

func (s *Server) handlePatchStep(w http.ResponseWriter, r *http.Request) {
	caseID := r.PathValue("caseId")
	sidStr := r.PathValue("stepId")
	stepID, err := strconv.ParseInt(sidStr, 10, 64)
	if err != nil {
		http.Error(w, "bad step id", http.StatusBadRequest)
		return
	}
	var body patchStepBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	if err := s.store.UpdateStep(ctx, caseID, stepID, body.Done, body.DoneBy, body.Notes, body.EvidenceURL); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		s.log.Error("update step", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	// audit
	if body.Done != nil {
		action := "step_undone"
		if *body.Done {
			action = "step_done"
		}
		by := ""
		if body.DoneBy != nil {
			by = *body.DoneBy
		}
		_ = s.store.LogAudit(ctx, caseID, by, action, fmt.Sprintf("step_id=%d", stepID))
	}
	steps, err := s.store.ListSteps(ctx, caseID)
	if err != nil {
		s.log.Error("list steps after update", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	stepAtts, _ := s.store.ListStepAttachmentsForCase(ctx, caseID)
	writeJSON(w, http.StatusOK, stepsWithAttachmentsToJSON(steps, stepAtts, caseID))
}

func (s *Server) handleCloseCase(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ctx := r.Context()
	c, err := s.store.GetCase(ctx, id)
	if err != nil || c == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	steps, err := s.store.ListSteps(ctx, id)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if errs := validate.CloseCase(c, steps); len(errs) > 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"errors": errs})
		return
	}
	now := time.Now().UTC()
	if err := s.store.UpdateCaseStatus(ctx, id, "resolved", &now); err != nil {
		s.log.Error("close", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	_ = s.store.LogAudit(ctx, id, "", "case_closed", "")
	c2, _ := s.store.GetCase(ctx, id)
	writeJSON(w, http.StatusOK, caseToJSON(*c2))
}

func (s *Server) handleListAudit(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ctx := r.Context()
	events, err := s.store.ListAudit(ctx, id)
	if err != nil {
		s.log.Error("list audit", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	out := make([]map[string]any, 0, len(events))
	for _, e := range events {
		out = append(out, map[string]any{
			"id":         e.ID,
			"actor":      e.Actor,
			"action":     e.Action,
			"detail":     e.Detail,
			"created_at": tz.FormatRFC3339(e.CreatedAt),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleSummary(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ctx := r.Context()
	c, err := s.store.GetCase(ctx, id)
	if err != nil || c == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	steps, err := s.store.ListSteps(ctx, id)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	format := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format")))
	switch format {
	case "html":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(render.HTML(c, steps)))
	case "pdf":
		attachments, err := s.store.ListAttachments(ctx, id)
		if err != nil {
			s.log.Error("list attachments for pdf", "err", err)
			attachments = nil
		}
		stepAtts, _ := s.store.ListStepAttachmentsForCase(ctx, id)
		opts := render.DefaultPDFOptions()
		// format knobs:
		// - include_checklist=0|1
		// - checklist_progress=0|1
		// - compression=0|1
		q := r.URL.Query()
		if q.Get("include_checklist") == "0" {
			opts.IncludeChecklist = false
		}
		if q.Get("checklist_progress") == "1" {
			opts.IncludeChecklistProgress = true
		}
		if q.Get("compression") == "1" {
			opts.Compression = true
		}

		pdfBytes, err := render.PDFWithOptions(c, steps, attachments, stepAtts, s.uploadRoot, opts)
		if err != nil {
			s.log.Error("render pdf", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", `attachment; filename="`+id+`.pdf"`)
		_, _ = w.Write(pdfBytes)
	default:
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		_, _ = w.Write([]byte(render.Markdown(c, steps)))
	}
}

const maxRCAFieldRunes = 8000

func (s *Server) handlePatchRCA(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body store.CaseRCA
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	body = body.Normalize()
	check := func(s string, name string) bool {
		if utf8.RuneCountInString(s) > maxRCAFieldRunes {
			http.Error(w, name+" terlalu panjang (maks "+strconv.Itoa(maxRCAFieldRunes)+" karakter)", http.StatusBadRequest)
			return false
		}
		return true
	}
	if !check(body.IncidentTimeline, "kronologi") || !check(body.RootCause, "akar masalah") ||
		!check(body.ContributingFactors, "faktor kontributor") || !check(body.CorrectiveActions, "tindakan korektif") ||
		!check(body.PreventiveActions, "tindakan pencegahan") {
		return
	}
	for i, w := range body.FiveWhys {
		if !check(w, "5 whys #"+strconv.Itoa(i+1)) {
			return
		}
	}
	j, err := store.MarshalCaseRCAJSON(body)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	ctx := r.Context()
	c0, err := s.store.GetCase(ctx, id)
	if err != nil || c0 == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err := s.store.UpdateCaseRCA(ctx, id, j); err != nil {
		s.log.Error("update rca", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	_ = s.store.LogAudit(ctx, id, "", "rca_updated", "")
	c, err := s.store.GetCase(ctx, id)
	if err != nil || c == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, caseToJSON(*c))
}

func (s *Server) handleFinalReport(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ctx := r.Context()
	c, err := s.store.GetCase(ctx, id)
	if err != nil || c == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	steps, err := s.store.ListSteps(ctx, id)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	report, err := render.FinalReport(ctx, c, steps, s.anthropicKey)
	if err != nil {
		s.log.Error("final report", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	_, _ = w.Write([]byte(report))
}

func (s *Server) handleListAttachments(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ctx := r.Context()
	list, err := s.store.ListAttachments(ctx, id)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	out := make([]map[string]any, 0, len(list))
	for _, a := range list {
		out = append(out, map[string]any{
			"id":            a.ID,
			"kind":          a.Kind,
			"original_name": a.OriginalName,
			"url":           fmt.Sprintf("/api/cases/%s/attachments/%d/raw", id, a.ID),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleAttachmentRaw(w http.ResponseWriter, r *http.Request) {
	caseID := r.PathValue("caseId")
	attStr := r.PathValue("attId")
	attID, err := strconv.ParseInt(attStr, 10, 64)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	found, err := s.store.GetAttachmentByID(ctx, caseID, attID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if found == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	base := filepath.Clean(s.uploadRoot)
	full := filepath.Clean(filepath.Join(s.uploadRoot, filepath.FromSlash(found.FilePath)))
	rel, err := filepath.Rel(base, full)
	if err != nil || strings.HasPrefix(rel, "..") {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	http.ServeFile(w, r, full)
}

func (s *Server) handleDeleteAttachment(w http.ResponseWriter, r *http.Request) {
	caseID := r.PathValue("caseId")
	attStr := r.PathValue("attId")
	attID, err := strconv.ParseInt(attStr, 10, 64)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	found, err := s.store.GetAttachmentByID(ctx, caseID, attID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if found == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	base := filepath.Clean(s.uploadRoot)
	full := filepath.Clean(filepath.Join(s.uploadRoot, filepath.FromSlash(found.FilePath)))
	rel, err := filepath.Rel(base, full)
	if err != nil || strings.HasPrefix(rel, "..") {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	_ = os.Remove(full)
	if err := s.store.DeleteAttachment(ctx, caseID, attID); err != nil {
		s.log.Error("delete attachment", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	_ = s.store.LogAudit(ctx, caseID, "", "attachment_deleted", fmt.Sprintf("att_id=%d", attID))
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleUploadStepAttachment(w http.ResponseWriter, r *http.Request) {
	caseID := r.PathValue("id")
	stepStr := r.PathValue("stepId")
	stepID, err := strconv.ParseInt(stepStr, 10, 64)
	if err != nil {
		http.Error(w, "bad step id", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	// verify step belongs to case
	st, err := s.store.GetStep(ctx, stepID, caseID)
	if err != nil || st == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	const maxMem = 32 << 20
	if err := r.ParseMultipartForm(maxMem); err != nil {
		http.Error(w, "bad multipart form", http.StatusBadRequest)
		return
	}
	file, hdr, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "file required", http.StatusBadRequest)
		return
	}
	defer file.Close()
	dir := filepath.Join(s.uploadRoot, caseID, "steps", stepStr)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	safe := filepath.Base(hdr.Filename)
	if safe == "." || safe == "/" {
		safe = "upload"
	}
	dest := filepath.Join(dir, fmt.Sprintf("%d_%s", time.Now().UnixNano(), safe))
	dst, err := os.Create(dest)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	_, copyErr := io.Copy(dst, file)
	_ = dst.Close()
	if copyErr != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	rel, _ := filepath.Rel(s.uploadRoot, dest)
	att := store.CaseAttachment{
		CaseID:       caseID,
		StepID:       &stepID,
		Kind:         "step_evidence",
		FilePath:     rel,
		OriginalName: hdr.Filename,
	}
	if err := s.store.AddAttachment(ctx, att); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	// return updated step list so frontend can refresh
	steps, _ := s.store.ListSteps(ctx, caseID)
	stepAtts, _ := s.store.ListStepAttachmentsForCase(ctx, caseID)
	writeJSON(w, http.StatusOK, stepsWithAttachmentsToJSON(steps, stepAtts, caseID))
}

// ── Helpers ─────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(true)
	_ = enc.Encode(v)
}

func casesToJSON(list []store.Case) []any {
	out := make([]any, 0, len(list))
	for _, c := range list {
		out = append(out, caseToJSON(c))
	}
	return out
}

func caseToJSON(c store.Case) map[string]any {
	m := map[string]any{
		"case_id":    c.CaseID,
		"title":      c.Title,
		"summary":    c.Summary,
		"service":    c.Service,
		"severity":   c.Severity,
		"status":     c.Status,
		"reporter":   c.Reporter,
		"created_at": tz.FormatRFC3339(c.CreatedAt),
		"updated_at": tz.FormatRFC3339(c.UpdatedAt),
		"sop_slug":   c.SOPSlug,
		"sop_title":  c.SOPTitle,
	}
	if c.SOPID != nil {
		m["sop_id"] = *c.SOPID
	}
	if c.SOPVersion != nil {
		m["sop_version"] = *c.SOPVersion
	}
	if c.ResolvedAt != nil {
		m["resolved_at"] = tz.FormatRFC3339(*c.ResolvedAt)
	}
	rca := store.ParseCaseRCAJSON(c.RCAJSON)
	rca = rca.Normalize()
	m["rca"] = map[string]any{
		"incident_timeline":     rca.IncidentTimeline,
		"five_whys":             rca.FiveWhys,
		"root_cause":            rca.RootCause,
		"contributing_factors":  rca.ContributingFactors,
		"corrective_actions":    rca.CorrectiveActions,
		"preventive_actions":    rca.PreventiveActions,
	}
	return m
}


func stepsWithAttachmentsToJSON(steps []store.CaseStep, stepAtts map[int64][]store.CaseAttachment, caseID string) []any {
	out := make([]any, 0, len(steps))
	for _, st := range steps {
		row := map[string]any{
			"id":                st.ID,
			"step_no":           st.StepNo,
			"title":             st.Title,
			"requires_evidence": st.RequiresEvidence,
			"optional":          st.Optional,
			"done_by":           st.DoneBy,
			"notes":             st.Notes,
			"evidence_url":      st.EvidenceURL,
		}
		if st.DoneAt != nil {
			row["done_at"] = tz.FormatRFC3339(*st.DoneAt)
		} else {
			row["done_at"] = nil
		}
		// per-step attachments
		atts := []map[string]any{}
		if stepAtts != nil {
			for _, a := range stepAtts[st.ID] {
				atts = append(atts, map[string]any{
					"id":            a.ID,
					"original_name": a.OriginalName,
					"url":           fmt.Sprintf("/api/cases/%s/attachments/%d/raw", caseID, a.ID),
				})
			}
		}
		row["attachments"] = atts
		out = append(out, row)
	}
	return out
}

func isImageFileHeader(fh *multipart.FileHeader) bool {
	if fh == nil {
		return false
	}
	ct := strings.ToLower(fh.Header.Get("Content-Type"))
	if strings.HasPrefix(ct, "image/") {
		return true
	}
	ext := strings.ToLower(filepath.Ext(fh.Filename))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".heic", ".bmp", ".svg":
		return true
	default:
		return false
	}
}
