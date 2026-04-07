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
	rebind func(string) string
}

// Open opens the database, applies migrations, and returns a Store.
func OpenSQLite(ctx context.Context, dbPath string) (*Store, error) {
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

func (s *Store) bind(query string) string {
	if s == nil || s.rebind == nil {
		return query
	}
	return s.rebind(query)
}

func (s *Store) queryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return s.db.QueryContext(ctx, s.bind(query), args...)
}

func (s *Store) queryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return s.db.QueryRowContext(ctx, s.bind(query), args...)
}

func (s *Store) execContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return s.db.ExecContext(ctx, s.bind(query), args...)
}

// NextCaseID returns the next OPS-YYYYMMDD-### id.
func (s *Store) NextCaseID(ctx context.Context) (string, error) {
	prefix := "OPS-" + time.Now().Format("20060102") + "-"
	var n int
	err := s.queryRowContext(ctx,
		`SELECT COUNT(*) FROM cases WHERE case_id LIKE ?`,
		prefix+"%").Scan(&n)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s%03d", prefix, n+1), nil
}

// ListSOPs returns all SOP definitions.
func (s *Store) ListSOPs(ctx context.Context) ([]SOP, error) {
	rows, err := s.queryContext(ctx,
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
	err := s.queryRowContext(ctx,
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
	err := s.queryRowContext(ctx,
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
	rows, err := s.queryContext(ctx,
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
	_, err := s.execContext(ctx, `
		INSERT INTO cases (case_id, title, summary, service, severity, status, sop_id, sop_version, reporter, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.CaseID, c.Title, c.Summary, c.Service, c.Severity, c.Status, sid, sv, c.Reporter, now, now)
	return err
}

// UpdateCaseSOP sets SOP and version and status.
func (s *Store) UpdateCaseSOP(ctx context.Context, caseID string, sopID int64, sopVersion int, status string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.execContext(ctx,
		`UPDATE cases SET sop_id = ?, sop_version = ?, status = ?, updated_at = ? WHERE case_id = ?`,
		sopID, sopVersion, status, now, caseID)
	return err
}

// UpdateCaseRCA replaces rca_json for a case.
func (s *Store) UpdateCaseRCA(ctx context.Context, caseID, rcaJSON string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.execContext(ctx,
		`UPDATE cases SET rca_json = ?, updated_at = ? WHERE case_id = ?`,
		rcaJSON, now, caseID)
	return err
}

// UpdateCaseStatus updates status and optional resolved_at.
func (s *Store) UpdateCaseStatus(ctx context.Context, caseID, status string, resolved *time.Time) error {
	now := time.Now().UTC().Format(time.RFC3339)
	var r any
	if resolved != nil {
		r = resolved.UTC().Format(time.RFC3339)
	}
	_, err := s.execContext(ctx,
		`UPDATE cases SET status = ?, updated_at = ?, resolved_at = COALESCE(?, resolved_at) WHERE case_id = ?`,
		status, now, r, caseID)
	return err
}

// AddAttachment records an uploaded file.
func (s *Store) AddAttachment(ctx context.Context, a CaseAttachment) error {
	now := time.Now().UTC().Format(time.RFC3339)
	var stepID any
	if a.StepID != nil {
		stepID = *a.StepID
	}
	_, err := s.execContext(ctx, `
		INSERT INTO case_attachment (case_id, step_id, kind, file_path, original_name, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		a.CaseID, stepID, a.Kind, a.FilePath, a.OriginalName, now)
	return err
}

// DeleteStepsForCase removes all checklist rows for a case.
func (s *Store) DeleteStepsForCase(ctx context.Context, caseID string) error {
	_, err := s.execContext(ctx, `DELETE FROM case_step WHERE case_id = ?`, caseID)
	return err
}

// InsertSteps inserts checklist rows from definitions.
func (s *Store) InsertSteps(ctx context.Context, caseID string, defs []SOPStepDef) error {
	for i, d := range defs {
		req := 0
		if d.RequiresEvidence {
			req = 1
		}
		opt := 0
		if d.Optional {
			opt = 1
		}
		_, err := s.execContext(ctx, `
			INSERT INTO case_step (case_id, step_no, title, requires_evidence, optional) VALUES (?, ?, ?, ?, ?)`,
			caseID, i+1, d.Title, req, opt)
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

// ListCases returns cases with optional filtering and pagination.
func (s *Store) ListCases(ctx context.Context, f CaseFilter) ([]Case, error) {
	if f.Limit <= 0 {
		f.Limit = 50
	}
	if f.Page <= 0 {
		f.Page = 1
	}
	offset := (f.Page - 1) * f.Limit
	var conds []string
	var args []any
	if f.Status != "" {
		conds = append(conds, "c.status = ?")
		args = append(args, f.Status)
	}
	if f.Severity != "" {
		conds = append(conds, "c.severity = ?")
		args = append(args, f.Severity)
	}
	if f.Service != "" {
		conds = append(conds, "LOWER(c.service) LIKE ?")
		args = append(args, "%"+strings.ToLower(f.Service)+"%")
	}
	if f.Search != "" {
		conds = append(conds, "(LOWER(c.title) LIKE ? OR LOWER(c.case_id) LIKE ?)")
		q := "%" + strings.ToLower(f.Search) + "%"
		args = append(args, q, q)
	}
	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}
	query := fmt.Sprintf(`
		SELECT c.case_id, c.title, c.summary, c.service, c.severity, c.status, c.sop_id, c.sop_version, c.reporter,
			c.created_at, c.updated_at, c.resolved_at, COALESCE(s.slug,''), COALESCE(s.title,'')
		FROM cases c
		LEFT JOIN sop s ON s.id = c.sop_id
		%s
		ORDER BY c.created_at DESC
		LIMIT ? OFFSET ?`, where)
	args = append(args, f.Limit, offset)
	rows, err := s.queryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCases(rows)
}

// GetCase returns one case with joined sop labels.
func (s *Store) GetCase(ctx context.Context, caseID string) (*Case, error) {
	row := s.queryRowContext(ctx, `
		SELECT c.case_id, c.title, c.summary, c.service, c.severity, c.status, c.sop_id, c.sop_version, c.reporter,
			c.created_at, c.updated_at, c.resolved_at, COALESCE(c.rca_json,''), COALESCE(s.slug,''), COALESCE(s.title,'')
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
		&created, &updated, &resolved, &c.RCAJSON, &c.SOPSlug, &c.SOPTitle); err != nil {
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
	rows, err := s.queryContext(ctx, `
		SELECT id, case_id, step_no, title, requires_evidence, optional, done_at, done_by, notes, evidence_url
		FROM case_step WHERE case_id = ? ORDER BY step_no`, caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CaseStep
	for rows.Next() {
		var st CaseStep
		var req, opt int
		var doneAt, doneBy, notes, evidence sql.NullString
		if err := rows.Scan(&st.ID, &st.CaseID, &st.StepNo, &st.Title, &req, &opt, &doneAt, &doneBy, &notes, &evidence); err != nil {
			return nil, err
		}
		st.RequiresEvidence = req != 0
		st.Optional = opt != 0
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
	var req, opt int
	var doneAt, doneBy, notes, evidence sql.NullString
	err := s.queryRowContext(ctx, `
		SELECT id, case_id, step_no, title, requires_evidence, optional, done_at, done_by, notes, evidence_url
		FROM case_step WHERE id = ? AND case_id = ?`, stepID, caseID).
		Scan(&st.ID, &st.CaseID, &st.StepNo, &st.Title, &req, &opt, &doneAt, &doneBy, &notes, &evidence)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	st.RequiresEvidence = req != 0
	st.Optional = opt != 0
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
	err := s.queryRowContext(ctx, `
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
	_, err = s.execContext(ctx, `
		UPDATE case_step SET done_at = ?, done_by = ?, notes = ?, evidence_url = ? WHERE id = ? AND case_id = ?`,
		doneStr, nullIfEmpty(st.DoneBy), nullIfEmpty(st.Notes), nullIfEmpty(st.EvidenceURL), stepID, caseID)
	return err
}

func nullIfEmpty(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

// CreateSOP inserts a new SOP definition.
func (s *Store) CreateSOP(ctx context.Context, slug, title, owner, stepsJSON string) (*SOP, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.execContext(ctx,
		`INSERT INTO sop (slug, title, version, owner, steps_json, created_at) VALUES (?, ?, 1, ?, ?, ?)`,
		slug, title, nullIfEmpty(owner), stepsJSON, now)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetSOPByID(ctx, id)
}

// UpdateSOP updates an existing SOP and bumps its version.
func (s *Store) UpdateSOP(ctx context.Context, slug, title, owner, stepsJSON string) (*SOP, error) {
	_, err := s.execContext(ctx,
		`UPDATE sop SET title = ?, owner = ?, steps_json = ?, version = version + 1 WHERE slug = ?`,
		title, nullIfEmpty(owner), stepsJSON, slug)
	if err != nil {
		return nil, err
	}
	return s.GetSOPBySlug(ctx, slug)
}

// DeleteSOP removes an SOP by slug.
func (s *Store) DeleteSOP(ctx context.Context, slug string) error {
	_, err := s.execContext(ctx, `DELETE FROM sop WHERE slug = ?`, slug)
	return err
}

// LogAudit records an audit event for a case.
func (s *Store) LogAudit(ctx context.Context, caseID, actor, action, detail string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.execContext(ctx,
		`INSERT INTO case_audit (case_id, actor, action, detail, created_at) VALUES (?, ?, ?, ?, ?)`,
		caseID, nullIfEmpty(actor), action, nullIfEmpty(detail), now)
	return err
}

// ListAudit returns audit events for a case, newest first.
func (s *Store) ListAudit(ctx context.Context, caseID string) ([]CaseAudit, error) {
	rows, err := s.queryContext(ctx,
		`SELECT id, case_id, COALESCE(actor,''), action, COALESCE(detail,''), created_at
		 FROM case_audit WHERE case_id = ? ORDER BY id DESC`, caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CaseAudit
	for rows.Next() {
		var a CaseAudit
		var created string
		if err := rows.Scan(&a.ID, &a.CaseID, &a.Actor, &a.Action, &a.Detail, &created); err != nil {
			return nil, err
		}
		a.CreatedAt = parseTime(created)
		out = append(out, a)
	}
	return out, rows.Err()
}

// ListAttachments returns case-level attachments (no step_id).
func (s *Store) ListAttachments(ctx context.Context, caseID string) ([]CaseAttachment, error) {
	rows, err := s.queryContext(ctx, `
		SELECT id, case_id, kind, file_path, original_name, created_at
		FROM case_attachment WHERE case_id = ? AND step_id IS NULL ORDER BY id`, caseID)
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

// ListStepAttachmentsForCase returns all step-level attachments for a case, keyed by step_id.
func (s *Store) ListStepAttachmentsForCase(ctx context.Context, caseID string) (map[int64][]CaseAttachment, error) {
	rows, err := s.queryContext(ctx, `
		SELECT id, case_id, step_id, kind, file_path, original_name, created_at
		FROM case_attachment WHERE case_id = ? AND step_id IS NOT NULL ORDER BY id`, caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[int64][]CaseAttachment)
	for rows.Next() {
		var a CaseAttachment
		var stepID int64
		var created string
		if err := rows.Scan(&a.ID, &a.CaseID, &stepID, &a.Kind, &a.FilePath, &a.OriginalName, &created); err != nil {
			return nil, err
		}
		a.StepID = &stepID
		a.CreatedAt = parseTime(created)
		out[stepID] = append(out[stepID], a)
	}
	return out, rows.Err()
}

// DeleteAttachment removes a row from case_attachment (case-level or step-level).
func (s *Store) DeleteAttachment(ctx context.Context, caseID string, attID int64) error {
	_, err := s.execContext(ctx,
		`DELETE FROM case_attachment WHERE id = ? AND case_id = ?`, attID, caseID)
	return err
}

// GetAttachmentByID returns any attachment by id scoped to a case.
func (s *Store) GetAttachmentByID(ctx context.Context, caseID string, attID int64) (*CaseAttachment, error) {
	var a CaseAttachment
	var stepID sql.NullInt64
	var created string
	err := s.queryRowContext(ctx, `
		SELECT id, case_id, step_id, kind, file_path, original_name, created_at
		FROM case_attachment WHERE id = ? AND case_id = ?`, attID, caseID).
		Scan(&a.ID, &a.CaseID, &stepID, &a.Kind, &a.FilePath, &a.OriginalName, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if stepID.Valid {
		a.StepID = &stepID.Int64
	}
	a.CreatedAt = parseTime(created)
	return &a, nil
}
