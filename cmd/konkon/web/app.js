const app = document.getElementById("app");

// ── Design tokens (dark theme) ────────────────────────────────────────────────
const IC  = "w-full px-3 py-2 bg-[#0f172a] border border-[#253041] rounded-lg text-sm text-[#e7ecf3] placeholder:text-[#7e8aa0] focus:outline-none focus:border-[#60a5fa] focus:ring-1 focus:ring-sky-300/20 transition-colors";
const TA  = IC + " resize-y min-h-[80px]";
const SC  = IC + "";
const LB  = "block text-[10px] font-semibold uppercase tracking-widest text-[#a9b4c4] mb-1.5";
const Btn = "inline-flex items-center gap-1.5 px-4 py-2 bg-[#60a5fa] text-[#07101f] text-sm font-semibold rounded-lg hover:bg-[#93c5fd] transition-colors border-0 cursor-pointer";
const BtnSm = "inline-flex items-center gap-1 px-3 py-1.5 bg-[#121a26] text-[#e7ecf3] text-xs font-medium rounded-lg hover:bg-[#172235] transition-colors border border-[#253041] cursor-pointer";
const BtnGhost = "inline-flex items-center gap-1 px-3 py-1.5 text-[#a9b4c4] text-xs font-medium rounded-lg hover:text-[#e7ecf3] hover:bg-[#121a26] transition-colors border-0 cursor-pointer bg-transparent";
const BtnDanger = "inline-flex items-center gap-1 px-3 py-1.5 text-[#f07178] text-xs font-medium rounded-lg hover:bg-[#2a1217] transition-colors border border-[#5a2a35] bg-transparent cursor-pointer";
const Card = "bg-[#121a26] border border-[#253041] rounded-2xl p-6 mb-4 shadow-[0_10px_30px_rgba(0,0,0,0.35)]";
const Err  = "text-[#f07178] text-xs mt-2 min-h-[1rem]";

// ── Utils ─────────────────────────────────────────────────────────────────────
/** Format API ISO timestamp for display as Jakarta (WIB); backend already sends +07:00. */
function fmtWIB(iso) {
  if (!iso) return "—";
  const s = String(iso);
  const i = s.indexOf("T");
  if (i === -1) return s;
  const date = s.slice(0, i);
  const rest = s.slice(i + 1);
  const timeEnd = rest.search(/[Z+-]/);
  const time = timeEnd === -1 ? rest.slice(0, 5) : rest.slice(0, Math.min(5, timeEnd));
  return `${date} ${time} WIB`;
}

