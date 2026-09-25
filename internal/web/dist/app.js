(() => {
  "use strict";

  const app = document.getElementById("app");
  let buildInfo = { demoBuild: false, clientName: "" };

  const SOURCE_LABELS = {
    document: "Document",
    report: "Report",
    paper: "Paper",
    web: "Web-derived text",
    record: "Record",
    other: "Other",
    dataset: "Dataset",
    interview: "Interview (legacy)",
    review: "Review (legacy)",
    support: "Support conversation (legacy)",
    sales: "Sales call (legacy)",
    survey: "Survey (legacy)",
    job_posting: "Job posting (legacy)",
    social_post: "Social post (legacy)",
  };

  const STEP_LABELS = {
    starting: "Starting analysis",
    extracting_observations: "Reading documents",
    detecting_traces: "Finding expectation / baseline mismatches",
    detecting_patterns: "Finding recurring patterns",
    generating_hypotheses: "Generating explanatory hypotheses",
    searching_evidence: "Searching for evidence and counter-evidence",
    deduplicating_insights: "Merging duplicate insights",
    scoring_confidence: "Calculating confidence",
    completed: "Complete",
  };

  // Two inspectable finding kinds: a mismatch against an expectation/baseline,
  // and a repeated regularity across grounded observations.
  const DEVIATION_LABELS = {
    contradiction: "Words contradict actions",
    excess_effort: "Extra effort despite urgency",
    excess_payment: "Pays more than planned",
    persistence: "Continues despite dissatisfaction",
    absence: "Expected action is absent",
    other: "Other expectation mismatch",
  };

  // App-side quality warnings. These are computed deterministically after
  // the model has spoken; they are hints for the researcher, not verdicts.
  const QUALITY_FLAG_LABELS = {
    stated_need_echo: {
      label: "Restates the stated need (legacy)",
      desc: "Legacy customer-research check; it applies only when a stated need exists.",
    },
    generic_term: {
      label: "Generic need language (legacy)",
      desc: "Legacy customer-research check; it applies only when a stated need exists.",
    },
    no_trace: {
      label: "No expectation mismatch",
      desc: "This hypothesis comes only from repetition, not a grounded mismatch against a baseline or expectation.",
    },
    abduction_incomplete: {
      label: "Incomplete reasoning",
      desc: "The baseline/expectation or surprising fact is missing, so the reader cannot audit the expectation-to-mismatch-to-hypothesis chain.",
    },
    insufficient_competing_hypotheses: {
      label: "Too few competing explanations",
      desc: "Fewer than three explanations were independently checked for this surprising fact. Consider plausible alternatives before deciding.",
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
        <div class="brand"><a href="#/">Insight Lab</a> <small>Evidence Research Engine</small></div>
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

  // ---------- Analysis runs ----------
  // Every result view is bound to exactly one analysis run. The run is named
  // by ?run=<analysisId> in the hash; without it the latest completed run is
  // shown. Queued, running and failed runs have no results to display.

  const NOT_RECORDED = "not recorded";

  function projectHash(projectID, page, runID) {
    const base = `#/projects/${encodeURIComponent(projectID)}${page ? `/${page}` : ""}`;
    return runID ? `${base}?run=${encodeURIComponent(runID)}` : base;
  }

  function runQuery(run) {
    return run ? `?analysisId=${encodeURIComponent(run.id)}` : "";
  }

  // pickRun resolves the selected run from the project's analysis list. An
  // unknown ?run= falls back to the latest completed run with a notice.
  function pickRun(analyses, runID) {
    if (runID) {
      const found = analyses.find((a) => a.id === runID);
      if (found) return { run: found, notice: "" };
    }
    // Same rule as the server: the most recently finished completed run.
    const latestCompleted = analyses
      .filter((a) => a.status === "completed")
      .sort((a, b) => String(b.finishedAt || b.createdAt).localeCompare(String(a.finishedAt || a.createdAt)))[0] || null;
    return { run: latestCompleted, notice: runID ? "The selected run was not found in this project; showing the latest completed run." : "" };
  }

  function runProvenance(run) {
    return (run && run.metrics && run.metrics.provenance) || {};
  }

  function runLabel(run) {
    const when = new Date(run.finishedAt || run.createdAt).toLocaleString();
    const model = runProvenance(run).model;
    return `${when} · ${run.status}${model ? ` · ${model}` : ""} · ${run.id}`;
  }

  // A deterministic run used no model; that is a recorded fact, not a gap.
  function modelScoped(prov, value) {
    if (value) return value;
    if (prov.mode === "deterministic") return "none (deterministic)";
    return NOT_RECORDED;
  }

  function provenanceChipsHTML(run) {
    if (!run) return "";
    const prov = runProvenance(run);
    const fingerprint = prov.promptFingerprint ? prov.promptFingerprint.slice(0, 7) : "";
    const chip = (label, value, full) => {
      const missing = value === NOT_RECORDED;
      return `<span class="chip${missing ? " chip-missing" : ""}" title="${escapeHtml(full || value)}">${escapeHtml(label)}: ${escapeHtml(value)}</span>`;
    };
    return `<div class="chips">
        ${chip("mode", prov.mode || NOT_RECORDED)}
        ${chip("model", modelScoped(prov, prov.model))}
        ${chip("prompt", modelScoped(prov, fingerprint), prov.promptFingerprint)}
        ${chip("rules", prov.ruleVersion || NOT_RECORDED)}
        ${chip("engine", engineLabel(run.executionSnapshot), run.executionSnapshot && run.executionSnapshot.gitCommit)}
        ${chip("execution", shortFingerprint(run.executionFingerprint), run.executionFingerprint)}
        ${chip("input", shortFingerprint(run.inputFingerprint), run.inputFingerprint)}
        ${run.researchQuestion ? chip("question", run.researchQuestion, run.researchQuestion) : ""}
      </div>`;
  }

  // Runs recorded before snapshots existed carry no fingerprints; that is
  // "not recorded", never "same as another run".
  function shortFingerprint(fp) {
    return fp ? fp.replace(/^sha256:/, "").slice(0, 7) : NOT_RECORDED;
  }

  function engineLabel(execution) {
    if (!execution) return NOT_RECORDED;
    const commit = execution.gitCommit && execution.gitCommit !== "UNKNOWN" ? `@${execution.gitCommit.slice(0, 7)}` : "";
    const dirty = execution.gitDirty === "true" ? "+dirty" : "";
    return `${execution.engineVersion || "UNKNOWN"}${commit}${dirty}`;
  }

  function runSelectorHTML(analyses, selected) {
    if (!analyses.length) return "";
    const options = analyses.map((a) => {
      const disabled = a.status !== "completed" ? " disabled" : "";
      const chosen = selected && a.id === selected.id ? " selected" : "";
      return `<option value="${escapeHtml(a.id)}"${chosen}${disabled}>${escapeHtml(runLabel(a))}</option>`;
    }).join("");
    return `
      <div class="run-selector">
        <label for="run-select">Showing results of run</label>
        <select id="run-select">${selected ? "" : `<option value="" selected>No completed run</option>`}${options}</select>
      </div>`;
  }

  function runListHTML(projectID, analyses, selected) {
    if (!analyses.length) return "";
    const items = analyses.map((a) => {
      const current = selected && a.id === selected.id;
      const label = escapeHtml(runLabel(a));
      const body = a.status === "completed" && !current
        ? `<a href="${projectHash(projectID, "", a.id)}">${label}</a>`
        : `<span>${label}</span>`;
      return `<li class="run-item status-${escapeHtml(a.status)}${current ? " run-current" : ""}">${body}${current ? ` <strong>(shown)</strong>` : ""}${a.status === "failed" && a.error ? ` <span class="meta">${escapeHtml(a.error)}</span>` : ""}</li>`;
    }).join("");
    return `<details class="run-list"><summary>All runs (${analyses.length})</summary><ul>${items}</ul></details>`;
  }

  function runHeaderHTML(run) {
    if (!run) return `<div class="empty">No completed analysis run yet.</div>`;
    return `<div class="meta">Run <code>${escapeHtml(run.id)}</code> · finished ${escapeHtml(run.finishedAt ? new Date(run.finishedAt).toLocaleString() : NOT_RECORDED)}</div>${provenanceChipsHTML(run)}`;
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
        <p>Turn supplied evidence into grounded observations, competing hypotheses, research gaps, and auditable next questions.</p>
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

  async function renderProject(projectID, errorMessage, runID) {
    closeActiveStream();

    let project, documents, insights, analyses, selectedRun, runNotice;
    try {
      [project, documents, analyses] = await Promise.all([
        api(`/api/projects/${encodeURIComponent(projectID)}`),
        api(`/api/projects/${encodeURIComponent(projectID)}/documents`),
        api(`/api/projects/${encodeURIComponent(projectID)}/analyses`),
      ]);
      ({ run: selectedRun, notice: runNotice } = pickRun(analyses, runID));
      insights = selectedRun
        ? await api(`/api/projects/${encodeURIComponent(projectID)}/insights${runQuery(selectedRun)}`)
        : [];
    } catch (e) {
      layout(`<a class="back-link" href="#/">&larr; Back to projects</a>${errorBox(e.message)}`);
      return;
    }
    const selectedRunID = selectedRun ? selectedRun.id : "";

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
        <div class="meta">${documents.length} documents / ${analyses.length} runs / ${insights.length} insights in the shown run</div>
      </div>

      ${errorBox(errorMessage)}
      ${runNotice ? `<div class="notice-box">${escapeHtml(runNotice)}</div>` : ""}

      <div class="card">
        <div class="section-title">Analysis</div>
        <div id="analysis-panel">
          ${analysisPanelHTML(latestAnalysis)}
        </div>
        ${runSelectorHTML(analyses, selectedRun)}
        ${provenanceChipsHTML(selectedRun)}
        ${runListHTML(projectID, analyses, selectedRun)}
        <div class="analysis-actions">
          <input id="research-question" class="analysis-question" type="text" maxlength="2000" placeholder="Optional research question — blank = open-ended discovery" aria-label="Research question">
          <button class="primary" id="run-analysis" ${isRunning || documents.length === 0 ? "disabled" : ""}>Run analysis</button>
          <a class="btn" href="${projectHash(projectID, "patterns", selectedRunID)}">View traces and patterns</a>
          <a class="btn" href="${projectHash(projectID, "evaluation", selectedRunID)}">View evaluation</a>
          <a class="btn" href="#/projects/${encodeURIComponent(projectID)}/research">Research publications</a>
          ${selectedRun ? `<a class="btn" href="/api/projects/${encodeURIComponent(projectID)}/report.md${runQuery(selectedRun)}" download>Download report</a>` : ""}
        </div>
      </div>

      <div class="card">
        <div class="section-title">Insights${selectedRun ? ` — run ${escapeHtml(selectedRun.id)}` : ""}</div>
        ${insightsHtml}
      </div>

      <div class="card">
        <div class="section-title">Paste text</div>
        <form class="paste-form" id="paste-form">
          <div>
            <label>Source type</label>
            <select name="source">
              <option value="document">Document</option>
              <option value="report">Report</option>
              <option value="paper">Paper</option>
              <option value="web">Web-derived text</option>
              <option value="record">Record</option>
              <option value="other">Other</option>
              <option value="dataset">Dataset observation</option>
              <option value="interview">Interview (legacy)</option>
              <option value="review">Review (legacy)</option>
              <option value="support">Support conversation (legacy)</option>
              <option value="sales">Sales call (legacy)</option>
              <option value="survey">Survey (legacy)</option>
              <option value="job_posting">Job posting (legacy)</option>
              <option value="social_post">Social post (legacy)</option>
            </select>
          </div>
          <div><label>Title</label><input type="text" name="title" placeholder="Example: Source document 15"></div>
          <div><label>Content</label><textarea name="content" placeholder="Paste the original text here" required></textarea></div>
          <div><button type="submit" class="primary">Add document</button></div>
        </form>
      </div>

      <div class="card">
        <div class="section-title">Import CSV</div>
		<p class="hint">Columns: id,source,title,content. Prefer document, report, paper, web, record, other, or dataset; legacy v1 source names remain accepted.</p>
        <form id="csv-form">
          <input type="file" name="file" accept=".csv,text/csv" required>
          <button type="submit" class="primary">Import</button>
        </form>
        <div id="csv-result"></div>
      </div>

	  <div class="card">
		<div class="section-title">Import ja-company analysis CSV</div>
		<p class="hint">Aggregates exported administrative records by month, location, and event type. Record counts are not treated as startup counts or causal evidence.</p>
		<form id="analysis-csv-form">
		  <input type="file" name="file" accept=".csv,text/csv" required>
		  <button type="submit" class="primary">Import analysis data</button>
		</form>
		<div id="analysis-csv-result"></div>
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

	document.getElementById("analysis-csv-form").addEventListener("submit", async (ev) => {
	  ev.preventDefault();
	  const file = ev.target.file.files[0];
	  if (!file) return;
	  const formData = new FormData();
	  formData.append("file", file);
	  try {
		const result = await api(`/api/projects/${encodeURIComponent(projectID)}/documents/import/analysis`, {
		  method: "POST", body: formData,
		});
		document.getElementById("analysis-csv-result").innerHTML =
		  `<div class="notice-box">Read ${result.recordsRead} records; imported ${result.imported} aggregate documents; skipped ${result.skipped}.</div>`;
		renderProject(projectID);
	  } catch (e) {
		document.getElementById("analysis-csv-result").innerHTML = errorBox(e.message);
	  }
	});

    const runSelect = document.getElementById("run-select");
    if (runSelect) {
      runSelect.addEventListener("change", () => {
        if (runSelect.value) location.hash = projectHash(projectID, "", runSelect.value);
      });
    }

    const runBtn = document.getElementById("run-analysis");
    runBtn.addEventListener("click", async () => {
      runBtn.disabled = true;
      try {
        const researchQuestion = (document.getElementById("research-question")?.value || "").trim();
        const analysis = await api(`/api/projects/${encodeURIComponent(projectID)}/analysis`, {
          method: "POST",
          body: JSON.stringify({ researchQuestion }),
        });
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
      const latest = projectHash(projectID);
      if (location.hash !== latest) location.hash = latest;
      else renderProject(projectID);
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
        <p class="hint">A synthesis is a testable explanation of grounded evidence. The complete observation → mismatch → hypothesis → evidence chain remains visible for review.</p>
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
          <div class="field-label">Stated need (legacy, when applicable)</div>
          <div>${escapeHtml(insight.statedNeed || "-")}</div>
        </div>
        <div class="field-block latent-block">
          <div class="field-label">Explanatory hypothesis</div>
          <div>${escapeHtml(insight.latentNeed || "-")}</div>
        </div>
        <div class="field-block">
          <div class="field-label">JTBD (legacy, when applicable)</div>
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
        ${step("1", "Expectation / baseline", insight.expectation, "abduction-expect")}
        ${step("2", "Mismatch / surprising fact", insight.surprisingFact, "abduction-fact")}
        ${step("3", "Explanatory hypothesis", insight.latentNeed, "abduction-hyp")}
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
    else html += `<div class="notice-box quality-notice">This hypothesis is based on repetition only, not an expectation/baseline mismatch.</div>`;
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

  async function renderEvaluation(projectID, runID) {
    let metrics, project, run;
    try {
      let analyses;
      [project, analyses] = await Promise.all([
        api(`/api/projects/${encodeURIComponent(projectID)}`),
        api(`/api/projects/${encodeURIComponent(projectID)}/analyses`),
      ]);
      run = pickRun(analyses, runID).run;
      if (!run) throw new Error("No completed analysis run yet.");
      metrics = await api(`/api/projects/${encodeURIComponent(projectID)}/evaluation${runQuery(run)}`);
    } catch (e) {
      layout(`<a class="back-link" href="${projectHash(projectID, "", runID)}">&larr; Back to project</a>${errorBox(e.message)}`);
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
      <a class="back-link" href="${projectHash(projectID, "", run.id)}">&larr; Back to project</a>
      <div class="card">
        <div class="section-title">Evaluation — ${escapeHtml(project.name)}</div>
        ${runHeaderHTML(run)}
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

  async function renderPatterns(projectID, runID) {
    let project, patterns, run;
    try {
      let analyses;
      [project, analyses] = await Promise.all([
        api(`/api/projects/${encodeURIComponent(projectID)}`),
        api(`/api/projects/${encodeURIComponent(projectID)}/analyses`),
      ]);
      run = pickRun(analyses, runID).run;
      patterns = run ? await api(`/api/projects/${encodeURIComponent(projectID)}/patterns${runQuery(run)}`) : [];
    } catch (e) {
      layout(`<a class="back-link" href="${projectHash(projectID, "", runID)}">&larr; Back to project</a>${errorBox(e.message)}`);
      return;
    }

    const traces = patterns.filter((p) => p.kind === "deviation");
    const repetitions = patterns.filter((p) => p.kind !== "deviation");

    layout(`
      <a class="back-link" href="${projectHash(projectID, "", run ? run.id : "")}">&larr; Back to project</a>
      <div class="card">
        <div class="section-title">Traces and patterns — ${escapeHtml(project.name)}</div>
        ${runHeaderHTML(run)}
        <p class="hint">Everything this run detected, including findings that did not become final insights.</p>
      </div>
      <div class="card">
        <div class="section-title">Expectation / baseline mismatches ${traces.length}</div>
        <p class="hint">Grounded observations that differ from a baseline, expectation, comparison, or expected sequence. Explanations remain hypotheses.</p>
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

  async function renderResearchRuns(projectID) {
    try {
      const runs = await api(`/api/projects/${encodeURIComponent(projectID)}/research-runs`);
      layout(`<a href="#/projects/${encodeURIComponent(projectID)}">Back to project</a>
        <h1>Research publications</h1>
        <div class="card">${(runs || []).map(run => `<p><a href="#/research-runs/${encodeURIComponent(run.id)}">${escapeHtml(run.question)}</a></p>`).join("") || "No research runs yet."}</div>
        <form id="new-research" class="card"><label>Research question <input name="question" required></label><button class="primary">Create from latest completed analysis</button></form>
        <div id="research-error" role="alert"></div>`);
      document.getElementById("new-research").onsubmit = async event => {
        event.preventDefault();
        const button = event.currentTarget.querySelector("button"); button.disabled = true;
        try {
          const run = await api(`/api/projects/${encodeURIComponent(projectID)}/research-runs`, {method: "POST", body: JSON.stringify({question: new FormData(event.currentTarget).get("question")})});
          location.hash = `#/research-runs/${encodeURIComponent(run.id)}`;
        } catch (error) { document.getElementById("research-error").textContent = error.message; button.disabled = false; }
      };
    } catch (error) { layout(errorBox(error.message)); }
  }

  async function renderPromotion(runID) {
    try {
      const base = `/api/research-runs/${encodeURIComponent(runID)}`;
      const run = await api(base);
      const iteration = run.iterations[run.iterations.length - 1];
      if (!iteration) { layout(errorBox("This research run has no iterations.")); return; }
      const promotion = iteration.promotion || {state: "DRAFT"};
      const gate = iteration.promotionGateInput || {};
      const terminal = ["PUBLISHED", "REJECTED_FOR_PUBLICATION"].includes(promotion.state);
      const judgments = [
        ["competingHypothesisConsidered", "Competing hypotheses or alternative interpretations considered"],
        ["independentValidationStatusAccurate", "Independent validation status accurately represented"],
        ["decisionReadinessHonestlyStated", "Decision readiness honestly stated"],
        ["makesStrongClaim", "The report makes a strong, decision-driving claim"],
        ["hasUnresolvedCriticalGap", "An unresolved critical research gap remains"],
        ["humanReviewCompleted", "I completed the human review of this output"],
      ];
      const contributions = ["CORRECTION", "REFINEMENT", "REPLICATION", "DEFINITION_AUDIT", "COUNTER_EVIDENCE", "INCONCLUSIVE_BUT_DECISION_RELEVANT", "NOVEL_MISMATCH", "OTHER"];
      layout(`<a href="#/projects/${encodeURIComponent(run.projectId)}/research">Back to research publications</a>
        <h1>${escapeHtml(run.question)}</h1>
        <div class="card"><h2>${escapeHtml(promotion.state || "DRAFT")}</h2>
          <p>Iteration ${iteration.sequence}. Review approval and publication are separate actions.</p>
          <ul>${(promotion.reasons || []).map(reason => `<li>${escapeHtml(reason)}</li>`).join("")}</ul>
          <p><a href="${base}/report.md" download>Research report</a> · <a href="${base}/artifact.json" target="_blank" rel="noopener">Current artifact</a>
          ${iteration.approvedArtifact ? ` · <a href="${base}/approved-artifact.json" target="_blank" rel="noopener">Approved artifact</a>` : ""}</p>
          ${iteration.approvedArtifact ? `<p>Approved reference: <code>${escapeHtml(iteration.approvedArtifact.reference)}</code></p>` : ""}
          <ul>${Object.entries(gate.Checklist || {}).map(([key, value]) => `<li>${value ? "✓" : "Missing:"} ${escapeHtml(key)}</li>`).join("")}</ul>
        </div>
        ${terminal ? "" : `<form id="promotion-review" class="card"><h2>Human publication review</h2>
          <p>Read the report and evidence before submitting. Each submission replaces the previous review.</p>
          <label>Contribution <select name="contribution" required><option value="">Choose a contribution</option>${contributions.map(value => `<option>${value}</option>`).join("")}</select></label>
          ${judgments.map(([key, label]) => `<p><label><input type="checkbox" name="${key}"> ${label}</label></p>`).join("")}
          <button class="primary">Save review and assess readiness</button></form>
          <div class="card">${promotion.state === "PUBLICATION_READY" && iteration.approvedArtifact ? `<button id="mark-published">Mark approved artifact as published</button><p>This records publication; it does not post to external services.</p>` : ""}
          <button id="reject-publication">Reject for publication</button></div>`}
        <div id="promotion-error" role="alert"></div>`);
      const submit = async (path, body, button) => {
        button.disabled = true;
        try { await api(`${base}/iterations/${encodeURIComponent(iteration.id)}/${path}`, {method: "PUT", body: JSON.stringify(body)}); await renderPromotion(runID); }
        catch (error) { document.getElementById("promotion-error").textContent = error.message; button.disabled = false; }
      };
      const form = document.getElementById("promotion-review");
      if (form) form.onsubmit = event => {
        event.preventDefault(); const data = new FormData(form); const body = {contribution: data.get("contribution")};
        judgments.forEach(([key]) => { body[key] = data.has(key); });
        submit("promotion-review", body, form.querySelector("button"));
      };
      for (const [id, targetState] of [["mark-published", "PUBLISHED"], ["reject-publication", "REJECTED_FOR_PUBLICATION"]]) {
        const button = document.getElementById(id);
        if (button) button.onclick = () => submit("promotion-transition", {targetState}, button);
      }
    } catch (error) { layout(errorBox(error.message)); }
  }

  // ---------- Router ----------

  function route() {
    closeActiveStream();
    const [hash, query = ""] = (location.hash || "#/").split("?");
    const runID = new URLSearchParams(query).get("run") || "";

    const researchMatch = hash.match(/^#\/projects\/([^/]+)\/research$/);
    if (researchMatch) { renderResearchRuns(decodeURIComponent(researchMatch[1])); return; }
    const promotionMatch = hash.match(/^#\/research-runs\/([^/]+)$/);
    if (promotionMatch) { renderPromotion(decodeURIComponent(promotionMatch[1])); return; }

    const patternsMatch = hash.match(/^#\/projects\/([^/]+)\/patterns$/);
    if (patternsMatch) { renderPatterns(decodeURIComponent(patternsMatch[1]), runID); return; }

    const evalMatch = hash.match(/^#\/projects\/([^/]+)\/evaluation$/);
    if (evalMatch) { renderEvaluation(decodeURIComponent(evalMatch[1]), runID); return; }

    const projectMatch = hash.match(/^#\/projects\/([^/]+)$/);
    if (projectMatch) { renderProject(decodeURIComponent(projectMatch[1]), undefined, runID); return; }

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
