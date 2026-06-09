/* ═══════════════════════════════════════════════════════════════
   resume forge — production-ready app
   ═══════════════════════════════════════════════════════════════ */

document.addEventListener("DOMContentLoaded", () => {
  const apiBase =
    window.location.hostname === "localhost" || window.location.hostname === "127.0.0.1"
      ? "http://localhost:3000"
      : "https://backend.resumeforge.zephex.in";

  // ─── STATE ──────────────────────────────────────────────
  const state = {
    template: "terminal",
    genCount: parseInt(localStorage.getItem("resumeForgeCount") || "0"),
    isGenerating: false,
    lastHTML: "",
    lastData: null,
    lastAIContent: null,
  };

  // ─── RESUME HISTORY ─────────────────────────────────────
  let resumeHistory = [];
  try {
    const stored = localStorage.getItem("resumeForgeHistory");
    if (stored) resumeHistory = JSON.parse(stored);
  } catch {}

  function saveHistory(entry) {
    resumeHistory.unshift(entry);
    // Keep last 20
    if (resumeHistory.length > 20) resumeHistory = resumeHistory.slice(0, 20);
    localStorage.setItem("resumeForgeHistory", JSON.stringify(resumeHistory));
    renderHistory();
    updateGenCountDisplay();
  }

  function deleteHistoryItem(id) {
    resumeHistory = resumeHistory.filter((e) => e.id !== id);
    localStorage.setItem("resumeForgeHistory", JSON.stringify(resumeHistory));
    renderHistory();
  }

  // ─── DOM REFS ───────────────────────────────────────────
  const form = document.getElementById("resume-form");
  const genBtn = document.getElementById("gen-btn");
  const genStatus = document.getElementById("gen-status");
  const preview = document.getElementById("resume-preview");
  const navCount = document.getElementById("nav-gen-count");
  const expContainer = document.getElementById("exp-container");
  const eduContainer = document.getElementById("edu-container");
  const projContainer = document.getElementById("proj-container");
  const achievementContainer = document.getElementById("achievement-container");
  const certContainer = document.getElementById("cert-container");
  const historyList = document.getElementById("history-list");
  const emptyHistory = document.getElementById("empty-history");
  const historyPanel = document.getElementById("history-panel");

  // ─── MODALS ─────────────────────────────────────────────
  const templatesModal = document.getElementById("templates-modal");
  const aboutModal = document.getElementById("about-modal");

  document
    .getElementById("nav-templates-btn")
    .addEventListener("click", () => templatesModal.classList.remove("hidden"));
  document
    .getElementById("modal-close")
    .addEventListener("click", () => templatesModal.classList.add("hidden"));
  templatesModal
    .querySelector(".modal-backdrop")
    .addEventListener("click", () => templatesModal.classList.add("hidden"));

  document
    .getElementById("nav-about-btn")
    .addEventListener("click", () => aboutModal.classList.remove("hidden"));
  document
    .querySelector(".about-close")
    .addEventListener("click", () => aboutModal.classList.add("hidden"));
  aboutModal
    .querySelector(".modal-backdrop")
    .addEventListener("click", () => aboutModal.classList.add("hidden"));

  // ─── TEMPLATE SELECTION ─────────────────────────────────
  async function loadTemplatePreview(tmpl) {
    if (state.lastHTML) {
      // If a resume has already been generated in this session, don't overwrite it with mock data
      return;
    }
    try {
      const res = await fetch(`${apiBase}/api/preview?template=${tmpl}`);
      if (res.ok) {
        const html = await res.text();
        preview.innerHTML = html;
        fitResumeToPage();
      }
    } catch (err) {
      console.error("Failed to load mock template preview:", err);
    }
  }

  document.querySelectorAll(".tmpl-option").forEach((opt) => {
    opt.addEventListener("click", () => {
      document
        .querySelectorAll(".tmpl-option")
        .forEach((o) => o.classList.remove("selected"));
      opt.classList.add("selected");
      const tmpl = opt.dataset.tmpl;
      state.template = tmpl;
      document
        .querySelectorAll('input[name="template"]')
        .forEach((r) => (r.checked = r.value === tmpl));
      loadTemplatePreview(tmpl);
    });
  });

  // Template preview cards in modal
  document.querySelectorAll(".tmpl-preview-card").forEach((card) => {
    card.addEventListener("click", () => {
      const tmpl = card.dataset.tmpl;
      state.template = tmpl;
      document
        .querySelectorAll(".tmpl-option")
        .forEach((o) =>
          o.classList.toggle("selected", o.dataset.tmpl === tmpl),
        );
      document
        .querySelectorAll('input[name="template"]')
        .forEach((r) => (r.checked = r.value === tmpl));
      templatesModal.classList.add("hidden");
      loadTemplatePreview(tmpl);
    });
  });

  // ─── TOAST SYSTEM ───────────────────────────────────────
  function showToast(msg, type = "info") {
    const existing = document.querySelector(".toast-msg");
    if (existing) existing.remove();
    const toast = document.createElement("div");
    toast.className = `toast-msg toast-${type}`;
    toast.textContent = msg;
    document.body.appendChild(toast);
    setTimeout(() => toast.classList.add("show"), 10);
    setTimeout(() => {
      toast.classList.remove("show");
      setTimeout(() => toast.remove(), 300);
    }, 4000);
  }

  // ─── EXPERIENCE ENTRIES ─────────────────────────────────
  function createExpEntry(data, idx) {
    const entryId = Math.random().toString(36).slice(2, 9);
    const div = document.createElement("div");
    div.className = "exp-entry";
    div.innerHTML = `
            <div class="exp-header">
                <span class="exp-counter">#${expContainer.children.length + 1}</span>
                <button type="button" class="remove-entry" title="remove" aria-label="Remove entry">✕</button>
            </div>
            <div class="form-row">
                <div class="form-group flex-1">
                    <label for="exp-company-${entryId}">company</label>
                    <input type="text" class="exp-company" id="exp-company-${entryId}" value="${data?.company || ""}" placeholder="Google">
                </div>
                <div class="form-group flex-1">
                    <label for="exp-role-${entryId}">role</label>
                    <input type="text" class="exp-role" id="exp-role-${entryId}" value="${data?.role || ""}" placeholder="Software Engineer">
                </div>
            </div>
            <div class="form-row">
                <div class="form-group flex-1">
                    <label for="exp-from-${entryId}">from</label>
                    <input type="text" class="exp-from" id="exp-from-${entryId}" value="${data?.from || ""}" placeholder="2024">
                </div>
                <div class="form-group flex-1">
                    <label for="exp-to-${entryId}">to</label>
                    <input type="text" class="exp-to" id="exp-to-${entryId}" value="${data?.to || ""}" placeholder="Present">
                </div>
            </div>
            <div class="form-group">
                <label for="exp-desc-${entryId}">description (optional — AI can expand)</label>
                <textarea class="exp-desc" id="exp-desc-${entryId}" rows="2" placeholder="What you did...">${data?.desc || data?.description || ""}</textarea>
            </div>
        `;
    div.querySelector(".remove-entry").addEventListener("click", () => {
      div.remove();
      updateExpCounters();
    });
    return div;
  }

  function updateExpCounters() {
    document
      .querySelectorAll(".exp-counter")
      .forEach((c, i) => (c.textContent = `#${i + 1}`));
  }

  document.getElementById("add-exp").addEventListener("click", () => {
    expContainer.appendChild(
      createExpEntry(null, expContainer.children.length),
    );
    updateExpCounters();
  });

  // ─── EDUCATION ENTRIES ──────────────────────────────────
  function createEduEntry(data) {
    const entryId = Math.random().toString(36).slice(2, 9);
    const div = document.createElement("div");
    div.className = "edu-entry";
    div.innerHTML = `
            <div class="exp-header">
                <span class="edu-counter">#${eduContainer.children.length + 1}</span>
                <button type="button" class="remove-entry" title="remove" aria-label="Remove entry">✕</button>
            </div>
            <div class="form-row">
                <div class="form-group flex-1">
                    <label for="edu-school-${entryId}">school</label>
                    <input type="text" class="edu-school" id="edu-school-${entryId}" value="${data?.school || ""}" placeholder="University Name">
                </div>
                <div class="form-group flex-1">
                    <label for="edu-degree-${entryId}">degree</label>
                    <input type="text" class="edu-degree" id="edu-degree-${entryId}" value="${data?.degree || ""}" placeholder="B.Tech CSE">
                </div>
            </div>
            <div class="form-row">
                <div class="form-group flex-1">
                    <label for="edu-from-${entryId}">from</label>
                    <input type="text" class="edu-from" id="edu-from-${entryId}" value="${data?.from || ""}" placeholder="2024">
                </div>
                <div class="form-group flex-1">
                    <label for="edu-to-${entryId}">to</label>
                    <input type="text" class="edu-to" id="edu-to-${entryId}" value="${data?.to || ""}" placeholder="2028">
                </div>
            </div>
        `;
    div.querySelector(".remove-entry").addEventListener("click", () => {
      div.remove();
      updateEduCounters();
    });
    return div;
  }

  function updateEduCounters() {
    document
      .querySelectorAll(".edu-counter")
      .forEach((c, i) => (c.textContent = `#${i + 1}`));
  }

  document.getElementById("add-edu").addEventListener("click", () => {
    eduContainer.appendChild(createEduEntry());
    updateEduCounters();
  });

  // ─── PROJECTS ENTRIES ───────────────────────────────────
  function createProjEntry(data) {
    const entryId = Math.random().toString(36).slice(2, 9);
    const div = document.createElement("div");
    div.className = "proj-entry";
    div.innerHTML = `
            <div class="exp-header">
                <span class="proj-counter">#${projContainer.children.length + 1}</span>
                <button type="button" class="remove-entry" title="remove" aria-label="Remove entry">✕</button>
            </div>
            <div class="form-row">
                <div class="form-group flex-1">
                    <label for="proj-title-${entryId}">project title</label>
                    <input type="text" class="proj-title" id="proj-title-${entryId}" value="${data?.title || ""}" placeholder="Project Name">
                </div>
            </div>
            <div class="form-row">
                <div class="form-group flex-1">
                    <label for="proj-desc-${entryId}">project description (one liner)</label>
                    <input type="text" class="proj-desc" id="proj-desc-${entryId}" value="${data?.desc || ""}" placeholder="A brief one-sentence description of what the project does.">
                </div>
            </div>
            <div class="form-row">
                <div class="form-group flex-1">
                    <label for="proj-tech-${entryId}">tech stack</label>
                    <input type="text" class="proj-tech" id="proj-tech-${entryId}" value="${data?.tech || ""}" placeholder="Go, React, Tailwind CSS">
                </div>
            </div>
        `;
    div.querySelector(".remove-entry").addEventListener("click", () => {
      div.remove();
      updateProjCounters();
    });
    return div;
  }

  function updateProjCounters() {
    document
      .querySelectorAll(".proj-counter")
      .forEach((c, i) => (c.textContent = `#${i + 1}`));
  }

  document.getElementById("add-proj").addEventListener("click", () => {
    projContainer.appendChild(createProjEntry());
    updateProjCounters();
  });

  // ─── ACHIEVEMENTS ENTRIES ────────────────────────────────
  function createAchEntry(data) {
    const entryId = Math.random().toString(36).slice(2, 9);
    const div = document.createElement("div");
    div.className = "achievement-entry";
    div.innerHTML = `
            <div class="exp-header">
                <span class="ach-counter">#${achievementContainer.children.length + 1}</span>
                <button type="button" class="remove-entry" title="remove" aria-label="Remove entry">✕</button>
            </div>
            <div class="form-group">
                <label for="ach-title-${entryId}">title</label>
                <input type="text" class="ach-title" id="ach-title-${entryId}" value="${data?.title || ""}" placeholder="Dean's List / Top Performer Award">
            </div>
            <div class="form-row">
                <div class="form-group flex-1">
                    <label for="ach-date-${entryId}">date</label>
                    <input type="text" class="ach-date" id="ach-date-${entryId}" value="${data?.date || ""}" placeholder="2024">
                </div>
            </div>
            <div class="form-group">
                <label for="ach-desc-${entryId}">description</label>
                <textarea class="ach-desc" id="ach-desc-${entryId}" rows="2" placeholder="Brief description of the achievement...">${data?.desc || data?.description || ""}</textarea>
            </div>
        `;
    div.querySelector(".remove-entry").addEventListener("click", () => {
      div.remove();
      updateAchCounters();
    });
    return div;
  }

  function updateAchCounters() {
    document
      .querySelectorAll(".ach-counter")
      .forEach((c, i) => (c.textContent = `#${i + 1}`));
  }

  document.getElementById("add-ach").addEventListener("click", () => {
    achievementContainer.appendChild(createAchEntry());
    updateAchCounters();
  });

  // ─── CERTIFICATIONS ENTRIES ──────────────────────────────
  function createCertEntry(data) {
    const entryId = Math.random().toString(36).slice(2, 9);
    const div = document.createElement("div");
    div.className = "cert-entry";
    div.innerHTML = `
            <div class="exp-header">
                <span class="cert-counter">#${certContainer.children.length + 1}</span>
                <button type="button" class="remove-entry" title="remove" aria-label="Remove entry">✕</button>
            </div>
            <div class="form-row">
                <div class="form-group flex-1">
                    <label for="cert-title-${entryId}">title</label>
                    <input type="text" class="cert-title" id="cert-title-${entryId}" value="${data?.title || ""}" placeholder="AWS Certified Solutions Architect">
                </div>
                <div class="form-group flex-1">
                    <label for="cert-issuer-${entryId}">issuer</label>
                    <input type="text" class="cert-issuer" id="cert-issuer-${entryId}" value="${data?.issuer || ""}" placeholder="Amazon Web Services">
                </div>
            </div>
            <div class="form-row">
                <div class="form-group flex-1">
                    <label for="cert-date-${entryId}">date</label>
                    <input type="text" class="cert-date" id="cert-date-${entryId}" value="${data?.date || ""}" placeholder="2024">
                </div>
                <div class="form-group flex-1">
                    <label for="cert-link-${entryId}">link (optional)</label>
                    <input type="text" class="cert-link" id="cert-link-${entryId}" value="${data?.link || ""}" placeholder="https://credential.example.com">
                </div>
            </div>
        `;
    div.querySelector(".remove-entry").addEventListener("click", () => {
      div.remove();
      updateCertCounters();
    });
    return div;
  }

  function updateCertCounters() {
    document
      .querySelectorAll(".cert-counter")
      .forEach((c, i) => (c.textContent = `#${i + 1}`));
  }

  document.getElementById("add-cert").addEventListener("click", () => {
    certContainer.appendChild(createCertEntry());
    updateCertCounters();
  });

  // ─── HISTORY ────────────────────────────────────────────
  document.getElementById("nav-history-btn").addEventListener("click", () => {
    historyPanel.classList.toggle("open");
  });
  document.getElementById("close-history").addEventListener("click", () => {
    historyPanel.classList.remove("open");
  });

  function renderHistory() {
    if (!resumeHistory.length) {
      emptyHistory.classList.remove("hidden");
      historyList.innerHTML = "";
      return;
    }
    emptyHistory.classList.add("hidden");
    historyList.innerHTML = resumeHistory
      .map((item, idx) => {
        const d = new Date(item.timestamp);
        const dateStr = d.toLocaleDateString();
        const timeStr = d.toLocaleTimeString([], {
          hour: "2-digit",
          minute: "2-digit",
        });
        return `
            <div class="history-item">
                <div class="hi-info">
                    <div class="hi-name">${item.name || "Untitled"}</div>
                    <div class="hi-meta">${item.role || ""} · ${item.template || ""} · ${dateStr} ${timeStr}</div>
                </div>
                <div class="hi-actions">
                    <button class="hi-btn" id="load-history-${idx}" data-idx="${idx}" title="View" aria-label="View resume">👁</button>
                    <button class="hi-btn hi-del" data-id="${item.id}" title="Delete" aria-label="Delete resume">🗑</button>
                </div>
            </div>`;
      })
      .join("");

    // Load from history
    historyList.querySelectorAll('[id^="load-history-"]').forEach((btn) => {
      btn.addEventListener("click", () => {
        const idx = parseInt(btn.dataset.idx);
        const item = resumeHistory[idx];
        if (item) {
          state.lastHTML = item.html;
          state.lastData = item.formData;
          state.lastAIContent = item.aiContent;
          preview.innerHTML = item.html;
          fitResumeToPage();
          if (item.formData) {
            fillFormFromData(item.formData);
          }
          showToast("📄 Loaded from history", "info");
          historyPanel.classList.remove("open");
        }
      });
    });

    // Delete from history
    historyList.querySelectorAll(".hi-del").forEach((btn) => {
      btn.addEventListener("click", (e) => {
        e.stopPropagation();
        if (confirm("Delete this resume?")) {
          deleteHistoryItem(btn.dataset.id);
          showToast("🗑 Deleted", "info");
        }
      });
    });
  }

  // ─── SKILLS RENDER HELPER ──────────────────────────────
  function renderSkillsBlock(data, mode) {
    // Check if categorized skills exist
    const hasCategorized =
      data.skills_languages ||
      data.skills_frameworks ||
      data.skills_tools ||
      data.skills_databases ||
      data.skills_cloud;

    if (hasCategorized) {
      const categories = [
        { label: "Languages", value: data.skills_languages },
        { label: "Frameworks", value: data.skills_frameworks },
        { label: "Tools & Platforms", value: data.skills_tools },
        { label: "Databases", value: data.skills_databases },
        { label: "Cloud", value: data.skills_cloud },
      ].filter((c) => c.value);

      if (mode === "compact") {
        return `<div><div style="font-size:0.62rem;font-weight:600;color:#00cc9e;text-transform:uppercase;letter-spacing:1px;margin-bottom:0.15rem;">Skills</div>
                    ${categories
                      .map(
                        (cat) => `
                        <div style="margin-bottom:0.3rem;">
                            <div style="font-size:0.6rem;font-weight:600;color:#555;margin-bottom:0.05rem;">${cat.label}</div>
                            <div style="display:flex;flex-wrap:wrap;gap:0.3rem;">
                                ${cat.value
                                  .split(",")
                                  .map((s) => s.trim())
                                  .filter(Boolean)
                                  .map(
                                    (s) =>
                                      `<span style="font-size:0.6rem;padding:0.1rem 0.4rem;background:#f0f0f0;border-radius:2px;">${s}</span>`,
                                  )
                                  .join("")}
                            </div>
                        </div>
                    `,
                      )
                      .join("")}
                </div>`;
      } else if (mode === "terminal") {
        return `<div class="rt-section"><div class="rt-section-title">// skills</div>
                    ${categories
                      .map(
                        (cat) => `
                        <div style="margin-bottom:0.3rem;">
                            <div style="font-size:0.6rem;font-weight:600;color:#00cc9e;margin-bottom:0.05rem;">${cat.label}</div>
                            <div class="rt-skills">
                                ${cat.value
                                  .split(",")
                                  .map((s) => s.trim())
                                  .filter(Boolean)
                                  .map(
                                    (s) =>
                                      `<span class="rt-skill-tag">${s}</span>`,
                                  )
                                  .join("")}
                            </div>
                        </div>
                    `,
                      )
                      .join("")}
                </div>`;
      } else if (mode === "minimal") {
        return `<div style="margin-top:0.8rem;"><div class="rm-section-title">Skills</div>
                    ${categories
                      .map(
                        (cat) => `
                        <div style="margin-top:0.3rem;">
                            <div style="font-size:0.6rem;font-weight:600;color:#555;margin-bottom:0.05rem;">${cat.label}</div>
                            <div style="display:flex;flex-wrap:wrap;gap:0.3rem;">
                                ${cat.value
                                  .split(",")
                                  .map((s) => s.trim())
                                  .filter(Boolean)
                                  .map(
                                    (s) =>
                                      `<span style="font-size:0.6rem;padding:0.08rem 0.4rem;border:1px solid #ddd;border-radius:2px;">${s}</span>`,
                                  )
                                  .join("")}
                            </div>
                        </div>
                    `,
                      )
                      .join("")}
                </div>`;
      } else if (mode === "modern") {
        return `<div><div class="mod-section-title">Skills</div>
                    ${categories
                      .map(
                        (cat) => `
                        <div style="margin-bottom:0.3rem;">
                            <div style="font-size:0.58rem;font-weight:600;color:#00ffc8;margin-bottom:0.05rem;">${cat.label}</div>
                            ${cat.value
                              .split(",")
                              .map((s) => s.trim())
                              .filter(Boolean)
                              .map(
                                (s) =>
                                  `<div class="mod-skill-item">▸ ${s}</div>`,
                              )
                              .join("")}
                        </div>
                    `,
                      )
                      .join("")}
                </div>`;
      } else if (mode === "executive") {
        return `<div style="margin-bottom:0.8rem;">
                    <div style="font-size:0.62rem;font-weight:600;color:#c8a951;text-transform:uppercase;letter-spacing:1px;margin-bottom:0.15rem;">Core Competencies</div>
                    ${categories
                      .map(
                        (cat) => `
                        <div style="margin-bottom:0.3rem;">
                            <div style="font-size:0.6rem;font-weight:600;color:#8899aa;margin-bottom:0.05rem;">${cat.label}</div>
                            <div style="display:flex;flex-wrap:wrap;gap:0.25rem;">
                                ${cat.value
                                  .split(",")
                                  .map((s) => s.trim())
                                  .filter(Boolean)
                                  .map(
                                    (s) =>
                                      `<span style="font-size:0.58rem;padding:0.08rem 0.35rem;border:1px solid #d4c5a9;border-radius:2px;color:#2c3e50;">${s}</span>`,
                                  )
                                  .join("")}
                            </div>
                        </div>
                    `,
                      )
                      .join("")}
                </div>`;
      } else if (mode === "creative") {
        return `<div style="margin-bottom:0.8rem;">
                    <div style="font-size:0.62rem;font-weight:700;color:#ff6b9d;text-transform:uppercase;letter-spacing:1px;margin-bottom:0.15rem;">✦ Skills</div>
                    ${categories
                      .map(
                        (cat) => `
                        <div style="margin-bottom:0.3rem;">
                            <div style="font-size:0.6rem;font-weight:600;color:#00d4aa;margin-bottom:0.05rem;">${cat.label}</div>
                            <div style="display:flex;flex-wrap:wrap;gap:0.25rem;">
                                ${cat.value
                                  .split(",")
                                  .map((s) => s.trim())
                                  .filter(Boolean)
                                  .map(
                                    (s) =>
                                      `<span style="font-size:0.58rem;padding:0.08rem 0.4rem;background:rgba(255,107,157,0.1);border-radius:8px;color:#6b2fa0;font-weight:500;">${s}</span>`,
                                  )
                                  .join("")}
                            </div>
                        </div>
                    `,
                      )
                      .join("")}
                </div>`;
      } else if (mode === "timeline") {
        return `<div style="margin-bottom:0.8rem;">
                    <div style="font-size:0.62rem;font-weight:700;color:#2c3e50;text-transform:uppercase;letter-spacing:1px;border-bottom:2px solid #3498db;padding-bottom:0.15rem;margin-bottom:0.2rem;">Skills</div>
                    ${categories
                      .map(
                        (cat) => `
                        <div style="margin-bottom:0.3rem;padding-left:1rem;border-left:2px solid #e0e0e0;">
                            <div style="font-size:0.6rem;font-weight:600;color:#3498db;margin-bottom:0.05rem;">${cat.label}</div>
                            <div style="display:flex;flex-wrap:wrap;gap:0.25rem;">
                                ${cat.value
                                  .split(",")
                                  .map((s) => s.trim())
                                  .filter(Boolean)
                                  .map(
                                    (s) =>
                                      `<span style="font-size:0.58rem;padding:0.08rem 0.35rem;background:#f8f9fa;border-radius:2px;color:#555;">${s}</span>`,
                                  )
                                  .join("")}
                            </div>
                        </div>
                    `,
                      )
                      .join("")}
                </div>`;
      } else if (mode === "columns") {
        return `<div style="margin-bottom:0.8rem;">
                    <div style="font-size:0.62rem;font-weight:700;color:#2c3e50;text-transform:uppercase;letter-spacing:1px;border-bottom:2px solid #2c3e50;padding-bottom:0.15rem;margin-bottom:0.2rem;">Skills</div>
                    ${categories
                      .map(
                        (cat) => `
                        <div style="margin-bottom:0.3rem;">
                            <div style="font-size:0.6rem;font-weight:600;color:#555;margin-bottom:0.05rem;">${cat.label}</div>
                            <div style="display:flex;flex-wrap:wrap;gap:0.2rem;">
                                ${cat.value
                                  .split(",")
                                  .map((s) => s.trim())
                                  .filter(Boolean)
                                  .map(
                                    (s) =>
                                      `<span style="font-size:0.58rem;padding:0.05rem 0.3rem;background:#f0f0f0;border-radius:2px;color:#333;">${s}</span>`,
                                  )
                                  .join("")}
                            </div>
                        </div>
                    `,
                      )
                      .join("")}
                </div>`;
      }
    } else {
      // Fallback to flat skills
      const skills = data.skills
        ? data.skills
            .split(",")
            .map((s) => s.trim())
            .filter(Boolean)
        : [];
      if (!skills.length) return "";

      if (mode === "compact") {
        return `<div><div style="font-size:0.62rem;font-weight:600;color:#00cc9e;text-transform:uppercase;letter-spacing:1px;margin-bottom:0.15rem;">Skills</div><div style="display:flex;flex-wrap:wrap;gap:0.3rem;">${skills.map((s) => `<span style="font-size:0.6rem;padding:0.1rem 0.4rem;background:#f0f0f0;border-radius:2px;">${s}</span>`).join("")}</div></div>`;
      } else if (mode === "terminal") {
        return `<div class="rt-section"><div class="rt-section-title">// skills</div><div class="rt-skills">${skills.map((s) => `<span class="rt-skill-tag">${s}</span>`).join("")}</div></div>`;
      } else if (mode === "minimal") {
        return `<div style="margin-top:0.4rem;"><div class="rm-section-title">Skills</div><div style="display:flex;flex-wrap:wrap;gap:0.2rem;margin-top:0.15rem;">${skills.map((s) => `<span style="font-size:0.6rem;padding:0.08rem 0.4rem;border:1px solid #ddd;border-radius:2px;">${s}</span>`).join("")}</div></div>`;
      } else if (mode === "modern") {
        return `<div><div class="mod-section-title">Skills</div>${skills.map((s) => `<div class="mod-skill-item">▸ ${s}</div>`).join("")}</div>`;
      } else if (mode === "executive") {
        return `<div style="margin-bottom:0.4rem;"><div style="font-size:0.62rem;font-weight:600;color:#c8a951;text-transform:uppercase;letter-spacing:1px;margin-bottom:0.15rem;">Skills</div><div style="display:flex;flex-wrap:wrap;gap:0.2rem;">${skills.map((s) => `<span style="font-size:0.58rem;padding:0.08rem 0.35rem;border:1px solid #d4c5a9;border-radius:2px;color:#2c3e50;">${s}</span>`).join("")}</div></div>`;
      } else if (mode === "creative") {
        return `<div style="margin-bottom:0.4rem;"><div style="font-size:0.62rem;font-weight:700;color:#ff6b9d;text-transform:uppercase;letter-spacing:1px;margin-bottom:0.15rem;">✦ Skills</div><div style="display:flex;flex-wrap:wrap;gap:0.2rem;">${skills.map((s) => `<span style="font-size:0.58rem;padding:0.08rem 0.4rem;background:rgba(255,107,157,0.1);border-radius:8px;color:#6b2fa0;font-weight:500;">${s}</span>`).join("")}</div></div>`;
      } else if (mode === "timeline") {
        return `<div style="margin-bottom:0.8rem;"><div style="font-size:0.62rem;font-weight:700;color:#2c3e50;text-transform:uppercase;letter-spacing:1px;border-bottom:2px solid #3498db;padding-bottom:0.15rem;margin-bottom:0.2rem;">Skills</div><div style="display:flex;flex-wrap:wrap;gap:0.25rem;padding-left:1rem;border-left:2px solid #e0e0e0;">${skills.map((s) => `<span style="font-size:0.58rem;padding:0.08rem 0.35rem;background:#f8f9fa;border-radius:2px;color:#555;">${s}</span>`).join("")}</div></div>`;
      } else if (mode === "columns") {
        return `<div style="margin-bottom:0.8rem;"><div style="font-size:0.62rem;font-weight:700;color:#2c3e50;text-transform:uppercase;letter-spacing:1px;border-bottom:2px solid #2c3e50;padding-bottom:0.15rem;margin-bottom:0.2rem;">Skills</div><div style="display:flex;flex-wrap:wrap;gap:0.2rem;">${skills.map((s) => `<span style="font-size:0.58rem;padding:0.05rem 0.3rem;background:#f0f0f0;border-radius:2px;color:#333;">${s}</span>`).join("")}</div></div>`;
      }
    }
    return "";
  }

  // ─── COLLECT FORM DATA ─────────────────────────────────
  function getFormData() {
    const exps = [];
    document.querySelectorAll(".exp-entry").forEach((el) => {
      exps.push({
        company: el.querySelector(".exp-company")?.value || "",
        role: el.querySelector(".exp-role")?.value || "",
        from: el.querySelector(".exp-from")?.value || "",
        to: el.querySelector(".exp-to")?.value || "",
        desc: el.querySelector(".exp-desc")?.value || "",
      });
    });
    const edus = [];
    document.querySelectorAll(".edu-entry").forEach((el) => {
      edus.push({
        school: el.querySelector(".edu-school")?.value || "",
        degree: el.querySelector(".edu-degree")?.value || "",
        from: el.querySelector(".edu-from")?.value || "",
        to: el.querySelector(".edu-to")?.value || "",
      });
    });
    const projs = [];
    document.querySelectorAll(".proj-entry").forEach((el) => {
      projs.push({
        title: el.querySelector(".proj-title")?.value || "",
        desc: el.querySelector(".proj-desc")?.value || "",
        tech: el.querySelector(".proj-tech")?.value || "",
      });
    });
    const achs = [];
    document.querySelectorAll(".achievement-entry").forEach((el) => {
      achs.push({
        title: el.querySelector(".ach-title")?.value || "",
        date: el.querySelector(".ach-date")?.value || "",
        desc: el.querySelector(".ach-desc")?.value || "",
      });
    });
    const certs = [];
    document.querySelectorAll(".cert-entry").forEach((el) => {
      certs.push({
        title: el.querySelector(".cert-title")?.value || "",
        issuer: el.querySelector(".cert-issuer")?.value || "",
        date: el.querySelector(".cert-date")?.value || "",
        link: el.querySelector(".cert-link")?.value || "",
      });
    });
    return {
      name: document.getElementById("name").value.trim() || "Your Name",
      role: document.getElementById("role").value.trim() || "Developer",
      email: document.getElementById("email").value.trim() || "",
      phone: document.getElementById("phone").value.trim() || "",
      location: document.getElementById("location").value.trim() || "",
      portfolio: document.getElementById("portfolio").value.trim() || "",
      summary: document.getElementById("summary").value.trim() || "",
      skills: document.getElementById("skills")?.value.trim() || "",
      skills_languages:
        document.getElementById("skills-languages").value.trim() || "",
      skills_frameworks:
        document.getElementById("skills-frameworks").value.trim() || "",
      skills_tools: document.getElementById("skills-tools").value.trim() || "",
      skills_databases:
        document.getElementById("skills-databases").value.trim() || "",
      skills_cloud: document.getElementById("skills-cloud").value.trim() || "",
      experience: exps.filter((e) => e.company && e.role),
      education: edus.filter((e) => e.school && e.degree),
      projects: projs.filter((p) => p.title),
      achievements: achs.filter((a) => a.title),
      certifications: certs.filter((c) => c.title),
      template: state.template,
    };
  }

  // ─── FILL FORM FROM HISTORY ────────────────────────────
  function fillFormFromData(data) {
    if (data.name) document.getElementById("name").value = data.name;
    if (data.role) document.getElementById("role").value = data.role;
    if (data.email) document.getElementById("email").value = data.email;
    if (data.phone) document.getElementById("phone").value = data.phone;
    if (data.location)
      document.getElementById("location").value = data.location;
    if (data.portfolio)
      document.getElementById("portfolio").value = data.portfolio;
    if (data.summary) document.getElementById("summary").value = data.summary;

    // Fill categorized skills
    if (document.getElementById("skills-languages")) {
      if (data.skills_languages)
        document.getElementById("skills-languages").value =
          data.skills_languages;
      if (data.skills_frameworks)
        document.getElementById("skills-frameworks").value =
          data.skills_frameworks;
      if (data.skills_tools)
        document.getElementById("skills-tools").value = data.skills_tools;
      if (data.skills_databases)
        document.getElementById("skills-databases").value =
          data.skills_databases;
      if (data.skills_cloud)
        document.getElementById("skills-cloud").value = data.skills_cloud;
    }
    // Fallback for old flat skills field
    if (data.skills && document.getElementById("skills")) {
      document.getElementById("skills").value = data.skills;
    }

    if (data.template) {
      state.template = data.template;
      document
        .querySelectorAll(".tmpl-option")
        .forEach((o) =>
          o.classList.toggle("selected", o.dataset.tmpl === data.template),
        );
      document
        .querySelectorAll('input[name="template"]')
        .forEach((r) => (r.checked = r.value === data.template));
    }

    // Experience
    expContainer.innerHTML = "";
    (data.experience || []).forEach((e, i) =>
      expContainer.appendChild(createExpEntry(e, i)),
    );
    if (!data.experience?.length)
      expContainer.appendChild(
        createExpEntry(null, expContainer.children.length),
      );
    updateExpCounters();

    // Education
    eduContainer.innerHTML = "";
    (data.education || []).forEach((e) =>
      eduContainer.appendChild(createEduEntry(e)),
    );
    if (!data.education?.length) eduContainer.appendChild(createEduEntry());
    updateEduCounters();

    // Projects
    projContainer.innerHTML = "";
    (data.projects || []).forEach((p) =>
      projContainer.appendChild(createProjEntry(p)),
    );
    if (!data.projects?.length) projContainer.appendChild(createProjEntry());
    updateProjCounters();

    // Achievements
    achievementContainer.innerHTML = "";
    (data.achievements || []).forEach((a) =>
      achievementContainer.appendChild(createAchEntry(a)),
    );
    if (!data.achievements?.length)
      achievementContainer.appendChild(createAchEntry());
    updateAchCounters();

    // Certifications
    certContainer.innerHTML = "";
    (data.certifications || []).forEach((c) =>
      certContainer.appendChild(createCertEntry(c)),
    );
    if (!data.certifications?.length)
      certContainer.appendChild(createCertEntry());
    updateCertCounters();
  }

  // ─── FIT RESUME TO PAGE (scale-to-fit) ────────────────
  function cleanupScaleWrapper() {
    const existing = document.querySelector('.resume-output-wrap');
    if (existing) {
      const el = existing.querySelector('.resume-output');
      if (el) {
        el.style.transform = '';
        el.style.transformOrigin = '';
        existing.parentNode.insertBefore(el, existing);
      }
      existing.remove();
    }
  }

  function fitResumeToPage() {
    cleanupScaleWrapper();

    Promise.all([
      document.fonts.ready,
      new Promise(r => requestAnimationFrame(r))
    ]).then(() => {
      const el = document.querySelector('.resume-output');
      if (!el) return;

      const targetPx = Math.round(297 * 3.779527559); // 297mm -> px

      if (el.scrollHeight <= targetPx) return;

      const scale = targetPx / el.scrollHeight;

      const wrap = document.createElement('div');
      wrap.className = 'resume-output-wrap';
      wrap.style.cssText = `width:210mm;height:297mm;overflow:hidden;margin:0 auto;`;

      el.parentNode.insertBefore(wrap, el);
      wrap.appendChild(el);

      el.style.transformOrigin = 'top left';
      el.style.transform = `scale(${scale})`;

      if (scale < 0.35) {
        console.warn(`Resume heavily scaled: ${Math.round(scale * 100)}%`);
      }
    });
  }

  // ─── RENDER RESUME ─────────────────────────────────────
  function renderResume(data, aiContent) {
    const summary =
      aiContent?.summary ||
      data.summary ||
      "Professional with experience in software development and technology.";
    const experiences = aiContent?.experience?.length
      ? aiContent.experience
      : data.experience;
    const educations = aiContent?.education?.length
      ? aiContent.education
      : data.education;
    let html = "";

    if (data.template === "compact") {
      html = `<div class="resume-output resume-compact" style="font-family:'Segoe UI',Arial,sans-serif;color:#222;padding:1.2rem 1.5rem;">
                <div style="border-bottom:2px solid #00cc9e;padding-bottom:0.5rem;margin-bottom:0.8rem;">
                    <h1 style="font-size:1.2rem;font-weight:700;margin:0;color:#0a0a0e;">${data.name}</h1>
                    <div style="font-size:0.8rem;color:#555;margin-top:0.15rem;">${data.role}</div>
                    <div style="font-size:0.6rem;color:#777;margin-top:0.2rem;display:flex;flex-wrap:wrap;gap:0.3rem;">
                        ${data.email ? `<span>✉ ${data.email}</span>` : ""}${data.phone ? `<span>📞 ${data.phone}</span>` : ""}${data.location ? `<span>📍 ${data.location}</span>` : ""}${data.portfolio ? `<span>🔗 ${data.portfolio}</span>` : ""}
                    </div>
                </div>
                ${summary ? `<div style="margin-bottom:0.8rem;"><div style="font-size:0.7rem;font-weight:600;color:#00cc9e;text-transform:uppercase;letter-spacing:1px;margin-bottom:0.2rem;">Summary</div><p style="font-size:0.65rem;line-height:1.35;color:#333;margin:0;">${summary}</p></div>` : ""}
                ${experiences.length ? `<div style="margin-bottom:0.8rem;"><div style="font-size:0.7rem;font-weight:600;color:#00cc9e;text-transform:uppercase;letter-spacing:1px;margin-bottom:0.3rem;">Experience</div>${experiences.map((e) => `<div style="margin-bottom:0.4rem;"><div style="font-size:0.78rem;font-weight:600;">${e.role} — <span style="font-weight:400;color:#555;">${e.company}</span></div><div style="font-size:0.65rem;color:#777;">${e.from || ""} — ${e.to || ""}</div><div style="font-size:0.7rem;color:#444;margin-top:0.1rem;line-height:1.4;">${e.description || e.desc || ""}</div></div>`).join("")}</div>` : ""}
                ${educations.length ? `<div style="margin-bottom:0.8rem;"><div style="font-size:0.7rem;font-weight:600;color:#00cc9e;text-transform:uppercase;letter-spacing:1px;margin-bottom:0.2rem;">Education</div>${educations.map((e) => `<div style="font-size:0.75rem;margin-bottom:0.15rem;"><span style="font-weight:600;">${e.degree}</span> — ${e.school} <span style="color:#777;">(${e.from || ""} — ${e.to || ""})</span></div>`).join("")}</div>` : ""}
                ${renderSkillsBlock(data, "compact")}
            </div>`;
    } else if (data.template === "terminal") {
      html = `<div class="resume-output resume-terminal">
                <div class="rt-header"><h1>${data.name}</h1><div class="rt-role">${data.role}</div><div class="rt-contact">${data.email ? `<span>✉ ${data.email}</span>` : ""}${data.phone ? `<span>📞 ${data.phone}</span>` : ""}${data.location ? `<span>📍 ${data.location}</span>` : ""}${data.portfolio ? `<span>🔗 ${data.portfolio}</span>` : ""}</div></div>
                <div class="rt-body">
                    ${summary ? `<div class="rt-section"><div class="rt-section-title">// summary</div><div class="rt-item-desc">${summary}</div></div>` : ""}
                    ${experiences.length ? `<div class="rt-section"><div class="rt-section-title">// experience</div>${experiences.map((e) => `<div class="rt-item"><div class="rt-item-title">${e.role} @ ${e.company}</div><div class="rt-item-sub">${e.from || ""} — ${e.to || ""}</div><div class="rt-item-desc">${e.description || e.desc || ""}</div></div>`).join("")}</div>` : ""}
                    ${educations.length ? `<div class="rt-section"><div class="rt-section-title">// education</div>${educations.map((e) => `<div class="rt-item"><div class="rt-item-title">${e.degree} @ ${e.school}</div><div class="rt-item-sub">${e.from || ""} — ${e.to || ""}</div></div>`).join("")}</div>` : ""}
                    ${renderSkillsBlock(data, "terminal")}
                </div>
            </div>`;
    } else if (data.template === "minimal") {
      html = `<div class="resume-output resume-minimal">
                <div class="rm-header"><h1>${data.name}</h1><div class="rm-role">${data.role}</div><div class="rm-contact">${data.email ? `<span>${data.email}</span>` : ""}${data.phone ? `<span>${data.phone}</span>` : ""}${data.location ? `<span>${data.location}</span>` : ""}${data.portfolio ? `<span>${data.portfolio}</span>` : ""}</div></div>
                <div class="rm-body">
                    <div class="rt-section-title">${"\u2500".repeat(30)}</div>
                    <p style="font-size:0.65rem;margin:0.3rem 0;line-height:1.35;color:#333;">${summary}</p>
                    ${experiences.length ? `<div style="margin-top:0.8rem;"><div class="rm-section-title">Experience</div>${experiences.map((e) => `<div style="margin-bottom:0.4rem;"><div style="font-size:0.78rem;font-weight:600;">${e.role} — ${e.company}</div><div style="font-size:0.68rem;color:#555;">${e.from || ""} — ${e.to || ""}</div><div style="font-size:0.7rem;color:#333;margin-top:0.1rem;">${e.description || e.desc || ""}</div></div>`).join("")}</div>` : ""}
                    ${renderSkillsBlock(data, "minimal")}
                    ${educations.length ? `<div style="margin-top:0.8rem;"><div class="rm-section-title">Education</div>${educations.map((e) => `<div style="margin-bottom:0.3rem;"><div style="font-size:0.78rem;font-weight:600;">${e.degree} — ${e.school}</div><div style="font-size:0.68rem;color:#555;">${e.from || ""} — ${e.to || ""}</div></div>`).join("")}</div>` : ""}
                </div>
            </div>`;
    } else if (data.template === "modern") {
      html = `<div class="resume-output resume-modern">
                <div class="mod-sidebar"><h1>${data.name}</h1><div class="mod-role">${data.role}</div><div class="mod-contact">${data.email ? `<div>✉ ${data.email}</div>` : ""}${data.phone ? `<div>📞 ${data.phone}</div>` : ""}${data.location ? `<div>📍 ${data.location}</div>` : ""}${data.portfolio ? `<div>🔗 ${data.portfolio}</div>` : ""}</div>
                    ${renderSkillsBlock(data, "modern")}
                    ${educations.length ? `<div><div class="mod-section-title">Education</div>${educations.map((e) => `<div style="margin-bottom:0.4rem;"><div style="font-size:0.7rem;font-weight:600;color:#fff;">${e.degree}</div><div style="font-size:0.62rem;color:#aaa;">${e.school}</div><div style="font-size:0.6rem;color:#777;">${e.from || ""} — ${e.to || ""}</div></div>`).join("")}</div>` : ""}
                </div>
                <div class="mod-main">
                    <div style="margin-bottom:1rem;"><div class="mod-section-title">About</div><p style="font-size:0.72rem;color:#333;line-height:1.5;">${summary}</p></div>
                    ${experiences.length ? `<div><div class="mod-section-title">Experience</div>${experiences.map((e) => `<div style="margin-bottom:0.5rem;"><div class="mod-item-title">${e.role}</div><div class="mod-item-sub">${e.company} · ${e.from || ""} — ${e.to || ""}</div><p style="font-size:0.68rem;color:#444;margin-top:0.1rem;line-height:1.4;">${e.description || e.desc || ""}</p></div>`).join("")}</div>` : ""}
                </div>
            </div>`;
    } else if (data.template === "executive") {
      // Executive: Dark navy header, gold accents, clean professional
      const contactItems = [
        data.email && `✉ ${data.email}`,
        data.phone && `📞 ${data.phone}`,
        data.location && `📍 ${data.location}`,
        data.portfolio && `🔗 ${data.portfolio}`,
      ].filter(Boolean);
      html = `<div class="resume-output resume-executive" style="font-family:'Georgia','Times New Roman',serif;color:#2c3e50;">
                <div style="background:linear-gradient(135deg,#0a1628 0%,#0d1f3c 100%);color:#fff;padding:1.5rem 2rem 1rem;">
                    <h1 style="font-size:1.6rem;font-weight:700;margin:0;letter-spacing:0.5px;color:#f0e6d0;">${data.name}</h1>
                    <div style="font-size:0.85rem;color:#c8a951;margin-top:0.2rem;font-style:italic;">${data.role}</div>
                    ${contactItems.length ? `<div style="font-size:0.65rem;color:#8899aa;margin-top:0.5rem;display:flex;flex-wrap:wrap;gap:0.6rem;border-top:1px solid rgba(200,169,81,0.3);padding-top:0.5rem;">${contactItems.map((c) => `<span>${c}</span>`).join("")}</div>` : ""}
                </div>
                <div style="padding:1rem 2rem 1.5rem;">
                    ${summary ? `<div style="margin-bottom:0.8rem;"><div style="font-size:0.7rem;font-weight:600;color:#c8a951;text-transform:uppercase;letter-spacing:1px;margin-bottom:0.2rem;">Professional Summary</div><p style="font-size:0.72rem;line-height:1.6;color:#444;margin:0;">${summary}</p></div>` : ""}
                    ${experiences.length ? `<div style="margin-bottom:0.8rem;"><div style="font-size:0.7rem;font-weight:600;color:#c8a951;text-transform:uppercase;letter-spacing:1px;border-bottom:1px solid #e8e0d0;padding-bottom:0.2rem;margin-bottom:0.4rem;">Experience</div>${experiences.map((e) => `<div style="margin-bottom:0.5rem;"><div style="font-size:0.8rem;font-weight:700;color:#0a1628;">${e.role}</div><div style="font-size:0.7rem;color:#c8a951;font-style:italic;">${e.company} · ${e.from || ""} — ${e.to || ""}</div><div style="font-size:0.7rem;color:#555;margin-top:0.1rem;line-height:1.5;">${e.description || e.desc || ""}</div></div>`).join("")}</div>` : ""}
                    ${renderSkillsBlock(data, "executive")}
                    ${educations.length ? `<div><div style="font-size:0.7rem;font-weight:600;color:#c8a951;text-transform:uppercase;letter-spacing:1px;border-bottom:1px solid #e8e0d0;padding-bottom:0.2rem;margin-bottom:0.3rem;">Education</div>${educations.map((e) => `<div style="margin-bottom:0.3rem;"><div style="font-size:0.75rem;font-weight:600;">${e.degree}</div><div style="font-size:0.68rem;color:#666;">${e.school} <span style="color:#999;">(${e.from || ""} — ${e.to || ""})</span></div></div>`).join("")}</div>` : ""}
                </div>
            </div>`;
    } else if (data.template === "creative") {
      // Creative: Gradient header, colorful, rounded sections
      const contactItems = [
        data.email && `✉ ${data.email}`,
        data.phone && `📞 ${data.phone}`,
        data.location && `📍 ${data.location}`,
        data.portfolio && `🔗 ${data.portfolio}`,
      ].filter(Boolean);
      html = `<div class="resume-output resume-creative" style="font-family:'Inter','Segoe UI',sans-serif;color:#2c3e50;">
                <div style="background:linear-gradient(135deg,#1a0033 0%,#330066 50%,#006666 100%);color:#fff;padding:1.5rem 2rem 1rem;border-radius:0 0 20px 20px;">
                    <h1 style="font-size:1.6rem;font-weight:800;margin:0;background:linear-gradient(90deg,#ff6b9d,#00d4aa);-webkit-background-clip:text;-webkit-text-fill-color:transparent;background-clip:text;">${data.name}</h1>
                    <div style="font-size:0.85rem;color:#ffcc00;margin-top:0.2rem;font-weight:500;">${data.role}</div>
                    ${contactItems.length ? `<div style="font-size:0.65rem;color:rgba(255,255,255,0.7);margin-top:0.5rem;display:flex;flex-wrap:wrap;gap:0.6rem;">${contactItems.map((c) => `<span>${c}</span>`).join("")}</div>` : ""}
                </div>
                <div style="padding:1rem 2rem 1.5rem;">
                    ${summary ? `<div style="margin-bottom:0.8rem;"><div style="font-size:0.7rem;font-weight:700;color:#ff6b9d;text-transform:uppercase;letter-spacing:1px;margin-bottom:0.2rem;">✦ About</div><p style="font-size:0.72rem;line-height:1.6;color:#444;margin:0;">${summary}</p></div>` : ""}
                    ${experiences.length ? `<div style="margin-bottom:0.8rem;"><div style="font-size:0.7rem;font-weight:700;color:#ff6b9d;text-transform:uppercase;letter-spacing:1px;margin-bottom:0.3rem;">✦ Experience</div>${experiences.map((e) => `<div style="margin-bottom:0.5rem;background:#fafafa;border-radius:12px;padding:0.5rem 0.7rem;border-left:3px solid #00d4aa;"><div style="font-size:0.8rem;font-weight:700;color:#330066;">${e.role}</div><div style="font-size:0.7rem;color:#ff6b9d;font-weight:500;">${e.company} · ${e.from || ""} — ${e.to || ""}</div><div style="font-size:0.7rem;color:#555;margin-top:0.1rem;line-height:1.5;">${e.description || e.desc || ""}</div></div>`).join("")}</div>` : ""}
                    ${renderSkillsBlock(data, "creative")}
                    ${educations.length ? `<div><div style="font-size:0.7rem;font-weight:700;color:#ff6b9d;text-transform:uppercase;letter-spacing:1px;margin-bottom:0.3rem;">✦ Education</div>${educations.map((e) => `<div style="margin-bottom:0.3rem;background:#fafafa;border-radius:10px;padding:0.4rem 0.7rem;"><div style="font-size:0.75rem;font-weight:600;color:#330066;">${e.degree}</div><div style="font-size:0.68rem;color:#666;">${e.school} <span style="color:#999;">(${e.from || ""} — ${e.to || ""})</span></div></div>`).join("")}</div>` : ""}
                </div>
            </div>`;
    } else if (data.template === "timeline") {
      // Timeline: Left date column, right details, connecting line
      const contactItems = [
        data.email && `✉ ${data.email}`,
        data.phone && `📞 ${data.phone}`,
        data.location && `📍 ${data.location}`,
        data.portfolio && `🔗 ${data.portfolio}`,
      ].filter(Boolean);
      html = `<div class="resume-output resume-timeline" style="font-family:'Inter','Segoe UI',sans-serif;color:#2c3e50;">
                <div style="text-align:center;padding:1.5rem 2rem 0.8rem;border-bottom:3px solid #3498db;">
                    <h1 style="font-size:1.6rem;font-weight:700;margin:0;color:#2c3e50;">${data.name}</h1>
                    <div style="font-size:0.85rem;color:#3498db;margin-top:0.15rem;">${data.role}</div>
                    ${contactItems.length ? `<div style="font-size:0.65rem;color:#777;margin-top:0.4rem;display:flex;justify-content:center;flex-wrap:wrap;gap:0.6rem;">${contactItems.map((c) => `<span>${c}</span>`).join("")}</div>` : ""}
                </div>
                <div style="padding:1rem 2rem 1.5rem;">
                    ${summary ? `<div style="margin-bottom:0.8rem;padding-left:1rem;border-left:3px solid #3498db;"><div style="font-size:0.7rem;font-weight:700;color:#2c3e50;text-transform:uppercase;letter-spacing:1px;margin-bottom:0.2rem;">Summary</div><p style="font-size:0.72rem;line-height:1.5;color:#555;margin:0;">${summary}</p></div>` : ""}
                    ${experiences.length ? `<div style="margin-bottom:0.8rem;"><div style="font-size:0.7rem;font-weight:700;color:#2c3e50;text-transform:uppercase;letter-spacing:1px;border-bottom:2px solid #e0e0e0;padding-bottom:0.2rem;margin-bottom:0.4rem;">Experience</div>${experiences.map((e) => `<div style="display:flex;gap:0.8rem;margin-bottom:0.6rem;position:relative;"><div style="width:80px;flex-shrink:0;text-align:right;font-size:0.65rem;color:#3498db;font-weight:600;padding-top:0.1rem;">${e.from || ""} — ${e.to || ""}</div><div style="flex:1;padding-left:0.8rem;border-left:2px solid #e0e0e0;position:relative;"><div style="position:absolute;left:-5px;top:4px;width:8px;height:8px;background:#3498db;border-radius:50%;"></div><div style="font-size:0.78rem;font-weight:600;">${e.role} — <span style="font-weight:400;color:#555;">${e.company}</span></div><div style="font-size:0.68rem;color:#666;margin-top:0.05rem;line-height:1.4;">${e.description || e.desc || ""}</div></div></div>`).join("")}</div>` : ""}
                    ${renderSkillsBlock(data, "timeline")}
                    ${educations.length ? `<div><div style="font-size:0.7rem;font-weight:700;color:#2c3e50;text-transform:uppercase;letter-spacing:1px;border-bottom:2px solid #e0e0e0;padding-bottom:0.2rem;margin-bottom:0.3rem;">Education</div>${educations.map((e) => `<div style="display:flex;gap:0.8rem;margin-bottom:0.3rem;"><div style="width:80px;flex-shrink:0;text-align:right;font-size:0.65rem;color:#3498db;font-weight:600;">${e.from || ""} — ${e.to || ""}</div><div style="flex:1;padding-left:0.8rem;border-left:2px solid #e0e0e0;"><div style="font-size:0.75rem;font-weight:600;">${e.degree}</div><div style="font-size:0.68rem;color:#666;">${e.school}</div></div></div>`).join("")}</div>` : ""}
                </div>
            </div>`;
    } else if (data.template === "columns") {
      // Columns: Two equal columns layout
      const contactItems = [
        data.email && `✉ ${data.email}`,
        data.phone && `📞 ${data.phone}`,
        data.location && `📍 ${data.location}`,
        data.portfolio && `🔗 ${data.portfolio}`,
      ].filter(Boolean);
      html = `<div class="resume-output resume-columns" style="font-family:'Inter','Segoe UI',sans-serif;color:#2c3e50;">
                <div style="padding:1.2rem 1.5rem;border-bottom:2px solid #2c3e50;text-align:center;">
                    <h1 style="font-size:1.4rem;font-weight:700;margin:0;color:#2c3e50;">${data.name}</h1>
                    <div style="font-size:0.8rem;color:#555;margin-top:0.1rem;">${data.role}</div>
                    ${contactItems.length ? `<div style="font-size:0.65rem;color:#777;margin-top:0.3rem;display:flex;justify-content:center;flex-wrap:wrap;gap:0.5rem;">${contactItems.map((c) => `<span>${c}</span>`).join("")}</div>` : ""}
                </div>
                <div style="display:flex;gap:1rem;padding:1rem 1.5rem;">
                    <div style="flex:1;">
                        ${summary ? `<div style="margin-bottom:0.8rem;"><div style="font-size:0.7rem;font-weight:700;color:#2c3e50;text-transform:uppercase;letter-spacing:1px;border-bottom:1px solid #e0e0e0;padding-bottom:0.15rem;margin-bottom:0.2rem;">About</div><p style="font-size:0.7rem;line-height:1.5;color:#444;margin:0;">${summary}</p></div>` : ""}
                        ${experiences.length ? `<div><div style="font-size:0.7rem;font-weight:700;color:#2c3e50;text-transform:uppercase;letter-spacing:1px;border-bottom:1px solid #e0e0e0;padding-bottom:0.15rem;margin-bottom:0.3rem;">Experience</div>${experiences.map((e) => `<div style="margin-bottom:0.5rem;"><div style="font-size:0.75rem;font-weight:600;">${e.role}</div><div style="font-size:0.65rem;color:#555;">${e.company} · ${e.from || ""} — ${e.to || ""}</div><div style="font-size:0.65rem;color:#555;margin-top:0.05rem;line-height:1.4;">${e.description || e.desc || ""}</div></div>`).join("")}</div>` : ""}
                    </div>
                    <div style="flex:1;">
                        ${renderSkillsBlock(data, "columns")}
                        ${educations.length ? `<div><div style="font-size:0.7rem;font-weight:700;color:#2c3e50;text-transform:uppercase;letter-spacing:1px;border-bottom:1px solid #e0e0e0;padding-bottom:0.15rem;margin-bottom:0.3rem;">Education</div>${educations.map((e) => `<div style="margin-bottom:0.3rem;"><div style="font-size:0.72rem;font-weight:600;">${e.degree}</div><div style="font-size:0.65rem;color:#555;">${e.school} <span style="color:#999;">(${e.from || ""} — ${e.to || ""})</span></div></div>`).join("")}</div>` : ""}
                    </div>
                </div>
            </div>`;
    }

    state.lastHTML = html;
    state.lastData = data;
    state.lastAIContent = aiContent;
    preview.innerHTML = html;
    fitResumeToPage();
  }

  // ─── FORM SUBMIT (WITH PAYMENT SAFETY) ────────────────
  form.addEventListener("submit", async (e) => {
    e.preventDefault();
    if (state.isGenerating) return;

    const data = getFormData();
    if (!data.name || data.name === "Your Name") {
      genStatus.textContent = "[!] enter your name";
      showToast("Enter your name", "error");
      return;
    }

    state.isGenerating = true;
    genBtn.disabled = true;
    genBtn.querySelector(".btn-text").textContent = "creating order...";
    genStatus.textContent = "⏳ setting up payment...";

    try {
      // Step 1: Create order via backend
      genStatus.textContent = "⏳ creating payment order...";
      const orderRes = await fetch(`${apiBase}/api/generate`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(data),
      });

      if (!orderRes.ok) {
        const err = await orderRes
          .json()
          .catch(() => ({ message: "Payment service unavailable" }));
        throw new Error(err.message || "Payment service unavailable");
      }

      const order = await orderRes.json();
      if (!order.success)
        throw new Error(order.message || "Order creation failed");

      // Step 2: Open Razorpay checkout
      genStatus.textContent = "💳 complete payment to continue...";
      const paymentResult = await new Promise((resolve, reject) => {
        const rzp = new Razorpay({
          key: order.key,
          order_id: order.order_id,
          amount: order.amount,
          currency: "INR",
          name: "Resume Forge",
          description: "AI Resume Generation",
          handler: (response) => resolve(response),
          modal: { ondismiss: () => reject(new Error("CANCELLED")) },
          prefill: { name: data.name, email: data.email, contact: data.phone },
          theme: { color: "#00ffc8" },
        });
        rzp.open();
      });

      // Step 3: Confirm payment + generate resume on backend
      genStatus.textContent = "⏳ payment verified! generating resume...";
      genBtn.querySelector(".btn-text").textContent = "generating...";

      const confirmRes = await fetch(`${apiBase}/api/confirm`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          ...data,
          order_id: paymentResult.razorpay_order_id,
          payment_id: paymentResult.razorpay_payment_id,
          signature: paymentResult.razorpay_signature,
        }),
      });

      if (!confirmRes.ok) {
        const err = await confirmRes
          .json()
          .catch(() => ({ message: "Generation failed" }));
        throw new Error(err.message || "Generation failed");
      }

      const result = await confirmRes.json();
      if (!result.success)
        throw new Error(result.message || "Generation failed");

      // Step 4: Display the rendered HTML from backend
      state.lastHTML = result.html;
      preview.innerHTML = result.html;
      fitResumeToPage();

      // Step 5: Save to history
      const historyEntry = {
        id: Date.now().toString(36) + Math.random().toString(36).slice(2, 6),
        timestamp: Date.now(),
        name: data.name,
        role: data.role,
        template: data.template,
        html: result.html,
        formData: data,
        paymentId: paymentResult.razorpay_payment_id || "",
      };
      saveHistory(historyEntry);

      state.genCount++;
      localStorage.setItem("resumeForgeCount", String(state.genCount));
      updateGenCountDisplay();

      document.title = `Resume - ${data.name} | Resume Forge`;
      genStatus.textContent = "✅ resume ready! download or copy below";
      showToast("🎉 Resume generated successfully!", "success");
    } catch (err) {
      if (err.message === "CANCELLED") {
        genStatus.textContent = "[!] payment was cancelled";
        showToast("Payment cancelled", "warn");
      } else {
        genStatus.textContent = `❌ ${err.message || "Something went wrong"}`;
        showToast("❌ " + (err.message || "Error"), "error");
      }
    }

    state.isGenerating = false;
    genBtn.disabled = false;
    genBtn.querySelector(".btn-text").textContent = "generate resume";
  });

  // Save details in local storage
  document.getElementById("save-details-btn").addEventListener("click", () => {
    const data = getFormData();
    localStorage.setItem("resumeForgeSavedDetails", JSON.stringify(data));
    showToast("💾 Form details saved to local storage!", "success");
  });

  // ─── UPDATE COUNTER ────────────────────────────────────
  function updateGenCountDisplay() {
    navCount.textContent = `${resumeHistory.length} saved · ${state.genCount} total`;
    const priceSpan = document.querySelector(".btn-price");
    if (priceSpan) priceSpan.textContent = `₹10 · ${state.genCount} made`;
  }

  // ─── DOWNLOAD PDF ─────────────────────────────────────
  document.getElementById("download-pdf").addEventListener("click", () => {
    const content = document.querySelector(".resume-output");
    if (!content) {
      showToast("Generate a resume first", "warn");
      return;
    }
    document.title = `resume-${document.getElementById("name").value || "untitled"}`;
    window.print();
    document.title =
      "Resume Forge — AI Resume Builder | Free Online Resume Generator";
  });

  // ─── COPY HTML ────────────────────────────────────────
  document.getElementById("copy-html").addEventListener("click", async () => {
    if (!state.lastHTML) {
      showToast("Generate a resume first", "warn");
      return;
    }
    try {
      await navigator.clipboard.writeText(state.lastHTML);
      showToast("✅ HTML copied to clipboard!", "success");
    } catch {
      showToast("Failed to copy", "error");
    }
  });

  // ─── PRINT STYLES ─────────────────────────────────────
  const printStyle = document.createElement("style");
  printStyle.textContent = `@media print { body * { visibility: hidden; } #resume-preview, #resume-preview * { visibility: visible; } #resume-preview { position: absolute; top: 0; left: 0; width: 100%; height: 100%; padding: 0 !important; overflow: hidden; } .resume-output { box-shadow: none !important; margin: 0 !important; border-radius: 0 !important; } @page { margin: 0; size: A4; } }`;
  document.head.appendChild(printStyle);

  // ─── TOAST STYLES ─────────────────────────────────────
  const toastStyle = document.createElement("style");
  toastStyle.textContent = `.toast-msg { position: fixed; bottom: 1.5rem; left: 50%; transform: translateX(-50%) translateY(10px); padding: 0.6rem 1.2rem; border-radius: 4px; font-size: 0.78rem; font-family: var(--font); z-index: 9999; opacity: 0; transition: all 0.3s ease; pointer-events: none; border: 1px solid var(--border); } .toast-msg.show { opacity: 1; transform: translateX(-50%) translateY(0); } .toast-success { background: rgba(0,255,200,0.1); color: var(--cyan); border-color: rgba(0,255,200,0.3); } .toast-error { background: rgba(255,51,85,0.1); color: var(--red); border-color: rgba(255,51,85,0.3); } .toast-warn { background: rgba(255,204,0,0.1); color: var(--yellow); border-color: rgba(255,204,0,0.3); } .toast-info { background: rgba(0,102,255,0.1); color: #66aaff; border-color: rgba(0,102,255,0.3); }`;
  document.head.appendChild(toastStyle);

  // ─── INITIALIZATION ────────────────────────────────────
  renderHistory();
  updateGenCountDisplay();

  const savedDetails = localStorage.getItem("resumeForgeSavedDetails");
  if (savedDetails) {
    try {
      fillFormFromData(JSON.parse(savedDetails));
      showToast("💾 Auto-filled saved details", "info");
    } catch (e) {
      console.error("Failed to parse saved details", e);
    }
  } else {
    // Dynamically replace the default static inputs to ensure counters and unique IDs are set up correctly
    if (expContainer && expContainer.querySelector(".exp-entry")) {
      expContainer.innerHTML = "";
      expContainer.appendChild(createExpEntry());
      updateExpCounters();
    }
    if (eduContainer && eduContainer.querySelector(".edu-entry")) {
      eduContainer.innerHTML = "";
      eduContainer.appendChild(createEduEntry());
      updateEduCounters();
    }
    if (projContainer && projContainer.querySelector(".proj-entry")) {
      projContainer.innerHTML = "";
      projContainer.appendChild(createProjEntry());
      updateProjCounters();
    }
    if (
      achievementContainer &&
      achievementContainer.querySelector(".achievement-entry")
    ) {
      achievementContainer.innerHTML = "";
      achievementContainer.appendChild(createAchEntry());
      updateAchCounters();
    }
    if (certContainer && certContainer.querySelector(".cert-entry")) {
      certContainer.innerHTML = "";
      certContainer.appendChild(createCertEntry());
      updateCertCounters();
    }
  }

  // Load the initial template preview
  loadTemplatePreview(state.template);
});