function esc(s) {
  return String(s ?? "")
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

async function api(path, opts = {}) {
  const res = await fetch(path, {
    headers: { Accept: "application/json", ...(opts.headers || {}) },
    ...opts,
  });
  const text = await res.text();
  let data = null;
  if (text) {
    try { data = JSON.parse(text); } catch { data = text; }
  }
  if (!res.ok) {
    const err = new Error(res.statusText);
    err.status = res.status;
    err.body = data;
    throw err;
  }
  return data;
}

function badge(status) {
  const s = (status || "").toLowerCase();
  const map = {
    needs_triage: "bg-amber-400/10 text-amber-200 border border-amber-400/20",
    open:         "bg-sky-400/10 text-sky-200 border border-sky-400/20",
    resolved:     "bg-emerald-400/10 text-emerald-200 border border-emerald-400/20",
  };
  const cls = map[s] || map.open;
  return `<span class="inline-flex items-center px-2 py-0.5 rounded-md text-[10px] font-bold uppercase tracking-wide ${cls}">${esc(status)}</span>`;
}

function actionColor(action) {
  const map = {
    case_created: "bg-sky-400/10 text-sky-200",
    sop_assigned: "bg-violet-400/10 text-violet-200",
    step_done:    "bg-emerald-400/10 text-emerald-200",
    step_undone:  "bg-slate-400/10 text-slate-200",
    case_closed:  "bg-amber-400/10 text-amber-200",
    rca_updated:  "bg-cyan-400/10 text-cyan-200",
    rca_draft_generated: "bg-indigo-400/10 text-indigo-200",
  };
  return map[action] || "bg-slate-400/10 text-slate-200";
}

function normalizeRca(rca) {
  const r = rca && typeof rca === "object" ? rca : {};
  const whys = (Array.isArray(r.five_whys) ? r.five_whys : []).map((v) => String(v ?? "")).filter((v) => v.trim()).slice(0, 12);
  const actionItems = (Array.isArray(r.action_items) ? r.action_items : []).map((v) => String(v ?? "")).filter((v) => v.trim()).slice(0, 12);
  return {
    incident_timeline: r.incident_timeline || "",
    five_whys: whys,
    root_cause: r.root_cause || "",
    contributing_factors: r.contributing_factors || "",
    corrective_actions: r.corrective_actions || "",
    preventive_actions: r.preventive_actions || "",
    action_items: actionItems,
    detection_gap: r.detection_gap || "",
  };
}

function normalizeIncidentTimeline(raw) {
  const lines = String(raw ?? "").split("\n");
  const out = [];
  const oldFmt = /^(\d{4})-(\d{2})-(\d{2})\s+(\d{2}):(\d{2})(?::(\d{2}))?\s*(?:WIB)?\s*[—-]\s*(.+)$/i;
  const newFmt = /^(\d{6})\s+(\d{2}):(\d{2}):(\d{2})\s*\|\s*(.+)$/;
  for (const ln of lines) {
    const s = ln.trim();
    if (!s) continue;
    const mNew = s.match(newFmt);
    if (mNew) {
      out.push(`${mNew[1]} ${mNew[2]}:${mNew[3]}:${mNew[4]} | ${mNew[5].trim()}`);
      continue;
    }
    const mOld = s.match(oldFmt);
    if (mOld) {
      const yy = mOld[1].slice(2);
      const mm = mOld[2];
      const dd = mOld[3];
      const hh = mOld[4];
      const mi = mOld[5];
      const ss = mOld[6] || "00";
      out.push(`${dd}${mm}${yy} ${hh}:${mi}:${ss} | ${mOld[7].trim()}`);
      continue;
    }
    // Keep unknown free-form lines, but still force single-space around the separator when present.
    if (s.includes("|")) {
      const [left, ...rest] = s.split("|");
      out.push(`${left.trim()} | ${rest.join("|").trim()}`);
    } else {
      out.push(s);
    }
  }
  return out.join("\n");
}

function ymdToDdMmYy(ymd) {
  const m = String(ymd || "").match(/^(\d{4})-(\d{2})-(\d{2})$/);
  if (!m) return "";
  return `${m[3]}${m[2]}${m[1].slice(2)}`;
}

function ddMmYyToYmd(ddmmyy) {
  const m = String(ddmmyy || "").match(/^(\d{2})(\d{2})(\d{2})$/);
  if (!m) return "";
  return `20${m[3]}-${m[2]}-${m[1]}`;
}

function parseTimelineEntries(raw) {
  const normalized = normalizeIncidentTimeline(raw);
  const lines = normalized ? normalized.split("\n") : [];
  const entries = [];
  const re = /^(\d{6})\s+(\d{2}):(\d{2}):(\d{2})\s*\|\s*(.+)$/;
  for (const line of lines) {
    const m = line.trim().match(re);
    if (!m) continue;
    entries.push({
      date: ddMmYyToYmd(m[1]),
      time: `${m[2]}:${m[3]}:${m[4]}`,
      detail: m[5].trim(),
    });
  }
  return entries;
}

function timelineEntryRow(entry = {}) {
  return `<div class="timeline-row grid sm:grid-cols-[150px_130px_1fr_auto] gap-2 items-start">
      <input type="date" class="${IC} text-xs timeline-date" value="${esc(entry.date || "")}" />
      <input type="time" step="1" class="${IC} text-xs timeline-time" value="${esc(entry.time || "")}" />
      <input type="text" class="${IC} text-xs timeline-detail" placeholder="detail kejadian" value="${esc(entry.detail || "")}" />
      <button type="button" class="timeline-del ${BtnGhost} text-xs mt-0.5">Hapus</button>
    </div>`;
}

function timelineFromRows() {
  const rows = [...document.querySelectorAll(".timeline-row")];
  const out = [];
  for (const row of rows) {
    const date = row.querySelector(".timeline-date")?.value || "";
    const time = row.querySelector(".timeline-time")?.value || "";
    const detail = (row.querySelector(".timeline-detail")?.value || "").trim();
    if (!date && !time && !detail) continue;
    if (!date || !time || !detail) continue;
    const ddmmyy = ymdToDdMmYy(date);
    if (!ddmmyy) continue;
    const hhmmss = time.length === 5 ? `${time}:00` : time;
    out.push(`${ddmmyy} ${hhmmss} | ${detail}`);
  }
  return out.join("\n");
}

function bindTimelineEditor(rawTimeline) {
  const host = document.getElementById("timelineRows");
  const addBtn = document.getElementById("addTimelineRow");
  if (!host || !addBtn) return;

  const addRow = (entry = {}) => {
    host.insertAdjacentHTML("beforeend", timelineEntryRow(entry));
    const row = host.lastElementChild;
    row?.querySelector(".timeline-del")?.addEventListener("click", () => row.remove());
  };

  const entries = parseTimelineEntries(rawTimeline);
  if (entries.length) {
    entries.forEach((e) => addRow(e));
  } else {
    addRow({});
  }
  addBtn.onclick = () => addRow({});
}

function rcaPayloadFromDom() {
  return {
    incident_timeline: timelineFromRows(),
    five_whys: [...document.querySelectorAll(".rca-why-item")].map((el) => el.value ?? "").filter((s) => String(s).trim()),
    root_cause: document.getElementById("rca-root")?.value ?? "",
    contributing_factors: document.getElementById("rca-contrib")?.value ?? "",
    corrective_actions: document.getElementById("rca-corrective")?.value ?? "",
    preventive_actions: document.getElementById("rca-preventive")?.value ?? "",
    action_items: [...document.querySelectorAll(".rca-action-item")].map((el) => el.value ?? "").filter((s) => String(s).trim()),
    detection_gap: document.getElementById("rca-detect-gap")?.value ?? "",
  };
}

function whyRowHtml(value = "") {
  return `<div class="rca-why-row flex gap-3 items-start">
      <div class="mt-2 w-6 h-6 rounded-full bg-[#121a26] border border-[#253041] text-[#a9b4c4] text-xs font-bold flex items-center justify-center shrink-0">*</div>
      <div class="flex-1">
        <label class="${LB}">Analisis <span class="text-[#a9b4c4] normal-case tracking-normal font-normal">(opsional)</span></label>
        <textarea rows="2" class="rca-why-item ${TA} min-h-[52px]">${esc(value)}</textarea>
      </div>
      <button type="button" class="del-why-row ${BtnDanger} text-xs mt-8">Hapus</button>
    </div>`;
}

function actionRowHtml(value = "") {
  return `<div class="rca-action-row grid sm:grid-cols-[1fr_auto] gap-2 items-start">
      <input type="text" class="rca-action-item ${IC} text-xs" placeholder="tulis action item" value="${esc(value)}" />
      <button type="button" class="del-action-row ${BtnDanger} text-xs">Hapus</button>
    </div>`;
}

function bindRcaDynamicRows(rca) {
  const whyHost = document.getElementById("whyRows");
  const actionHost = document.getElementById("actionRows");
  const addWhy = document.getElementById("addWhyRow");
  const addAction = document.getElementById("addActionRow");
  if (!whyHost || !actionHost || !addWhy || !addAction) return;

  const whys = (rca?.five_whys || []).slice(0, 12);
  const actions = (rca?.action_items || []).slice(0, 12);
  whyHost.innerHTML = (whys.length ? whys : [""]).map((v) => whyRowHtml(v)).join("");
  actionHost.innerHTML = (actions.length ? actions : [""]).map((v) => actionRowHtml(v)).join("");

  const bindDelete = () => {
    whyHost.querySelectorAll(".del-why-row").forEach((btn) => {
      btn.onclick = () => {
        btn.closest(".rca-why-row")?.remove();
        if (!whyHost.querySelector(".rca-why-row")) whyHost.insertAdjacentHTML("beforeend", whyRowHtml(""));
      };
    });
    actionHost.querySelectorAll(".del-action-row").forEach((btn) => {
      btn.onclick = () => {
        btn.closest(".rca-action-row")?.remove();
        if (!actionHost.querySelector(".rca-action-row")) actionHost.insertAdjacentHTML("beforeend", actionRowHtml(""));
        bindDelete();
      };
    });
  };
  bindDelete();

  addWhy.onclick = () => {
    if (whyHost.querySelectorAll(".rca-why-row").length >= 12) return;
    whyHost.insertAdjacentHTML("beforeend", whyRowHtml(""));
    bindDelete();
  };
  addAction.onclick = () => {
    if (actionHost.querySelectorAll(".rca-action-row").length >= 12) return;
    actionHost.insertAdjacentHTML("beforeend", actionRowHtml(""));
    bindDelete();
  };
}

function applyRcaDraftToDom(draft) {
  const next = normalizeRca(draft);
  const current = normalizeRca(rcaPayloadFromDom());
  const keep = (value, fallback) => String(value || "").trim() ? value : (fallback || "");

  document.getElementById("rca-root").value = keep(current.root_cause, next.root_cause);
  document.getElementById("rca-contrib").value = keep(current.contributing_factors, next.contributing_factors);
  document.getElementById("rca-corrective").value = keep(current.corrective_actions, next.corrective_actions);
  document.getElementById("rca-preventive").value = keep(current.preventive_actions, next.preventive_actions);
  document.getElementById("rca-detect-gap").value = keep(current.detection_gap, next.detection_gap);
  const timelineHost = document.getElementById("timelineRows");
  if (timelineHost) {
    timelineHost.innerHTML = "";
    bindTimelineEditor(keep(current.incident_timeline, next.incident_timeline));
  }

  const currentWhys = [...document.querySelectorAll(".rca-why-item")].map((el) => el.value || "").filter((v) => v.trim());
  const currentActions = [...document.querySelectorAll(".rca-action-item")].map((el) => el.value || "").filter((v) => v.trim());
  bindRcaDynamicRows({
    five_whys: currentWhys.length ? currentWhys : next.five_whys,
    action_items: currentActions.length ? currentActions : next.action_items,
  });
}

async function flushChecklistFieldsToServer(caseId) {
  const items = [...document.querySelectorAll("#steps li[data-id]")];
  await Promise.all(
    items.map((li) => {
      const sid = li.getAttribute("data-id");
      const ev = li.querySelector(".ev");
      const notes = li.querySelector(".notes");
      const who = li.querySelector(".who");
      return api(`/api/cases/${encodeURIComponent(caseId)}/steps/${encodeURIComponent(sid)}`, {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          evidence_url: ev?.value ?? "",
          notes: notes?.value ?? "",
          done_by: who?.value ?? "",
        }),
      });
    })
  );
}

async function exportCaseSummary(id, format) {
  try {
    await api(`/api/cases/${encodeURIComponent(id)}/rca`, {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(rcaPayloadFromDom()),
    });
    await flushChecklistFieldsToServer(id);
  } catch (e) {
    const msg = typeof e.body === "string" ? e.body : e.body?.error || e.message;
    const ok = await confirmDialog(
      `Gagal menyimpan isian form ke server sebelum ekspor: ${msg}\n\nLanjutkan ekspor memakai data terakhir yang tersimpan di server?`
    );
    if (!ok) return;
  }
  window.open(`/api/cases/${encodeURIComponent(id)}/summary?format=${encodeURIComponent(format)}`, "_blank", "noopener");
}

function caseMetaHtml(c) {
  const rows = [
    ["Layanan",    c.service],
    ["Severity",   c.severity],
    ["Pelapor",    c.reporter],
    ["Dibuat",     fmtWIB(c.created_at)],
    ["Diperbarui", fmtWIB(c.updated_at)],
  ];
  if (c.resolved_at) rows.push(["Selesai", fmtWIB(c.resolved_at)]);
  return `<dl class="grid grid-cols-2 sm:grid-cols-3 gap-x-6 gap-y-4">
    ${rows.map(([label, val]) => `
      <div>
        <dt class="text-[10px] font-semibold uppercase tracking-widest text-[#a9b4c4] mb-0.5">${esc(label)}</dt>
        <dd class="text-sm font-medium text-[#e7ecf3]">${esc(val || "—")}</dd>
      </div>`).join("")}
  </dl>`;
}

