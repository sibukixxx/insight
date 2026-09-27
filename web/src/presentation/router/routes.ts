// Hash routes. Every hash the pre-TypeScript UI used keeps its meaning
// (#/, #/settings, #/projects/:id[?run=], …/patterns, …/evaluation,
// …/research, #/research-runs/:id, #/insights/:id); new screens add paths
// under the project. An unknown hash shows Home, as before.

export type Route =
  | { readonly name: "home" }
  | { readonly name: "settings" }
  | { readonly name: "workspace"; readonly projectId: string; readonly runId?: string }
  | { readonly name: "input"; readonly projectId: string }
  | { readonly name: "analysis"; readonly projectId: string }
  | { readonly name: "findings"; readonly projectId: string; readonly runId?: string }
  | { readonly name: "patterns"; readonly projectId: string; readonly runId?: string }
  | { readonly name: "evaluation"; readonly projectId: string; readonly runId?: string }
  | { readonly name: "runs"; readonly projectId: string; readonly from?: string; readonly to?: string }
  | { readonly name: "research"; readonly projectId: string }
  | { readonly name: "promotion"; readonly researchRunId: string }
  | { readonly name: "insight"; readonly insightId: string };

export type ProjectPage = "input" | "analysis" | "findings" | "patterns" | "evaluation" | "runs" | "research";
const projectPages: readonly ProjectPage[] = ["input", "analysis", "findings", "patterns", "evaluation", "runs", "research"];

function decode(segment: string): string | undefined {
  try {
    const v = decodeURIComponent(segment);
    return v === "" ? undefined : v;
  } catch {
    return undefined;
  }
}

const withRun = <T extends object>(route: T, runId: string | null): T => (runId ? { ...route, runId } : route);

export function parseHash(hash: string): Route {
  const [path = "", query = ""] = (hash || "#/").replace(/^#/, "").split("?");
  const params = new URLSearchParams(query);
  const runId = params.get("run");
  const parts = path.split("/").filter(Boolean);

  if (parts.length === 1 && parts[0] === "settings") return { name: "settings" };
  if (parts[0] === "insights" && parts.length === 2) {
    const insightId = decode(parts[1] ?? "");
    if (insightId) return { name: "insight", insightId };
  }
  if (parts[0] === "research-runs" && parts.length === 2) {
    const researchRunId = decode(parts[1] ?? "");
    if (researchRunId) return { name: "promotion", researchRunId };
  }
  if (parts[0] === "projects" && (parts.length === 2 || parts.length === 3)) {
    const projectId = decode(parts[1] ?? "");
    if (projectId) {
      const page = parts[2];
      if (page === undefined) return withRun({ name: "workspace", projectId }, runId);
      switch (page as ProjectPage) {
        case "input": return { name: "input", projectId };
        case "analysis": return { name: "analysis", projectId };
        case "research": return { name: "research", projectId };
        case "findings": return withRun({ name: "findings", projectId }, runId);
        case "patterns": return withRun({ name: "patterns", projectId }, runId);
        case "evaluation": return withRun({ name: "evaluation", projectId }, runId);
        case "runs": {
          const from = params.get("a");
          const to = params.get("b");
          return { name: "runs", projectId, ...(from ? { from } : {}), ...(to ? { to } : {}) };
        }
      }
    }
  }
  return { name: "home" };
}

const enc = encodeURIComponent;

export function projectHash(projectId: string, page?: ProjectPage, runId?: string): string {
  const base = `#/projects/${enc(projectId)}${page ? `/${page}` : ""}`;
  return runId ? `${base}?run=${enc(runId)}` : base;
}

export function buildHash(route: Route): string {
  switch (route.name) {
    case "home": return "#/";
    case "settings": return "#/settings";
    case "workspace": return projectHash(route.projectId, undefined, route.runId);
    case "input":
    case "analysis":
    case "research": return projectHash(route.projectId, route.name);
    case "findings":
    case "patterns":
    case "evaluation": return projectHash(route.projectId, route.name, route.runId);
    case "runs": {
      const q = new URLSearchParams();
      if (route.from) q.set("a", route.from);
      if (route.to) q.set("b", route.to);
      const qs = q.toString();
      return `${projectHash(route.projectId, "runs")}${qs ? `?${qs}` : ""}`;
    }
    case "promotion": return `#/research-runs/${enc(route.researchRunId)}`;
    case "insight": return `#/insights/${enc(route.insightId)}`;
  }
}

export function isProjectPage(value: string): value is ProjectPage {
  return (projectPages as readonly string[]).includes(value);
}
