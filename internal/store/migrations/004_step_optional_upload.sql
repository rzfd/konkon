ALTER TABLE case_step ADD COLUMN optional INTEGER NOT NULL DEFAULT 0;
ALTER TABLE case_attachment ADD COLUMN step_id INTEGER NULL REFERENCES case_step(id) ON DELETE SET NULL;

UPDATE sop SET steps_json = '[
  {"title": "Konfirmasi dampak dan severity (P1-P4)", "requires_evidence": false, "optional": false},
  {"title": "Kumpulkan bukti: dashboard / log / trace id", "requires_evidence": true, "optional": false},
  {"title": "Mitigasi atau rollback sesuai runbook", "requires_evidence": true, "optional": false},
  {"title": "Komunikasi status ke stakeholder internal", "requires_evidence": false, "optional": true},
  {"title": "Verifikasi pemulihan dan tutup monitoring sementara", "requires_evidence": true, "optional": false}
]' WHERE slug = 'incident-generic';