// ── Confirm dialog ────────────────────────────────────────────────────────────
function confirmDialog(message) {
  return new Promise((resolve) => {
    const overlay = document.createElement("div");
    overlay.className = "fixed inset-0 bg-black/60 backdrop-blur-sm flex items-center justify-center z-50";
    overlay.innerHTML = `
      <div class="bg-[#121a26] border border-[#253041] rounded-2xl p-6 max-w-sm w-[90vw] shadow-[0_20px_60px_rgba(0,0,0,0.55)]">
        <p class="text-sm text-[#e7ecf3] leading-relaxed mb-6">${esc(message)}</p>
        <div class="flex justify-end gap-2">
          <button class="modal-cancel ${BtnGhost}">Batal</button>
          <button class="modal-ok px-4 py-2 text-sm font-semibold text-[#07101f] bg-[#f07178] rounded-lg hover:bg-[#ff8a90] transition-colors border-0 cursor-pointer">Ya, lanjutkan</button>
        </div>
      </div>`;
    document.body.appendChild(overlay);
    overlay.querySelector(".modal-cancel").onclick = () => { overlay.remove(); resolve(false); };
    overlay.querySelector(".modal-ok").onclick     = () => { overlay.remove(); resolve(true); };
  });
}

// ── Home ──────────────────────────────────────────────────────────────────────
let homeState = { page: 1, status: "", severity: "", search: "", timer: null };

async function renderHome() {
  app.innerHTML = `<div class="${Card}"><p class="text-sm text-[#a9b4c4]">Memuat…</p></div>`;
  const { page, status, severity, search } = homeState;
  const params = new URLSearchParams({ page, limit: 20 });
  if (status)   params.set("status", status);
  if (severity) params.set("severity", severity);
  if (search)   params.set("search", search);

  try {
    const cases = await api(`/api/cases?${params}`);
    const rows = (cases || []).map((c) => `
      <tr class="border-t border-[#253041] hover:bg-[#0f172a] transition-colors cursor-pointer group">
        <td class="py-3 pr-5 align-middle">
          <a href="#/case/${esc(c.case_id)}" class="text-xs font-mono font-semibold text-[#93c5fd] no-underline group-hover:text-[#e7ecf3]">${esc(c.case_id)}</a>
        </td>
        <td class="py-3 pr-5 align-middle text-sm text-[#e7ecf3] max-w-[240px] truncate">${esc(c.title)}</td>
        <td class="py-3 pr-5 align-middle text-xs text-[#a9b4c4]">${esc(c.service || "—")}</td>
        <td class="py-3 pr-5 align-middle">
          ${c.severity ? `<span class="text-[10px] font-bold text-[#a9b4c4] bg-[#121a26] border border-[#253041] px-1.5 py-0.5 rounded">${esc(c.severity)}</span>` : "<span class='text-[#a9b4c4]'>—</span>"}
        </td>
        <td class="py-3 pr-5 align-middle text-xs text-[#a9b4c4]">${esc(c.reporter || "—")}</td>
        <td class="py-3 pr-5 align-middle">${badge(c.status)}</td>
        <td class="py-3 align-middle text-xs font-mono text-[#a9b4c4]">${esc(c.sop_slug || "—")}</td>
      </tr>`).join("");

    app.innerHTML = `
      <div class="${Card}">
        <div class="flex items-center justify-between mb-5">
          <h2 class="text-base font-semibold text-[#e7ecf3] m-0">Daftar case</h2>
          <a href="#/new" class="${Btn} no-underline text-xs">+ Case baru</a>
        </div>

        <!-- Filter bar -->
        <div class="flex flex-wrap gap-2 mb-5">
          <div class="flex-1 min-w-[160px]">
            <input id="fSearch" type="search" placeholder="Cari judul atau ID…"
              value="${esc(search)}"
              class="${IC} !py-1.5 text-xs" />
          </div>
          <select id="fStatus" class="${SC} !py-1.5 text-xs w-auto">
            <option value="">Semua status</option>
            <option value="needs_triage"${status === "needs_triage" ? " selected" : ""}>needs_triage</option>
            <option value="open"${status === "open" ? " selected" : ""}>open</option>
            <option value="resolved"${status === "resolved" ? " selected" : ""}>resolved</option>
          </select>
          <select id="fSeverity" class="${SC} !py-1.5 text-xs w-auto">
            <option value="">Semua severity</option>
            <option${severity === "P1" ? " selected" : ""}>P1</option>
            <option${severity === "P2" ? " selected" : ""}>P2</option>
            <option${severity === "P3" ? " selected" : ""}>P3</option>
            <option${severity === "P4" ? " selected" : ""}>P4</option>
          </select>
          <button id="fReset" class="${BtnGhost} !py-1.5 text-xs">Reset</button>
        </div>

        <!-- Table -->
        <div class="overflow-x-auto">
          <table class="w-full border-collapse">
            <thead>
              <tr>
                <th class="text-left text-[10px] font-semibold uppercase tracking-widest text-[#a9b4c4] pb-3 pr-5">ID</th>
                <th class="text-left text-[10px] font-semibold uppercase tracking-widest text-[#a9b4c4] pb-3 pr-5">Judul</th>
                <th class="text-left text-[10px] font-semibold uppercase tracking-widest text-[#a9b4c4] pb-3 pr-5">Layanan</th>
                <th class="text-left text-[10px] font-semibold uppercase tracking-widest text-[#a9b4c4] pb-3 pr-5">Sev</th>
                <th class="text-left text-[10px] font-semibold uppercase tracking-widest text-[#a9b4c4] pb-3 pr-5">Pelapor</th>
                <th class="text-left text-[10px] font-semibold uppercase tracking-widest text-[#a9b4c4] pb-3 pr-5">Status</th>
                <th class="text-left text-[10px] font-semibold uppercase tracking-widest text-[#a9b4c4] pb-3">SOP</th>
              </tr>
            </thead>
            <tbody>${rows || `<tr><td colspan="7" class="py-8 text-center text-sm text-[#a9b4c4]">Tidak ada case ditemukan</td></tr>`}</tbody>
          </table>
        </div>

        <!-- Pagination -->
        <div class="flex items-center justify-center gap-3 mt-5 select-none">
          <button id="pgPrev" ${page <= 1 ? "disabled" : ""}
            class="!mt-0 !w-9 !h-9 !p-0 !rounded-full !bg-[#121a26] !text-[#e7ecf3] !text-xl !font-bold hover:!bg-[#60a5fa] hover:!text-[#07101f] disabled:!opacity-30 disabled:!cursor-not-allowed !transition-colors !duration-150 flex items-center justify-center border border-[#253041] cursor-pointer">
            ‹
          </button>
          <span class="text-xs text-[#a9b4c4] min-w-[5rem] text-center tabular-nums">Halaman ${page}</span>
          <button id="pgNext" ${(cases || []).length < 20 ? "disabled" : ""}
            class="!mt-0 !w-9 !h-9 !p-0 !rounded-full !bg-[#121a26] !text-[#e7ecf3] !text-xl !font-bold hover:!bg-[#60a5fa] hover:!text-[#07101f] disabled:!opacity-30 disabled:!cursor-not-allowed !transition-colors !duration-150 flex items-center justify-center border border-[#253041] cursor-pointer">
            ›
          </button>
        </div>
      </div>`;

    function applyFilter() {
      homeState.status   = document.getElementById("fStatus").value;
      homeState.severity = document.getElementById("fSeverity").value;
      homeState.page     = 1;
      renderHome();
    }
    document.getElementById("fStatus").addEventListener("change", applyFilter);
    document.getElementById("fSeverity").addEventListener("change", applyFilter);
    document.getElementById("fSearch").addEventListener("input", (e) => {
      clearTimeout(homeState.timer);
      homeState.timer = setTimeout(() => {
        homeState.search = e.target.value.trim();
        homeState.page   = 1;
        renderHome();
      }, 300);
    });
    document.getElementById("fReset").addEventListener("click", () => {
      homeState = { page: 1, status: "", severity: "", search: "", timer: null };
      renderHome();
    });
    document.getElementById("pgPrev").addEventListener("click", () => {
      if (homeState.page > 1) { homeState.page--; renderHome(); }
    });
    document.getElementById("pgNext").addEventListener("click", () => {
      homeState.page++; renderHome();
    });
  } catch (e) {
    app.innerHTML = `<div class="${Card}"><p class="text-sm text-[#f07178]">Gagal memuat: ${esc(e.message)}</p></div>`;
  }
}

