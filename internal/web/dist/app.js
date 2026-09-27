(() => {
  "use strict";

  const app = document.getElementById("app");
  let buildInfo = { demoBuild: false, clientName: "" };

  // ---------- Locale ----------
  // UI display language only (#124). Research content, evidence, quotes,
  // server messages, IDs and enum codes are never translated here; the
  // language of model-generated text is a separate setting (#125).

  const LOCALES = { en: { tag: "en-US", name: "English" }, ja: { tag: "ja-JP", name: "日本語" } };
  const LOCALE_STORAGE_KEY = "insight-lab.locale";
  const dictionaries = {};
  let locale = "en";

  function initialLocale() {
    try {
      const saved = localStorage.getItem(LOCALE_STORAGE_KEY);
      if (Object.prototype.hasOwnProperty.call(LOCALES, saved)) return saved;
    } catch (_) { /* storage unavailable */ }
    for (const lang of navigator.languages || [navigator.language || ""]) {
      const base = String(lang).toLowerCase().split("-")[0];
      if (Object.prototype.hasOwnProperty.call(LOCALES, base)) return base;
    }
    return "en";
  }

  function applyLocale(next) {
    locale = Object.prototype.hasOwnProperty.call(LOCALES, next) ? next : "en";
    document.documentElement.lang = locale;
    document.title = t("app.title");
  }

  async function loadDictionaries() {
    await Promise.all(Object.keys(LOCALES).map(async (code) => {
      const res = await fetch(`/locales/${code}.json`);
      if (!res.ok) throw new Error(`/locales/${code}.json: ${res.status}`);
      dictionaries[code] = await res.json();
    }));
  }

  // t returns plain text: the current locale, then English, then a visibly
  // broken marker so a missing key is noticed instead of guessed.
  function t(key, params) {
    let text = (dictionaries[locale] || {})[key];
    if (text === undefined) text = (dictionaries.en || {})[key];
    if (text === undefined) {
      console.error(`missing UI text: ${key}`);
      return `⟦${key}⟧`;
    }
    return text.replace(/\{(\w+)\}/g, (m, name) => (params && name in params ? String(params[name]) : m));
  }

  // trustedHtml marks a parameter that is already safe markup built here.
  function trustedHtml(markup) {
    return { trustedHtml: String(markup) };
  }

  // th returns HTML: dictionary text and parameters are escaped, except
  // parameters explicitly wrapped with trustedHtml.
  function th(key, params) {
    const tokens = {};
    const plain = {};
    let n = 0;
    for (const [name, value] of Object.entries(params || {})) {
      if (value && typeof value === "object" && "trustedHtml" in value) {
        const token = `\u0000${n++}\u0000`;
        tokens[token] = value.trustedHtml;
        plain[name] = token;
      } else {
        plain[name] = value;
      }
    }
    let html = escapeHtml(t(key, plain));
    for (const [token, markup] of Object.entries(tokens)) html = html.split(token).join(markup);
    return html;
  }

  // label maps a stable code to its display label; an unknown code stays
  // visible as-is rather than being guessed or dropped.
  function label(table, code) {
    return Object.prototype.hasOwnProperty.call(table, code) ? t(table[code]) : String(code ?? "");
  }

  function formatDateTime(value) {
    if (!value) return t("common.notRecorded");
    const d = new Date(value);
    if (Number.isNaN(d.getTime())) return String(value);
    return new Intl.DateTimeFormat(LOCALES[locale].tag, { dateStyle: "medium", timeStyle: "short" }).format(d);
  }

  function formatPercent(ratio) {
    return new Intl.NumberFormat(LOCALES[locale].tag, { style: "percent", maximumFractionDigits: 0 }).format(ratio || 0);
  }

  function formatNumber(value, fractionDigits) {
    return new Intl.NumberFormat(LOCALES[locale].tag, { minimumFractionDigits: fractionDigits, maximumFractionDigits: fractionDigits }).format(value || 0);
  }

  function localeSelectHTML(id) {
    const options = Object.entries(LOCALES).map(([code, meta]) =>
      `<option value="${code}" lang="${code}"${code === locale ? " selected" : ""}>${escapeHtml(meta.name)}</option>`).join("");
    return `<select id="${id}" class="locale-select" aria-label="${th("locale.label")}">${options}</select>`;
  }

  function bindLocaleSelects() {
    document.querySelectorAll(".locale-select").forEach((select) => {
      select.addEventListener("change", () => switchLocale(select.value));
    });
  }

  // switchLocale re-renders the current route in place. The hash (project,
  // run, page) is untouched, and values typed into the page are restored.
  async function switchLocale(next) {
    try { localStorage.setItem(LOCALE_STORAGE_KEY, next); } catch (_) { /* storage unavailable */ }
    const fields = Array.from(app.querySelectorAll("input, textarea, select"))
      .filter((el) => el.type !== "file" && !el.classList.contains("locale-select") && (el.id || el.name))
      .map((el) => ({ id: el.id, name: el.name, form: el.form ? el.form.id : "", value: el.value, checked: el.checked }));
    const scroll = window.scrollY;
    applyLocale(next);
    await route();
    for (const f of fields) {
      const el = f.id
        ? document.getElementById(f.id)
        : document.querySelector(`${f.form ? `#${CSS.escape(f.form)} ` : ""}[name="${CSS.escape(f.name)}"]`);
      if (!el || el.classList.contains("locale-select")) continue;
      if (el.type === "checkbox" || el.type === "radio") el.checked = f.checked;
      else el.value = f.value;
    }
    window.scrollTo(0, scroll);
    const select = document.querySelector(".locale-select");
    if (select) select.focus();
  }

  // ---------- Display labels for stable codes ----------

  const SOURCE_LABELS = {
    document: "source.document",
    report: "source.report",
    paper: "source.paper",
    web: "source.web",
    record: "source.record",
    other: "source.other",
    dataset: "source.dataset",
    interview: "source.interview",
    review: "source.review",
    support: "source.support",
    sales: "source.sales",
    survey: "source.survey",
    job_posting: "source.jobPosting",
    social_post: "source.socialPost",
  };

  const STEP_LABELS = {
    starting: "step.starting",
    extracting_observations: "step.extractingObservations",
    detecting_traces: "step.detectingTraces",
    detecting_patterns: "step.detectingPatterns",
    generating_hypotheses: "step.generatingHypotheses",
    searching_evidence: "step.searchingEvidence",
    deduplicating_insights: "step.deduplicatingInsights",
    scoring_confidence: "step.scoringConfidence",
    completed: "step.completed",
  };

  const RUN_STATUS_LABELS = {
    queued: "runStatus.queued",
    running: "runStatus.running",
    completed: "runStatus.completed",
    failed: "runStatus.failed",
  };

  const MODE_LABELS = {
    deterministic: "mode.deterministic",
    model_backed: "mode.modelBacked",
  };

  const REASONING_PROFILE_LABELS = {
    GENERAL_RESEARCH: "profile.generalResearch",
    CUSTOMER_INSIGHT: "profile.customerInsight",
  };

  // Two inspectable finding kinds: a mismatch against an expectation/baseline,
  // and a repeated regularity across grounded observations.
  const DEVIATION_LABELS = {
    contradiction: "deviation.contradiction",
    excess_effort: "deviation.excessEffort",
    excess_payment: "deviation.excessPayment",
    persistence: "deviation.persistence",
    absence: "deviation.absence",
    other: "deviation.other",
  };

  // App-side quality warnings. These are computed deterministically after
  // the model has spoken; they are hints for the researcher, not verdicts.
  const QUALITY_FLAG_LABELS = {
    stated_need_echo: { label: "quality.statedNeedEcho.label", desc: "quality.statedNeedEcho.desc" },
    generic_term: { label: "quality.genericTerm.label", desc: "quality.genericTerm.desc" },
    no_trace: { label: "quality.noTrace.label", desc: "quality.noTrace.desc" },
    abduction_incomplete: { label: "quality.abductionIncomplete.label", desc: "quality.abductionIncomplete.desc" },
    insufficient_competing_hypotheses: { label: "quality.insufficientCompeting.label", desc: "quality.insufficientCompeting.desc" },
  };

  const PROMOTION_STATE_LABELS = {
    DRAFT: "promotionState.draft",
    RESEARCH_COMPLETE: "promotionState.researchComplete",
    HUMAN_REVIEW_REQUIRED: "promotionState.humanReviewRequired",
    PUBLICATION_READY: "promotionState.publicationReady",
    PUBLISHED: "promotionState.published",
    REJECTED_FOR_PUBLICATION: "promotionState.rejected",
  };

  const CONTRIBUTION_LABELS = {
    CORRECTION: "contribution.correction",
    REFINEMENT: "contribution.refinement",
    REPLICATION: "contribution.replication",
    DEFINITION_AUDIT: "contribution.definitionAudit",
    COUNTER_EVIDENCE: "contribution.counterEvidence",
    INCONCLUSIVE_BUT_DECISION_RELEVANT: "contribution.inconclusiveButDecisionRelevant",
    NOVEL_MISMATCH: "contribution.novelMismatch",
    OTHER: "contribution.other",
  };

  function qualityFlagMeta(code) {
    const keys = QUALITY_FLAG_LABELS[code];
    return keys ? { label: t(keys.label), desc: t(keys.desc) } : { label: String(code ?? ""), desc: "" };
  }

  function qualityBadgesHTML(flags, { withDesc = false } = {}) {
    if (!flags || flags.length === 0) return "";
    const badges = flags.map((f) => {
      const meta = qualityFlagMeta(f.code);
      const text = f.detail ? t("quality.badgeWithDetail", { label: meta.label, detail: f.detail }) : meta.label;
      return `<span class="quality-badge" title="${escapeHtml(meta.desc)}">⚠ ${escapeHtml(text)}</span>`;
    }).join("");
    if (!withDesc) return `<div class="quality-badges">${badges}</div>`;
    const descs = flags.map((f) => {
      const meta = qualityFlagMeta(f.code);
      return `<li><strong>${escapeHtml(meta.label)}${f.detail ? `（${escapeHtml(f.detail)}）` : ""}</strong> — ${escapeHtml(meta.desc)}</li>`;
    }).join("");
    return `
      <div class="quality-box">
        <div class="quality-box-title">${th("quality.boxTitle")}</div>
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
    if (buildInfo.demoBuild) return `<span class="badge demo">${th("build.demo")}</span>`;
    return `<span class="badge delivery">${th("build.delivery")}</span>`;
  }

  function confidentialBanner() {
    if (!buildInfo.clientName) return "";
    return `<div class="confidential-banner">${th("build.confidential", { client: buildInfo.clientName })}</div>`;
  }

  function layout(inner) {
    app.innerHTML = `
      <header class="top">
        <div class="brand"><a href="#/">Insight Lab</a> <small>${th("app.tagline")}</small></div>
        <div class="header-actions">
          ${buildBadge()}
          ${localeSelectHTML("locale-select-header")}
          <a class="settings-link" href="#/settings" title="${th("nav.settings")}">⚙ ${th("nav.settings")}</a>
        </div>
      </header>
      <main>
        ${confidentialBanner()}
        ${inner}
      </main>
      <footer class="privacy">
        ${th("app.privacy")}
      </footer>
    `;
    bindLocaleSelects();
  }

  function errorBox(message) {
    return message ? `<div class="error-box">${escapeHtml(message)}</div>` : "";
  }

  function backLink(href, key) {
    return `<a class="back-link" href="${href}">&larr; ${th(key)}</a>`;
  }

  function confidenceBar(value) {
    const pct = Math.round((value || 0) * 100);
    return `
      <div class="confidence-row">
        <span class="confidence-label">${th("insight.confidence")}</span>
        <div class="confidence-bar"><div class="confidence-fill" style="width:${pct}%"></div></div>
        <span class="confidence-pct">${escapeHtml(formatPercent(value))}</span>
      </div>`;
  }

  // ---------- Analysis runs ----------
  // Every result view is bound to exactly one analysis run. The run is named
  // by ?run=<analysisId> in the hash; without it the latest completed run is
  // shown. Queued, running and failed runs have no results to display.

  // NOT_RECORDED marks a provenance value the run did not record.
  const NOT_RECORDED = Symbol("not recorded");

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
    return { run: latestCompleted, notice: runID ? t("run.notFoundNotice") : "" };
  }

  function runProvenance(run) {
    return (run && run.metrics && run.metrics.provenance) || {};
  }

  function runLabel(run) {
    const when = formatDateTime(run.finishedAt || run.createdAt);
    const model = runProvenance(run).model;
    return `${when} · ${label(RUN_STATUS_LABELS, run.status)}${model ? ` · ${model}` : ""} · ${run.id}`;
  }

  // A deterministic run used no model; that is a recorded fact, not a gap.
  function modelScoped(prov, value) {
    if (value) return value;
    if (prov.mode === "deterministic") return t("provenance.noModel");
    return NOT_RECORDED;
  }

  function provenanceChipsHTML(run) {
    if (!run) return "";
    const prov = runProvenance(run);
    const fingerprint = prov.promptFingerprint ? prov.promptFingerprint.slice(0, 7) : "";
    const chip = (key, value, full) => {
      const missing = value === NOT_RECORDED;
      const shown = missing ? t("common.notRecorded") : value;
      return `<span class="chip${missing ? " chip-missing" : ""}" title="${escapeHtml(full || shown)}">${th(key)}: ${escapeHtml(shown)}</span>`;
    };
    return `<div class="chips">
        ${chip("provenance.mode", prov.mode ? label(MODE_LABELS, prov.mode) : NOT_RECORDED, prov.mode)}
        ${chip("provenance.model", modelScoped(prov, prov.model))}
        ${chip("provenance.prompt", modelScoped(prov, fingerprint), prov.promptFingerprint)}
        ${chip("provenance.rules", prov.ruleVersion || NOT_RECORDED)}
        ${chip("provenance.engine", engineLabel(run.executionSnapshot), run.executionSnapshot && run.executionSnapshot.gitCommit)}
        ${chip("provenance.execution", shortFingerprint(run.executionFingerprint), run.executionFingerprint)}
        ${chip("provenance.input", shortFingerprint(run.inputFingerprint), run.inputFingerprint)}
        ${run.researchQuestion ? chip("provenance.question", run.researchQuestion, run.researchQuestion) : ""}
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
        <label for="run-select">${th("run.selectorLabel")}</label>
        <select id="run-select">${selected ? "" : `<option value="" selected>${th("run.noCompleted")}</option>`}${options}</select>
      </div>`;
  }

  function runListHTML(projectID, analyses, selected) {
    if (!analyses.length) return "";
    const items = analyses.map((a) => {
      const current = selected && a.id === selected.id;
      const text = escapeHtml(runLabel(a));
      const body = a.status === "completed" && !current
        ? `<a href="${projectHash(projectID, "", a.id)}">${text}</a>`
        : `<span>${text}</span>`;
      return `<li class="run-item status-${escapeHtml(a.status)}${current ? " run-current" : ""}">${body}${current ? ` <strong>${th("run.shown")}</strong>` : ""}${a.status === "failed" && a.error ? ` <span class="meta">${escapeHtml(a.error)}</span>` : ""}</li>`;
    }).join("");
    return `<details class="run-list"><summary>${th("run.allRuns", { count: analyses.length })}</summary><ul>${items}</ul></details>`;
  }

  function runHeaderHTML(run) {
    if (!run) return `<div class="empty">${th("run.noCompletedYet")}</div>`;
    return `<div class="meta">${th("run.header", { id: trustedHtml(`<code>${escapeHtml(run.id)}</code>`), finished: formatDateTime(run.finishedAt) })}</div>${provenanceChipsHTML(run)}`;
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
              <div class="meta">${escapeHtml(formatDateTime(p.createdAt))}</div>
            </div>
            <span>${th("home.open")} &rarr;</span>
          </a>`).join("")}</div>`
      : `<div class="empty">${th("home.noProjects")}</div>`;

    layout(`
      <div class="hero">
        <h1>Insight Lab</h1>
        <p>${th("home.lead")}</p>
        <div class="actions">
          <button class="primary" id="try-demo" ${buildInfo.demoBuild ? "" : "disabled"}>${th("home.tryDemo")}</button>
          <button id="new-project">${th("home.newProject")}</button>
        </div>
        ${!buildInfo.demoBuild ? `<p class="hint">${th("home.deliveryHint", { command: trustedHtml("<code>make build-demo</code>") })}</p>` : ""}
      </div>
      ${errorBox(errorMessage || loadError)}
      <div class="section-title">${th("home.projects")}</div>
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
      const name = prompt(t("home.projectNamePrompt"));
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
      layout(`${backLink("#/", "nav.back")}${errorBox(e.message)}`);
      return;
    }

    layout(`
      ${backLink("#/", "nav.back")}
      <div class="card">
        <div class="section-title">${th("settings.displayLanguage")}</div>
        <div class="locale-setting">
          <label for="locale-select-settings">${th("locale.label")}</label>
          ${localeSelectHTML("locale-select-settings")}
        </div>
        <p class="hint">${th("settings.localeHint")}</p>
      </div>
      <div class="card">
        <div class="section-title">${th("settings.llm")}</div>
        ${errorBox(errorMessage)}
        ${notice ? `<div class="notice-box">${escapeHtml(notice)}</div>` : ""}
        <form class="paste-form" id="settings-form">
          <div>
            <label for="settings-base-url">${th("settings.baseUrl")}</label>
            <input type="text" id="settings-base-url" name="baseUrl" value="${escapeHtml(settings.baseUrl)}" placeholder="https://api.openai.com/v1">
          </div>
          <div>
            <label for="settings-model">${th("settings.model")}</label>
            <input type="text" id="settings-model" name="model" value="${escapeHtml(settings.model)}" placeholder="gpt-5">
          </div>
          <div>
            <label for="settings-api-key">${settings.hasApiKey ? th("settings.apiKeyConfigured", { masked: settings.maskedApiKey }) : th("settings.apiKey")}</label>
            <input type="password" id="settings-api-key" name="apiKey" placeholder="${settings.hasApiKey ? th("settings.apiKeyReplace") : "sk-..."}">
          </div>
          <div class="settings-actions">
            <button type="submit" class="primary">${th("settings.save")}</button>
            <button type="button" id="test-connection">${th("settings.testConnection")}</button>
          </div>
        </form>
        <p class="hint">${th("settings.apiKeyHint")}</p>
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
        renderSettings(null, t("settings.saved"));
      } catch (e) {
        renderSettings(e.message);
      }
    });

    document.getElementById("test-connection").addEventListener("click", async () => {
      try {
        const result = await api("/api/settings/test", { method: "POST" });
        renderSettings(null, t("settings.connectionOk", { mode: label(MODE_LABELS, result.mode) }));
      } catch (e) {
        renderSettings(t("settings.connectionFailed", { message: e.message }));
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
      layout(`${backLink("#/", "nav.backToProjects")}${errorBox(e.message)}`);
      return;
    }
    const selectedRunID = selectedRun ? selectedRun.id : "";

    const latestAnalysis = analyses[0] || null;
    const isRunning = latestAnalysis && (latestAnalysis.status === "running" || latestAnalysis.status === "queued");

    const docsHtml = documents.length
      ? documents.map((d) => `
          <div class="doc-item">
            <div class="doc-head">
              <span class="source-tag source-${escapeHtml(d.source)}">${escapeHtml(label(SOURCE_LABELS, d.source))}</span>
              <span class="doc-title">${d.title ? escapeHtml(d.title) : th("project.untitled")}</span>
            </div>
            <div class="doc-content">${escapeHtml(d.content)}</div>
          </div>`).join("")
      : `<div class="empty">${th("project.noDocuments")}</div>`;

    const insightsHtml = insights.length
      ? `<div class="insight-list">${insights.map((i) => `
          <a class="card insight-card${(i.qualityFlags || []).length ? " insight-card-flagged" : ""}" href="#/insights/${encodeURIComponent(i.id)}">
            <div class="insight-card-title">${escapeHtml(i.title)}</div>
            <div class="insight-card-latent">${escapeHtml(i.hypothesis || i.latentNeed)}</div>
            ${i.surprisingFact ? `<div class="insight-card-trace">${th("project.deviation", { fact: i.surprisingFact })}</div>` : ""}
            ${confidenceBar(i.confidence)}
            ${qualityBadgesHTML(i.qualityFlags)}
          </a>`).join("")}</div>`
      : `<div class="empty">${th("project.noInsights")}</div>`;

    const sourceOptions = Object.keys(SOURCE_LABELS).map((code) =>
      `<option value="${code}">${escapeHtml(label(SOURCE_LABELS, code))}</option>`).join("");
    const profileOptions = Object.keys(REASONING_PROFILE_LABELS).map((code) =>
      `<option value="${code}">${escapeHtml(label(REASONING_PROFILE_LABELS, code))}</option>`).join("");

    layout(`
      ${backLink("#/", "nav.backToProjects")}
      <div class="card">
        <div class="section-title">${th("project.title")}</div>
        <h2 style="margin:0 0 4px;">${escapeHtml(project.name)}</h2>
        <div class="meta">${th("project.counts", { documents: documents.length, runs: analyses.length, insights: insights.length })}</div>
      </div>

      ${errorBox(errorMessage)}
      ${runNotice ? `<div class="notice-box">${escapeHtml(runNotice)}</div>` : ""}

      <div class="card">
        <div class="section-title">${th("project.analysis")}</div>
        <div id="analysis-panel">
          ${analysisPanelHTML(latestAnalysis)}
        </div>
        ${runSelectorHTML(analyses, selectedRun)}
        ${provenanceChipsHTML(selectedRun)}
        ${runListHTML(projectID, analyses, selectedRun)}
        <div class="analysis-actions">
          <input id="research-question" class="analysis-question" type="text" maxlength="2000" placeholder="${th("project.questionPlaceholder")}" aria-label="${th("project.questionLabel")}">
          <select id="reasoning-profile" aria-label="${th("project.profileLabel")}">
            ${profileOptions}
          </select>
          <button class="primary" id="run-analysis" ${isRunning || documents.length === 0 ? "disabled" : ""}>${th("project.runAnalysis")}</button>
          <a class="btn" href="${projectHash(projectID, "patterns", selectedRunID)}">${th("project.viewPatterns")}</a>
          <a class="btn" href="${projectHash(projectID, "evaluation", selectedRunID)}">${th("project.viewEvaluation")}</a>
          <a class="btn" href="#/projects/${encodeURIComponent(projectID)}/research">${th("project.researchPublications")}</a>
          ${selectedRun ? `<a class="btn" href="/api/projects/${encodeURIComponent(projectID)}/report.md${runQuery(selectedRun)}" download>${th("project.downloadReport")}</a>` : ""}
        </div>
      </div>

      <div class="card">
        <div class="section-title">${selectedRun ? th("project.insightsForRun", { id: selectedRun.id }) : th("project.insights")}</div>
        ${insightsHtml}
      </div>

      <div class="card">
        <div class="section-title">${th("project.pasteText")}</div>
        <form class="paste-form" id="paste-form">
          <div>
            <label for="paste-source">${th("project.sourceType")}</label>
            <select id="paste-source" name="source">${sourceOptions}</select>
          </div>
          <div><label for="paste-title">${th("project.docTitle")}</label><input type="text" id="paste-title" name="title" placeholder="${th("project.docTitlePlaceholder")}"></div>
          <div><label for="paste-content">${th("project.docContent")}</label><textarea id="paste-content" name="content" placeholder="${th("project.docContentPlaceholder")}" required></textarea></div>
          <div><button type="submit" class="primary">${th("project.addDocument")}</button></div>
        </form>
      </div>

      <div class="card">
        <div class="section-title">${th("project.importCsv")}</div>
        <p class="hint">${th("project.importCsvHint")}</p>
        <form id="csv-form">
          <input type="file" name="file" accept=".csv,text/csv" required aria-label="${th("project.csvFile")}">
          <button type="submit" class="primary">${th("project.import")}</button>
        </form>
        <div id="csv-result"></div>
      </div>

      <div class="card">
        <div class="section-title">${th("project.importAnalysisCsv")}</div>
        <p class="hint">${th("project.importAnalysisCsvHint")}</p>
        <form id="analysis-csv-form">
          <input type="file" name="file" accept=".csv,text/csv" required aria-label="${th("project.csvFile")}">
          <button type="submit" class="primary">${th("project.importAnalysisData")}</button>
        </form>
        <div id="analysis-csv-result"></div>
      </div>

      <div class="card">
        <div class="section-title">${th("project.documents")}</div>
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
          `<div class="notice-box">${th("project.csvImported", { imported: result.imported, skipped: result.skipped })}</div>`;
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
          `<div class="notice-box">${th("project.analysisCsvImported", { read: result.recordsRead, imported: result.imported, skipped: result.skipped })}</div>`;
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
        const reasoningProfile = document.getElementById("reasoning-profile")?.value || "GENERAL_RESEARCH";
        const analysis = await api(`/api/projects/${encodeURIComponent(projectID)}/analysis`, {
          method: "POST",
          body: JSON.stringify({ researchQuestion, reasoningProfile }),
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

  function progressHTML(stepLabel, progress, serverMessage) {
    return `
      <div class="analysis-status status-running">
        <div class="progress-label"${serverMessage ? ` title="${escapeHtml(serverMessage)}"` : ""}>${escapeHtml(stepLabel)}</div>
        <div class="progress-bar"><div class="progress-fill" style="width:${progress}%"></div></div>
      </div>`;
  }

  function analysisPanelHTML(analysis) {
    if (!analysis) return `<div class="empty">${th("analysis.none")}</div>`;
    if (analysis.status === "completed") {
      return `<div class="analysis-status status-completed">${th("analysis.completed", { finished: formatDateTime(analysis.finishedAt) })}</div>`;
    }
    if (analysis.status === "failed") {
      return `<div class="analysis-status status-failed">${th("analysis.failed", { message: analysis.error || "" })}</div>`;
    }
    const step = analysis.currentStep ? label(STEP_LABELS, analysis.currentStep) : label(RUN_STATUS_LABELS, analysis.status);
    return progressHTML(t("analysis.inProgress", { step }), analysis.progress);
  }

  function watchAnalysis(projectID, analysisID) {
    closeActiveStream();
    const panel = document.getElementById("analysis-panel");
    const es = new EventSource(`/api/analysis/${encodeURIComponent(analysisID)}/events`);
    activeEventSource = es;

    es.addEventListener("progress", (ev) => {
      const data = JSON.parse(ev.data);
      // A known step gets the localized label; the server's own message
      // stays inspectable as a tooltip, and an unknown step stays visible.
      const known = Object.prototype.hasOwnProperty.call(STEP_LABELS, data.step);
      const stepLabel = known ? t(STEP_LABELS[data.step]) : (data.message || data.step);
      if (panel) panel.innerHTML = progressHTML(stepLabel, data.progress, known ? data.message : "");
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
      renderProject(projectID, t("analysis.streamFailed", { message: data.message || t("analysis.unknownError") }));
    });
  }

  // ---------- Insight detail ----------

  async function renderInsight(insightID) {
    let insight;
    try {
      insight = await api(`/api/insights/${encodeURIComponent(insightID)}`);
    } catch (e) {
      layout(`${backLink("#/", "nav.back")}${errorBox(e.message)}`);
      return;
    }

    const support = insight.evidence.filter((e) => e.type === "support");
    const counter = insight.evidence.filter((e) => e.type === "counter");
    const field = (key, value, cls) => `
        <div class="field-block${cls ? ` ${cls}` : ""}">
          <div class="field-label">${th(key)}</div>
          <div>${escapeHtml(value || "-")}</div>
        </div>`;

    layout(`
      ${backLink(`#/projects/${encodeURIComponent(insight.projectId)}`, "nav.backToProject")}

      <div class="card reasoning-trail">
        <div class="section-title">${th("insight.trailTitle")}</div>
        <p class="hint">${th("insight.trailHint")}</p>
        ${abductionHTML(insight)}
        <div class="trail-subtitle">${th("insight.sourceObservations")}</div>
        ${patternsSectionHTML(insight.patterns)}
      </div>

      <div class="card">
        <div class="section-title">${th("insight.title")}</div>
        <h2 style="margin:0 0 12px;">${escapeHtml(insight.title)}</h2>
        ${confidenceBar(insight.confidence)}
        ${qualityBadgesHTML(insight.qualityFlags, { withDesc: true })}
        ${field("insight.observation", insight.observation, "fact-block")}
        ${field("insight.statedNeed", insight.statedNeed)}
        ${field("insight.hypothesis", insight.latentNeed, "latent-block")}
        ${field("insight.jtbd", insight.jtbd)}
        ${field("insight.interpretation", insight.interpretation, "interpretation-block")}
        ${field("insight.alternative", insight.alternativeInterpretation, "alt-block")}
        ${insight.productOpportunity ? field("insight.productOpportunity", insight.productOpportunity) : ""}
        ${insight.monetizationAngle ? field("insight.monetizationAngle", insight.monetizationAngle, "money-block") : ""}
      </div>

      <div class="card">
        <div class="section-title">${th("insight.evidence")}</div>
        ${support.length ? support.map(evidenceRowHTML).join("") : `<div class="empty">${th("common.none")}</div>`}
      </div>

      <div class="card">
        <div class="section-title">${th("insight.counterEvidence")}</div>
        ${counter.length ? counter.map(evidenceRowHTML).join("") : `<div class="empty">${th("insight.noCounterEvidence")}</div>`}
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
    const step = (n, key, value, cls) => `
      <div class="abduction-step ${cls}">
        <div class="abduction-num">${n}</div>
        <div class="abduction-body">
          <div class="field-label">${th(key)}</div>
          <div>${value ? escapeHtml(value) : `<span class="missing">${th("common.notRecordedTitle")}</span>`}</div>
        </div>
      </div>`;
    return `
      <div class="abduction">
        ${step("1", "insight.expectation", insight.expectation, "abduction-expect")}
        ${step("2", "insight.surprisingFact", insight.surprisingFact, "abduction-fact")}
        ${step("3", "insight.hypothesis", insight.latentNeed, "abduction-hyp")}
        ${step("4", "insight.explanation", insight.rationale, "abduction-why")}
      </div>`;
  }

  function patternBlockHTML(p) {
    const isTrace = p.kind === "deviation";
    const kindBadge = isTrace
      ? `<span class="kind-badge kind-deviation">${th("pattern.trace")}</span>${p.deviationType ? `<span class="deviation-type">${escapeHtml(label(DEVIATION_LABELS, p.deviationType))}</span>` : ""}`
      : `<span class="kind-badge kind-repetition">${th("pattern.repetition")}</span>`;
    const body = isTrace
      ? `
        <div class="trace-gap">
          <div class="trace-cell trace-expect"><div class="field-label">${th("pattern.expected")}</div><div>${escapeHtml(p.expectation || "-")}</div></div>
          <div class="trace-arrow">≠</div>
          <div class="trace-cell trace-actual"><div class="field-label">${th("pattern.observed")}</div><div>${escapeHtml(p.description || "-")}</div></div>
        </div>`
      : (p.description ? `<div class="pattern-desc">${escapeHtml(p.description)}</div>` : "");
    return `
      <div class="pattern-block ${isTrace ? "pattern-trace" : ""}">
        <div class="pattern-head">${kindBadge}<span class="pattern-title">${escapeHtml(p.title)}</span></div>
        ${body}
        <div class="pattern-observations">
          ${(p.observations || []).map(evidenceRowHTML).join("") || `<div class="empty">${th("pattern.noObservations")}</div>`}
        </div>
      </div>`;
  }

  function patternsSectionHTML(patterns) {
    if (!patterns || patterns.length === 0) {
      return `<div class="empty">${th("pattern.noneForInsight")}</div>`;
    }
    const traces = patterns.filter((p) => p.kind === "deviation");
    const repetitions = patterns.filter((p) => p.kind !== "deviation");
    let html = "";
    if (traces.length) html += traces.map(patternBlockHTML).join("");
    else html += `<div class="notice-box quality-notice">${th("pattern.repetitionOnly")}</div>`;
    if (repetitions.length) html += repetitions.map(patternBlockHTML).join("");
    return html;
  }

  function evidenceRowHTML(e) {
    return `
      <div class="evidence-row">
        <button class="evidence-toggle" data-doc-id="${escapeHtml(e.documentId)}" data-start="${e.startOffset}" data-end="${e.endOffset}">
          "${escapeHtml(e.quote)}" <span class="evidence-reveal">${th("evidence.viewInSource")} &darr;</span>
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
      if (!run) throw new Error(t("run.noCompletedYet"));
      metrics = await api(`/api/projects/${encodeURIComponent(projectID)}/evaluation${runQuery(run)}`);
    } catch (e) {
      layout(`${backLink(projectHash(projectID, "", runID), "nav.backToProject")}${errorBox(e.message)}`);
      return;
    }

    const rows = [
      ["evaluation.evidenceCoverage.label", "evaluation.evidenceCoverage.desc", metrics.evidenceCoverage],
      ["evaluation.unsupportedClaimRate.label", "evaluation.unsupportedClaimRate.desc", metrics.unsupportedClaimRate],
      ["evaluation.counterEvidenceCoverage.label", "evaluation.counterEvidenceCoverage.desc", metrics.counterEvidenceCoverage],
      ["evaluation.insightDuplication.label", "evaluation.insightDuplication.desc", metrics.insightDuplicationRate],
      ["evaluation.traceBacked.label", "evaluation.traceBacked.desc", metrics.traceBackedInsightRate],
      ["evaluation.qualityFlagged.label", "evaluation.qualityFlagged.desc", metrics.qualityFlaggedInsightRate],
    ];
    const tile = (value, labelKey, descKey) => `
            <div class="metric-tile">
              <div class="metric-value">${escapeHtml(value)}</div>
              <div class="metric-label">${th(labelKey)}</div>
              <div class="metric-desc">${th(descKey)}</div>
            </div>`;
    const flagCounts = metrics.qualityFlagCounts || {};
    const flagSummary = Object.keys(QUALITY_FLAG_LABELS)
      .filter((code) => flagCounts[code])
      .map((code) => `${qualityFlagMeta(code).label}: ${flagCounts[code]}`)
      .join(" / ");

    layout(`
      ${backLink(projectHash(projectID, "", run.id), "nav.backToProject")}
      <div class="card">
        <div class="section-title">${th("evaluation.title", { project: project.name })}</div>
        ${runHeaderHTML(run)}
        <div class="metric-grid">
          ${rows.map(([labelKey, descKey, value]) => tile(formatPercent(value), labelKey, descKey)).join("")}
          ${tile(formatNumber(metrics.averageEvidencePerInsight, 1), "evaluation.avgEvidence.label", "evaluation.avgEvidence.desc")}
        </div>
        <p class="hint">
          ${th("evaluation.summaryObservations", { grounded: metrics.groundedObservations, total: metrics.totalObservationCandidates })}
          ${th("evaluation.summaryFindings", { patterns: metrics.patternCount, traces: metrics.traceCount || 0, drafts: metrics.totalInsightDrafts, final: metrics.finalInsightCount })}
          ${flagSummary ? th("evaluation.qualityWarnings", { summary: flagSummary }) : th("evaluation.noQualityWarnings")}
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
      layout(`${backLink(projectHash(projectID, "", runID), "nav.backToProject")}${errorBox(e.message)}`);
      return;
    }

    const traces = patterns.filter((p) => p.kind === "deviation");
    const repetitions = patterns.filter((p) => p.kind !== "deviation");

    layout(`
      ${backLink(projectHash(projectID, "", run ? run.id : ""), "nav.backToProject")}
      <div class="card">
        <div class="section-title">${th("patterns.title", { project: project.name })}</div>
        ${runHeaderHTML(run)}
        <p class="hint">${th("patterns.hint")}</p>
      </div>
      <div class="card">
        <div class="section-title">${th("patterns.traces", { count: traces.length })}</div>
        <p class="hint">${th("patterns.tracesHint")}</p>
        ${traces.length ? traces.map(patternBlockHTML).join("") : `<div class="empty">${th("patterns.noTraces")}</div>`}
      </div>
      <div class="card">
        <div class="section-title">${th("patterns.repetitions", { count: repetitions.length })}</div>
        <p class="hint">${th("patterns.repetitionsHint")}</p>
        ${repetitions.length ? repetitions.map(patternBlockHTML).join("") : `<div class="empty">${th("patterns.noRepetitions")}</div>`}
      </div>
    `);

    bindEvidenceToggles();
  }

  // ---------- Research publications ----------

  async function renderResearchRuns(projectID) {
    try {
      const runs = await api(`/api/projects/${encodeURIComponent(projectID)}/research-runs`);
      layout(`${backLink(`#/projects/${encodeURIComponent(projectID)}`, "nav.backToProject")}
        <h1>${th("research.title")}</h1>
        <div class="card">${(runs || []).map(run => `<p><a href="#/research-runs/${encodeURIComponent(run.id)}">${escapeHtml(run.question)}</a></p>`).join("") || th("research.none")}</div>
        <form id="new-research" class="card"><label>${th("research.question")} <input name="question" required></label><button class="primary">${th("research.create")}</button></form>
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
      if (!iteration) { layout(errorBox(t("promotion.noIterations"))); return; }
      const promotion = iteration.promotion || {state: "DRAFT"};
      const gate = iteration.promotionGateInput || {};
      const state = promotion.state || "DRAFT";
      const terminal = ["PUBLISHED", "REJECTED_FOR_PUBLICATION"].includes(state);
      const judgments = [
        ["competingHypothesisConsidered", "promotion.judgment.competingHypothesisConsidered"],
        ["independentValidationStatusAccurate", "promotion.judgment.independentValidationStatusAccurate"],
        ["decisionReadinessHonestlyStated", "promotion.judgment.decisionReadinessHonestlyStated"],
        ["makesStrongClaim", "promotion.judgment.makesStrongClaim"],
        ["hasUnresolvedCriticalGap", "promotion.judgment.hasUnresolvedCriticalGap"],
        ["humanReviewCompleted", "promotion.judgment.humanReviewCompleted"],
      ];
      layout(`${backLink(`#/projects/${encodeURIComponent(run.projectId)}/research`, "nav.backToResearch")}
        <h1>${escapeHtml(run.question)}</h1>
        <div class="card"><h2>${escapeHtml(label(PROMOTION_STATE_LABELS, state))} <code>${escapeHtml(state)}</code></h2>
          <p>${th("promotion.iteration", { sequence: iteration.sequence })}</p>
          <ul>${(promotion.reasons || []).map(reason => `<li>${escapeHtml(reason)}</li>`).join("")}</ul>
          <p><a href="${base}/report.md" download>${th("promotion.report")}</a> · <a href="${base}/artifact.json" target="_blank" rel="noopener">${th("promotion.currentArtifact")}</a>
          ${iteration.approvedArtifact ? ` · <a href="${base}/approved-artifact.json" target="_blank" rel="noopener">${th("promotion.approvedArtifact")}</a>` : ""}</p>
          ${iteration.approvedArtifact ? `<p>${th("promotion.approvedReference", { reference: trustedHtml(`<code>${escapeHtml(iteration.approvedArtifact.reference)}</code>`) })}</p>` : ""}
          <ul>${Object.entries(gate.Checklist || {}).map(([key, value]) => `<li>${value ? "✓" : th("promotion.missing")} ${escapeHtml(key)}</li>`).join("")}</ul>
        </div>
        ${terminal ? "" : `<form id="promotion-review" class="card"><h2>${th("promotion.reviewTitle")}</h2>
          <p>${th("promotion.reviewHint")}</p>
          <label>${th("promotion.contribution")} <select name="contribution" required><option value="">${th("promotion.chooseContribution")}</option>${Object.keys(CONTRIBUTION_LABELS).map(value => `<option value="${value}">${escapeHtml(label(CONTRIBUTION_LABELS, value))}</option>`).join("")}</select></label>
          ${judgments.map(([key, labelKey]) => `<p><label><input type="checkbox" name="${key}"> ${th(labelKey)}</label></p>`).join("")}
          <button class="primary">${th("promotion.saveReview")}</button></form>
          <div class="card">${state === "PUBLICATION_READY" && iteration.approvedArtifact ? `<button id="mark-published">${th("promotion.markPublished")}</button><p>${th("promotion.markPublishedHint")}</p>` : ""}
          <button id="reject-publication">${th("promotion.reject")}</button></div>`}
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

  // route renders the view for the current hash and returns its promise so a
  // locale switch can restore page state after the new render.
  function route() {
    closeActiveStream();
    const [hash, query = ""] = (location.hash || "#/").split("?");
    const runID = new URLSearchParams(query).get("run") || "";

    const researchMatch = hash.match(/^#\/projects\/([^/]+)\/research$/);
    if (researchMatch) return renderResearchRuns(decodeURIComponent(researchMatch[1]));
    const promotionMatch = hash.match(/^#\/research-runs\/([^/]+)$/);
    if (promotionMatch) return renderPromotion(decodeURIComponent(promotionMatch[1]));

    const patternsMatch = hash.match(/^#\/projects\/([^/]+)\/patterns$/);
    if (patternsMatch) return renderPatterns(decodeURIComponent(patternsMatch[1]), runID);

    const evalMatch = hash.match(/^#\/projects\/([^/]+)\/evaluation$/);
    if (evalMatch) return renderEvaluation(decodeURIComponent(evalMatch[1]), runID);

    const projectMatch = hash.match(/^#\/projects\/([^/]+)$/);
    if (projectMatch) return renderProject(decodeURIComponent(projectMatch[1]), undefined, runID);

    const insightMatch = hash.match(/^#\/insights\/([^/]+)$/);
    if (insightMatch) return renderInsight(decodeURIComponent(insightMatch[1]));

    if (hash === "#/settings") return renderSettings();

    return renderHome();
  }

  async function boot() {
    try {
      await loadDictionaries();
    } catch (e) {
      // Without dictionaries every label would be a missing-key marker;
      // fail visibly instead.
      app.innerHTML = `<div class="error-box">Insight Lab could not load its UI text (${escapeHtml(e.message)}).</div>`;
      return;
    }
    applyLocale(initialLocale());
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
