CREATE TABLE IF NOT EXISTS case_audit (
    id BIGSERIAL PRIMARY KEY,
    case_id TEXT NOT NULL,
    actor TEXT,
    action TEXT NOT NULL,
    detail TEXT,
    created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_case_audit_case_id ON case_audit(case_id);