// ── New Case ──────────────────────────────────────────────────────────────────
const DRAFT_KEY = "konkon_new_case_draft";

async function renderNew() {
  /** @type {File[]} */
  let pendingIntakeImages = [];

  function renderIntakeList() {
    const list = document.getElementById("intake-list");
    if (!list) return;
    if (!pendingIntakeImages.length) {
      list.innerHTML = `<p class="text-xs text-[#a9b4c4] m-0">Belum ada gambar. Tambah beberapa lalu hapus yang tidak dipakai sebelum kirim.</p>`;
      return;
    }
    list.innerHTML = pendingIntakeImages.map((file, idx) => `
      <div class="flex items-center gap-3 py-2 border-b border-[#253041] last:border-0 intake-row" data-idx="${idx}">
        <div class="w-14 h-14 rounded-lg overflow-hidden border border-[#253041] shrink-0 bg-[#0f172a] flex items-center justify-center">
          <img src="" alt="" class="intake-thumb max-w-full max-h-full object-contain hidden" />
        </div>
        <div class="min-w-0 flex-1">
          <p class="text-xs font-medium text-[#e7ecf3] truncate m-0">${esc(file.name)}</p>
          <p class="text-[10px] text-[#a9b4c4] m-0">${(file.size / 1024).toFixed(1)} KB</p>
        </div>
        <button type="button" class="intake-remove ${BtnDanger} shrink-0 text-[10px] py-1 px-2">Hapus</button>
      </div>`).join("");
    list.querySelectorAll(".intake-row").forEach((row) => {
      const idx = Number(row.getAttribute("data-idx"));
      const url = URL.createObjectURL(pendingIntakeImages[idx]);
      const img = row.querySelector(".intake-thumb");
      img.src = url;
      img.classList.remove("hidden");
      img.onload = () => URL.revokeObjectURL(url);
      row.querySelector(".intake-remove").addEventListener("click", () => {
        pendingIntakeImages = pendingIntakeImages.filter((_, j) => j !== idx);
        renderIntakeList();
        attachFormChecker({ getScreensFilled: () => pendingIntakeImages.length > 0 });
      });
    });
  }

  app.innerHTML = `
    <div class="${Card}">
      <div class="mb-6">
        <h2 class="text-base font-semibold text-[#e7ecf3] m-0 mb-1">Buat case baru</h2>
        <p class="text-xs text-[#a9b4c4] m-0">Sistem memetakan SOP secara otomatis via rule berdasarkan service, keyword, dan severity.</p>
      </div>

      <form id="f" class="space-y-4">
        <div>
          <label class="${LB}">Judul <span class="text-[#f07178] normal-case tracking-normal font-normal">*</span><span class="field-status" id="fst-title"></span></label>
          <input name="title" required placeholder="mis. Timeout checkout payment" class="${IC}" />
        </div>
        <div>
          <label class="${LB}">Ringkasan <span class="text-[#a9b4c4] normal-case tracking-normal font-normal">(opsional)</span><span class="field-status" id="fst-summary"></span></label>
          <textarea name="summary" placeholder="Gejala singkat, dampak, dan konteks…" class="${TA}"></textarea>
        </div>
        <div class="grid grid-cols-2 gap-4">
          <div>
            <label class="${LB}">Layanan <span class="text-[#a9b4c4] normal-case tracking-normal font-normal">(opsional)</span><span class="field-status" id="fst-service"></span></label>
            <input name="service" placeholder="payment, auth, …" class="${IC}" />
          </div>
          <div>
            <label class="${LB}">Severity <span class="text-[#a9b4c4] normal-case tracking-normal font-normal">(opsional)</span><span class="field-status" id="fst-severity"></span></label>
            <select name="severity" class="${SC}">
              <option value="">— pilih —</option>
              <option>P1</option><option>P2</option><option>P3</option><option>P4</option>
            </select>
          </div>
        </div>
        <div>
          <label class="${LB}">Pelapor <span class="text-[#a9b4c4] normal-case tracking-normal font-normal">(opsional)</span><span class="field-status" id="fst-reporter"></span></label>
          <input name="reporter" placeholder="nama / @handle" class="${IC}" />
        </div>
        <div>
          <label class="${LB}">Gambar lampiran <span class="text-[#a9b4c4] normal-case tracking-normal font-normal">(opsional, bisa banyak)</span><span class="field-status" id="fst-screenshots"></span></label>
          <input type="file" id="intake-pick" accept="image/*" multiple class="hidden" />
          <button type="button" id="intake-add" class="${BtnGhost} text-xs mb-2">+ Tambah gambar</button>
          <div id="intake-list" class="rounded-lg border border-[#253041] bg-[#0f172a]/40 px-3 py-2"></div>
        </div>

        <div id="form-checker-status" class="form-checker has-warn"></div>

        <div class="flex items-center gap-3 pt-2">
          <button type="submit" class="${Btn}">Buat case</button>
          <button type="button" id="clearDraft" class="${BtnGhost}">Hapus draft</button>
        </div>
        <div id="err" class="${Err}"></div>
      </form>
    </div>`;

  restoreDraft();
  renderIntakeList();
  attachFormChecker({ getScreensFilled: () => pendingIntakeImages.length > 0 });

  document.getElementById("intake-add").addEventListener("click", () => document.getElementById("intake-pick").click());
  document.getElementById("intake-pick").addEventListener("change", (ev) => {
    const files = Array.from(ev.target.files || []);
    ev.target.value = "";
    const max = 24;
    const room = max - pendingIntakeImages.length;
    if (room <= 0) return;
    pendingIntakeImages.push(...files.slice(0, room));
    renderIntakeList();
    attachFormChecker({ getScreensFilled: () => pendingIntakeImages.length > 0 });
  });

  document.getElementById("f").addEventListener("submit", async (ev) => {
    ev.preventDefault();
    const errEl = document.getElementById("err");
    errEl.textContent = "";
    const fd = new FormData(ev.target);
    fd.delete("screenshots");
    pendingIntakeImages.forEach((file) => fd.append("screenshots", file, file.name));
    try {
      const res = await fetch("/api/cases", { method: "POST", body: fd });
      const data = await res.json().catch(() => ({}));
      if (!res.ok) { errEl.textContent = data.error || res.statusText; return; }
      if (data.screenshot_warning) {
        errEl.style.color = "var(--warn)";
        errEl.textContent = "⚠ " + data.screenshot_warning;
      }
      localStorage.removeItem(DRAFT_KEY);
      location.hash = `#/case/${data.case_id}`;
    } catch (e) {
      errEl.textContent = e.message;
    }
  });

  document.getElementById("clearDraft").addEventListener("click", () => {
    localStorage.removeItem(DRAFT_KEY);
    document.querySelectorAll("#f input:not([type=file]), #f textarea, #f select").forEach((el) => { el.value = ""; });
    pendingIntakeImages = [];
    renderIntakeList();
    attachFormChecker({ getScreensFilled: () => pendingIntakeImages.length > 0 });
  });
}

function saveDraft() {
  const f = document.getElementById("f");
  if (!f) return;
  const draft = {};
  f.querySelectorAll("input:not([type=file]), textarea, select").forEach((el) => { draft[el.name] = el.value; });
  localStorage.setItem(DRAFT_KEY, JSON.stringify(draft));
}

function restoreDraft() {
  const raw = localStorage.getItem(DRAFT_KEY);
  if (!raw) return;
  try {
    const draft = JSON.parse(raw);
    const f = document.getElementById("f");
    if (!f) return;
    Object.entries(draft).forEach(([name, value]) => {
      const el = f.querySelector(`[name="${name}"]`);
      if (el) el.value = value;
    });
  } catch {}
}

