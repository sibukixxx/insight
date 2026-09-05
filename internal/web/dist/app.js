(() => {
  "use strict";

  const app = document.getElementById("app");
  let buildInfo = { demoBuild: false, clientName: "" };

  const SOURCE_LABELS = {
    interview: "Interview",
    review: "Review",
    support: "Support conversation",
    sales: "Sales call",
    survey: "Survey",
    job_posting: "Job posting",
    social_post: "Social post",
  };

  const STEP_LABELS = {
    starting: "Starting analysis",
    extracting_observations: "Reading documents",
    detecting_traces: "Finding deviations from expected behavior",
    detecting_patterns: "Finding recurring patterns",
    generating_hypotheses: "Generating hidden-need hypotheses",
    searching_evidence: "Searching for evidence and counter-evidence",
    deduplicating_insights: "Merging duplicate insights",
    scoring_confidence: "Calculating confidence",
    completed: "Complete",
  };

  // Two kinds of "noticing" the pipeline records (see docs/detailed-design.md §23):
  // a deviation is a behavior that broke a common-sense expectation (the
  // trace an unconscious desire leaves behind); a repetition is a behavior
  // seen across several people.
  const DEVIATION_LABELS = {
    contradiction: "Words contradict actions",
    excess_effort: "Extra effort despite urgency",
    excess_payment: "Pays more than planned",
    persistence: "Continues despite dissatisfaction",
    absence: "Expected action is absent",
    other: "Other unexpected behavior",
  };

  // App-side quality warnings. These are computed deterministically after
  // the model has spoken; they are hints for the researcher, not verdicts.
  const QUALITY_FLAG_LABELS = {
    stated_need_echo: {
      label: "Restates the stated need",
      desc: "The latent need closely matches the stated need. A need the participant already recognizes and expresses is not a hidden insight.",
    },
    generic_term: {
      label: "Generic language",
      desc: "The latent need relies on a broad abstraction. Check whether it can name the specific desire that drove the behavior.",
    },
    no_trace: {
      label: "No behavioral trace",
      desc: "This hypothesis comes only from repetition, not a deviation from expected behavior. It may be well supported but unsurprising.",
    },
    abduction_incomplete: {
      label: "Incomplete reasoning",
      desc: "The expected behavior or surprising fact is missing, so the reader cannot audit the expectation-to-deviation-to-hypothesis chain.",
    },
  };

  function qualityBadgesHTML(flags, { withDesc = false } = {}) {
    if (!flags || flags.length === 0) return "";
    const badges = flags.map((f) => {
      const meta = QUALITY_FLAG_LABELS[f.code] || { label: f.code, desc: "" };
      const text = f.detail ? `${meta.label}: ${f.detail}` : meta.label;
      return `<span class="quality-badge" title="${escapeHtml(meta.desc)}">⚠ ${escapeHtml(text)}</span>`;
    }).join("");
    if (!withDesc) return `<div class="quality-badges">${badges}</div>`;
    const descs = flags.map((f) => {
      const meta = QUALITY_FLAG_LABELS[f.code] || { label: f.code, desc: "" };
      return `<li><strong>${escapeHtml(meta.label)}${f.detail ? `（${escapeHtml(f.detail)}）` : ""}</strong> — ${escapeHtml(meta.desc)}</li>`;
    }).join("");
    return `
      <div class="quality-box">
        <div class="quality-box-title">Automated quality checks</div>
        <div class="quality-badges">${badges}</div>
        <ul class="quality-desc">${descs}</ul>
      </div>`;
  }

  function escapeHtml(s) {
    return String(s ?? "").replace(/[&<>"']/g, (c) => ({
      "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;",
    }[c]));
  }

  async function api(path, options) {
    const res = await fetch(path, {
      headers: options && options.body instanceof FormData ? {} : { "Content-Type": "application/json" },
      ...options,
    });
    let body = null;
    try { body = await res.json(); } catch (_) { /* no body */ }
    if (!res.ok) {
      const message = (body && body.error) || `${res.status} ${res.statusText}`;
      throw new Error(message);
    }
    return body;
  }

  function buildBadge() {
    if (buildInfo.demoBuild) return `<span class="badge demo">Demo build</span>`;
    return `<span class="badge delivery">Production build (no demo data)</span>`;
  }

  function confidentialBanner() {
    if (!buildInfo.clientName) return "";
    return `<div class="confidential-banner">Confidential — prepared for ${escapeHtml(buildInfo.clientName)}</div>`;
  }

  function layout(inner) {
    app.innerHTML = `
      <header class="top">
        <div class="brand"><a href="#/">Insight Lab</a> <small>Hidden Needs Finder</small></div>
        <div class="header-actions">
          ${buildBadge()}
          <a class="settings-link" href="#/settings" title="Settings">⚙ Settings</a>
        </div>
      </header>
      <main>
        ${confidentialBanner()}
        ${inner}
      </main>
      <footer class="privacy">
        Uploaded data is stored locally. Only text required for analysis is sent to the configured AI provider.
      </footer>
    `;
  }

  function errorBox(message) {
    return message ? `<div class="error-box">${escapeHtml(message)}</div>` : "";
  }

  function confidenceBar(value) {
    const pct = Math.round((value || 0) * 100);
    return `
      <div class="confidence-row">
        <span class="confidence-label">Confidence</span>
        <div class="confidence-bar"><div class="confidence-fill" style="width:${pct}%"></div></div>
        <span class="confidence-pct">${pct}%</span>
      </div>`;
  }

  // ---------- Home ----------

  async function renderHome(errorMessage) {
    let projects = [];
    let loadError = "";
    try {
      projects = await api("/api/projects");
    } catch (e) {
      loadError = e.message;
    }

    const list = projects.length
      ? `<div class="project-list">${projects.map((p) => `
          <a class="card project-item" href="#/projects/${encodeURIComponent(p.id)}">
            <div>
              <div class="name">${escapeHtml(p.name)}</div>
              <div class="meta">${new Date(p.createdAt).toLocaleString()}</div>
            </div>
            <span>Open &rarr;</span>
          </a>`).join("")}</div>`
      : `<div class="empty">No projects yet. Try the demo or create a project.</div>`;

    layout(`
      <div class="hero">
        <h1>Insight Lab</h1>
        <p>Find the needs your customers do not put into words.</p>
        <div class="actions">
          <button class="primary" id="try-demo" ${buildInfo.demoBuild ? "" : "disabled"}>Try the demo</button>
          <button id="new-project">New project</button>
        </div>
        ${!buildInfo.demoBuild ? `<p class="hint">This production build contains no demo data. Run <code>make build-demo</code> to try the demo.</p>` : ""}
      </div>
      ${errorBox(errorMessage || loadError)}
      <div class="section-title">Projects</div>
      ${list}
    `);

    document.getElementById("try-demo").addEventListener("click", async () => {
      try {
        const project = await api("/api/demo", { method: "POST" });
        location.hash = `#/projects/${project.id}`;
      } catch (e) {
        renderHome(e.message);
      }
    });

    document.getElementById("new-project").addEventListener("click", async () => {
      const name = prompt("Project name");
      if (!name) return;
      try {
        const project = await api("/api/projects", { method: "POST", body: JSON.stringify({ name }) });
        location.hash = `#/projects/${project.id}`;
      } catch (e) {
        renderHome(e.message);
      }
    });
  }

  // ---------- Settings ----------

  async function renderSettings(errorMessage, notice) {
    let settings;
    try {
      settings = await api("/api/settings");
    } catch (e) {
      layout(`<a class="back-link" href="#/">&larr; Back</a>${errorBox(e.message)}`);
      return;
    }

    layout(`
      <a class="back-link" href="#/">&larr; Back</a>
      <div class="card">
        <div class="section-title">LLM settings</div>
        ${errorBox(errorMessage)}
        ${notice ? `<div class="notice-box">${escapeHtml(notice)}</div>` : ""}
        <form class="paste-form" id="settings-form">
          <div>
            <label>Base URL (OpenAI-compatible endpoint)</label>
            <input type="text" name="baseUrl" value="${escapeHtml(settings.baseUrl)}" placeholder="https://api.openai.com/v1">
          </div>
          <div>
            <label>Model</label>
            <input type="text" name="model" value="${escapeHtml(settings.model)}" placeholder="gpt-5">
          </div>
          <div>
            <label>API key ${settings.hasApiKey ? `(configured: ${escapeHtml(settings.maskedApiKey)})` : ""}</label>
            <input type="password" name="apiKey" placeholder="${settings.hasApiKey ? "Enter only to replace it" : "sk-..."}">
          </div>
          <div class="settings-actions">
            <button type="submit" class="primary">Save</button>
            <button type="button" id="test-connection">Test connection</button>
          </div>
        </form>
        <p class="hint">The API key is held in process memory only. It is not saved to disk or the database.</p>
      </div>
    `);

    document.getElementById("settings-form").addEventListener("submit", async (ev) => {
      ev.preventDefault();
      const f = ev.target;
      try {
        await api("/api/settings", {
          method: "PUT",
          body: JSON.stringify({ baseUrl: f.baseUrl.value, model: f.model.value, apiKey: f.apiKey.value }),
        });
        renderSettings(null, "Settings saved.");
      } catch (e) {
        renderSettings(e.message);
      }
    });

    document.getElementById("test-connection").addEventListener("click", async () => {
      try {
        const result = await api("/api/settings/test", { method: "POST" });
        renderSettings(null, `Connection successful (mode: ${result.mode}).`);
      } catch (e) {
        renderSettings(`Connection test failed: ${e.message}`);
      }
    });
  }

  // ---------- Project ----------

  let activeEventSource = null;

  function closeActiveStream() {
    if (activeEventSource) {
      activeEventSource.close();
      activeEventSource = null;
    }
  }

  async function renderProject(projectID, errorMessage) {
    closeActiveStream();

    let project, documents, insights, analyses;
    try {
      [project, documents, insights, analyses] = await Promise.all([
        api(`/api/projects/${encodeURIComponent(projectID)}`),
        api(`/api/projects/${encodeURIComponent(projectID)}/documents`),
        api(`/api/projects/${encodeURIComponent(projectID)}/insights`),
        api(`/api/projects/${encodeURIComponent(projectID)}/analyses`),
      ]);
    } catch (e) {
      layout(`<a class="back-link" href="#/">&larr; Back to projects</a>${errorBox(e.message)}`);
      return;
    }

    const latestAnalysis = analyses[0] || null;
    const isRunning = latestAnalysis && (latestAnalysis.status === "running" || latestAnalysis.status === "queued");

    const docsHtml = documents.length
      ? documents.map((d) => `
          <div class="doc-item">
            <div class="doc-head">
              <span class="source-tag source-${escapeHtml(d.source)}">${escapeHtml(SOURCE_LABELS[d.source] || d.source)}</span>
              <span class="doc-title">${escapeHtml(d.title || "(Untitled)")}</span>
            </div>
            <div class="doc-content">${escapeHtml(d.content)}</div>
          </div>`).join("")
      : `<div class="empty">No documents yet. Paste text below or import a CSV.</div>`;

    const insightsHtml = insights.length
      ? `<div class="insight-list">${insights.map((i) => `
          <a class="card insight-card${(i.qualityFlags || []).length ? " insight-card-flagged" : ""}" href="#/insights/${encodeURIComponent(i.id)}">
            <div class="insight-card-title">${escapeHtml(i.title)}</div>
            <div class="insight-card-latent">${escapeHtml(i.latentNeed)}</div>
            ${i.surprisingFact ? `<div class="insight-card-trace">Deviation: ${escapeHtml(i.surprisingFact)}</div>` : ""}
            ${confidenceBar(i.confidence)}
            ${qualityBadgesHTML(i.qualityFlags)}
          </a>`).join("")}</div>`
      : `<div class="empty">No insights yet. Run an analysis.</div>`;

    layout(`
      <a class="back-link" href="#/">&larr; Back to projects</a>
      <div class="card">
        <div class="section-title">Project</div>
        <h2 style="margin:0 0 4px;">${escapeHtml(project.name)}</h2>
        <div class="meta">${documents.length} documents / ${insights.length} insights</div>
      </div>

      ${errorBox(errorMessage)}

      <div class="card">
        <div class="section-title">Analysis</div>
        <div id="analysis-panel">
          ${analysisPanelHTML(latestAnalysis)}
        </div>
        <div class="analysis-actions">
          <button class="primary" id="run-analysis" ${isRunning || documents.length === 0 ? "disabled" : ""}>Run analysis</button>
          <a class="btn" href="#/projects/${encodeURIComponent(projectID)}/patterns">View traces and patterns</a>
          <a class="btn" href="#/projects/${encodeURIComponent(projectID)}/evaluation">View evaluation</a>
          <a class="btn" href="/api/projects/${encodeURIComponent(projectID)}/report.md" download>Download report</a>
        </div>
      </div>

      <div class="card">
        <div class="section-title">Insights</div>
        ${insightsHtml}
      </div>

      <div class="card">
        <div class="section-title">Paste text</div>
        <form class="paste-form" id="paste-form">
          <div>
            <label>Source type</label>
            <select name="source">
              <option value="interview">Interview</option>
              <option value="review">Review</option>
              <option value="support">Support conversation</option>
              <option value="sales">Sales call</option>
              <option value="survey">Survey</option>
              <option value="job_posting">Job posting</option>
              <option value="social_post">Social post</option>
            </select>
          </div>
          <div><label>Title</label><input type="text" name="title" placeholder="Example: Interview #15"></div>
          <div><label>Content</label><textarea name="content" placeholder="Paste the original text here" required></textarea></div>
          <div><button type="submit" class="primary">Add document</button></div>
        </form>
      </div>

      <div class="card">
        <div class="section-title">Import CSV</div>
        <p class="hint">Columns: id,source,title,content. Source must be interview, review, support, sales, survey, job_posting, or social_post.</p>
        <form id="csv-form">
          <input type="file" name="file" accept=".csv,text/csv" required>
          <button type="submit" class="primary">Import</button>
        </form>
        <div id="csv-result"></div>
      </div>

      <div class="card">
        <div class="section-title">Documents</div>
        ${docsHtml}
      </div>
    `);

    document.getElementById("paste-form").addEventListener("submit", async (ev) => {
      ev.preventDefault();
      const f = ev.target;
      try {
        await api(`/api/projects/${encodeURIComponent(projectID)}/documents`, {
          method: "POST",
          body: JSON.stringify({ source: f.source.value, title: f.title.value, content: f.content.value }),
        });
        renderProject(projectID);
      } catch (e) {
        renderProject(projectID, e.message);
      }
    });

    document.getElementById("csv-form").addEventListener("submit", async (ev) => {
      ev.preventDefault();
      const file = ev.target.file.files[0];
      if (!file) return;
      const formData = new FormData();
      formData.append("file", file);
      try {
        const result = await api(`/api/projects/${encodeURIComponent(projectID)}/documents/import`, {
          method: "POST", body: formData,
        });
        document.getElementById("csv-result").innerHTML =
          `<div class="notice-box">Imported ${result.imported}; skipped ${result.skipped}.</div>`;
        renderProject(projectID);
      } catch (e) {
        document.getElementById("csv-result").innerHTML = errorBox(e.message);
      }
    });

    const runBtn = document.getElementById("run-analysis");
    runBtn.addEventListener("click", async () => {
      runBtn.disabled = true;
      try {
        const analysis = await api(`/api/projects/${encodeURIComponent(projectID)}/analysis`, { method: "POST" });
        watchAnalysis(projectID, analysis.id);
      } catch (e) {
        renderProject(projectID, e.message);
      }
    });

    if (isRunning) {
      watchAnalysis(projectID, latestAnalysis.id);
    }
  }

  function analysisPanelHTML(analysis) {
    if (!analysis) return `<div class="empty">No analysis has been run.</div>`;
    const label = STEP_LABELS[analysis.currentStep] || analysis.currentStep || analysis.status;
    if (analysis.status === "completed") {
      return `<div class="analysis-status status-completed">Completed (${new Date(analysis.finishedAt).toLocaleString()})</div>`;
    }
    if (analysis.status === "failed") {
      return `<div class="analysis-status status-failed">Failed: ${escapeHtml(analysis.error || "")}</div>`;
    }
    return `
      <div class="analysis-status status-running">
        <div class="progress-label">${escapeHtml(label)}...</div>
        <div class="progress-bar"><div class="progress-fill" style="width:${analysis.progress}%"></div></div>
      </div>`;
  }

  function watchAnalysis(projectID, analysisID) {
    closeActiveStream();
    const panel = document.getElementById("analysis-panel");
    const es = new EventSource(`/api/analysis/${encodeURIComponent(analysisID)}/events`);
    activeEventSource = es;

    es.addEventListener("progress", (ev) => {
      const data = JSON.parse(ev.data);
      const label = STEP_LABELS[data.step] || data.message || data.step;
      if (panel) {
        panel.innerHTML = `
          <div class="analysis-status status-running">
            <div class="progress-label">${escapeHtml(data.message || label)}</div>
            <div class="progress-bar"><div class="progress-fill" style="width:${data.progress}%"></div></div>
          </div>`;
      }
    });

    es.addEventListener("completed", () => {
      closeActiveStream();
      renderProject(projectID);
    });

    // A server-sent named "error" event and the browser's own
    // connection-level error both surface as Event type "error" in
    // EventSource; only the former carries .data.
    es.addEventListener("error", (ev) => {
      if (typeof ev.data !== "string") return;
      closeActiveStream();
      let data = {};
      try { data = JSON.parse(ev.data); } catch (_) { /* ignore */ }
      renderProject(projectID, `Analysis failed: ${data.message || "unknown error"}`);
    });
  }

  // ---------- Insight detail ----------

  async function renderInsight(insightID) {
    let insight;
    try {
      insight = await api(`/api/insights/${encodeURIComponent(insightID)}`);
    } catch (e) {
      layout(`<a class="back-link" href="#/">&larr; Back</a>${errorBox(e.message)}`);
      return;
    }

    const support = insight.evidence.filter((e) => e.type === "support");
    const counter = insight.evidence.filter((e) => e.type === "counter");

    layout(`
      <a class="back-link" href="#/projects/${encodeURIComponent(insight.projectId)}">&larr; Back to project</a>

      <div class="card reasoning-trail">
        <div class="section-title">Reasoning trail &mdash; expectation → deviation → hypothesis</div>
        <p class="hint">An insight is a hypothesis that explains the gap between expected and observed behavior. The complete reasoning chain remains visible for review.</p>
        ${abductionHTML(insight)}
        <div class="trail-subtitle">Source observations</div>
        ${patternsSectionHTML(insight.patterns)}
      </div>

      <div class="card">
        <div class="section-title">Insight</div>
        <h2 style="margin:0 0 12px;">${escapeHtml(insight.title)}</h2>
        ${confidenceBar(insight.confidence)}
        ${qualityBadgesHTML(insight.qualityFlags, { withDesc: true })}

        <div class="field-block fact-block">
          <div class="field-label">Observation</div>
          <div>${escapeHtml(insight.observation || "-")}</div>
        </div>
        <div class="field-block">
          <div class="field-label">Stated need</div>
          <div>${escapeHtml(insight.statedNeed || "-")}</div>
        </div>
        <div class="field-block latent-block">
          <div class="field-label">Latent need</div>
          <div>${escapeHtml(insight.latentNeed || "-")}</div>
        </div>
        <div class="field-block">
          <div class="field-label">JTBD</div>
          <div>${escapeHtml(insight.jtbd || "-")}</div>
        </div>
        <div class="field-block interpretation-block">
          <div class="field-label">Interpretation (AI-generated)</div>
          <div>${escapeHtml(insight.interpretation || "-")}</div>
        </div>
        <div class="field-block alt-block">
          <div class="field-label">Alternative interpretation</div>
          <div>${escapeHtml(insight.alternativeInterpretation || "-")}</div>
        </div>
        <div class="field-block">
          <div class="field-label">Product opportunity</div>
          <div>${escapeHtml(insight.productOpportunity || "-")}</div>
        </div>
        ${insight.monetizationAngle ? `
        <div class="field-block money-block">
          <div class="field-label">Monetization angle</div>
          <div>${escapeHtml(insight.monetizationAngle)}</div>
        </div>` : ""}
      </div>

      <div class="card">
        <div class="section-title">Evidence</div>
        ${support.length ? support.map(evidenceRowHTML).join("") : `<div class="empty">None</div>`}
      </div>

      <div class="card">
        <div class="section-title">Counter-evidence</div>
        ${counter.length ? counter.map(evidenceRowHTML).join("") : `<div class="empty">No counter-evidence found</div>`}
      </div>
    `);

    bindEvidenceToggles();
  }

  // Click-to-reveal is shared by Evidence rows (insight detail) and
  // Pattern observation rows (insight detail + the patterns page), since
  // both render the same {documentId, quote, startOffset, endOffset}
  // shape via evidenceRowHTML.
  function bindEvidenceToggles() {
    document.querySelectorAll(".evidence-toggle").forEach((btn) => {
      btn.addEventListener("click", async () => {
        const panel = btn.nextElementSibling;
        if (panel.dataset.loaded === "1") {
          panel.hidden = !panel.hidden;
          return;
        }
        try {
          const doc = await api(`/api/documents/${encodeURIComponent(btn.dataset.docId)}`);
          panel.innerHTML = evidenceContextHTML(doc, Number(btn.dataset.start), Number(btn.dataset.end));
          panel.dataset.loaded = "1";
          panel.hidden = false;
        } catch (e) {
          panel.innerHTML = errorBox(e.message);
          panel.hidden = false;
        }
      });
    });
  }

  // The abductive triad: a surprising fact C that broke an expectation,
  // and a hypothesis H such that, if H were true, C would be a matter of
  // course. Each step is shown even when empty so a missing link is visible.
  function abductionHTML(insight) {
    const step = (n, label, value, cls) => `
      <div class="abduction-step ${cls}">
        <div class="abduction-num">${n}</div>
        <div class="abduction-body">
          <div class="field-label">${label}</div>
          <div>${value ? escapeHtml(value) : `<span class="missing">Not recorded</span>`}</div>
        </div>
      </div>`;
    return `
      <div class="abduction">
        ${step("1", "Expected behavior", insight.expectation, "abduction-expect")}
        ${step("2", "Surprising fact (the deviation)", insight.surprisingFact, "abduction-fact")}
        ${step("3", "Hypothesis (the hidden need that makes step 2 reasonable)", insight.latentNeed, "abduction-hyp")}
        ${step("4", "Explanation", insight.rationale, "abduction-why")}
      </div>`;
  }

  function patternBlockHTML(p) {
    const isTrace = p.kind === "deviation";
    const kindBadge = isTrace
      ? `<span class="kind-badge kind-deviation">Trace</span>${p.deviationType ? `<span class="deviation-type">${escapeHtml(DEVIATION_LABELS[p.deviationType] || p.deviationType)}</span>` : ""}`
      : `<span class="kind-badge kind-repetition">Repetition</span>`;
    const body = isTrace
      ? `
        <div class="trace-gap">
          <div class="trace-cell trace-expect"><div class="field-label">Expected</div><div>${escapeHtml(p.expectation || "-")}</div></div>
          <div class="trace-arrow">≠</div>
          <div class="trace-cell trace-actual"><div class="field-label">Observed</div><div>${escapeHtml(p.description || "-")}</div></div>
        </div>`
      : (p.description ? `<div class="pattern-desc">${escapeHtml(p.description)}</div>` : "");
    return `
      <div class="pattern-block ${isTrace ? "pattern-trace" : ""}">
        <div class="pattern-head">${kindBadge}<span class="pattern-title">${escapeHtml(p.title)}</span></div>
        ${body}
        <div class="pattern-observations">
          ${(p.observations || []).map(evidenceRowHTML).join("") || `<div class="empty">No observations found</div>`}
        </div>
      </div>`;
  }

  function patternsSectionHTML(patterns) {
    if (!patterns || patterns.length === 0) {
      return `<div class="empty">No source traces or patterns were recorded for this insight.</div>`;
    }
    const traces = patterns.filter((p) => p.kind === "deviation");
    const repetitions = patterns.filter((p) => p.kind !== "deviation");
    let html = "";
    if (traces.length) html += traces.map(patternBlockHTML).join("");
    else html += `<div class="notice-box quality-notice">This insight is based on repetition only, not a deviation from expected behavior.</div>`;
    if (repetitions.length) html += repetitions.map(patternBlockHTML).join("");
    return html;
  }

  function evidenceRowHTML(e) {
    return `
      <div class="evidence-row">
        <button class="evidence-toggle" data-doc-id="${escapeHtml(e.documentId)}" data-start="${e.startOffset}" data-end="${e.endOffset}">
          "${escapeHtml(e.quote)}" <span class="evidence-reveal">View in source &darr;</span>
        </button>
        <div class="evidence-context" hidden></div>
      </div>`;
  }

  function evidenceContextHTML(doc, start, end) {
    const content = doc.content;
    const before = escapeHtml(content.slice(0, start));
    const mark = escapeHtml(content.slice(start, end));
    const after = escapeHtml(content.slice(end));
    return `
      <div class="evidence-doc-title">${escapeHtml(doc.title || doc.id)}</div>
      <div class="evidence-quote-context">${before}<mark>${mark}</mark>${after}</div>`;
  }

  // ---------- Evaluation ----------

  async function renderEvaluation(projectID) {
    let metrics, project;
    try {
      [project, metrics] = await Promise.all([
        api(`/api/projects/${encodeURIComponent(projectID)}`),
        api(`/api/projects/${encodeURIComponent(projectID)}/evaluation`),
      ]);
    } catch (e) {
      layout(`<a class="back-link" href="#/projects/${encodeURIComponent(projectID)}">&larr; Back to project</a>${errorBox(e.message)}`);
      return;
    }

    const rows = [
      ["Evidence Coverage", metrics.evidenceCoverage, "Insights with supporting evidence"],
      ["Unsupported Claim Rate", metrics.unsupportedClaimRate, "Quotes discarded because they could not be verified"],
      ["Counter-evidence Coverage", metrics.counterEvidenceCoverage, "Insights checked for counter-evidence"],
      ["Insight Duplication", metrics.insightDuplicationRate, "Draft insights merged as duplicates"],
      ["Trace-backed Insights", metrics.traceBackedInsightRate, "Insights backed by a behavioral deviation"],
      ["Quality Flagged", metrics.qualityFlaggedInsightRate, "Insights with a quality warning (lower is better)"],
    ];
    const flagCounts = metrics.qualityFlagCounts || {};
    const flagSummary = Object.keys(QUALITY_FLAG_LABELS)
      .filter((code) => flagCounts[code])
      .map((code) => `${QUALITY_FLAG_LABELS[code].label}: ${flagCounts[code]}`)
      .join(" / ");

    layout(`
      <a class="back-link" href="#/projects/${encodeURIComponent(projectID)}">&larr; Back to project</a>
      <div class="card">
        <div class="section-title">Evaluation — ${escapeHtml(project.name)}</div>
        <div class="metric-grid">
          ${rows.map(([label, value, desc]) => `
            <div class="metric-tile">
              <div class="metric-value">${Math.round((value || 0) * 100)}%</div>
              <div class="metric-label">${escapeHtml(label)}</div>
              <div class="metric-desc">${escapeHtml(desc)}</div>
            </div>`).join("")}
          <div class="metric-tile">
            <div class="metric-value">${(metrics.averageEvidencePerInsight || 0).toFixed(1)}</div>
            <div class="metric-label">Avg Evidence / Insight</div>
            <div class="metric-desc">Average evidence items per insight</div>
          </div>
        </div>
        <p class="hint">
          ${metrics.groundedObservations} of ${metrics.totalObservationCandidates} observation candidates were verified against source text.
          ${metrics.patternCount} findings, including ${metrics.traceCount || 0} deviations, produced ${metrics.totalInsightDrafts} drafts and ${metrics.finalInsightCount} final insights.
          ${flagSummary ? `Quality warnings: ${escapeHtml(flagSummary)}.` : "No quality warnings."}
        </p>
      </div>
    `);
  }

  // ---------- Patterns ----------

  async function renderPatterns(projectID) {
    let project, patterns;
    try {
      [project, patterns] = await Promise.all([
        api(`/api/projects/${encodeURIComponent(projectID)}`),
        api(`/api/projects/${encodeURIComponent(projectID)}/patterns`),
      ]);
    } catch (e) {
      layout(`<a class="back-link" href="#/projects/${encodeURIComponent(projectID)}">&larr; Back to project</a>${errorBox(e.message)}`);
      return;
    }

    const traces = patterns.filter((p) => p.kind === "deviation");
    const repetitions = patterns.filter((p) => p.kind !== "deviation");

    layout(`
      <a class="back-link" href="#/projects/${encodeURIComponent(projectID)}">&larr; Back to project</a>
      <div class="card">
        <div class="section-title">Traces and patterns — ${escapeHtml(project.name)}</div>
        <p class="hint">Everything detected during analysis, including findings that did not become final insights.</p>
      </div>
      <div class="card">
        <div class="section-title">Behavioral traces (deviations) ${traces.length}</div>
        <p class="hint">Places where observed behavior differs from a reasonable expectation. Hidden needs are proposed as hypotheses that explain these deviations.</p>
        ${traces.length ? traces.map(patternBlockHTML).join("") : `<div class="empty">No deviations detected.</div>`}
      </div>
      <div class="card">
        <div class="section-title">Recurring patterns ${repetitions.length}</div>
        <p class="hint">Behavior or language repeated across documents. Hypotheses based only on repetition can easily restate an explicit need.</p>
        ${repetitions.length ? repetitions.map(patternBlockHTML).join("") : `<div class="empty">No recurring patterns detected. Run an analysis first.</div>`}
      </div>
    `);

    bindEvidenceToggles();
  }

  // ---------- Router ----------

  function route() {
    closeActiveStream();
    const hash = location.hash || "#/";

    const patternsMatch = hash.match(/^#\/projects\/([^/]+)\/patterns$/);
    if (patternsMatch) { renderPatterns(decodeURIComponent(patternsMatch[1])); return; }

    const evalMatch = hash.match(/^#\/projects\/([^/]+)\/evaluation$/);
    if (evalMatch) { renderEvaluation(decodeURIComponent(evalMatch[1])); return; }

    const projectMatch = hash.match(/^#\/projects\/([^/]+)$/);
    if (projectMatch) { renderProject(decodeURIComponent(projectMatch[1])); return; }

    const insightMatch = hash.match(/^#\/insights\/([^/]+)$/);
    if (insightMatch) { renderInsight(decodeURIComponent(insightMatch[1])); return; }

    if (hash === "#/settings") { renderSettings(); return; }

    renderHome();
  }

  async function boot() {
    try {
      buildInfo = await api("/api/health");
    } catch (_) {
      // Fall back to defaults; the UI still renders, API errors surface per-action.
    }
    route();
  }

  window.addEventListener("hashchange", route);
  boot();
})();
