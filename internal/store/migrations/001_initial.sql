PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS sop (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    slug TEXT NOT NULL UNIQUE,
    title TEXT NOT NULL,
    version INTEGER NOT NULL DEFAULT 1,
    owner TEXT,
    steps_json TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS sop_rule (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    sop_id INTEGER NOT NULL REFERENCES sop(id) ON DELETE CASCADE,
    priority INTEGER NOT NULL DEFAULT 0,
    service TEXT,
    keyword TEXT,
    severity_min TEXT
);

CREATE INDEX IF NOT EXISTS idx_sop_rule_priority ON sop_rule(priority DESC);

CREATE TABLE IF NOT EXISTS cases (
    case_id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    summary TEXT,
    service TEXT,
    severity TEXT,
    status TEXT NOT NULL,
    sop_id INTEGER REFERENCES sop(id),
    sop_version INTEGER,
    reporter TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    resolved_at TEXT
);

CREATE INDEX IF NOT EXISTS idx_cases_status ON cases(status);
CREATE INDEX IF NOT EXISTS idx_cases_created ON cases(created_at);

CREATE TABLE IF NOT EXISTS case_step (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    case_id TEXT NOT NULL REFERENCES cases(case_id) ON DELETE CASCADE,
    step_no INTEGER NOT NULL,
    title TEXT NOT NULL,
    requires_evidence INTEGER NOT NULL DEFAULT 0,
    done_at TEXT,
    done_by TEXT,
    notes TEXT,
    evidence_url TEXT,
    UNIQUE(case_id, step_no)
);

CREATE TABLE IF NOT EXISTS case_attachment (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    case_id TEXT NOT NULL REFERENCES cases(case_id) ON DELETE CASCADE,
    kind TEXT NOT NULL DEFAULT 'screenshot',
    file_path TEXT NOT NULL,
    original_name TEXT,
    created_at TEXT NOT NULL
);
