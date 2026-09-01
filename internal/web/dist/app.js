(() => {
  "use strict";

  const app = document.getElementById("app");
  let buildInfo = { demoBuild: false, clientName: "" };

  const SOURCE_LABELS = {
    interview: "インタビュー",
    review: "レビュー",
    support: "問い合わせ",
    sales: "商談ログ",
    survey: "アンケート",
    job_posting: "案件・募集文",
    social_post: "SNS投稿",
  };

  const STEP_LABELS = {
    starting: "解析を開始しています",
    extracting_observations: "インタビューを読んでいます",
    detecting_patterns: "繰り返しのパターンを探しています",
    generating_hypotheses: "潜在ニーズの仮説を立てています",
    searching_evidence: "根拠と反証を探しています",
    deduplicating_insights: "重複する洞察を統合しています",
    scoring_confidence: "確信度を計算しています",
    completed: "完了しました",
  };

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
    if (buildInfo.demoBuild) return `<span class="badge demo">デモビルド</span>`;
    return `<span class="badge delivery">納品ビルド（デモデータなし）</span>`;
  }

  function confidentialBanner() {
    if (!buildInfo.clientName) return "";
    return `<div class="confidential-banner">Confidential — ${escapeHtml(buildInfo.clientName)} 様向け納品版</div>`;
  }

  function layout(inner) {
    app.innerHTML = `
      <header class="top">
        <div class="brand"><a href="#/">Insight Lab</a> <small>Hidden Needs Finder</small></div>
        <div class="header-actions">
          ${buildBadge()}
          <a class="settings-link" href="#/settings" title="設定">⚙ 設定</a>
        </div>
      </header>
      <main>
        ${confidentialBanner()}
        ${inner}
      </main>
      <footer class="privacy">
        アップロードされたデータはローカルに保存されます。解析に必要なテキストのみ、設定されたAIプロバイダへ送信されます。
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
              <div class="meta">${new Date(p.createdAt).toLocaleString("ja-JP")}</div>
            </div>
            <span>開く &rarr;</span>
          </a>`).join("")}</div>`
      : `<div class="empty">まだプロジェクトがありません。デモを試すか、新規プロジェクトを作成してください。</div>`;

    layout(`
      <div class="hero">
        <h1>Insight Lab</h1>
        <p>顧客が言葉にしていないニーズを見つける。</p>
        <div class="actions">
          <button class="primary" id="try-demo" ${buildInfo.demoBuild ? "" : "disabled"}>デモを試す</button>
          <button id="new-project">新規プロジェクト</button>
        </div>
        ${!buildInfo.demoBuild ? `<p class="hint">この納品ビルドにはデモデータが含まれていません。デモビルド（<code>make build-demo</code>）で起動すると試せます。</p>` : ""}
      </div>
      ${errorBox(errorMessage || loadError)}
      <div class="section-title">プロジェクト</div>
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
      const name = prompt("プロジェクト名を入力してください");
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
      layout(`<a class="back-link" href="#/">&larr; 戻る</a>${errorBox(e.message)}`);
      return;
    }

    layout(`
      <a class="back-link" href="#/">&larr; 戻る</a>
      <div class="card">
        <div class="section-title">LLM 設定</div>
        ${errorBox(errorMessage)}
        ${notice ? `<div class="notice-box">${escapeHtml(notice)}</div>` : ""}
        <form class="paste-form" id="settings-form">
          <div>
            <label>Base URL（OpenAI互換エンドポイント）</label>
            <input type="text" name="baseUrl" value="${escapeHtml(settings.baseUrl)}" placeholder="https://api.openai.com/v1">
          </div>
          <div>
            <label>Model</label>
            <input type="text" name="model" value="${escapeHtml(settings.model)}" placeholder="gpt-5">
          </div>
          <div>
            <label>API Key ${settings.hasApiKey ? `（設定済み: ${escapeHtml(settings.maskedApiKey)}）` : ""}</label>
            <input type="password" name="apiKey" placeholder="${settings.hasApiKey ? "変更する場合のみ入力" : "sk-..."}">
          </div>
          <div class="settings-actions">
            <button type="submit" class="primary">保存する</button>
            <button type="button" id="test-connection">接続テスト</button>
          </div>
        </form>
        <p class="hint">APIキーはローカルディスクやデータベースには保存されません。プロセスのメモリ上にのみ保持されます。</p>
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
        renderSettings(null, "設定を保存しました");
      } catch (e) {
        renderSettings(e.message);
      }
    });

    document.getElementById("test-connection").addEventListener("click", async () => {
      try {
        const result = await api("/api/settings/test", { method: "POST" });
        renderSettings(null, `接続に成功しました（モード: ${result.mode}）`);
      } catch (e) {
        renderSettings(`接続テストに失敗しました: ${e.message}`);
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
      layout(`<a class="back-link" href="#/">&larr; プロジェクト一覧に戻る</a>${errorBox(e.message)}`);
      return;
    }

    const latestAnalysis = analyses[0] || null;
    const isRunning = latestAnalysis && (latestAnalysis.status === "running" || latestAnalysis.status === "queued");

    const docsHtml = documents.length
      ? documents.map((d) => `
          <div class="doc-item">
            <div class="doc-head">
              <span class="source-tag source-${escapeHtml(d.source)}">${escapeHtml(SOURCE_LABELS[d.source] || d.source)}</span>
              <span class="doc-title">${escapeHtml(d.title || "(無題)")}</span>
            </div>
            <div class="doc-content">${escapeHtml(d.content)}</div>
          </div>`).join("")
      : `<div class="empty">まだドキュメントがありません。下のフォームからテキストを貼り付けてください。</div>`;

    const insightsHtml = insights.length
      ? `<div class="insight-list">${insights.map((i) => `
          <a class="card insight-card" href="#/insights/${encodeURIComponent(i.id)}">
            <div class="insight-card-title">${escapeHtml(i.title)}</div>
            <div class="insight-card-latent">${escapeHtml(i.latentNeed)}</div>
            ${confidenceBar(i.confidence)}
          </a>`).join("")}</div>`
      : `<div class="empty">まだ洞察はありません。解析を実行してください。</div>`;

    layout(`
      <a class="back-link" href="#/">&larr; プロジェクト一覧に戻る</a>
      <div class="card">
        <div class="section-title">プロジェクト</div>
        <h2 style="margin:0 0 4px;">${escapeHtml(project.name)}</h2>
        <div class="meta">ドキュメント ${documents.length} 件 / 洞察 ${insights.length} 件</div>
      </div>

      ${errorBox(errorMessage)}

      <div class="card">
        <div class="section-title">解析</div>
        <div id="analysis-panel">
          ${analysisPanelHTML(latestAnalysis)}
        </div>
        <div class="analysis-actions">
          <button class="primary" id="run-analysis" ${isRunning || documents.length === 0 ? "disabled" : ""}>解析を実行</button>
          <a class="btn" href="#/projects/${encodeURIComponent(projectID)}/patterns">検出されたパターン一覧</a>
          <a class="btn" href="#/projects/${encodeURIComponent(projectID)}/evaluation">評価指標を見る</a>
        </div>
      </div>

      <div class="card">
        <div class="section-title">洞察（Insights）</div>
        ${insightsHtml}
      </div>

      <div class="card">
        <div class="section-title">テキストを貼り付け</div>
        <form class="paste-form" id="paste-form">
          <div>
            <label>種類</label>
            <select name="source">
              <option value="interview">インタビュー</option>
              <option value="review">レビュー</option>
              <option value="support">問い合わせ</option>
              <option value="sales">商談ログ</option>
              <option value="survey">アンケート</option>
              <option value="job_posting">案件・募集文</option>
              <option value="social_post">SNS投稿</option>
            </select>
          </div>
          <div><label>タイトル</label><input type="text" name="title" placeholder="例: Interview #15"></div>
          <div><label>本文</label><textarea name="content" placeholder="発言をそのまま貼り付けてください" required></textarea></div>
          <div><button type="submit" class="primary">追加する</button></div>
        </form>
      </div>

      <div class="card">
        <div class="section-title">CSVインポート</div>
        <p class="hint">列: id,source,title,content（source は interview/review/support/sales/survey のいずれか）</p>
        <form id="csv-form">
          <input type="file" name="file" accept=".csv,text/csv" required>
          <button type="submit" class="primary">インポート</button>
        </form>
        <div id="csv-result"></div>
      </div>

      <div class="card">
        <div class="section-title">ドキュメント</div>
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
          `<div class="notice-box">${result.imported}件取り込み、${result.skipped}件スキップしました。</div>`;
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
    if (!analysis) return `<div class="empty">まだ解析を実行していません。</div>`;
    const label = STEP_LABELS[analysis.currentStep] || analysis.currentStep || analysis.status;
    if (analysis.status === "completed") {
      return `<div class="analysis-status status-completed">完了（${new Date(analysis.finishedAt).toLocaleString("ja-JP")}）</div>`;
    }
    if (analysis.status === "failed") {
      return `<div class="analysis-status status-failed">失敗: ${escapeHtml(analysis.error || "")}</div>`;
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
      renderProject(projectID, `解析に失敗しました: ${data.message || "unknown error"}`);
    });
  }

  // ---------- Insight detail ----------

  async function renderInsight(insightID) {
    let insight;
    try {
      insight = await api(`/api/insights/${encodeURIComponent(insightID)}`);
    } catch (e) {
      layout(`<a class="back-link" href="#/">&larr; 戻る</a>${errorBox(e.message)}`);
      return;
    }

    const support = insight.evidence.filter((e) => e.type === "support");
    const counter = insight.evidence.filter((e) => e.type === "counter");

    layout(`
      <a class="back-link" href="#/projects/${encodeURIComponent(insight.projectId)}">&larr; プロジェクトに戻る</a>

      <div class="card reasoning-trail">
        <div class="section-title">推論の過程 &mdash; どうやってこの洞察に至ったか</div>
        <p class="hint">AIが最終的な洞察だけを提示するのではなく、複数の発言から気づいたパターンと、そこからの推論の飛躍を、たどれるようにしています。</p>
        ${patternsSectionHTML(insight.patterns)}
        ${insight.rationale ? `
        <div class="field-block rationale-block">
          <div class="field-label">Rationale（なぜこの仮説に至ったか）</div>
          <div>${escapeHtml(insight.rationale)}</div>
        </div>` : ""}
      </div>

      <div class="card">
        <div class="section-title">Insight</div>
        <h2 style="margin:0 0 12px;">${escapeHtml(insight.title)}</h2>
        ${confidenceBar(insight.confidence)}

        <div class="field-block fact-block">
          <div class="field-label">Observation（観察された事実）</div>
          <div>${escapeHtml(insight.observation || "-")}</div>
        </div>
        <div class="field-block">
          <div class="field-label">Stated Need（表面的なニーズ）</div>
          <div>${escapeHtml(insight.statedNeed || "-")}</div>
        </div>
        <div class="field-block latent-block">
          <div class="field-label">Latent Need（潜在ニーズ）</div>
          <div>${escapeHtml(insight.latentNeed || "-")}</div>
        </div>
        <div class="field-block">
          <div class="field-label">JTBD</div>
          <div>${escapeHtml(insight.jtbd || "-")}</div>
        </div>
        <div class="field-block interpretation-block">
          <div class="field-label">Interpretation（AIによる解釈）</div>
          <div>${escapeHtml(insight.interpretation || "-")}</div>
        </div>
        <div class="field-block alt-block">
          <div class="field-label">Alternative Interpretation（別の解釈）</div>
          <div>${escapeHtml(insight.alternativeInterpretation || "-")}</div>
        </div>
        <div class="field-block">
          <div class="field-label">Product Opportunity（改善提案）</div>
          <div>${escapeHtml(insight.productOpportunity || "-")}</div>
        </div>
        ${insight.monetizationAngle ? `
        <div class="field-block money-block">
          <div class="field-label">Monetization Angle（自分で売るなら）</div>
          <div>${escapeHtml(insight.monetizationAngle)}</div>
        </div>` : ""}
      </div>

      <div class="card">
        <div class="section-title">Evidence（根拠）</div>
        ${support.length ? support.map(evidenceRowHTML).join("") : `<div class="empty">なし</div>`}
      </div>

      <div class="card">
        <div class="section-title">Counter Evidence（反証）</div>
        ${counter.length ? counter.map(evidenceRowHTML).join("") : `<div class="empty">反証は見つかりませんでした</div>`}
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

  function patternsSectionHTML(patterns) {
    if (!patterns || patterns.length === 0) {
      return `<div class="empty">この洞察の元になった繰り返しパターンは記録されていません。</div>`;
    }
    return patterns.map((p) => `
      <div class="pattern-block">
        <div class="pattern-title">${escapeHtml(p.title)}</div>
        ${p.description ? `<div class="pattern-desc">${escapeHtml(p.description)}</div>` : ""}
        <div class="pattern-observations">
          ${(p.observations || []).map(evidenceRowHTML).join("") || `<div class="empty">観察が見つかりません</div>`}
        </div>
      </div>`).join("");
  }

  function evidenceRowHTML(e) {
    return `
      <div class="evidence-row">
        <button class="evidence-toggle" data-doc-id="${escapeHtml(e.documentId)}" data-start="${e.startOffset}" data-end="${e.endOffset}">
          "${escapeHtml(e.quote)}" <span class="evidence-reveal">原文を見る &darr;</span>
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
      layout(`<a class="back-link" href="#/projects/${encodeURIComponent(projectID)}">&larr; プロジェクトに戻る</a>${errorBox(e.message)}`);
      return;
    }

    const rows = [
      ["Evidence Coverage", metrics.evidenceCoverage, "根拠Evidenceを持つInsightの割合"],
      ["Unsupported Claim Rate", metrics.unsupportedClaimRate, "原文照合できず破棄された引用の割合"],
      ["Counter Evidence Coverage", metrics.counterEvidenceCoverage, "反証を検索したInsightの割合"],
      ["Insight Duplication", metrics.insightDuplicationRate, "重複として統合されたInsightの割合"],
    ];

    layout(`
      <a class="back-link" href="#/projects/${encodeURIComponent(projectID)}">&larr; プロジェクトに戻る</a>
      <div class="card">
        <div class="section-title">評価指標 — ${escapeHtml(project.name)}</div>
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
            <div class="metric-desc">Insightあたりの平均Evidence数</div>
          </div>
        </div>
        <p class="hint">
          観察候補 ${metrics.totalObservationCandidates} 件中 ${metrics.groundedObservations} 件を原文照合できました。
          検出されたパターン ${metrics.patternCount} 件から、洞察候補 ${metrics.totalInsightDrafts} 件が生まれ、最終的に ${metrics.finalInsightCount} 件が採用されました。
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
      layout(`<a class="back-link" href="#/projects/${encodeURIComponent(projectID)}">&larr; プロジェクトに戻る</a>${errorBox(e.message)}`);
      return;
    }

    layout(`
      <a class="back-link" href="#/projects/${encodeURIComponent(projectID)}">&larr; プロジェクトに戻る</a>
      <div class="card">
        <div class="section-title">検出されたパターン — ${escapeHtml(project.name)}</div>
        <p class="hint">複数のドキュメントにまたがって繰り返し現れた行動・発言です。最終的なInsightに至らなかったパターンもここに残ります。</p>
      </div>
      <div class="card">
        ${patterns.length ? patternsSectionHTML(patterns) : `<div class="empty">まだパターンは検出されていません。解析を実行してください。</div>`}
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
