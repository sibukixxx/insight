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
    detecting_traces: "常識的な予想とのズレ（欲望の痕跡）を探しています",
    detecting_patterns: "繰り返しのパターンを探しています",
    generating_hypotheses: "潜在ニーズの仮説を立てています",
    searching_evidence: "根拠と反証を探しています",
    deduplicating_insights: "重複する洞察を統合しています",
    scoring_confidence: "確信度を計算しています",
    completed: "完了しました",
  };

  // Two kinds of "noticing" the pipeline records (see docs/detailed-design.md §23):
  // a deviation is a behavior that broke a common-sense expectation (the
  // trace an unconscious desire leaves behind); a repetition is a behavior
  // seen across several people.
  const DEVIATION_LABELS = {
    contradiction: "言行不一致",
    excess_effort: "急いでいるのに手間をかける",
    excess_payment: "予定より多く払う",
    persistence: "不満なのに使い続ける",
    absence: "起きるはずの行動がない",
    other: "その他の不合理な行動",
  };

  // App-side quality warnings. These are computed deterministically after
  // the model has spoken; they are hints for the researcher, not verdicts.
  const QUALITY_FLAG_LABELS = {
    stated_need_echo: {
      label: "顕在ニーズの言い換え",
      desc: "Latent Need が Stated Need とほぼ同じ文言です。本人が自覚して口にしているニーズはインサイトではありません。",
    },
    generic_term: {
      label: "抽象語",
      desc: "Latent Need が抽象的な言葉（コスパ・安心・承認欲求・自分らしさ など）に頼っています。人を動かした具体的な欲求に言い換えられないか確認してください。",
    },
    no_trace: {
      label: "痕跡なし",
      desc: "この仮説は「予想とのズレ」を根拠にしておらず、繰り返しだけから導かれています。よく支持されていても、当たり前の観察に留まっている可能性があります。",
    },
    abduction_incomplete: {
      label: "推論が不完全",
      desc: "常識的予想または驚くべき事実が記録されていないため、予想 → ズレ → 仮説 の連鎖を読者が検証できません。",
    },
  };

  const ROLE_LABELS = {
    customer: "回答者（分析対象）",
    interviewer: "質問者",
    agent: "担当者",
    other: "その他",
  };
  const PROVENANCE_LABELS = {
    firsthand: "本人の発言",
    secondhand: "第三者のメモ",
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
        <div class="quality-box-title">品質チェック（アプリ側の自動判定）</div>
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

  async function renderProject(projectID, errorMessage, notice) {
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
              ${d.provenance === "secondhand" ? `<span class="kind-badge kind-secondhand" title="第三者による要約・メモ。Evidence としての重みは下がります">第三者のメモ</span>` : ""}
              ${(d.spans || []).length ? `<span class="kind-badge kind-spans" title="話者分離済み。回答者の発言だけが引用対象です">話者分離 ${d.spans.filter((s) => s.role === "customer").length}/${d.spans.length}</span>` : ""}
              ${d.masked ? `<span class="kind-badge kind-masked" title="個人情報をマスク済み">マスク済み</span>` : ""}
              ${d.situation ? `<span class="doc-situation">${escapeHtml(d.situation)}</span>` : ""}
            </div>
            <div class="doc-content">${escapeHtml(d.content)}</div>
          </div>`).join("")
      : `<div class="empty">まだドキュメントがありません。下のフォームからテキストを貼り付けてください。</div>`;

    const insightsHtml = insights.length
      ? `<div class="insight-list">${insights.map((i) => `
          <a class="card insight-card${(i.qualityFlags || []).length ? " insight-card-flagged" : ""}" href="#/insights/${encodeURIComponent(i.id)}">
            <div class="insight-card-title">${escapeHtml(i.title)}</div>
            <div class="insight-card-latent">${escapeHtml(i.latentNeed)}</div>
            ${i.surprisingFact ? `<div class="insight-card-trace">ズレ: ${escapeHtml(i.surprisingFact)}</div>` : ""}
            ${confidenceBar(i.confidence)}
            ${qualityBadgesHTML(i.qualityFlags)}
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
      ${notice ? `<div class="notice-box">${notice}</div>` : ""}

      <div class="card">
        <div class="section-title">解析</div>
        <div id="analysis-panel">
          ${analysisPanelHTML(latestAnalysis)}
        </div>
        <div class="analysis-actions">
          <button class="primary" id="run-analysis" ${isRunning || documents.length === 0 ? "disabled" : ""}>解析を実行</button>
          <a class="btn" href="#/projects/${encodeURIComponent(projectID)}/patterns">痕跡・パターン一覧</a>
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
          <div>
            <label>出所</label>
            <select name="provenance">
              <option value="">自動（商談ログは第三者のメモ、それ以外は本人の発言）</option>
              <option value="firsthand">本人の発言・記述そのもの</option>
              <option value="secondhand">第三者による要約・メモ（営業メモなど）</option>
            </select>
          </div>
          <div><label>タイトル</label><input type="text" name="title" placeholder="例: Interview #15"></div>
          <div><label>本文</label><textarea name="content" placeholder="発言をそのまま貼り付けてください。「面接官: 」「Q. 」「田中: 」のような話者ラベル付きの書き起こしは、話者ごとに分けて回答者の発言だけを分析対象にします" required></textarea></div>
          <div class="analysis-actions">
            <button type="button" id="intake-preview-btn">取り込みプレビュー</button>
            <button type="submit" class="primary">そのまま追加する</button>
          </div>
        </form>
        <div id="intake-preview"></div>
      </div>

      <div class="card">
        <div class="section-title">CSV / TSV インポート</div>
        <p class="hint">Zendesk のエクスポート、アンケートの自由記述、レビューの一覧など、どんな列構成でも取り込めます。ファイルを選ぶと列の対応（本文・タイトル・種別・発言者の属性）を提案し、確認してから取り込みます。対応はプロジェクトに記憶されます。</p>
        <form id="csv-form">
          <input type="file" name="file" accept=".csv,.tsv,text/csv,text/tab-separated-values" required>
          <button type="submit">列の対応を確認する</button>
        </form>
        <div id="csv-preview"></div>
        <div id="csv-result"></div>
      </div>

      <div class="card">
        <div class="section-title">ドキュメント</div>
        ${docsHtml}
      </div>
    `);

    const pasteForm = document.getElementById("paste-form");
    pasteForm.addEventListener("submit", async (ev) => {
      ev.preventDefault();
      const f = ev.target;
      try {
        await api(`/api/projects/${encodeURIComponent(projectID)}/documents`, {
          method: "POST",
          body: JSON.stringify({ source: f.source.value, provenance: f.provenance.value, title: f.title.value, content: f.content.value }),
        });
        renderProject(projectID, null, "ドキュメントを追加しました。");
      } catch (e) {
        renderProject(projectID, e.message);
      }
    });

    document.getElementById("intake-preview-btn").addEventListener("click", () => {
      runIntakePreview(projectID, pasteForm, {});
    });

    document.getElementById("csv-form").addEventListener("submit", async (ev) => {
      ev.preventDefault();
      const file = ev.target.file.files[0];
      if (!file) return;
      const formData = new FormData();
      formData.append("file", file);
      const box = document.getElementById("csv-preview");
      document.getElementById("csv-result").innerHTML = "";
      box.innerHTML = `<div class="hint">列を読み取っています...</div>`;
      try {
        const preview = await api(`/api/projects/${encodeURIComponent(projectID)}/documents/import/preview`, {
          method: "POST", body: formData,
        });
        renderImportMapping(projectID, file, preview);
      } catch (e) {
        box.innerHTML = errorBox(e.message);
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

  // ---------- Intake preview ----------
  //
  // The preview is the intake counterpart of the reasoning trail: before
  // any text becomes a document, the user sees which parts will be
  // treated as the customer's voice, which are excluded (interviewer,
  // agent), and what was guessed rather than known. Role changes re-run
  // the deterministic parser on the server; the confirmed roles are then
  // remembered by the project.
  async function runIntakePreview(projectID, form, speakerRoles) {
    const box = document.getElementById("intake-preview");
    const content = form.content.value;
    if (!content.trim()) {
      box.innerHTML = errorBox("本文を貼り付けてください");
      return;
    }
    box.innerHTML = `<div class="hint">話者を検出しています...</div>`;
    let preview;
    try {
      preview = await api(`/api/projects/${encodeURIComponent(projectID)}/intake/preview`, {
        method: "POST",
        body: JSON.stringify({ source: form.source.value, provenance: form.provenance.value, content, speakerRoles }),
      });
    } catch (e) {
      box.innerHTML = errorBox(e.message);
      return;
    }
    box.innerHTML = intakePreviewHTML(preview);

    box.querySelectorAll("select.role-select").forEach((sel) => {
      sel.addEventListener("change", () => {
        const roles = { ...speakerRoles };
        box.querySelectorAll("select.role-select").forEach((s) => { roles[s.dataset.label] = s.value; });
        runIntakePreview(projectID, form, roles);
      });
    });

    const commit = document.getElementById("intake-commit");
    if (commit) {
      commit.addEventListener("click", async () => {
        commit.disabled = true;
        const roles = {};
        box.querySelectorAll("select.role-select").forEach((s) => { roles[s.dataset.label] = s.value; });
        try {
          await api(`/api/projects/${encodeURIComponent(projectID)}/documents`, {
            method: "POST",
            body: JSON.stringify({
              source: form.source.value, provenance: preview.provenance, title: form.title.value, content,
              spans: preview.spans, speakerRoles: preview.detected ? roles : {},
            }),
          });
          renderProject(projectID, null, preview.detected
            ? `ドキュメントを追加しました（話者 ${preview.speakers.length} 人、回答者の発言 ${preview.customerChars} 字を分析対象にしました）。`
            : "ドキュメントを追加しました。");
        } catch (e) {
          box.innerHTML = errorBox(e.message) + intakePreviewHTML(preview);
        }
      });
    }
  }

  // ---------- Spreadsheet import mapping ----------
  //
  // Each column gets one job: the customer's text, a title, an id, the
  // source type, a speaker attribute (reserved metadata key), free-form
  // metadata, or nothing. The suggestion comes from header names; the
  // project's last confirmed mapping wins over the suggestion.
  const COLUMN_JOBS = [
    ["", "取り込まない"],
    ["content", "本文（分析対象のテキスト）"],
    ["title", "タイトル"],
    ["id", "ID（追跡用）"],
    ["source", "種別（interview/review/...）"],
    ["meta:role", "属性: 役職・立場"],
    ["meta:company_size", "属性: 会社規模"],
    ["meta:segment", "属性: セグメント・業種"],
    ["meta:plan", "属性: 契約プラン"],
    ["meta:volume", "属性: 利用量"],
    ["meta:participant_id", "属性: 回答者ID"],
    ["meta:date", "属性: 日付"],
    ["meta:rating", "属性: 評価点"],
    ["meta:*", "属性: その他（列名をキーにする）"],
  ];

  function jobForColumn(mapping, header) {
    if (!mapping) return "";
    if (mapping.contentColumn === header) return "content";
    if (mapping.titleColumn === header) return "title";
    if (mapping.idColumn === header) return "id";
    if (mapping.sourceColumn === header) return "source";
    const key = (mapping.metadataColumns || {})[header];
    if (key) {
      return COLUMN_JOBS.some(([v]) => v === `meta:${key}`) ? `meta:${key}` : "meta:*";
    }
    return "";
  }

  function renderImportMapping(projectID, file, preview) {
    const box = document.getElementById("csv-preview");
    const base = preview.savedMapping || preview.suggested;
    const usingSaved = !!preview.savedMapping;

    const columnRows = preview.headers.map((h, i) => {
      const samples = preview.sample.map((row) => row[i] || "").filter(Boolean).slice(0, 2)
        .map((v) => escapeHtml(v.length > 40 ? v.slice(0, 40) + "…" : v)).join(" ／ ");
      const job = jobForColumn(base, h);
      return `
        <tr>
          <td class="speaker-label">${escapeHtml(h)}</td>
          <td>
            <select class="job-select" data-header="${escapeHtml(h)}">
              ${COLUMN_JOBS.map(([v, label]) => `<option value="${v}" ${v === job ? "selected" : ""}>${label}</option>`).join("")}
            </select>
          </td>
          <td class="sample-cell">${samples || "<span class='hint'>（空）</span>"}</td>
        </tr>`;
    }).join("");

    box.innerHTML = `
      <div class="intake-box">
        <div class="intake-head">
          <span class="kind-badge kind-spans">${preview.rowCount} 行・${preview.headers.length} 列（${preview.delimiter === "\t" ? "TSV" : "CSV"}）</span>
          <span>${usingSaved ? "このプロジェクトで前回使った列の対応を適用しています。" : preview.guessedContent ? "本文らしい列名が見つからなかったため、文字数の最も多い列を本文として提案しています。" : "列名から対応を提案しています。"}</span>
        </div>
        <table class="speaker-table">
          <thead><tr><th>列</th><th>役割</th><th>サンプル</th></tr></thead>
          <tbody>${columnRows}</tbody>
        </table>
        <div class="mapping-options">
          <div>
            <label>種別の列がない行の既定の種別</label>
            <select id="map-default-source">
              ${Object.keys(SOURCE_LABELS).map((k) => `<option value="${k}" ${(base.defaultSource || "interview") === k ? "selected" : ""}>${SOURCE_LABELS[k]}</option>`).join("")}
            </select>
          </div>
          <div>
            <label>出所</label>
            <select id="map-provenance">
              <option value="" ${!base.provenance ? "selected" : ""}>自動（商談ログは第三者のメモ、それ以外は本人の発言）</option>
              <option value="firsthand" ${base.provenance === "firsthand" ? "selected" : ""}>本人の発言・記述そのもの</option>
              <option value="secondhand" ${base.provenance === "secondhand" ? "selected" : ""}>第三者による要約・メモ</option>
            </select>
          </div>
        </div>
        <p class="hint">本文のセルが「顧客: ／ サポート: 」のような会話になっている場合は、貼り付けと同じ話者分離が自動で適用されます。</p>
        <div class="analysis-actions"><button class="primary" id="csv-import-btn">この対応で ${preview.rowCount} 行を取り込む</button></div>
      </div>`;

    document.getElementById("csv-import-btn").addEventListener("click", async () => {
      const mapping = { metadataColumns: {} };
      box.querySelectorAll("select.job-select").forEach((sel) => {
        const h = sel.dataset.header;
        const v = sel.value;
        if (v === "content") mapping.contentColumn = h;
        else if (v === "title") mapping.titleColumn = h;
        else if (v === "id") mapping.idColumn = h;
        else if (v === "source") mapping.sourceColumn = h;
        else if (v === "meta:*") mapping.metadataColumns[h] = h;
        else if (v.startsWith("meta:")) mapping.metadataColumns[h] = v.slice(5);
      });
      mapping.defaultSource = document.getElementById("map-default-source").value;
      mapping.provenance = document.getElementById("map-provenance").value || undefined;
      if (!mapping.contentColumn) {
        document.getElementById("csv-result").innerHTML = errorBox("本文の列を1つ選んでください");
        return;
      }
      const formData = new FormData();
      formData.append("file", file);
      formData.append("mapping", JSON.stringify(mapping));
      try {
        const result = await api(`/api/projects/${encodeURIComponent(projectID)}/documents/import`, { method: "POST", body: formData });
        const errs = (result.errors || []).slice(0, 5).map((e) => `${e.row}行目: ${escapeHtml(e.reason)}`).join("<br>");
        renderProject(projectID, null,
          `${result.imported}件取り込み、${result.skipped}件スキップしました。` +
          `${result.withSpeakers ? ` 話者分離 ${result.withSpeakers}件。` : ""}${result.masked ? ` 個人情報マスク ${result.masked}箇所。` : ""}` +
          `${errs ? `<br>${errs}` : ""}`);
      } catch (e) {
        document.getElementById("csv-result").innerHTML = errorBox(e.message);
      }
    });
  }

  function intakePreviewHTML(p) {
    const pct = p.totalChars ? Math.round((p.customerChars / p.totalChars) * 100) : 0;
    const warnings = (p.warnings || []).map((w) => `<div class="notice-box intake-warning">⚠ ${escapeHtml(w)}</div>`).join("");
    const provenance = `<span class="kind-badge ${p.provenance === "secondhand" ? "kind-secondhand" : "kind-firsthand"}">${escapeHtml(PROVENANCE_LABELS[p.provenance] || p.provenance)}</span>`;

    if (!p.detected) {
      return `
        <div class="intake-box">
          <div class="intake-head">${provenance}<span>話者ラベルは検出されませんでした。全文（${p.totalChars} 字）を回答者の発言として扱います。</span></div>
          ${warnings}
          <div class="analysis-actions"><button class="primary" id="intake-commit">この内容で追加する</button></div>
        </div>`;
    }

    const speakers = p.speakers.map((s) => `
      <tr>
        <td class="speaker-label">${escapeHtml(s.label)}${s.guessed ? ` <span class="guess-tag" title="役割は推定です">推定</span>` : ""}</td>
        <td>
          <select class="role-select" data-label="${escapeHtml(s.label)}">
            ${Object.keys(ROLE_LABELS).map((r) => `<option value="${r}" ${r === s.role ? "selected" : ""}>${ROLE_LABELS[r]}</option>`).join("")}
          </select>
        </td>
        <td class="num">${s.turns}</td>
        <td class="num">${s.chars}</td>
      </tr>`).join("");

    const turns = p.turns.map((t) => `
      <div class="turn turn-${escapeHtml(t.role)}">
        <div class="turn-speaker">${escapeHtml(t.speaker)} <span class="turn-role">${escapeHtml(ROLE_LABELS[t.role] || t.role)}</span></div>
        <div class="turn-text">${escapeHtml(t.text)}</div>
      </div>`).join("");

    return `
      <div class="intake-box">
        <div class="intake-head">
          ${provenance}
          <span>話者 ${p.speakers.length} 人・${p.turns.length} ターンを検出。分析対象（回答者）は ${p.customerChars} 字（全体の ${pct}%）、除外 ${p.excludedChars} 字。</span>
        </div>
        ${warnings}
        <table class="speaker-table">
          <thead><tr><th>話者ラベル</th><th>役割</th><th class="num">ターン</th><th class="num">文字数</th></tr></thead>
          <tbody>${speakers}</tbody>
        </table>
        <p class="hint">役割を変えると再判定します。確定した対応はこのプロジェクトに記憶され、次の貼り付けから自動で適用されます。回答者以外の発言は文脈として読まれますが、引用（Evidence）にはなりません。</p>
        <div class="turn-list">${turns}</div>
        <div class="analysis-actions"><button class="primary" id="intake-commit">この内容で追加する</button></div>
      </div>`;
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
        <div class="section-title">推論の過程 &mdash; 予想 → ズレ → 仮説</div>
        <p class="hint">インサイトは人の心を読んで見つけるものではなく、「人はこう動くはずだ」という予想と実際の行動のズレ（欲望の痕跡）を、アブダクションで説明する仮説として立てるものです。AIが最終的な洞察だけを提示するのではなく、その連鎖をたどれるようにしています。</p>
        ${abductionHTML(insight)}
        <div class="trail-subtitle">元になった気づき</div>
        ${patternsSectionHTML(insight.patterns)}
      </div>

      <div class="card">
        <div class="section-title">Insight</div>
        <h2 style="margin:0 0 12px;">${escapeHtml(insight.title)}</h2>
        ${confidenceBar(insight.confidence)}
        ${qualityBadgesHTML(insight.qualityFlags, { withDesc: true })}

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

  // The abductive triad: a surprising fact C that broke an expectation,
  // and a hypothesis H such that, if H were true, C would be a matter of
  // course. Each step is shown even when empty so a missing link is visible.
  function abductionHTML(insight) {
    const step = (n, label, value, cls) => `
      <div class="abduction-step ${cls}">
        <div class="abduction-num">${n}</div>
        <div class="abduction-body">
          <div class="field-label">${label}</div>
          <div>${value ? escapeHtml(value) : `<span class="missing">記録なし</span>`}</div>
        </div>
      </div>`;
    return `
      <div class="abduction">
        ${step("①", "常識的な予想（人はこう動くはず）", insight.expectation, "abduction-expect")}
        ${step("②", "驚くべき事実（予想とのズレ ＝ 欲望の痕跡）", insight.surprisingFact, "abduction-fact")}
        ${step("③", "仮説（この無自覚な欲求があれば、②は当然の行動になる）", insight.latentNeed, "abduction-hyp")}
        ${step("④", "説明（なぜ③なら②が当たり前になるのか）", insight.rationale, "abduction-why")}
      </div>`;
  }

  function patternBlockHTML(p) {
    const isTrace = p.kind === "deviation";
    const kindBadge = isTrace
      ? `<span class="kind-badge kind-deviation">痕跡</span>${p.deviationType ? `<span class="deviation-type">${escapeHtml(DEVIATION_LABELS[p.deviationType] || p.deviationType)}</span>` : ""}`
      : `<span class="kind-badge kind-repetition">繰り返し</span>`;
    const body = isTrace
      ? `
        <div class="trace-gap">
          <div class="trace-cell trace-expect"><div class="field-label">予想</div><div>${escapeHtml(p.expectation || "-")}</div></div>
          <div class="trace-arrow">≠</div>
          <div class="trace-cell trace-actual"><div class="field-label">実際</div><div>${escapeHtml(p.description || "-")}</div></div>
        </div>`
      : (p.description ? `<div class="pattern-desc">${escapeHtml(p.description)}</div>` : "");
    return `
      <div class="pattern-block ${isTrace ? "pattern-trace" : ""}">
        <div class="pattern-head">${kindBadge}<span class="pattern-title">${escapeHtml(p.title)}</span></div>
        ${body}
        <div class="pattern-observations">
          ${(p.observations || []).map(evidenceRowHTML).join("") || `<div class="empty">観察が見つかりません</div>`}
        </div>
      </div>`;
  }

  function patternsSectionHTML(patterns) {
    if (!patterns || patterns.length === 0) {
      return `<div class="empty">この洞察の元になった痕跡・パターンは記録されていません。</div>`;
    }
    const traces = patterns.filter((p) => p.kind === "deviation");
    const repetitions = patterns.filter((p) => p.kind !== "deviation");
    let html = "";
    if (traces.length) html += traces.map(patternBlockHTML).join("");
    else html += `<div class="notice-box quality-notice">この洞察は「予想とのズレ」を根拠にしていません（繰り返しのみ）。</div>`;
    if (repetitions.length) html += repetitions.map(patternBlockHTML).join("");
    return html;
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
      ["Trace-backed Insights", metrics.traceBackedInsightRate, "「予想とのズレ」を根拠に持つInsightの割合"],
      ["Quality Flagged", metrics.qualityFlaggedInsightRate, "品質チェックで警告が付いたInsightの割合（低いほど良い）"],
    ];
    const flagCounts = metrics.qualityFlagCounts || {};
    const flagSummary = Object.keys(QUALITY_FLAG_LABELS)
      .filter((code) => flagCounts[code])
      .map((code) => `${QUALITY_FLAG_LABELS[code].label} ${flagCounts[code]}件`)
      .join(" / ");

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
          「予想とのズレ」${metrics.traceCount || 0} 件を含む気づき ${metrics.patternCount} 件から、洞察候補 ${metrics.totalInsightDrafts} 件が生まれ、最終的に ${metrics.finalInsightCount} 件が採用されました。
          ${flagSummary ? `品質チェックの内訳: ${escapeHtml(flagSummary)}。` : "品質チェックの警告はありません。"}
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

    const traces = patterns.filter((p) => p.kind === "deviation");
    const repetitions = patterns.filter((p) => p.kind !== "deviation");

    layout(`
      <a class="back-link" href="#/projects/${encodeURIComponent(projectID)}">&larr; プロジェクトに戻る</a>
      <div class="card">
        <div class="section-title">痕跡とパターン — ${escapeHtml(project.name)}</div>
        <p class="hint">解析が「気づいた」ことの一覧です。最終的なInsightに至らなかったものもここに残ります。</p>
      </div>
      <div class="card">
        <div class="section-title">欲望の痕跡（予想とのズレ） ${traces.length} 件</div>
        <p class="hint">「常識的にはこう動くはず」という予想と、実際の行動が食い違った箇所です。インサイト（人を動かす無自覚な欲求）は、この痕跡を説明する仮説として立てます。</p>
        ${traces.length ? traces.map(patternBlockHTML).join("") : `<div class="empty">予想とのズレはまだ検出されていません。</div>`}
      </div>
      <div class="card">
        <div class="section-title">繰り返しのパターン ${repetitions.length} 件</div>
        <p class="hint">複数のドキュメントにまたがって繰り返し現れた行動・発言です。繰り返しだけを根拠にした仮説は、顕在ニーズの言い換えになりやすい点に注意してください。</p>
        ${repetitions.length ? repetitions.map(patternBlockHTML).join("") : `<div class="empty">まだパターンは検出されていません。解析を実行してください。</div>`}
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
