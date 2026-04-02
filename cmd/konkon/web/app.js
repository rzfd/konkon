const app = document.getElementById("app");

// ── Design tokens ─────────────────────────────────────────────────────────────
const IC  = "w-full px-3 py-2 bg-[#0c1016] border border-[#2a3545] rounded-lg text-sm text-[#e7ecf3] focus:outline-none focus:border-[#3d8bfd] focus:ring-1 focus:ring-[#3d8bfd]/20 transition-colors";
const TA  = IC + " resize-y min-h-[80px]";
const SC  = "w-full px-3 py-2 bg-[#0c1016] border border-[#2a3545] rounded-lg text-sm text-[#e7ecf3] focus:outline-none focus:border-[#3d8bfd] transition-colors";
const LB  = "block text-[10px] font-semibold uppercase tracking-widest text-[#8b98a8] mb-1.5";
const Btn = "inline-flex items-center gap-1.5 px-4 py-2 bg-[#3d8bfd] text-white text-sm font-semibold rounded-lg hover:bg-[#2d78ed] transition-colors border-0 cursor-pointer";
const BtnSm = "inline-flex items-center gap-1 px-3 py-1.5 bg-[#2a3545] text-[#e7ecf3] text-xs font-medium rounded-lg hover:bg-[#334155] transition-colors border-0 cursor-pointer";
const BtnGhost = "inline-flex items-center gap-1 px-3 py-1.5 text-[#8b98a8] text-xs font-medium rounded-lg hover:text-[#e7ecf3] hover:bg-[#2a3545] transition-colors border-0 cursor-pointer bg-transparent";
const BtnDanger = "inline-flex items-center gap-1 px-3 py-1.5 text-[#f07178] text-xs font-medium rounded-lg hover:bg-[#f07178]/10 transition-colors border border-[#f07178]/20 bg-transparent cursor-pointer";
const Card = "bg-[#1a2332] border border-[#2a3545] rounded-xl p-6 mb-4";
const Err  = "text-[#f07178] text-xs mt-2 min-h-[1rem]";

// ── Utils ─────────────────────────────────────────────────────────────────────
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
    needs_triage: "bg-[#f0c14b]/15 text-[#fcd34d] border border-[#f0c14b]/20",
    open:         "bg-[#3d8bfd]/15 text-[#93c5fd] border border-[#3d8bfd]/20",
    resolved:     "bg-[#3ecf8e]/15 text-[#6ee7b7] border border-[#3ecf8e]/20",
  };
  const cls = map[s] || map.open;
  return `<span class="inline-flex items-center px-2 py-0.5 rounded-md text-[10px] font-bold uppercase tracking-wide ${cls}">${esc(status)}</span>`;
}

function actionColor(action) {
  const map = {
    case_created: "bg-[#3d8bfd]/15 text-[#93c5fd]",
    sop_assigned: "bg-[#8b5cf6]/15 text-[#c4b5fd]",
    step_done:    "bg-[#3ecf8e]/15 text-[#6ee7b7]",
    step_undone:  "bg-[#8b98a8]/15 text-[#8b98a8]",
    case_closed:  "bg-[#f0c14b]/15 text-[#fde68a]",
  };
  return map[action] || "bg-[#2a3545] text-[#8b98a8]";
}

