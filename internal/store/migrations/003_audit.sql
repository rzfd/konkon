CREATE TABLE IF NOT EXISTS case_audit (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    case_id    TEXT    NOT NULL,
    actor      TEXT,
    action     TEXT    NOT NULL,
    detail     TEXT,
    created_at TEXT    NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_case_audit_case_id ON case_audit(case_id);
CREATE INDEX IF NOT EXISTS idx_case_step_case_id  ON case_step(case_id);
