PRAGMA foreign_keys = ON;

INSERT OR IGNORE INTO sop (id, slug, title, version, owner, steps_json, created_at) VALUES
(1, 'incident-generic', 'Respons insiden generik TechOps', 1, 'techops', '[
  {"title": "Konfirmasi dampak dan severity (P1–P4)", "requires_evidence": false},
  {"title": "Kumpulkan bukti: dashboard / log / trace id", "requires_evidence": true},
  {"title": "Mitigasi atau rollback sesuai runbook", "requires_evidence": true},
  {"title": "Komunikasi status ke stakeholder internal", "requires_evidence": false},
  {"title": "Verifikasi pemulihan dan tutup monitoring sementara", "requires_evidence": true}
]', datetime('now')),

(2, 'payment-latency', 'Investigasi latency / timeout pembayaran', 1, 'techops', '[
  {"title": "Cek health endpoint payment gateway", "requires_evidence": true},
  {"title": "Review error rate dan p95 latency (metrics)", "requires_evidence": true},
  {"title": "Cek deployment / config terakhir terkait payment", "requires_evidence": false},
  {"title": "Eskalasi ke tim payment jika perlu", "requires_evidence": false},
  {"title": "Dokumentasikan RCA singkat di case", "requires_evidence": true}
]', datetime('now'));

INSERT OR IGNORE INTO sop_rule (id, sop_id, priority, service, keyword, severity_min) VALUES
(1, 2, 100, 'payment', NULL, NULL),
(2, 2, 90, NULL, 'timeout', NULL),
(3, 2, 80, NULL, 'latency', NULL),
(4, 1, 10, NULL, NULL, NULL);