function caseMetaHtml(c) {
  const rows = [
    ["Layanan",    c.service],
    ["Severity",   c.severity],
    ["Pelapor",    c.reporter],
    ["Dibuat",     c.created_at?.slice(0, 16).replace("T", " ")],
    ["Diperbarui", c.updated_at?.slice(0, 16).replace("T", " ")],
  ];
  if (c.resolved_at) rows.push(["Selesai", c.resolved_at?.slice(0, 16).replace("T", " ")]);
  return `<dl class="grid grid-cols-2 sm:grid-cols-3 gap-x-6 gap-y-4">
    ${rows.map(([label, val]) => `
      <div>
        <dt class="text-[10px] font-semibold uppercase tracking-widest text-[#8b98a8] mb-0.5">${esc(label)}</dt>
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
      <div class="bg-[#1a2332] border border-[#2a3545] rounded-2xl p-6 max-w-sm w-[90vw] shadow-2xl">
        <p class="text-sm text-[#e7ecf3] leading-relaxed mb-6">${esc(message)}</p>
        <div class="flex justify-end gap-2">
          <button class="modal-cancel ${BtnGhost}">Batal</button>
          <button class="modal-ok px-4 py-2 text-sm font-semibold text-white bg-[#f07178] rounded-lg hover:bg-[#e05c6b] transition-colors border-0 cursor-pointer">Ya, lanjutkan</button>
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
  app.innerHTML = `<div class="${Card}"><p class="text-sm text-[#8b98a8]">Memuat…</p></div>`;
  const { page, status, severity, search } = homeState;
  const params = new URLSearchParams({ page, limit: 20 });
  if (status)   params.set("status", status);
  if (severity) params.set("severity", severity);
  if (search)   params.set("search", search);

  try {
    const cases = await api(`/api/cases?${params}`);
    const rows = (cases || []).map((c) => `
      <tr class="border-t border-[#2a3545] hover:bg-[#3d8bfd]/5 transition-colors cursor-pointer group">
        <td class="py-3 pr-5 align-middle">
          <a href="#/case/${esc(c.case_id)}" class="text-xs font-mono font-semibold text-[#3d8bfd] no-underline group-hover:text-[#60a5fa]">${esc(c.case_id)}</a>
        </td>
        <td class="py-3 pr-5 align-middle text-sm text-[#e7ecf3] max-w-[240px] truncate">${esc(c.title)}</td>
        <td class="py-3 pr-5 align-middle text-xs text-[#8b98a8]">${esc(c.service || "—")}</td>
        <td class="py-3 pr-5 align-middle">
          ${c.severity ? `<span class="text-[10px] font-bold text-[#8b98a8] bg-[#2a3545] px-1.5 py-0.5 rounded">${esc(c.severity)}</span>` : "<span class='text-[#8b98a8]'>—</span>"}
        </td>
        <td class="py-3 pr-5 align-middle text-xs text-[#8b98a8]">${esc(c.reporter || "—")}</td>
        <td class="py-3 pr-5 align-middle">${badge(c.status)}</td>
        <td class="py-3 align-middle text-xs font-mono text-[#8b98a8]">${esc(c.sop_slug || "—")}</td>
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
                <th class="text-left text-[10px] font-semibold uppercase tracking-widest text-[#8b98a8] pb-3 pr-5">ID</th>
                <th class="text-left text-[10px] font-semibold uppercase tracking-widest text-[#8b98a8] pb-3 pr-5">Judul</th>
                <th class="text-left text-[10px] font-semibold uppercase tracking-widest text-[#8b98a8] pb-3 pr-5">Layanan</th>
                <th class="text-left text-[10px] font-semibold uppercase tracking-widest text-[#8b98a8] pb-3 pr-5">Sev</th>
                <th class="text-left text-[10px] font-semibold uppercase tracking-widest text-[#8b98a8] pb-3 pr-5">Pelapor</th>
                <th class="text-left text-[10px] font-semibold uppercase tracking-widest text-[#8b98a8] pb-3 pr-5">Status</th>
                <th class="text-left text-[10px] font-semibold uppercase tracking-widest text-[#8b98a8] pb-3">SOP</th>
              </tr>
            </thead>
            <tbody>${rows || `<tr><td colspan="7" class="py-8 text-center text-sm text-[#8b98a8]">Tidak ada case ditemukan</td></tr>`}</tbody>
          </table>
        </div>

        <!-- Pagination -->
        <div class="flex items-center justify-center gap-3 mt-5 select-none">
          <button id="pgPrev" ${page <= 1 ? "disabled" : ""}
            class="!mt-0 !w-9 !h-9 !p-0 !rounded-full !bg-[#2a3545] !text-[#e7ecf3] !text-xl !font-bold hover:!bg-[#3d8bfd] disabled:!opacity-30 disabled:!cursor-not-allowed !transition-colors !duration-150 flex items-center justify-center border-0 cursor-pointer">
            ‹
          </button>
          <span class="text-xs text-[#8b98a8] min-w-[5rem] text-center tabular-nums">Halaman ${page}</span>
          <button id="pgNext" ${(cases || []).length < 20 ? "disabled" : ""}
            class="!mt-0 !w-9 !h-9 !p-0 !rounded-full !bg-[#2a3545] !text-[#e7ecf3] !text-xl !font-bold hover:!bg-[#3d8bfd] disabled:!opacity-30 disabled:!cursor-not-allowed !transition-colors !duration-150 flex items-center justify-center border-0 cursor-pointer">
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
  app.innerHTML = `
    <div class="${Card}">
      <div class="mb-6">
        <h2 class="text-base font-semibold text-[#e7ecf3] m-0 mb-1">Buat case baru</h2>
        <p class="text-xs text-[#8b98a8] m-0">Sistem memetakan SOP secara otomatis via rule berdasarkan service, keyword, dan severity.</p>
      </div>

      <form id="f" class="space-y-4">
        <div>
          <label class="${LB}">Judul <span class="text-[#f07178] normal-case tracking-normal font-normal">*</span><span class="field-status" id="fst-title"></span></label>
          <input name="title" required placeholder="mis. Timeout checkout payment" class="${IC}" />
        </div>
        <div>
          <label class="${LB}">Ringkasan<span class="field-status" id="fst-summary"></span></label>
          <textarea name="summary" placeholder="Gejala singkat, dampak, dan konteks…" class="${TA}"></textarea>
        </div>
        <div class="grid grid-cols-2 gap-4">
          <div>
            <label class="${LB}">Layanan<span class="field-status" id="fst-service"></span></label>
            <input name="service" placeholder="payment, auth, …" class="${IC}" />
          </div>
          <div>
            <label class="${LB}">Severity<span class="field-status" id="fst-severity"></span></label>
            <select name="severity" class="${SC}">
              <option value="">— pilih —</option>
              <option>P1</option><option>P2</option><option>P3</option><option>P4</option>
            </select>
          </div>
        </div>
        <div>
          <label class="${LB}">Pelapor<span class="field-status" id="fst-reporter"></span></label>
          <input name="reporter" placeholder="nama / @handle" class="${IC}" />
        </div>
        <div>
          <label class="${LB}">Screenshot<span class="field-status" id="fst-screenshot"></span></label>
          <input type="file" name="screenshot" accept="image/*"
            class="w-full text-xs text-[#8b98a8] file:mr-3 file:px-3 file:py-1.5 file:rounded-lg file:border-0 file:text-xs file:font-medium file:bg-[#2a3545] file:text-[#e7ecf3] file:cursor-pointer hover:file:bg-[#334155] file:transition-colors" />
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
  attachFormChecker();

  document.getElementById("f").addEventListener("submit", async (ev) => {
    ev.preventDefault();
    const errEl = document.getElementById("err");
    errEl.textContent = "";
    const fd = new FormData(ev.target);
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
    attachFormChecker();
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
  app.innerHTML = `<div class="${Card}"><p class="text-sm text-[#8b98a8]">Memuat…</p></div>`;
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
      if ((a.kind || "") === "screenshot" && a.url) {
        return `<div class="attach">
          <a href="${esc(a.url)}" target="_blank" rel="noopener" class="text-xs text-[#3d8bfd] no-underline hover:underline">${esc(a.original_name || "screenshot")}</a>
          <br/><img src="${esc(a.url)}" alt="" />
        </div>`;
      }
      return `<div><a href="${esc(a.url)}" target="_blank" rel="noopener" class="text-xs text-[#3d8bfd] no-underline hover:underline">${esc(a.original_name || "file")}</a></div>`;
    }).join("");

    const stepItems = (steps || []).map((st) => {
      const done = !!st.done_at;
      const borderCls = done ? "border-[#3ecf8e]/20" : "border-[#2a3545]";
      const bgCls     = done ? "bg-[#3ecf8e]/5" : "bg-[#0f1419]/40";
      const circleCls = done ? "bg-[#3ecf8e]/20 text-[#3ecf8e]" : "bg-[#2a3545] text-[#8b98a8]";
      const badges = [
        st.requires_evidence ? `<span class="flex-none text-[10px] font-bold uppercase tracking-wide text-[#f0c14b] bg-[#f0c14b]/10 border border-[#f0c14b]/20 px-1.5 py-0.5 rounded">bukti wajib</span>` : "",
        st.optional ? `<span class="flex-none text-[10px] font-bold uppercase tracking-wide text-[#8b98a8] bg-[#8b98a8]/10 border border-[#8b98a8]/20 px-1.5 py-0.5 rounded">opsional</span>` : "",
      ].filter(Boolean).join("");
      const uploadedImgs = (st.attachments || []).map(a =>
        `<div class="mt-2">
           <p class="text-[10px] text-[#8b98a8] mb-1">${esc(a.original_name)}</p>
           <img src="${esc(a.url)}" class="max-w-full rounded-lg border border-[#2a3545]" style="max-height:200px;object-fit:contain" />
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
            <p class="text-xs text-[#8b98a8] mb-3">${done ? `Selesai ${esc(st.done_at?.slice(0,16).replace("T"," "))} · ${esc(st.done_by || "")}` : "Belum selesai"}</p>

            <!-- Fields -->
            <div class="space-y-2.5 pt-3 border-t border-[#2a3545]/60">
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
                <label class="${LB}">Upload Bukti (JPG/PNG)</label>
                <input type="file" accept="image/jpeg,image/png,image/gif" class="step-upload-file hidden" />
                <button type="button" class="step-upload-btn ${BtnGhost} text-xs">+ Upload Gambar</button>
                <div class="step-upload-progress text-xs text-[#8b98a8] mt-1 hidden">Mengunggah…</div>
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

    const auditHtml = (audit || []).length
      ? `<div class="space-y-3">
          ${(audit || []).map((e) => `
          <div class="flex gap-3 items-start">
            <span class="text-[10px] font-mono text-[#8b98a8] whitespace-nowrap mt-0.5 w-32 shrink-0">
              ${esc(e.created_at?.slice(0,16).replace("T"," "))}
            </span>
            <div class="flex flex-wrap gap-1.5 items-baseline">
              <span class="inline-flex px-1.5 py-0.5 rounded text-[10px] font-bold uppercase tracking-wide ${actionColor(e.action)}">${esc(e.action)}</span>
              ${e.actor  ? `<span class="text-xs font-medium text-[#e7ecf3]">${esc(e.actor)}</span>` : ""}
              ${e.detail ? `<span class="text-xs text-[#8b98a8]">${esc(e.detail)}</span>` : ""}
            </div>
          </div>`).join("")}
        </div>`
      : `<p class="text-xs text-[#8b98a8]">Belum ada aktivitas.</p>`;

    app.innerHTML = `
      <!-- Header -->
      <div class="${Card}">
        <a href="#/" class="inline-flex items-center gap-1 text-xs text-[#8b98a8] hover:text-[#e7ecf3] no-underline mb-4 transition-colors">← Kembali</a>
        <div class="flex items-start justify-between gap-4">
          <div class="min-w-0">
            <p class="text-xs font-mono text-[#8b98a8] mb-1">${esc(c.case_id)}</p>
            <h1 class="text-lg font-bold text-[#e7ecf3] m-0 mb-2 leading-snug">${esc(c.title)}</h1>
            <div class="flex items-center gap-2 flex-wrap">
              ${badge(c.status)}
              ${c.sop_slug ? `<span class="text-xs text-[#8b98a8]">SOP: <span class="font-mono text-[#8b98a8]">${esc(c.sop_slug)}</span>${c.sop_version != null ? ` <span class="text-[10px]">v${esc(c.sop_version)}</span>` : ""}</span>` : ""}
            </div>
          </div>
          <div class="flex gap-2 shrink-0">
            <a class="${BtnSm} no-underline" href="/api/cases/${encodeURIComponent(id)}/summary?format=md" target="_blank" rel="noopener">MD</a>
            <a class="${BtnSm} no-underline" href="/api/cases/${encodeURIComponent(id)}/summary?format=html" target="_blank" rel="noopener">HTML</a>
            <a class="${BtnSm} no-underline" href="/api/cases/${encodeURIComponent(id)}/summary?format=pdf" target="_blank" rel="noopener">PDF</a>
          </div>
        </div>
        ${c.summary ? `<p class="text-sm text-[#8b98a8] mt-3 mb-0">${esc(c.summary)}</p>` : ""}
      </div>

      <!-- Metadata -->
      <div class="${Card}">
        <h2 class="text-xs font-semibold uppercase tracking-widest text-[#8b98a8] mb-4 m-0">Metadata</h2>
        ${caseMetaHtml(c)}
      </div>

      ${attHtml ? `
      <div class="${Card}">
        <h2 class="text-xs font-semibold uppercase tracking-widest text-[#8b98a8] mb-4 m-0">Lampiran</h2>
        ${attHtml}
      </div>` : ""}

      <!-- SOP Triage -->
      <div class="${Card}">
        <h2 class="text-xs font-semibold uppercase tracking-widest text-[#8b98a8] mb-1 m-0">Triage SOP</h2>
        <p class="text-xs text-[#8b98a8] mb-3">Pilih SOP dan terapkan — checklist akan di-reset sesuai prosedur baru.</p>
        <div class="flex gap-2">
          <select id="sopPick" class="${SC} text-xs">${sopOpts}</select>
          <button type="button" id="applySop" class="${BtnSm} shrink-0">Terapkan</button>
        </div>
        <div id="sopErr" class="${Err}"></div>
      </div>

      <!-- Checklist -->
      <div class="${Card}">
        <div class="flex items-center justify-between mb-4">
          <h2 class="text-xs font-semibold uppercase tracking-widest text-[#8b98a8] m-0">Checklist</h2>
          ${steps?.length ? `<span class="text-xs text-[#8b98a8]">${steps.filter(s=>s.done_at).length}/${steps.length} selesai</span>` : ""}
        </div>
        <ol class="list-none p-0 m-0" id="steps">${stepItems || `<li class="text-sm text-[#8b98a8] py-4 text-center">Belum ada langkah — pilih SOP terlebih dahulu.</li>`}</ol>
        <div class="pt-4 border-t border-[#2a3545] mt-4">
          <button type="button" id="closeCase" class="${Btn}">Tutup case</button>
          <div id="closeErr" class="${Err}"></div>
        </div>
      </div>

      <!-- Audit -->
      <div class="${Card}">
        <h2 class="text-xs font-semibold uppercase tracking-widest text-[#8b98a8] mb-4 m-0">Riwayat aktivitas</h2>
        ${auditHtml}
      </div>`;

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
        const form = new FormData();
        form.append("file", inp.files[0]);
        try {
          await fetch(`/api/cases/${encodeURIComponent(id)}/steps/${sid}/attachment`, {
            method: "POST", body: form,
          });
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
  app.innerHTML = `<div class="${Card}"><p class="text-sm text-[#8b98a8]">Memuat…</p></div>`;
  try {
    const sops = await api("/api/sops");
    const rows = (sops || []).map((s) => `
      <div class="flex items-center gap-4 py-4 border-t border-[#2a3545] group first:border-0">
        <div class="flex-1 min-w-0">
          <div class="flex items-center gap-2 mb-0.5">
            <code class="text-xs font-mono font-semibold text-[#3d8bfd]">${esc(s.slug)}</code>
            <span class="text-[10px] text-[#8b98a8] bg-[#2a3545] px-1.5 py-0.5 rounded-md font-mono">v${esc(s.version)}</span>
            ${s.owner ? `<span class="text-[10px] text-[#8b98a8]">· ${esc(s.owner)}</span>` : ""}
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
            <p class="text-xs text-[#8b98a8] m-0">Buat, edit, atau hapus Standard Operating Procedure.</p>
          </div>
          <button type="button" id="sopNewBtn" class="${Btn}">+ Tambah SOP</button>
        </div>
        <div>${rows || `<p class="text-sm text-[#8b98a8] py-4 text-center">Belum ada SOP. Buat yang pertama!</p>`}</div>
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
      <h3 class="text-sm font-semibold text-[#e7ecf3] m-0 mb-5">${slug ? `Edit SOP: <code class="text-[#3d8bfd] font-mono">${esc(slug)}</code>` : "SOP Baru"}</h3>

      <div class="space-y-4">
        <div class="grid grid-cols-2 gap-4">
          <div>
            <label class="${LB}">Slug <span class="normal-case tracking-normal font-normal text-[#8b98a8]/60">(unik, huruf kecil)</span></label>
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
            <span class="text-[10px] text-[#8b98a8]">centang = bukti wajib</span>
          </div>
          <ul id="stepEditor" class="space-y-2 p-0 m-0 list-none">${stepsHtml}</ul>
          <button type="button" id="addStep" class="${BtnGhost} mt-2 text-xs">+ Tambah langkah</button>
        </div>
      </div>

      <div class="flex items-center gap-3 mt-6 pt-5 border-t border-[#2a3545]">
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
    <input type="text" class="step-title flex-1 px-3 py-2 bg-[#0c1016] border border-[#2a3545] rounded-lg text-sm text-[#e7ecf3] focus:outline-none focus:border-[#3d8bfd] transition-colors" placeholder="Judul langkah…" value="${esc(title)}" />
    <label class="flex items-center gap-1.5 text-[10px] font-medium text-[#8b98a8] whitespace-nowrap cursor-pointer select-none">
      <input type="checkbox" class="step-req accent-[#3d8bfd]" ${requiresEvidence ? "checked" : ""} />
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
function attachFormChecker() {
  const fields = [
    { name: "title",      label: "Judul",      required: true  },
    { name: "service",    label: "Layanan",    required: true  },
    { name: "severity",   label: "Severity",   required: true  },
    { name: "reporter",   label: "Pelapor",    required: true  },
    { name: "summary",    label: "Ringkasan",  required: false },
    { name: "screenshot", label: "Screenshot", required: false },
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
    saveDraft();
    const statusDiv = document.getElementById("form-checker-status");
    if (!statusDiv) return;
    if (missing.length === 0 && filledCount === fields.length) {
      statusDiv.className   = "form-checker all-ok";
      statusDiv.textContent = "✓ Semua field terisi — form siap dikirim";
    } else if (missing.length === 0) {
      statusDiv.className   = "form-checker partial-ok";
      statusDiv.textContent = `✓ Field wajib sudah diisi (${filledCount}/${fields.length} field terisi)`;
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
        warnDiv.textContent = (reqEv && !evVal) ? "⚠ URL bukti wajib diisi untuk langkah ini" : "";
      }
      if (saveBtn) {
        saveBtn.classList.toggle("step-ready", (!reqEv || !!evVal) && !!whoVal);
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