// ── Case Detail ───────────────────────────────────────────────────────────────
async function renderCase(id) {
  app.innerHTML = `<div class="${Card}"><p class="text-sm text-[#a9b4c4]">Memuat…</p></div>`;
  try {
    const [c, steps, atts, sops, audit] = await Promise.all([
      api(`/api/cases/${encodeURIComponent(id)}`),
      api(`/api/cases/${encodeURIComponent(id)}/steps`),
      api(`/api/cases/${encodeURIComponent(id)}/attachments`).catch(() => []),
      api("/api/sops").catch(() => []),
      api(`/api/cases/${encodeURIComponent(id)}/audit`).catch(() => []),
    ]);

    const sopOpts = (sops || []).map((s) =>
      `<option value="${esc(s.slug)}">${esc(s.slug)} — ${esc(s.title)}</option>`).join("");

    const attHtml = (atts || []).map((a) => {
      const del = `<button type="button" class="del-case-att ${BtnDanger} text-[10px] py-0.5 px-2 shrink-0" data-att-id="${esc(String(a.id))}">Hapus</button>`;
      if ((a.kind || "") === "screenshot" && a.url) {
        return `<div class="attach">
          <div class="flex items-start justify-between gap-2 mb-1">
            <a href="${esc(a.url)}" target="_blank" rel="noopener" class="text-xs text-[#93c5fd] no-underline hover:underline min-w-0">${esc(a.original_name || "screenshot")}</a>
            ${del}
          </div>
          <img src="${esc(a.url)}" alt="" />
        </div>`;
      }
      return `<div class="flex items-center justify-between gap-2">
          <a href="${esc(a.url)}" target="_blank" rel="noopener" class="text-xs text-[#93c5fd] no-underline hover:underline min-w-0">${esc(a.original_name || "file")}</a>
          ${del}
        </div>`;
    }).join("");

    const stepItems = (steps || []).map((st) => {
      const done = !!st.done_at;
      const borderCls = done ? "border-[#3ecf8e]/20" : "border-[#253041]";
      const bgCls     = done ? "bg-emerald-400/10" : "bg-slate-400/5";
      const circleCls = done ? "bg-[#3ecf8e]/20 text-[#3ecf8e]" : "bg-[#121a26] text-[#a9b4c4] border border-[#253041]";
      const badges = [
        st.requires_evidence ? `<span class="flex-none text-[10px] font-bold uppercase tracking-wide text-[#f0c14b] bg-[#f0c14b]/10 border border-[#f0c14b]/20 px-1.5 py-0.5 rounded">bukti wajib</span>` : "",
        st.optional ? `<span class="flex-none text-[10px] font-bold uppercase tracking-wide text-[#a9b4c4] bg-[#a9b4c4]/10 border border-[#a9b4c4]/20 px-1.5 py-0.5 rounded">opsional</span>` : "",
      ].filter(Boolean).join("");
      const uploadedImgs = (st.attachments || []).map((a) =>
        `<div class="mt-2">
           <div class="flex items-center justify-between gap-2 mb-1">
             <p class="text-[10px] text-[#a9b4c4] m-0 truncate">${esc(a.original_name)}</p>
             <button type="button" class="del-step-att ${BtnDanger} text-[10px] py-0.5 px-2 shrink-0" data-att-id="${esc(String(a.id))}">Hapus</button>
           </div>
           <img src="${esc(a.url)}" class="max-w-full rounded-lg border border-[#253041]" style="max-height:200px;object-fit:contain" alt="" />
         </div>`
      ).join("");
      return `<li data-id="${st.id}" data-req-ev="${st.requires_evidence}"
          class="flex gap-4 p-4 rounded-xl border ${borderCls} ${bgCls} mb-3 last:mb-0">
          <!-- Circle -->
          <div class="flex-none pt-0.5">
            <div class="w-7 h-7 rounded-full flex items-center justify-center text-xs font-bold ${circleCls}">
              ${done ? "✓" : st.step_no}
            </div>
          </div>
          <!-- Body -->
          <div class="flex-1 min-w-0">
            <div class="flex items-start justify-between gap-2 mb-1">
              <span class="text-sm font-medium text-[#e7ecf3]">${esc(st.title)}</span>
              <div class="flex gap-1 flex-wrap justify-end">${badges}</div>
            </div>
            <p class="text-xs text-[#a9b4c4] mb-3">${done ? `Selesai ${esc(fmtWIB(st.done_at))} · ${esc(st.done_by || "")}` : "Belum selesai"}</p>

            <!-- Fields -->
            <div class="space-y-2.5 pt-3 border-t border-[#253041]">
              <div>
                <label class="${LB}">URL Bukti</label>
                <input type="url" class="ev ${IC} text-xs" placeholder="https://…" value="${esc(st.evidence_url || "")}" />
                <div class="step-ev-warn"></div>
              </div>
              <div class="grid grid-cols-2 gap-2.5">
                <div>
                  <label class="${LB}">Catatan</label>
                  <input type="text" class="notes ${IC} text-xs" placeholder="Ringkas temuan…" value="${esc(st.notes || "")}" />
                </div>
                <div>
                  <label class="${LB}">Diselesaikan oleh</label>
                  <input type="text" class="who ${IC} text-xs" placeholder="nama / @handle" value="${esc(st.done_by || "")}" />
                </div>
              </div>
              <!-- Upload bukti gambar -->
              <div>
                <label class="${LB}">Upload bukti <span class="text-[#a9b4c4] normal-case tracking-normal font-normal">(opsional, bisa beberapa)</span></label>
                <input type="file" accept="image/jpeg,image/png,image/gif" multiple class="step-upload-file hidden" />
                <button type="button" class="step-upload-btn ${BtnGhost} text-xs">+ Upload gambar</button>
                <div class="step-upload-progress text-xs text-[#a9b4c4] mt-1 hidden">Mengunggah…</div>
              </div>
              ${uploadedImgs ? `<div class="step-imgs space-y-2">${uploadedImgs}</div>` : `<div class="step-imgs space-y-2"></div>`}
              <div class="flex gap-2 pt-1">
                <button type="button" class="save-step ${BtnSm}" data-done="1">✓ Tandai selesai</button>
                <button type="button" class="save-step ${BtnGhost}" data-done="0">Batal selesai</button>
              </div>
            </div>
          </div>
        </li>`;
    }).join("");

    const rca = normalizeRca(c.rca);

    const auditHtml = (audit || []).length
      ? `<div class="space-y-3">
          ${(audit || []).map((e) => `
          <div class="flex gap-3 items-start">
            <span class="text-[10px] font-mono text-[#a9b4c4] whitespace-nowrap mt-0.5 w-32 shrink-0">
              ${esc(fmtWIB(e.created_at))}
            </span>
            <div class="flex flex-wrap gap-1.5 items-baseline">
              <span class="inline-flex px-1.5 py-0.5 rounded text-[10px] font-bold uppercase tracking-wide ${actionColor(e.action)}">${esc(e.action)}</span>
              ${e.actor  ? `<span class="text-xs font-medium text-[#e7ecf3]">${esc(e.actor)}</span>` : ""}
              ${e.detail ? `<span class="text-xs text-[#a9b4c4]">${esc(e.detail)}</span>` : ""}
            </div>
          </div>`).join("")}
        </div>`
      : `<p class="text-xs text-[#a9b4c4]">Belum ada aktivitas.</p>`;

    app.innerHTML = `
      <!-- RCA header (selaras PDF / HTML export) -->
      <div class="mb-4">
        <a href="#/" class="inline-flex items-center gap-1 text-xs text-[#a9b4c4] hover:text-[#e7ecf3] no-underline mb-3 transition-colors">← Kembali</a>
        <div class="rounded-2xl border border-[#253041] bg-[#121a26] overflow-hidden relative shadow-[0_10px_30px_rgba(0,0,0,0.35)]">
          <div class="absolute bottom-0 left-0 right-0 h-1 bg-[#60a5fa]"></div>
          <div class="p-5 pb-6 pr-4 sm:pr-44">
            <p class="text-[11px] font-bold uppercase tracking-[0.06em] text-[#93c5fd] m-0 mb-1.5">Konkon TechOps · Root Cause Analysis (RCA)</p>
            <p class="text-sm font-mono font-bold text-[#93c5fd] m-0 mb-1">${esc(c.case_id)}</p>
            <h1 class="text-xl font-extrabold text-[#e7ecf3] m-0 mb-3 leading-snug">${esc(c.title)}</h1>
            <div class="flex flex-wrap gap-2 items-center">
              ${badge(c.status)}
              ${c.severity ? `<span class="inline-flex items-center px-2.5 py-1 rounded-md text-[10px] font-bold uppercase tracking-wide bg-sky-400/10 text-sky-200 border border-sky-400/20">${esc(c.severity)}</span>` : ""}
              ${c.service ? `<span class="inline-flex items-center px-2.5 py-1 rounded-md text-[10px] font-bold uppercase tracking-wide bg-slate-400/10 text-slate-200 border border-slate-400/20">${esc(c.service)}</span>` : ""}
              ${c.sop_slug ? `<span class="text-xs text-[#a9b4c4]">SOP: <span class="font-mono">${esc(c.sop_slug)}</span>${c.sop_version != null ? ` <span class="text-[10px]">v${esc(c.sop_version)}</span>` : ""}</span>` : ""}
            </div>
            ${c.summary ? `<p class="text-sm text-[#a9b4c4] mt-4 mb-0 leading-relaxed border-t border-[#253041] pt-4"><span class="text-[11px] font-bold uppercase tracking-wider text-[#93c5fd] block mb-1.5">Ringkasan eksekutif</span>${esc(c.summary)}</p>` : ""}
          </div>
          <div class="absolute top-4 right-4 flex flex-col sm:flex-row gap-2">
            <button type="button" class="case-export-btn ${BtnSm}" data-format="md">MD</button>
            <button type="button" class="case-export-btn ${BtnSm}" data-format="html">HTML</button>
            <button type="button" class="case-export-btn ${BtnSm}" data-format="pdf">PDF</button>
          </div>
        </div>
      </div>

      <!-- Metadata -->
      <div class="${Card}">
        <h2 class="text-xs font-semibold uppercase tracking-widest text-[#a9b4c4] mb-4 m-0">Informasi insiden</h2>
        ${caseMetaHtml(c)}
      </div>

      <!-- RCA (masuk PDF / HTML / MD export) -->
      <div class="${Card}">
        <h2 class="text-xs font-semibold uppercase tracking-widest text-[#a9b4c4] mb-1 m-0">Analisis RCA</h2>
        <div class="space-y-4">
          <div>
            <label class="${LB}">Root cause <span class="text-[#a9b4c4] normal-case tracking-normal font-normal">(opsional)</span></label>
            <textarea id="rca-root" rows="3" class="${TA}">${esc(rca.root_cause)}</textarea>
          </div>
          <div>
            <label class="${LB}">Timeline insiden <span class="text-[#a9b4c4] normal-case tracking-normal font-normal">(opsional)</span></label>
            <div id="timelineRows" class="space-y-2"></div>
            <div class="pt-1">
              <button type="button" id="addTimelineRow" class="${BtnGhost} text-xs">+ Tambah kejadian</button>
            </div>
            <p class="text-[11px] text-[#a9b4c4] mt-2 mb-0">Isi via kalender + jam. Sistem akan simpan sebagai <span class="font-mono">ddmmyy hh:mm:ss | detail kejadian</span>.</p>
          </div>
          <div class="space-y-2">
            <h3 class="text-xs font-semibold uppercase tracking-widest text-[#a9b4c4] m-0">Analisis (opsional)</h3>
            <div id="whyRows" class="space-y-3"></div>
            <button type="button" id="addWhyRow" class="${BtnGhost} text-xs">+ Tambah analisis</button>
          </div>
          <div>
            <label class="${LB}">Temuan utama <span class="text-[#a9b4c4] normal-case tracking-normal font-normal">(opsional)</span></label>
            <textarea id="rca-contrib" rows="3" class="${TA}">${esc(rca.contributing_factors)}</textarea>
          </div>
          <div>
            <label class="${LB}">Perbaikan yang diterapkan <span class="text-[#a9b4c4] normal-case tracking-normal font-normal">(opsional)</span></label>
            <textarea id="rca-corrective" rows="3" class="${TA}">${esc(rca.corrective_actions)}</textarea>
          </div>
          <div>
            <label class="${LB}">Pencegahan & tindak lanjut <span class="text-[#a9b4c4] normal-case tracking-normal font-normal">(opsional)</span></label>
            <textarea id="rca-preventive" rows="3" class="${TA}">${esc(rca.preventive_actions)}</textarea>
          </div>
          <div class="space-y-2">
            <label class="${LB}">Action items <span class="text-[#a9b4c4] normal-case tracking-normal font-normal">(opsional)</span></label>
            <div id="actionRows" class="space-y-2"></div>
            <button type="button" id="addActionRow" class="${BtnGhost} text-xs">+ Tambah actions</button>
          </div>
          <div>
            <label class="${LB}">Celah deteksi <span class="text-[#a9b4c4] normal-case tracking-normal font-normal">(opsional)</span></label>
            <textarea id="rca-detect-gap" rows="3" class="${TA}">${esc(rca.detection_gap)}</textarea>
          </div>
          <div class="flex gap-2 pt-1">
            <button type="button" id="generateRca" class="${BtnGhost}">Generate draft RCA</button>
            <button type="button" id="saveRca" class="${BtnSm}">Simpan analisis RCA</button>
          </div>
          <div id="rcaMsg" class="text-[#a9b4c4] text-xs mt-2 min-h-[1rem]"></div>
          <div id="rcaErr" class="${Err}"></div>
        </div>
      </div>

      ${attHtml ? `
      <div class="${Card}">
        <h2 class="text-xs font-semibold uppercase tracking-widest text-[#a9b4c4] mb-4 m-0">Lampiran</h2>
        ${attHtml}
      </div>` : ""}

      <!-- SOP Triage -->
      <div class="${Card}">
        <h2 class="text-xs font-semibold uppercase tracking-widest text-[#a9b4c4] mb-1 m-0">Triage SOP</h2>
        <p class="text-xs text-[#a9b4c4] mb-3">Pilih SOP dan terapkan — checklist akan di-reset sesuai prosedur baru.</p>
        <div class="flex gap-2">
          <select id="sopPick" class="${SC} text-xs">${sopOpts}</select>
          <button type="button" id="applySop" class="${BtnSm} shrink-0">Terapkan</button>
        </div>
        <div id="sopErr" class="${Err}"></div>
      </div>

      <!-- Checklist intentionally hidden -->

      <!-- Audit -->
      <div class="${Card}">
        <h2 class="text-xs font-semibold uppercase tracking-widest text-[#a9b4c4] mb-4 m-0">Riwayat aktivitas</h2>
        ${auditHtml}
      </div>`;

    bindTimelineEditor(rca.incident_timeline);
    bindRcaDynamicRows(rca);

    app.querySelectorAll(".case-export-btn").forEach((btn) => {
      btn.addEventListener("click", () => exportCaseSummary(id, btn.getAttribute("data-format") || "pdf"));
    });

    document.getElementById("saveRca").addEventListener("click", async () => {
      const errEl = document.getElementById("rcaErr");
      const msgEl = document.getElementById("rcaMsg");
      errEl.textContent = "";
      msgEl.textContent = "";
      const body = rcaPayloadFromDom();
      try {
        await api(`/api/cases/${encodeURIComponent(id)}/rca`, {
          method: "PATCH",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(body),
        });
        await renderCase(id);
      } catch (e) {
        const msg = typeof e.body === "string" ? e.body : e.body?.error || e.message;
        errEl.textContent = msg || "Gagal menyimpan";
      }
    });

    document.getElementById("generateRca").addEventListener("click", async (ev) => {
      const btn = ev.currentTarget;
      const errEl = document.getElementById("rcaErr");
      const msgEl = document.getElementById("rcaMsg");
      errEl.textContent = "";
      msgEl.textContent = "";
      const prev = btn.textContent;
      btn.disabled = true;
      btn.textContent = "Menyusun draft…";
      try {
        await flushChecklistFieldsToServer(id);
        const out = await api(`/api/cases/${encodeURIComponent(id)}/rca/draft`, { method: "POST" });
        applyRcaDraftToDom(out?.rca || {});
        const source = out?.source || "heuristic";
        const confidence = out?.confidence ? ` (${out.confidence})` : "";
        msgEl.textContent = `Draft RCA ditambahkan dari ${source}${confidence}. Field yang sudah terisi tidak diubah; review lalu simpan.`;
      } catch (e) {
        const msg = typeof e.body === "string" ? e.body : e.body?.error || e.message;
        errEl.textContent = msg || "Gagal membuat draft RCA";
      } finally {
        btn.disabled = false;
        btn.textContent = prev;
      }
    });

    app.querySelectorAll(".del-case-att").forEach((btn) => {
      btn.addEventListener("click", async () => {
        if (!(await confirmDialog("Hapus lampiran ini dari case? File akan dihapus permanen."))) return;
        const attId = btn.getAttribute("data-att-id");
        try {
          const res = await fetch(`/api/cases/${encodeURIComponent(id)}/attachments/${encodeURIComponent(attId)}`, { method: "DELETE" });
          if (!res.ok) throw new Error((await res.text()) || res.statusText);
          await renderCase(id);
        } catch (e) {
          alert(e.message || "Gagal menghapus lampiran");
        }
      });
    });

    app.querySelectorAll(".del-step-att").forEach((btn) => {
      btn.addEventListener("click", async () => {
        if (!(await confirmDialog("Hapus gambar bukti dari langkah ini?"))) return;
        const attId = btn.getAttribute("data-att-id");
        try {
          const res = await fetch(`/api/cases/${encodeURIComponent(id)}/attachments/${encodeURIComponent(attId)}`, { method: "DELETE" });
          if (!res.ok) throw new Error((await res.text()) || res.statusText);
          await renderCase(id);
        } catch (e) {
          alert(e.message || "Gagal menghapus gambar");
        }
      });
    });

    app.querySelectorAll(".save-step").forEach((btn) => {
      btn.addEventListener("click", async () => {
        const li  = btn.closest("li");
        const sid = li.getAttribute("data-id");
        const done = btn.getAttribute("data-done") === "1";
        const body = {
          done,
          evidence_url: li.querySelector(".ev").value || null,
          notes:        li.querySelector(".notes").value || null,
          done_by:      li.querySelector(".who").value || null,
        };
        try {
          await api(`/api/cases/${encodeURIComponent(id)}/steps/${sid}`, {
            method: "PATCH",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify(body),
          });
          await renderCase(id);
        } catch (e) {
          alert(e.body?.errors?.join?.("\n") || e.message);
        }
      });
    });

    app.querySelectorAll(".step-upload-btn").forEach((btn) => {
      btn.addEventListener("click", () => {
        btn.closest("li").querySelector(".step-upload-file").click();
      });
    });

    app.querySelectorAll(".step-upload-file").forEach((inp) => {
      inp.addEventListener("change", async () => {
        if (!inp.files?.length) return;
        const li       = inp.closest("li");
        const sid      = li.getAttribute("data-id");
        const progress = li.querySelector(".step-upload-progress");
        progress.classList.remove("hidden");
        const files = Array.from(inp.files);
        try {
          for (const file of files) {
            const form = new FormData();
            form.append("file", file);
            const res = await fetch(`/api/cases/${encodeURIComponent(id)}/steps/${sid}/attachment`, {
              method: "POST", body: form,
            });
            if (!res.ok) {
              const t = await res.text().catch(() => "");
              throw new Error(t || res.statusText);
            }
          }
          await renderCase(id);
        } catch (e) {
          alert(e.message || "Gagal upload");
        } finally {
          progress.classList.add("hidden");
          inp.value = "";
        }
      });
    });

    document.getElementById("applySop")?.addEventListener("click", async () => {
      const slug  = document.getElementById("sopPick").value;
      const errEl = document.getElementById("sopErr");
      errEl.textContent = "";
      try {
        await api(`/api/cases/${encodeURIComponent(id)}/sop`, {
          method: "PATCH",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ slug }),
        });
        await renderCase(id);
      } catch (e) {
        errEl.textContent = JSON.stringify(e.body || e.message);
      }
    });

    document.getElementById("closeCase")?.addEventListener("click", async () => {
      const errEl = document.getElementById("closeErr");
      errEl.textContent = "";
      const ok = await confirmDialog("Tutup case ini sebagai resolved?\nPastikan semua checklist sudah selesai dan bukti terlampir.");
      if (!ok) return;
      try {
        await api(`/api/cases/${encodeURIComponent(id)}/close`, { method: "POST" });
        await renderCase(id);
      } catch (e) {
        errEl.textContent = (e.body?.errors?.join?.("\n")) || e.message;
      }
    });

    attachStepChecker();
  } catch (e) {
    app.innerHTML = `<div class="${Card}"><p class="text-sm text-[#f07178]">Tidak ditemukan atau error: ${esc(e.message)}</p></div>`;
  }
}

