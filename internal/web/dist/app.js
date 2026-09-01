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
  };

  function escapeHtml(s) {
    return String(s ?? "").replace(/[&<>"']/g, (c) => ({
      "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;",
    }[c]));
  }

  async function api(path, options) {
    const res = await fetch(path, {
      headers: { "Content-Type": "application/json" },
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
    if (buildInfo.demoBuild) {
      return `<span class="badge demo">デモビルド</span>`;
    }
    return `<span class="badge delivery">納品ビルド（デモデータなし）</span>`;
  }

  function confidentialBanner() {
    if (!buildInfo.clientName) return "";
    return `<div class="confidential-banner">Confidential — ${escapeHtml(buildInfo.clientName)} 様向け納品版</div>`;
  }

  function layout(inner) {
    app.innerHTML = `
      <header class="top">
        <div class="brand">Insight Lab <small>Hidden Needs Finder</small></div>
        ${buildBadge()}
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
          <button class="primary" id="try-demo" ${buildInfo.demoBuild ? "" : "disabled"}>
            デモを試す
          </button>
          <button id="new-project">新規プロジェクト</button>
        </div>
        ${!buildInfo.demoBuild ? `<p class="meta" style="margin-top:10px;color:var(--text-muted);font-size:12px;">
          この納品ビルドにはデモデータが含まれていません。デモビルド（<code>make build-demo</code>）で起動すると試せます。
        </p>` : ""}
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
        const project = await api("/api/projects", {
          method: "POST",
          body: JSON.stringify({ name }),
        });
        location.hash = `#/projects/${project.id}`;
      } catch (e) {
        renderHome(e.message);
      }
    });
  }

  async function renderProject(projectID, errorMessage) {
    let project, documents;
    try {
      [project, documents] = await Promise.all([
        api(`/api/projects/${encodeURIComponent(projectID)}`),
        api(`/api/projects/${encodeURIComponent(projectID)}/documents`),
      ]);
    } catch (e) {
      layout(`<a class="back-link" href="#/">&larr; プロジェクト一覧に戻る</a>${errorBox(e.message)}`);
      return;
    }

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

    layout(`
      <a class="back-link" href="#/">&larr; プロジェクト一覧に戻る</a>
      <div class="card">
        <div class="section-title">プロジェクト</div>
        <h2 style="margin:0 0 4px;">${escapeHtml(project.name)}</h2>
        <div class="meta" style="color:var(--text-muted);font-size:13px;">ドキュメント ${documents.length} 件</div>
      </div>

      ${errorBox(errorMessage)}

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
            </select>
          </div>
          <div>
            <label>タイトル</label>
            <input type="text" name="title" placeholder="例: Interview #15">
          </div>
          <div>
            <label>本文</label>
            <textarea name="content" placeholder="発言をそのまま貼り付けてください" required></textarea>
          </div>
          <div>
            <button type="submit" class="primary">追加する</button>
          </div>
        </form>
      </div>

      <div class="card">
        <div class="section-title">ドキュメント</div>
        ${docsHtml}
      </div>
    `);

    document.getElementById("paste-form").addEventListener("submit", async (ev) => {
      ev.preventDefault();
      const form = ev.target;
      const payload = {
        source: form.source.value,
        title: form.title.value,
        content: form.content.value,
      };
      try {
        await api(`/api/projects/${encodeURIComponent(projectID)}/documents`, {
          method: "POST",
          body: JSON.stringify(payload),
        });
        renderProject(projectID);
      } catch (e) {
        renderProject(projectID, e.message);
      }
    });
  }

  function route() {
    const hash = location.hash || "#/";
    const projectMatch = hash.match(/^#\/projects\/([^/]+)$/);
    if (projectMatch) {
      renderProject(decodeURIComponent(projectMatch[1]));
      return;
    }
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
