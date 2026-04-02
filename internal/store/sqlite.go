package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Store is the SQLite persistence layer.
type Store struct {
	db *sql.DB
}

// Open opens the database, applies migrations, and returns a Store.
func Open(ctx context.Context, dbPath string) (*Store, error) {
	dsn := "file:" + dbPath + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := runMigrations(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

// Close closes the database.
func (s *Store) Close() error {
	return s.db.Close()
}

// DB exposes the underlying pool for tests.
func (s *Store) DB() *sql.DB {
	return s.db
}

// NextCaseID returns the next OPS-YYYYMMDD-### id.
func (s *Store) NextCaseID(ctx context.Context) (string, error) {
	prefix := "OPS-" + time.Now().Format("20060102") + "-"
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM cases WHERE case_id LIKE ?`,
		prefix+"%").Scan(&n)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s%03d", prefix, n+1), nil
}

// ListSOPs returns all SOP definitions.
func (s *Store) ListSOPs(ctx context.Context) ([]SOP, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, slug, title, version, owner, steps_json, created_at FROM sop ORDER BY slug`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SOP
	for rows.Next() {
		var o SOP
		var created string
		if err := rows.Scan(&o.ID, &o.Slug, &o.Title, &o.Version, &o.Owner, &o.StepsJSON, &created); err != nil {
			return nil, err
		}
		o.CreatedAt, _ = time.Parse(time.RFC3339, created)
		if o.CreatedAt.IsZero() {
			o.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", created)
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// GetSOPBySlug loads an SOP by slug.
func (s *Store) GetSOPBySlug(ctx context.Context, slug string) (*SOP, error) {
	var o SOP
	var created string
	err := s.db.QueryRowContext(ctx,
		`SELECT id, slug, title, version, owner, steps_json, created_at FROM sop WHERE slug = ?`, slug).
		Scan(&o.ID, &o.Slug, &o.Title, &o.Version, &o.Owner, &o.StepsJSON, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	o.CreatedAt, _ = time.Parse(time.RFC3339, created)
	if o.CreatedAt.IsZero() {
		o.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", created)
	}
	return &o, nil
}

// GetSOPByID loads an SOP by id.
func (s *Store) GetSOPByID(ctx context.Context, id int64) (*SOP, error) {
	var o SOP
	var created string
	err := s.db.QueryRowContext(ctx,
		`SELECT id, slug, title, version, owner, steps_json, created_at FROM sop WHERE id = ?`, id).
		Scan(&o.ID, &o.Slug, &o.Title, &o.Version, &o.Owner, &o.StepsJSON, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	o.CreatedAt, _ = time.Parse(time.RFC3339, created)
	if o.CreatedAt.IsZero() {
		o.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", created)
	}
	return &o, nil
}

// ListSOPRules returns rules ordered by priority descending.
func (s *Store) ListSOPRules(ctx context.Context) ([]SOPRule, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, sop_id, priority, COALESCE(service,''), COALESCE(keyword,''), COALESCE(severity_min,'') FROM sop_rule ORDER BY priority DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SOPRule
	for rows.Next() {
		var r SOPRule
		var svc, kw, sev string
		if err := rows.Scan(&r.ID, &r.SOPID, &r.Priority, &svc, &kw, &sev); err != nil {
			return nil, err
		}
		r.Service = svc
		r.Keyword = kw
		r.SeverityMin = sev
		out = append(out, r)
	}
	return out, rows.Err()
}

// CreateCase inserts a case. sopID/sopVersion nil means needs_triage unless you set status separately.
func (s *Store) CreateCase(ctx context.Context, c Case) error {
	now := time.Now().UTC().Format(time.RFC3339)
	var sid any
	var sv any
	if c.SOPID != nil {
		sid = *c.SOPID
	} else {
		sid = nil
	}
	if c.SOPVersion != nil {
		sv = *c.SOPVersion
	} else {
		sv = nil
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO cases (case_id, title, summary, service, severity, status, sop_id, sop_version, reporter, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.CaseID, c.Title, c.Summary, c.Service, c.Severity, c.Status, sid, sv, c.Reporter, now, now)
	return err
}

// UpdateCaseSOP sets SOP and version and status.
func (s *Store) UpdateCaseSOP(ctx context.Context, caseID string, sopID int64, sopVersion int, status string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx,
		`UPDATE cases SET sop_id = ?, sop_version = ?, status = ?, updated_at = ? WHERE case_id = ?`,
		sopID, sopVersion, status, now, caseID)
	return err
}

// UpdateCaseStatus updates status and optional resolved_at.
func (s *Store) UpdateCaseStatus(ctx context.Context, caseID, status string, resolved *time.Time) error {
	now := time.Now().UTC().Format(time.RFC3339)
	var r any
	if resolved != nil {
		r = resolved.UTC().Format(time.RFC3339)
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE cases SET status = ?, updated_at = ?, resolved_at = COALESCE(?, resolved_at) WHERE case_id = ?`,
		status, now, r, caseID)
	return err
}

// AddAttachment records an uploaded file.
func (s *Store) AddAttachment(ctx context.Context, a CaseAttachment) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO case_attachment (case_id, kind, file_path, original_name, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		a.CaseID, a.Kind, a.FilePath, a.OriginalName, now)
	return err
}

// DeleteStepsForCase removes all checklist rows for a case.
func (s *Store) DeleteStepsForCase(ctx context.Context, caseID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM case_step WHERE case_id = ?`, caseID)
	return err
}

// InsertSteps inserts checklist rows from definitions.
func (s *Store) InsertSteps(ctx context.Context, caseID string, defs []SOPStepDef) error {
	for i, d := range defs {
		req := 0
		if d.RequiresEvidence {
			req = 1
		}
		_, err := s.db.ExecContext(ctx, `
			INSERT INTO case_step (case_id, step_no, title, requires_evidence) VALUES (?, ?, ?, ?)`,
			caseID, i+1, d.Title, req)
		if err != nil {
			return err
		}
	}
	return nil
}

// ParseSOPSteps parses steps_json into definitions.
func ParseSOPSteps(stepsJSON string) ([]SOPStepDef, error) {
	var defs []SOPStepDef
	if err := json.Unmarshal([]byte(stepsJSON), &defs); err != nil {
		return nil, err
	}
	return defs, nil
}

// ListCases returns recent cases.
func (s *Store) ListCases(ctx context.Context, limit int) ([]Case, error) {
	if limit <= 0 {
		limit = 100
	}
	q := `
		SELECT c.case_id, c.title, c.summary, c.service, c.severity, c.status, c.sop_id, c.sop_version, c.reporter,
			c.created_at, c.updated_at, c.resolved_at, COALESCE(s.slug,''), COALESCE(s.title,'')
		FROM cases c
		LEFT JOIN sop s ON s.id = c.sop_id
		ORDER BY c.created_at DESC
		LIMIT ?`
	rows, err := s.db.QueryContext(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCases(rows)
}

// GetCase returns one case with joined sop labels.
func (s *Store) GetCase(ctx context.Context, caseID string) (*Case, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT c.case_id, c.title, c.summary, c.service, c.severity, c.status, c.sop_id, c.sop_version, c.reporter,
			c.created_at, c.updated_at, c.resolved_at, COALESCE(s.slug,''), COALESCE(s.title,'')
		FROM cases c
		LEFT JOIN sop s ON s.id = c.sop_id
		WHERE c.case_id = ?`, caseID)
	var c Case
	var sopID sql.NullInt64
	var sopVer sql.NullInt64
	var summary, service, severity, reporter sql.NullString
	var resolved sql.NullString
	var created, updated string
	if err := row.Scan(&c.CaseID, &c.Title, &summary, &service, &severity, &c.Status, &sopID, &sopVer, &reporter,
		&created, &updated, &resolved, &c.SOPSlug, &c.SOPTitle); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if sopID.Valid {
		v := sopID.Int64
		c.SOPID = &v
	}
	if sopVer.Valid {
		v := int(sopVer.Int64)
		c.SOPVersion = &v
	}
	c.CreatedAt = parseTime(created)
	c.UpdatedAt = parseTime(updated)
	if resolved.Valid && resolved.String != "" {
		t := parseTime(resolved.String)
		c.ResolvedAt = &t
	}
	c.Summary = nullStr(summary)
	c.Service = nullStr(service)
	c.Severity = nullStr(severity)
	c.Reporter = nullStr(reporter)
	return &c, nil
}

func scanCases(rows *sql.Rows) ([]Case, error) {
	var out []Case
	for rows.Next() {
		var c Case
		var sopID sql.NullInt64
		var sopVer sql.NullInt64
		var summary, service, severity, reporter sql.NullString
		var resolved sql.NullString
		var created, updated string
		if err := rows.Scan(&c.CaseID, &c.Title, &summary, &service, &severity, &c.Status, &sopID, &sopVer, &reporter,
			&created, &updated, &resolved, &c.SOPSlug, &c.SOPTitle); err != nil {
			return nil, err
		}
		if sopID.Valid {
			v := sopID.Int64
			c.SOPID = &v
		}
		if sopVer.Valid {
			v := int(sopVer.Int64)
			c.SOPVersion = &v
		}
		c.CreatedAt = parseTime(created)
		c.UpdatedAt = parseTime(updated)
		if resolved.Valid && resolved.String != "" {
			t := parseTime(resolved.String)
			c.ResolvedAt = &t
		}
		c.Summary = nullStr(summary)
		c.Service = nullStr(service)
		c.Severity = nullStr(severity)
		c.Reporter = nullStr(reporter)
		out = append(out, c)
	}
	return out, rows.Err()
}

func parseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err == nil {
		return t
	}
	t, err = time.Parse("2006-01-02 15:04:05", s)
	if err == nil {
		return t
	}
	return time.Time{}
}

// ListSteps returns steps for a case ordered by step_no.
func (s *Store) ListSteps(ctx context.Context, caseID string) ([]CaseStep, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, case_id, step_no, title, requires_evidence, done_at, done_by, notes, evidence_url
		FROM case_step WHERE case_id = ? ORDER BY step_no`, caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CaseStep
	for rows.Next() {
		var st CaseStep
		var req int
		var doneAt, doneBy, notes, evidence sql.NullString
		if err := rows.Scan(&st.ID, &st.CaseID, &st.StepNo, &st.Title, &req, &doneAt, &doneBy, &notes, &evidence); err != nil {
			return nil, err
		}
		st.RequiresEvidence = req != 0
		st.DoneBy = nullStr(doneBy)
		st.Notes = nullStr(notes)
		st.EvidenceURL = nullStr(evidence)
		if doneAt.Valid && doneAt.String != "" {
			t := parseTime(doneAt.String)
			st.DoneAt = &t
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

func nullStr(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

// GetStep returns a single step by id (scoped by caseID for safety).
func (s *Store) GetStep(ctx context.Context, stepID int64, caseID string) (*CaseStep, error) {
	var st CaseStep
	var req int
	var doneAt, doneBy, notes, evidence sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT id, case_id, step_no, title, requires_evidence, done_at, done_by, notes, evidence_url
		FROM case_step WHERE id = ? AND case_id = ?`, stepID, caseID).
		Scan(&st.ID, &st.CaseID, &st.StepNo, &st.Title, &req, &doneAt, &doneBy, &notes, &evidence)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	st.RequiresEvidence = req != 0
	st.DoneBy = nullStr(doneBy)
	st.Notes = nullStr(notes)
	st.EvidenceURL = nullStr(evidence)
	if doneAt.Valid && doneAt.String != "" {
		t := parseTime(doneAt.String)
		st.DoneAt = &t
	}
	return &st, nil
}

// UpdateStep updates optional fields on a step (must belong to caseID).
func (s *Store) UpdateStep(ctx context.Context, caseID string, stepID int64, done *bool, doneBy, notes, evidenceURL *string) error {
	var st CaseStep
	var req int
	var doneAt, doneByNS, notesNS, evidenceNS sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT id, case_id, step_no, title, requires_evidence, done_at, done_by, notes, evidence_url FROM case_step WHERE id = ? AND case_id = ?`,
		stepID, caseID).Scan(&st.ID, &st.CaseID, &st.StepNo, &st.Title, &req, &doneAt, &doneByNS, &notesNS, &evidenceNS)
	if err != nil {
		return err
	}
	st.DoneBy = nullStr(doneByNS)
	st.Notes = nullStr(notesNS)
	st.EvidenceURL = nullStr(evidenceNS)
	if doneBy != nil {
		st.DoneBy = *doneBy
	}
	if notes != nil {
		st.Notes = *notes
	}
	if evidenceURL != nil {
		st.EvidenceURL = *evidenceURL
	}
	if doneBy != nil {
		st.DoneBy = *doneBy
	}
	if notes != nil {
		st.Notes = *notes
	}
	if evidenceURL != nil {
		st.EvidenceURL = *evidenceURL
	}
	nowStr := time.Now().UTC().Format(time.RFC3339)
	var doneStr any
	if done != nil {
		if *done {
			doneStr = nowStr
		} else {
			doneStr = nil
		}
	} else if doneAt.Valid && doneAt.String != "" {
		doneStr = doneAt.String
	} else {
		doneStr = nil
	}
	_, err = s.db.ExecContext(ctx, `
		UPDATE case_step SET done_at = ?, done_by = ?, notes = ?, evidence_url = ? WHERE id = ? AND case_id = ?`,
		doneStr, nullIfEmpty(st.DoneBy), st.Notes, st.EvidenceURL, stepID, caseID)
	return err
}

func nullIfEmpty(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

// ListAttachments for a case.
func (s *Store) ListAttachments(ctx context.Context, caseID string) ([]CaseAttachment, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, case_id, kind, file_path, original_name, created_at FROM case_attachment WHERE case_id = ? ORDER BY id`, caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CaseAttachment
	for rows.Next() {
		var a CaseAttachment
		var created string
		if err := rows.Scan(&a.ID, &a.CaseID, &a.Kind, &a.FilePath, &a.OriginalName, &created); err != nil {
			return nil, err
		}
		a.CreatedAt = parseTime(created)
		out = append(out, a)
	}
	return out, rows.Err()
}