// ── SOP Management ────────────────────────────────────────────────────────────
async function renderSOPs() {
  app.innerHTML = `<div class="${Card}"><p class="text-sm text-[#a9b4c4]">Memuat…</p></div>`;
  try {
    const sops = await api("/api/sops");
    const rows = (sops || []).map((s) => `
      <div class="flex items-center gap-4 py-4 border-t border-[#253041] group first:border-0">
        <div class="flex-1 min-w-0">
          <div class="flex items-center gap-2 mb-0.5">
            <code class="text-xs font-mono font-semibold text-[#2563eb]">${esc(s.slug)}</code>
            <span class="text-[10px] text-[#a9b4c4] bg-[#121a26] border border-[#253041] px-1.5 py-0.5 rounded-md font-mono">v${esc(s.version)}</span>
            ${s.owner ? `<span class="text-[10px] text-[#a9b4c4]">· ${esc(s.owner)}</span>` : ""}
          </div>
          <p class="text-sm font-medium text-[#e7ecf3] m-0">${esc(s.title)}</p>
        </div>
        <div class="flex gap-2 opacity-50 group-hover:opacity-100 transition-opacity">
          <button class="sop-edit ${BtnSm}" data-slug="${esc(s.slug)}">Edit</button>
          <button class="sop-del ${BtnDanger}" data-slug="${esc(s.slug)}">Hapus</button>
        </div>
      </div>`).join("");

    app.innerHTML = `
      <div class="${Card}">
        <div class="flex items-center justify-between mb-5">
          <div>
            <h2 class="text-base font-semibold text-[#e7ecf3] m-0 mb-1">Kelola SOP</h2>
            <p class="text-xs text-[#a9b4c4] m-0">Buat, edit, atau hapus Standard Operating Procedure.</p>
          </div>
          <button type="button" id="sopNewBtn" class="${Btn}">+ Tambah SOP</button>
        </div>
        <div>${rows || `<p class="text-sm text-[#a9b4c4] py-4 text-center">Belum ada SOP. Buat yang pertama!</p>`}</div>
      </div>
      <div id="sopFormArea"></div>`;

    app.querySelectorAll(".sop-edit").forEach((btn) => {
      btn.addEventListener("click", () => renderSOPForm(btn.dataset.slug));
    });
    app.querySelectorAll(".sop-del").forEach((btn) => {
      btn.addEventListener("click", async () => {
        const slug = btn.dataset.slug;
        const ok = await confirmDialog(`Hapus SOP "${slug}"?\nCase yang sudah menggunakan SOP ini tidak terpengaruh.`);
        if (!ok) return;
        try {
          await api(`/api/sops/${encodeURIComponent(slug)}`, { method: "DELETE" });
          await renderSOPs();
        } catch (e) { alert(e.message); }
      });
    });
    document.getElementById("sopNewBtn").addEventListener("click", () => renderSOPForm(null));
  } catch (e) {
    app.innerHTML = `<div class="${Card}"><p class="text-sm text-[#f07178]">Gagal memuat: ${esc(e.message)}</p></div>`;
  }
}

