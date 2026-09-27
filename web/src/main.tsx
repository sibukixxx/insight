import { render } from "preact";
import "./styles/tokens.css";
import "./styles/reset.css";
import "./styles/global.css";
import { App } from "./app/App";
import { createCompositionRoot } from "./app/composition-root";

async function boot(root: HTMLElement) {
  const useCases = createCompositionRoot();
  let locale;
  try {
    locale = await useCases.locale.bootLocale();
  } catch (e) {
    // Without dictionaries every label would be a missing-key marker; fail visibly instead.
    const box = document.createElement("div");
    box.setAttribute("role", "alert");
    box.textContent = `Insight Lab could not load its UI text (${e instanceof Error ? e.message : String(e)}).`;
    root.replaceChildren(box);
    return;
  }
  const build = await useCases.system.loadBuildInfo();
  render(<App useCases={useCases} build={build} dictionaries={locale.dictionaries} locale={locale.locale} />, root);
}

const root = document.getElementById("app");
if (root) void boot(root);
