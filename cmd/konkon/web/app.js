const app = document.getElementById("app");

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
    try {
      data = JSON.parse(text);
    } catch {
      data = text;
    }
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
  let cls = "badge-open";
  if (s === "needs_triage") cls = "badge-triage";
  if (s === "resolved") cls = "badge-done";
  return `<span class="badge ${cls}">${esc(status)}</span>`;
}

/** Renders case metadata: same keys as GET /api/cases/{id} (snake_case). */
function caseMetaHtml(c) {
  const rows = [
    ["Layanan", "service", c.service],
    ["Severity", "severity", c.severity],
    ["Pelapor", "reporter", c.reporter],
    ["Dibuat", "created_at", c.created_at],
    ["Diperbarui", "updated_at", c.updated_at],
  ];
  if (c.resolved_at) rows.push(["Selesai", "resolved_at", c.resolved_at]);
  return rows
    .map(
      ([label, key, val]) => `
    <div class="meta-row">
      <span class="meta-label">${esc(label)}</span>
      <span class="meta-key">${esc(key)}</span>
      <span class="meta-val">${esc(val || "—")}</span>
    </div>`
    )
    .join("");
}

async function renderHome() {
  app.innerHTML = `<p class="muted">Memuat…</p>`;
  try {
    const cases = await api("/api/cases");
    const rows = (cases || [])
      .map(
        (c) => `
      <tr>
        <td><a href="#/case/${esc(c.case_id)}">${esc(c.case_id)}</a></td>
        <td>${esc(c.title)}</td>
        <td>${esc(c.service || "—")}</td>
        <td>${esc(c.severity || "—")}</td>
        <td class="muted">${esc(c.reporter || "—")}</td>
        <td>${badge(c.status)}</td>
        <td class="muted">${esc((c.sop_slug || "—") + "")}</td>
      </tr>`
      )
      .join("");
    app.innerHTML = `
      <div class="card">
        <h2>Case terbaru</h2>
        <table>
          <thead><tr><th>ID</th><th>Judul</th><th>Layanan</th><th>Sev</th><th>Pelapor</th><th>Status</th><th>SOP</th></tr></thead>
          <tbody>${rows || `<tr><td colspan="7" class="muted">Belum ada case</td></tr>`}</tbody>
        </table>
      </div>`;
  } catch (e) {
    app.innerHTML = `<div class="card error">Gagal memuat: ${esc(e.message)}</div>`;
  }
}

async function renderNew() {
  app.innerHTML = `
    <div class="card">
      <h2>Case baru</h2>
      <p class="muted">Isi form; sistem memetakan SOP via rule (payment / timeout / latency / fallback generik).</p>
      <form id="f">
        <label>Judul * <code class="hint">title</code></label>
        <input name="title" required placeholder="mis. Timeout checkout" />
        <label>Ringkasan <code class="hint">summary</code></label>
        <textarea name="summary" placeholder="Gejala singkat"></textarea>
        <label>Layanan <code class="hint">service</code></label>
        <input name="service" placeholder="payment, auth, …" />
        <label>Severity <code class="hint">severity</code></label>
        <select name="severity">
          <option value="">—</option>
          <option>P1</option><option>P2</option><option>P3</option><option>P4</option>
        </select>
        <label>Pelapor <code class="hint">reporter</code></label>
        <input name="reporter" placeholder="nama / handle" />
        <label>Berkas screenshot <code class="hint">screenshot</code></label>
        <input type="file" name="screenshot" accept="image/*" />
        <p class="muted form-hint">POST <code>/api/cases</code> memakai <code>multipart/form-data</code> dengan nama field di atas.</p>
        <div><button type="submit">Buat case</button></div>
        <div id="err" class="error"></div>
      </form>
    </div>`;
  document.getElementById("f").addEventListener("submit", async (ev) => {
    ev.preventDefault();
    const errEl = document.getElementById("err");
    errEl.textContent = "";
    const fd = new FormData(ev.target);
    try {
      const res = await fetch("/api/cases", { method: "POST", body: fd });
      const data = await res.json().catch(() => ({}));
      if (!res.ok) {
        errEl.textContent = data.error || res.statusText;
        return;
      }
      location.hash = `#/case/${data.case_id}`;
    } catch (e) {
      errEl.textContent = e.message;
    }
  });
}