async function renderSOPForm(slug) {
  const area = document.getElementById("sopFormArea");
  if (!area) return;

  let existing = null;
  if (slug) {
    try { existing = await api(`/api/sops/${encodeURIComponent(slug)}`); } catch {}
  }

  const steps = existing?.steps || [{ title: "", requires_evidence: false }];
  const stepsHtml = steps.map((st) => stepEditorRow(st.title, st.requires_evidence)).join("");

  area.innerHTML = `
    <div class="${Card}">
      <h3 class="text-sm font-semibold text-[#e7ecf3] m-0 mb-5">${slug ? `Edit SOP: <code class="text-[#93c5fd] font-mono">${esc(slug)}</code>` : "SOP Baru"}</h3>

      <div class="space-y-4">
        <div class="grid grid-cols-2 gap-4">
          <div>
            <label class="${LB}">Slug <span class="normal-case tracking-normal font-normal text-[#a9b4c4]/60">(unik, huruf kecil)</span></label>
            <input id="sopSlug" ${slug ? "readonly class='opacity-60 cursor-not-allowed'" : ""} placeholder="mis. payment-latency"
              value="${esc(existing?.slug || "")}" class="${IC}" />
          </div>
          <div>
            <label class="${LB}">Owner / Tim</label>
            <input id="sopOwner" placeholder="platform, payment-team, …" value="${esc(existing?.owner || "")}" class="${IC}" />
          </div>
        </div>
        <div>
          <label class="${LB}">Judul SOP</label>
          <input id="sopTitle" placeholder="Nama prosedur" value="${esc(existing?.title || "")}" class="${IC}" />
        </div>

        <div>
          <div class="flex items-center justify-between mb-2">
            <label class="${LB} mb-0">Langkah-langkah</label>
            <span class="text-[10px] text-[#a9b4c4]">centang = bukti wajib</span>
          </div>
          <ul id="stepEditor" class="space-y-2 p-0 m-0 list-none">${stepsHtml}</ul>
          <button type="button" id="addStep" class="${BtnGhost} mt-2 text-xs">+ Tambah langkah</button>
        </div>
      </div>

      <div class="flex items-center gap-3 mt-6 pt-5 border-t border-[#253041]">
        <button type="button" id="sopSave" class="${Btn}">${slug ? "Simpan perubahan" : "Buat SOP"}</button>
        <button type="button" id="sopCancel" class="${BtnGhost}">Batal</button>
      </div>
      <div id="sopFormErr" class="${Err}"></div>
    </div>`;

  area.querySelectorAll(".step-remove").forEach(attachRemoveStep);

  document.getElementById("addStep").addEventListener("click", () => {
    const ul  = document.getElementById("stepEditor");
    const li  = document.createElement("li");
    li.innerHTML = stepEditorRow("", false);
    ul.appendChild(li);
    attachRemoveStep(li.querySelector(".step-remove"));
    li.querySelector(".step-title").focus();
  });

  document.getElementById("sopCancel").addEventListener("click", () => { area.innerHTML = ""; });

  document.getElementById("sopSave").addEventListener("click", async () => {
    const errEl    = document.getElementById("sopFormErr");
    errEl.textContent = "";
    const sopSlug  = document.getElementById("sopSlug").value.trim();
    const sopTitle = document.getElementById("sopTitle").value.trim();
    const sopOwner = document.getElementById("sopOwner").value.trim();
    const steps    = [];
    document.querySelectorAll("#stepEditor li").forEach((li) => {
      const title = li.querySelector(".step-title").value.trim();
      const req   = li.querySelector(".step-req").checked;
      if (title) steps.push({ title, requires_evidence: req });
    });
    if (!sopSlug || !sopTitle) { errEl.textContent = "Slug dan judul wajib diisi."; return; }
    if (!steps.length)          { errEl.textContent = "Minimal satu langkah diperlukan."; return; }
    const body = { slug: sopSlug, title: sopTitle, owner: sopOwner, steps };
    try {
      if (slug) {
        await api(`/api/sops/${encodeURIComponent(slug)}`, {
          method: "PATCH", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body),
        });
      } else {
        await api("/api/sops", {
          method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body),
        });
      }
      await renderSOPs();
    } catch (e) {
      errEl.textContent = typeof e.body === "string" ? e.body : e.message;
    }
  });
}

