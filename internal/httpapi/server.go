package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/rzfd/metatech/konkon/internal/render"
	"github.com/rzfd/metatech/konkon/internal/sop"
	"github.com/rzfd/metatech/konkon/internal/store"
	"github.com/rzfd/metatech/konkon/internal/validate"
)

// Server wires HTTP routes to the store.
type Server struct {
	log        *slog.Logger
	store      *store.Store
	uploadRoot string
}

// New creates an API server.
func New(logger *slog.Logger, st *store.Store, uploadRoot string) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	return &Server{log: logger, store: st, uploadRoot: uploadRoot}
}

// Register attaches routes to mux (Go 1.22+ patterns).
func (s *Server) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /api/sops", s.handleListSOPs)
	mux.HandleFunc("GET /api/cases", s.handleListCases)
	mux.HandleFunc("POST /api/cases", s.handleCreateCase)
	mux.HandleFunc("GET /api/cases/{id}", s.handleGetCase)
	mux.HandleFunc("GET /api/cases/{id}/steps", s.handleListSteps)
	mux.HandleFunc("PATCH /api/cases/{id}/sop", s.handlePatchSOP)
	mux.HandleFunc("PATCH /api/cases/{caseId}/steps/{stepId}", s.handlePatchStep)
	mux.HandleFunc("POST /api/cases/{id}/close", s.handleCloseCase)
	mux.HandleFunc("GET /api/cases/{id}/summary", s.handleSummary)
	mux.HandleFunc("GET /api/cases/{id}/attachments", s.handleListAttachments)
	mux.HandleFunc("GET /api/cases/{caseId}/attachments/{attId}/raw", s.handleAttachmentRaw)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"ok":true}`))
}

func (s *Server) handleListSOPs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
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

func (s *Server) handleListCases(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx := r.Context()
	list, err := s.store.ListCases(ctx, 200)
	if err != nil {
		s.log.Error("list cases", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, casesToJSON(list))
}

func (s *Server) handleGetCase(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
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
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := r.PathValue("id")
	ctx := r.Context()
	steps, err := s.store.ListSteps(ctx, id)
	if err != nil {
		s.log.Error("list steps", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, stepsToJSON(steps))
}

func (s *Server) handleCreateCase(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
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

	file, hdr, err := r.FormFile("screenshot")
	if err == nil {
		defer file.Close()
		dir := filepath.Join(s.uploadRoot, caseID)
		if err := os.MkdirAll(dir, 0o750); err != nil {
			s.log.Error("mkdir upload", "err", err)
		} else {
			safe := filepath.Base(hdr.Filename)
			if safe == "." || safe == "/" {
				safe = "screenshot"
			}
			dest := filepath.Join(dir, fmt.Sprintf("%d_%s", time.Now().UnixNano(), safe))
			dst, err := os.Create(dest)
			if err != nil {
				s.log.Error("create upload", "err", err)
			} else {
				_, copyErr := io.Copy(dst, file)
				_ = dst.Close()
				if copyErr != nil {
					s.log.Error("copy upload", "err", copyErr)
				} else {
					rel, _ := filepath.Rel(s.uploadRoot, dest)
					_ = s.store.AddAttachment(ctx, store.CaseAttachment{
						CaseID:       caseID,
						Kind:         "screenshot",
						FilePath:     rel,
						OriginalName: hdr.Filename,
					})
				}
			}
		}
	}

	out, err := s.store.GetCase(ctx, caseID)
	if err != nil || out == nil {
		writeJSON(w, http.StatusCreated, map[string]any{"case_id": caseID})
		return
	}
	writeJSON(w, http.StatusCreated, caseToJSON(*out))
}

type patchSOPBody struct {
	Slug string `json:"slug"`
}

func (s *Server) handlePatchSOP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
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
	c, err := s.store.GetCase(ctx, id)
	if err != nil || c == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, caseToJSON(*c))
}

type patchStepBody struct {
	Done         *bool   `json:"done"`
	DoneBy       *string `json:"done_by"`
	Notes        *string `json:"notes"`
	EvidenceURL  *string `json:"evidence_url"`
}

func (s *Server) handlePatchStep(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
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
	steps, _ := s.store.ListSteps(ctx, caseID)
	writeJSON(w, http.StatusOK, stepsToJSON(steps))
}

func (s *Server) handleCloseCase(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
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
	c2, _ := s.store.GetCase(ctx, id)
	writeJSON(w, http.StatusOK, caseToJSON(*c2))
}

func (s *Server) handleSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
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
	if format == "html" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(render.HTML(c, steps)))
		return
	}
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	_, _ = w.Write([]byte(render.Markdown(c, steps)))
}

func (s *Server) handleListAttachments(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
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
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	caseID := r.PathValue("caseId")
	attStr := r.PathValue("attId")
	attID, err := strconv.ParseInt(attStr, 10, 64)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	list, err := s.store.ListAttachments(ctx, caseID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	var found *store.CaseAttachment
	for i := range list {
		if list[i].ID == attID {
			found = &list[i]
			break
		}
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
		"created_at": c.CreatedAt.UTC().Format(time.RFC3339),
		"updated_at": c.UpdatedAt.UTC().Format(time.RFC3339),
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
		m["resolved_at"] = c.ResolvedAt.UTC().Format(time.RFC3339)
	}
	return m
}

func stepsToJSON(steps []store.CaseStep) []any {
	out := make([]any, 0, len(steps))
	for _, st := range steps {
		row := map[string]any{
			"id":                st.ID,
			"step_no":           st.StepNo,
			"title":             st.Title,
			"requires_evidence": st.RequiresEvidence,
			"done_by":           st.DoneBy,
			"notes":             st.Notes,
			"evidence_url":      st.EvidenceURL,
		}
		if st.DoneAt != nil {
			row["done_at"] = st.DoneAt.UTC().Format(time.RFC3339)
		} else {
			row["done_at"] = nil
		}
		out = append(out, row)
	}
	return out
}