async function renderCase(id) {
  app.innerHTML = `<p class="muted">Memuat…</p>`;
  try {
    const [c, steps, atts, sops] = await Promise.all([
      api(`/api/cases/${encodeURIComponent(id)}`),
      api(`/api/cases/${encodeURIComponent(id)}/steps`),
      api(`/api/cases/${encodeURIComponent(id)}/attachments`).catch(() => []),
      api("/api/sops").catch(() => []),
    ]);
    const sopOpts = (sops || [])
      .map((s) => `<option value="${esc(s.slug)}">${esc(s.slug)} — ${esc(s.title)}</option>`)
      .join("");
    const attHtml = (atts || [])
      .map((a) => {
        if ((a.kind || "") === "screenshot" && a.url) {
          return `<div class="attach"><a href="${esc(a.url)}" target="_blank" rel="noopener">${esc(
            a.original_name || "screenshot"
          )}</a><br/><img src="${esc(a.url)}" alt="" /></div>`;
        }
        return `<div><a href="${esc(a.url)}" target="_blank" rel="noopener">${esc(a.original_name || "file")}</a></div>`;
      })
      .join("");
    const stepItems = (steps || [])
      .map((st) => {
        const done = !!st.done_at;
        return `<li data-id="${st.id}">
          <strong>${st.step_no}. ${esc(st.title)}</strong>
          ${st.requires_evidence ? ` <span class="muted">(bukti wajib)</span>` : ""}
          <div class="step-meta">${done ? `Selesai ${esc(st.done_at)} — ${esc(st.done_by || "")}` : "Belum"}</div>
          <div class="step-fields">
            <label>URL bukti <code class="hint">evidence_url</code></label>
            <input type="url" class="ev" name="evidence_url" autocomplete="off" placeholder="https://…" value="${esc(st.evidence_url || "")}" />
            <label>Catatan <code class="hint">notes</code></label>
            <input type="text" class="notes" name="notes" placeholder="Ringkas tindakan / temuan" value="${esc(st.notes || "")}" />
            <label>Diselesaikan oleh <code class="hint">done_by</code></label>
            <input type="text" class="who" name="done_by" placeholder="Nama / handle" value="${esc(st.done_by || "")}" />
            <div class="step-actions">
              <button type="button" class="secondary save-step" data-done="1">Tandai selesai</button>
              <button type="button" class="secondary save-step" data-done="0">Batal selesai</button>
            </div>
          </div>
        </li>`;
      })
      .join("");
    app.innerHTML = `
      <div class="card">
        <p><a href="#/">← Kembali</a></p>
        <h2>${esc(c.case_id)} — ${esc(c.title)}</h2>
        <p class="muted summary">${esc(c.summary || "")}</p>
        <p>${badge(c.status)} &nbsp; <span class="muted">SOP: ${esc(c.sop_slug || "—")} ${c.sop_version != null ? `v${esc(c.sop_version)}` : ""}</span></p>
        <div class="case-meta">
          <h3 class="meta-heading">Metadata <span class="hint">API: GET /api/cases/{id}</span></h3>
          ${caseMetaHtml(c)}
        </div>
        <p><a class="btn secondary" href="/api/cases/${encodeURIComponent(id)}/summary?format=md" target="_blank" rel="noopener">Ringkasan Markdown</a>
        <a class="btn secondary" href="/api/cases/${encodeURIComponent(id)}/summary?format=html" target="_blank" rel="noopener">Ringkasan HTML</a></p>
      </div>
      ${attHtml ? `<div class="card"><h2>Lampiran</h2>${attHtml}</div>` : ""}
      <div class="card">
        <h2>Triage SOP</h2>
        <p class="muted">Jika status needs_triage atau ingin ganti prosedur, pilih SOP lalu terapkan (checklist di-reset).</p>
        <select id="sopPick">${sopOpts}</select>
        <button type="button" id="applySop" class="secondary">Terapkan SOP</button>
        <div id="sopErr" class="error"></div>
      </div>
      <div class="card">
        <h2>Checklist</h2>
        <p class="muted form-hint">Perbarui via <code>PATCH /api/cases/{caseId}/steps/{stepId}</code> — body JSON: <code>done</code>, <code>evidence_url</code>, <code>notes</code>, <code>done_by</code>.</p>
        <ol class="steps" id="steps">${stepItems || `<li class="muted">Belum ada langkah (pilih SOP)</li>`}</ol>
        <button type="button" id="closeCase">Tutup case (resolved)</button>
        <div id="closeErr" class="error"></div>
      </div>`;

    app.querySelectorAll(".save-step").forEach((btn) => {
      btn.addEventListener("click", async () => {
        const li = btn.closest("li");
        const sid = li.getAttribute("data-id");
        const done = btn.getAttribute("data-done") === "1";
        const body = {
          done,
          evidence_url: li.querySelector(".ev").value || null,
          notes: li.querySelector(".notes").value || null,
          done_by: li.querySelector(".who").value || null,
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

    document.getElementById("applySop")?.addEventListener("click", async () => {
      const slug = document.getElementById("sopPick").value;
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
      try {
        await api(`/api/cases/${encodeURIComponent(id)}/close`, { method: "POST" });
        await renderCase(id);
      } catch (e) {
        errEl.textContent = (e.body && e.body.errors && e.body.errors.join("\n")) || e.message;
      }
    });
  } catch (e) {
    app.innerHTML = `<div class="card error">Tidak ditemukan atau error: ${esc(e.message)}</div>`;
  }
}

function route() {
  const h = (location.hash || "#/").slice(1);
  if (h === "/" || h === "") return renderHome();
  if (h === "/new") return renderNew();
  const m = h.match(/^\/case\/(.+)$/);
  if (m) return renderCase(decodeURIComponent(m[1]));
  renderHome();
}

window.addEventListener("hashchange", route);
route();