function stepEditorRow(title, requiresEvidence) {
  return `<li class="flex gap-2 items-center">
    <input type="text" class="step-title flex-1 px-3 py-2 bg-[#0f172a] border border-[#253041] rounded-lg text-sm text-[#e7ecf3] placeholder:text-[#7e8aa0] focus:outline-none focus:border-[#60a5fa] transition-colors" placeholder="Judul langkah…" value="${esc(title)}" />
    <label class="flex items-center gap-1.5 text-[10px] font-medium text-[#a9b4c4] whitespace-nowrap cursor-pointer select-none">
      <input type="checkbox" class="step-req accent-[#60a5fa]" ${requiresEvidence ? "checked" : ""} />
      bukti?
    </label>
    <button type="button" class="step-remove w-7 h-7 flex items-center justify-center text-[#f07178] bg-[#f07178]/10 hover:bg-[#f07178]/20 rounded-lg text-xs transition-colors border-0 cursor-pointer flex-none">✕</button>
  </li>`;
}

function attachRemoveStep(btn) {
  btn.addEventListener("click", () => {
    const editor = document.getElementById("stepEditor");
    if (editor && editor.querySelectorAll("li").length > 1) btn.closest("li").remove();
  });
}

// ── Form Checker ──────────────────────────────────────────────────────────────
/** @param {{ getScreensFilled?: () => boolean }} [opts] */
function attachFormChecker(opts = {}) {
  const getScreensFilled = typeof opts.getScreensFilled === "function"
    ? opts.getScreensFilled
    : () => {
        const el = document.querySelector(`#f [name="screenshots"]`);
        return !!(el && el.files && el.files.length);
      };

  const fields = [
    { name: "title",    label: "Judul",     required: true  },
    { name: "service",  label: "Layanan",   required: false },
    { name: "severity", label: "Severity",  required: false },
    { name: "reporter", label: "Pelapor",   required: false },
    { name: "summary",  label: "Ringkasan", required: false },
  ];

  function isFilled(el) {
    if (!el) return false;
    if (el.type === "file") return el.files && el.files.length > 0;
    return (el.value || "").trim() !== "";
  }

  function checkForm() {
    let filledCount = 0;
    const missing = [];
    fields.forEach(({ name, label, required }) => {
      const el       = document.querySelector(`#f [name="${name}"]`);
      const statusEl = document.getElementById(`fst-${name}`);
      if (!el || !statusEl) return;
      const ok = isFilled(el);
      if (ok) {
        filledCount++;
        statusEl.textContent = "✓";
        statusEl.className   = "field-status ok";
      } else {
        statusEl.textContent = required ? "⚠" : "○";
        statusEl.className   = `field-status ${required ? "warn" : "empty"}`;
        if (required) missing.push(label);
      }
    });

    const ssOk = getScreensFilled();
    const ssStatus = document.getElementById("fst-screenshots");
    if (ssStatus) {
      if (ssOk) {
        filledCount++;
        ssStatus.textContent = "✓";
        ssStatus.className   = "field-status ok";
      } else {
        ssStatus.textContent = "○";
        ssStatus.className   = "field-status empty";
      }
    }

    const totalSlots = fields.length + 1;
    saveDraft();
    const statusDiv = document.getElementById("form-checker-status");
    if (!statusDiv) return;
    if (missing.length === 0 && filledCount === totalSlots) {
      statusDiv.className   = "form-checker all-ok";
      statusDiv.textContent = "✓ Semua field terisi — form siap dikirim";
    } else if (missing.length === 0) {
      statusDiv.className   = "form-checker partial-ok";
      statusDiv.textContent = `✓ Field wajib lengkap (${filledCount}/${totalSlots} slot terisi, termasuk gambar opsional)`;
    } else {
      statusDiv.className   = "form-checker has-warn";
      statusDiv.textContent = `⚠ Belum diisi: ${missing.join(", ")}`;
    }
  }

  document.querySelectorAll("#f input, #f textarea, #f select").forEach((el) => {
    el.addEventListener("input",  checkForm);
    el.addEventListener("change", checkForm);
  });
  checkForm();
}

// ── Step Checker ──────────────────────────────────────────────────────────────
function attachStepChecker() {
  document.querySelectorAll("#steps li").forEach((li) => {
    const reqEv   = li.getAttribute("data-req-ev") === "true";
    const ev      = li.querySelector(".ev");
    const who     = li.querySelector(".who");
    const saveBtn = li.querySelector(".save-step[data-done='1']");
    const warnDiv = li.querySelector(".step-ev-warn");

    function checkStep() {
      const evVal  = (ev?.value  || "").trim();
      const whoVal = (who?.value || "").trim();
      if (warnDiv) {
        // URL bukti tidak wajib; bukti bisa berupa upload gambar atau catatan internal.
        warnDiv.textContent = "";
      }
      if (saveBtn) {
        // Syarat UI untuk menandai selesai hanya "diselesaikan oleh"; URL bukti opsional.
        saveBtn.classList.toggle("step-ready", !!whoVal);
      }
    }

    li.querySelectorAll(".ev, .who, .notes").forEach((inp) => inp.addEventListener("input", checkStep));
    checkStep();
  });
}

// ── Router ────────────────────────────────────────────────────────────────────
function route() {
  const h = (location.hash || "#/").slice(1);
  if (h === "/" || h === "") return renderHome();
  if (h === "/new")  return renderNew();
  if (h === "/sops") return renderSOPs();
  const m = h.match(/^\/case\/(.+)$/);
  if (m) return renderCase(decodeURIComponent(m[1]));
  renderHome();
}

window.addEventListener("hashchange", route);
route();
