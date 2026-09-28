import { fireEvent, screen, waitFor, within } from "@testing-library/preact";
import { describe, expect, it } from "vitest";
import { fakePorts, fakeState, formats, sampleScenario } from "./fakePorts";
import { renderApp } from "./renderApp";

const xss = "<img src=x onerror=\"window.__xss=1\">";

describe("Home", () => {
  it("guides a first run on a delivery build without offering the sample", async () => {
    renderApp(fakePorts(fakeState({ projects: [] })));
    expect(await screen.findByText("No projects yet. Try the demo or create a project.")).toBeTruthy();
    expect(screen.getByRole("list", { name: "How an analysis works" })).toBeTruthy();
    expect((screen.getByRole("button", { name: "Try the demo" }) as HTMLButtonElement).disabled).toBe(true);
    expect(screen.getByText("make build-demo").tagName).toBe("CODE");
  });

  it("opens the sample project on a demo build", async () => {
    const ports = fakePorts();
    renderApp(ports, { build: { demoBuild: true, clientName: "" } });
    fireEvent.click(await screen.findByRole("button", { name: "Try the demo" }));
    await waitFor(() => expect(window.location.hash).toBe("#/projects/p1"));
    expect(ports.projects.createSample).toHaveBeenCalled();
  });

  it("creates a project and continues to its input page", async () => {
    const ports = fakePorts(fakeState({ projects: [] }));
    renderApp(ports);
    fireEvent.input(await screen.findByLabelText("Project name"), { target: { value: "Market study" } });
    fireEvent.click(screen.getByRole("button", { name: "Create" }));
    await waitFor(() => expect(window.location.hash).toBe("#/projects/p1/input"));
    expect(ports.projects.create).toHaveBeenCalledWith("Market study");
  });
});

describe("question-first entry", () => {
  it("saves a project from a sentence alone and goes to the analysis page, with no file or model", async () => {
    const ports = fakePorts(fakeState({ projects: [] }));
    renderApp(ports);
    fireEvent.input(await screen.findByLabelText("Research question"), { target: { value: "Why do stores open where the population falls?" } });
    fireEvent.click(screen.getByRole("button", { name: "Start with this question" }));
    await waitFor(() => expect(window.location.hash).toBe("#/projects/p1/analysis"));
    expect(ports.projects.createFromQuestion).toHaveBeenCalledWith("Why do stores open where the population falls?");
    expect(ports.projects.create).not.toHaveBeenCalled();
  });

  it("does not create a project from an empty question", async () => {
    const ports = fakePorts(fakeState({ projects: [] }));
    renderApp(ports);
    fireEvent.click(await screen.findByRole("button", { name: "Start with this question" }));
    expect(ports.projects.createFromQuestion).not.toHaveBeenCalled();
  });
});

describe("sample gallery", () => {
  const demo = { build: { demoBuild: true, clientName: "" } } as const;
  const official = sampleScenario({
    id: "ja-official-population", projectId: "demo-scenario-ja-official-population", dataKind: "official", rows: 8,
    sources: [{ kind: "official", publisher: "総務省統計局", survey: "国勢調査", unit: "人", url: "https://dashboard.e-stat.go.jp/", license: "PDL1.0", licenseUrl: "https://dashboard.e-stat.go.jp/static/terms", retrievedAt: "2026-09-27T16:50:00Z", rawSha256: "163f", rows: [], regions: [{ code: "08201", name: "水戸市" }], periods: ["2015", "2020"] }],
  });

  it("asks what to investigate and labels each sample's data kind and input before any analysis", async () => {
    renderApp(fakePorts(fakeState({ projects: [], samples: [sampleScenario(), official] })), demo);
    const gallery = await screen.findByRole("region", { name: "What would you like to investigate?" });
    const cards = within(gallery).getAllByRole("listitem");
    expect(cards).toHaveLength(2);
    expect(within(cards[0] as HTMLElement).getByText("Synthetic data")).toBeTruthy();
    expect(within(cards[1] as HTMLElement).getByText("Real data (official statistics)")).toBeTruthy();
    expect(within(cards[0] as HTMLElement).getByText("Documents CSV, 9 rows (accepted as is)")).toBeTruthy();
    expect(within(cards[1] as HTMLElement).getByText("総務省統計局 「国勢調査」")).toBeTruthy();
    expect(within(gallery).getByText(/They are not results/)).toBeTruthy();
    // No model: the gallery says what works without one instead of promising analysis.
    expect(within(gallery).getByText(/No AI model is connected/)).toBeTruthy();
    expect(within(gallery).queryByText(/View the results/)).toBeNull();
  });

  it("opens the scenario's own project on its input page and previews the bundled CSV through the ordinary importer", async () => {
    const ports = fakePorts(fakeState({ projects: [], samples: [sampleScenario()] }));
    renderApp(ports, demo);
    fireEvent.click(await screen.findByRole("button", { name: "Try this example" }));
    await waitFor(() => expect(window.location.hash).toBe("#/projects/demo-scenario-ja-shop-records/input"));
    expect(ports.samples.openProject).toHaveBeenCalledWith("ja-shop-records");
    const panel = await screen.findByRole("region", { name: "Find what changed in fictional shop records — and what is still missing" });
    expect(within(panel).getByText("Synthetic data")).toBeTruthy();
    fireEvent.click(screen.getByText("Source, retrieval and transformation record"));
    expect(within(panel).getByText("All data is fictional.")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Preview this CSV" }));
    await waitFor(() => expect(ports.evidence.preview).toHaveBeenCalledWith("demo-scenario-ja-shop-records", "documents", expect.objectContaining({ name: "insight-sample-ja-shop-records.csv" })));
    expect(ports.samples.input).toHaveBeenCalledWith("ja-shop-records");
  });

  it("links to the last completed run of a sample that was already analysed", async () => {
    const scenario = sampleScenario();
    const state = fakeState({
      projects: [{ id: scenario.projectId, name: "Sample 01", createdAt: "" }], samples: [scenario],
      runs: [{ id: "r9", projectId: scenario.projectId, status: "completed", progress: 100, createdAt: "2026-01-01T00:00:00Z" }],
      settings: { model: "m", baseUrl: "http://x", maskedApiKey: "", hasApiKey: false, configured: true },
    });
    renderApp(fakePorts(state), demo);
    const link = await screen.findByRole("link", { name: /View the results of the last completed run/ });
    expect(link.getAttribute("href")).toBe("#/projects/demo-scenario-ja-shop-records/findings?run=r9");
    expect(screen.getByText(/AI model connected \(m\)/)).toBeTruthy();
  });

  it("keeps the data kind visible on the other screens of a sample project", async () => {
    const scenario = official;
    renderApp(fakePorts(fakeState({ projects: [{ id: scenario.projectId, name: "Sample 02", createdAt: "" }], samples: [scenario] })), { ...demo, hash: `#/projects/${scenario.projectId}/analysis` });
    expect(await screen.findByText("Sample project:")).toBeTruthy();
    expect(screen.getByText("Real data (official statistics)")).toBeTruthy();
  });

  it("never asks a delivery build for samples", async () => {
    const ports = fakePorts(fakeState({ samples: [sampleScenario()] }));
    renderApp(ports);
    await screen.findByText("Project One");
    expect(screen.queryByRole("region", { name: "What would you like to investigate?" })).toBeNull();
    expect(ports.samples.list).not.toHaveBeenCalled();
  });

  it("shows the gallery in Japanese with the same data-kind distinction", async () => {
    renderApp(fakePorts(fakeState({ projects: [], samples: [sampleScenario()] })), { ...demo, locale: "ja" });
    expect(await screen.findByRole("heading", { name: "何を調べてみますか？" })).toBeTruthy();
    expect(screen.getByText("架空データ")).toBeTruthy();
    expect(screen.getByRole("button", { name: "この例で試す" })).toBeTruthy();
  });
});

describe("rendering untrusted text", () => {
  it("shows project names and documents as text, never as markup", async () => {
    const state = fakeState({
      projects: [{ id: "p1", name: xss, createdAt: "" }],
      documents: [{ id: "d1", projectId: "p1", source: "web", title: xss, content: `<script>window.__xss=2</script>${xss}`, metadata: {}, createdAt: "" }],
    });
    const { container } = renderApp(fakePorts(state), { hash: "#/projects/p1/input" });
    await screen.findAllByText(xss);
    expect(container.querySelector("img")).toBeNull();
    expect(container.querySelector("script")).toBeNull();
    expect((window as unknown as { __xss?: number }).__xss).toBeUndefined();
  });
});

describe("insight page", () => {
  const span = (id: string, quote: string, start: number) => ({ id, documentId: "d1", quote, startOffset: start, endOffset: start + quote.length });
  const insight = (over: Record<string, unknown> = {}) => ({
    id: "i1", projectId: "p1", analysisId: "r1", title: "Insight T", observation: "obs", statedNeed: "", latentNeed: "hyp", hypothesis: "hyp", jtbd: "",
    expectation: "", surprisingFact: "", rationale: "", interpretation: "interp", alternativeInterpretation: "alt", productOpportunity: "", monetizationAngle: "",
    confidence: 0.5, qualityFlags: [], createdAt: "",
    patterns: [{ id: "pt1", kind: "deviation", title: "Pattern", observations: [{ ...span("o1", "shared quote", 0), behavior: "b" }, { ...span("o2", "only in the trail", 20), behavior: "b" }] }],
    evidence: [{ ...span("e1", "shared quote", 0), type: "support", relevanceScore: 1 }],
    ...over,
  });

  it("lists a quote once: observations that are also evidence are folded, the rest stay open", async () => {
    const ports = fakePorts();
    ports.results.insight.mockResolvedValue(insight());
    renderApp(ports, { hash: "#/insights/i1" });
    await screen.findByText("Insight T");
    // The evidence card keeps its quote; the trail keeps only what evidence does not list.
    expect(screen.getAllByText(/"shared quote"/)).toHaveLength(2);
    expect(screen.getByText(/"only in the trail"/)).toBeTruthy();
    const folded = screen.getByText("1 quotes are also listed under Evidence and Counter-evidence below");
    expect(folded.closest("details")?.hasAttribute("open")).toBe(false);
    expect(within(folded.closest("details") as HTMLElement).getByText(/"shared quote"/)).toBeTruthy();
  });

  it("leaves out legacy fields the run never recorded and says so for the others", async () => {
    const ports = fakePorts();
    ports.results.insight.mockResolvedValue(insight({ interpretation: "" }));
    renderApp(ports, { hash: "#/insights/i1" });
    await screen.findByText("Insight T");
    expect(screen.queryByText(/Stated need/)).toBeNull();
    expect(screen.queryByText(/JTBD/)).toBeNull();
    expect(screen.queryByText("-")).toBeNull();
    expect(screen.getAllByText("not recorded").length).toBeGreaterThan(0);
  });
});

describe("locale switch", () => {
  it("changes the language in place and keeps the route and typed values", async () => {
    const ports = fakePorts(fakeState({ documents: [] }));
    renderApp(ports, { hash: "#/projects/p1/input" });
    const content = await screen.findByLabelText("Content");
    fireEvent.input(content, { target: { value: "typed but not saved" } });
    fireEvent.change(screen.getAllByLabelText("Display language")[0] as HTMLSelectElement, { target: { value: "ja" } });
    expect(await screen.findByLabelText("本文")).toBeTruthy();
    expect((screen.getByLabelText("本文") as HTMLTextAreaElement).value).toBe("typed but not saved");
    expect(window.location.hash).toBe("#/projects/p1/input");
    expect(document.documentElement.lang).toBe("ja");
    expect(ports.locale.saveLocale).toHaveBeenCalledWith("ja");
  });
});

describe("CSV import", () => {
  it("previews with the server before importing and imports only on confirmation", async () => {
    const ports = fakePorts();
    renderApp(ports, { hash: "#/projects/p1/input" });
    const file = new File(["id,source,title,content\n"], "docs.csv", { type: "text/csv" });
    fireEvent.change(await screen.findByLabelText("CSV file"), { target: { files: [file] } });
    const preview = await screen.findByRole("region", { name: "Preview" });
    expect(within(preview).getByText("Row 2: invalid source: \"bogus\"")).toBeTruthy();
    expect(ports.evidence.import).not.toHaveBeenCalled();
    fireEvent.click(within(preview).getByRole("button", { name: "Import" }));
    expect(await screen.findByText("Imported 1; skipped 1.")).toBeTruthy();
    expect(ports.evidence.import).toHaveBeenCalledWith("p1", "documents", expect.objectContaining({ name: "docs.csv" }));
  });

  it("rejects an unsupported file type without uploading it", async () => {
    const ports = fakePorts();
    renderApp(ports, { hash: "#/projects/p1/input" });
    const file = new File(["%PDF"], "report.pdf", { type: "application/pdf" });
    fireEvent.change(await screen.findByLabelText("CSV file"), { target: { files: [file] } });
    expect(await screen.findByText("report.pdf is not a supported file. Supported: .csv.")).toBeTruthy();
    expect(ports.evidence.preview).not.toHaveBeenCalled();
  });

  it("offers the selected server format template and groups other formats", async () => {
    renderApp(fakePorts(), { hash: "#/projects/p1/input" });
    const links = await screen.findAllByRole("link", { name: /Download template/ });
    expect(links.map((a) => a.getAttribute("href"))).toEqual(["/template/documents"]);
    fireEvent.click(screen.getByText("Other input formats (specialized analysis)"));
    fireEvent.click(screen.getByRole("radio", { name: "Corporate-event analysis CSV" }));
    expect((await screen.findByRole("link", { name: /Download template/ })).getAttribute("href")).toBe("/template/analysis");
  });
});

describe("analysis", () => {
  const failed = { id: "r1", projectId: "p1", status: "failed", progress: 30, createdAt: "", error: "the LLM is not configured", researchQuestion: "why?", reasoningProfile: "CUSTOMER_INSIGHT" };

  const configured = { model: "m", baseUrl: "https://x", maskedApiKey: "", hasApiKey: true, configured: true };
  const noEvidence = /No evidence yet\. You can still explore hypotheses/;

  it("offers a question-only exploration, not a data run, when there is no evidence", async () => {
    renderApp(fakePorts(fakeState({ settings: configured })), { hash: "#/projects/p1/analysis" });
    expect((await screen.findAllByText(noEvidence)).length).toBeGreaterThan(0);
    expect(screen.queryByRole("button", { name: "Run analysis" })).toBeNull();
    const explore = screen.getByRole("button", { name: "Explore hypotheses (no evidence)" }) as HTMLButtonElement;
    expect(explore.disabled).toBe(true); // an empty question cannot be explored
    expect(screen.getByText("Enter a research question to explore.")).toBeTruthy();
    fireEvent.input(screen.getByLabelText(/Research question/), { target: { value: "Why?" } });
    expect((screen.getByRole("button", { name: "Explore hypotheses (no evidence)" }) as HTMLButtonElement).disabled).toBe(false);
  });

  it("starts an exploratory run with the question and the exploratory flag", async () => {
    const ports = fakePorts(fakeState({ settings: configured }));
    renderApp(ports, { hash: "#/projects/p1/analysis" });
    fireEvent.input(await screen.findByLabelText(/Research question/), { target: { value: "  Why?  " } });
    fireEvent.click(screen.getByRole("button", { name: "Explore hypotheses (no evidence)" }));
    await waitFor(() => expect(ports.analysis.start).toHaveBeenCalledWith("p1", { researchQuestion: "Why?", reasoningProfile: "GENERAL_RESEARCH", outputLocale: "", exploratory: true }));
  });

  it("starts with the saved theme and, without a model, keeps it but explains and links to settings", async () => {
    const ports = fakePorts(fakeState({ projects: [{ id: "p1", name: "T", researchQuestion: "Saved theme?", createdAt: "" }] }));
    renderApp(ports, { hash: "#/projects/p1/analysis" });
    expect(((await screen.findByLabelText(/Research question/)) as HTMLTextAreaElement).value).toBe("Saved theme?");
    expect(screen.getByText(/Exploring from a question needs a connected model/)).toBeTruthy();
    expect(screen.getByRole("link", { name: "Open settings" }).getAttribute("href")).toBe("#/settings");
    expect((screen.getByRole("button", { name: "Explore hypotheses (no evidence)" }) as HTMLButtonElement).disabled).toBe(true);
    expect(ports.analysis.start).not.toHaveBeenCalled();
  });

  it("retries a failed exploration as an exploration", async () => {
    const failedExploration = { id: "r1", projectId: "p1", status: "failed", progress: 20, createdAt: "", error: "boom", researchQuestion: "why?", reasoningProfile: "GENERAL_RESEARCH", exploratory: true };
    const ports = fakePorts(fakeState({ settings: configured, runs: [failedExploration] }));
    renderApp(ports, { hash: "#/projects/p1/analysis" });
    fireEvent.click(await screen.findByRole("button", { name: "Retry with the same settings" }));
    // The server retry restores the exploration from the run snapshot (jobmanager.go); the UI sends only the run id.
    await waitFor(() => expect(ports.analysis.retry).toHaveBeenCalledWith("r1"));
    expect(ports.analysis.start).not.toHaveBeenCalled();
  });

  it("shows an exploration result as unverified candidates with falsification, missing data and a way to add evidence", async () => {
    const exploration = {
      status: "EXPLORATORY_UNVERIFIED", verified: false, decisionReady: false, evidenceCount: 0, question: "Why?",
      candidates: [{ title: "Candidate A", explanation: "maybe structural", competingExplanations: [{ title: "Chance", explanation: "noise" }], falsificationConditions: ["unchanged elsewhere"], requiredData: [{ description: "a time series", why: "to compare" }] }],
      limitations: ["All candidates are unverified."],
    };
    const done = { id: "r1", projectId: "p1", status: "completed", progress: 100, createdAt: "", finishedAt: "2026-01-02T00:00:00Z", exploratory: true, metrics: { exploration } };
    renderApp(fakePorts(fakeState({ runs: [done] })), { hash: "#/projects/p1/findings?run=r1" });
    const result = await screen.findByRole("region", { name: "Hypothesis candidates" });
    expect(within(result).getByText("Unverified — no evidence used")).toBeTruthy();
    expect(within(result).getByText("Candidate A")).toBeTruthy();
    expect(within(result).getByText("unchanged elsewhere")).toBeTruthy();
    expect(within(result).getByText(/a time series/)).toBeTruthy();
    expect(screen.getByText("All candidates are unverified.")).toBeTruthy();
    expect(screen.getByRole("link", { name: /Add evidence to test these/ }).getAttribute("href")).toBe("#/projects/p1/input");
    expect(screen.queryByText(/verified$/i)).toBeNull();
  });

  it("keeps a visible link to an earlier exploration once evidence exists", async () => {
    const earlier = { id: "r0", projectId: "p1", status: "completed", progress: 100, createdAt: "", exploratory: true, metrics: { exploration: { status: "EXPLORATORY_UNVERIFIED", verified: false, decisionReady: false, evidenceCount: 0, question: "Why?", candidates: [], limitations: [] } } };
    const state = fakeState({ settings: configured, runs: [earlier], documents: [{ id: "d1", projectId: "p1", source: "dataset", title: "", content: "x", metadata: {}, createdAt: "" }] });
    renderApp(fakePorts(state), { hash: "#/projects/p1/analysis" });
    expect(await screen.findByRole("link", { name: "View those candidates" })).toBeTruthy();
  });

  it("shows a server failure and retries when its prerequisites are available", async () => {
    const ports = fakePorts(fakeState({ documents: [{ id: "d1", projectId: "p1", source: "dataset", title: "", content: "x", metadata: {}, createdAt: "" }], runs: [failed] }));
    renderApp(ports, { hash: "#/projects/p1/analysis" });
    expect(await screen.findByText(/No model is configured: the run is deterministic/)).toBeTruthy();
    expect(screen.getByText("Failed: the LLM is not configured")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Retry with the same settings" }));
    await waitFor(() => expect(ports.analysis.retry).toHaveBeenCalledWith("r1"));
    expect(ports.analysis.start).not.toHaveBeenCalled();
  });

  const englishSource = { id: "d1", projectId: "p1", source: "dataset", title: "", content: "Sales fell in March.", metadata: {}, createdAt: "" };

  it("offers the output language in the main start control and sends an explicit Japanese request for English sources", async () => {
    const ports = fakePorts(fakeState({ documents: [englishSource] }));
    renderApp(ports, { hash: "#/projects/p1/analysis" });
    const select = await screen.findByLabelText("Language of model-written text") as HTMLSelectElement;
    expect(select.closest("details")).toBeNull();
    expect(select.value).toBe("");
    fireEvent.change(select, { target: { value: "ja-JP" } });
    fireEvent.click(screen.getByRole("button", { name: "Run analysis" }));
    await waitFor(() => expect(ports.analysis.start).toHaveBeenCalledWith("p1", { researchQuestion: "", reasoningProfile: "GENERAL_RESEARCH", outputLocale: "ja-JP" }));
    expect(screen.queryByText(/did not record the requested output language/)).toBeNull();
  });

  it("starts with the output language of the project's latest run when the page is reopened", async () => {
    const earlier = { ...failed, outputLocale: "ja-JP" };
    renderApp(fakePorts(fakeState({ documents: [englishSource], runs: [earlier] })), { hash: "#/projects/p1/analysis" });
    expect((await screen.findByLabelText("Language of model-written text") as HTMLSelectElement).value).toBe("ja-JP");
  });

  it("warns when a legacy server does not record the requested output language", async () => {
    const ports = fakePorts(fakeState({ documents: [englishSource] }));
    ports.analysis.start.mockImplementationOnce(async () => ({ id: "r1", projectId: "p1", status: "queued", progress: 0, createdAt: "" }));
    renderApp(ports, { hash: "#/projects/p1/analysis" });
    fireEvent.change(await screen.findByLabelText("Language of model-written text"), { target: { value: "ja-JP" } });
    fireEvent.click(screen.getByRole("button", { name: "Run analysis" }));
    expect(await screen.findByText(/did not record the requested output language \(Japanese \(ja-JP\)\)/)).toBeTruthy();
  });
});

describe("workspace", () => {
  it("falls back to the latest completed run with a notice when ?run= is unknown", async () => {
    const done = { id: "r1", projectId: "p1", status: "completed", progress: 100, createdAt: "", finishedAt: "2026-01-02T00:00:00Z", metrics: { provenance: { mode: "deterministic", ruleVersion: "v1" } } };
    renderApp(fakePorts(fakeState({ runs: [done] })), { hash: "#/projects/p1?run=missing" });
    expect(await screen.findByText("The selected run was not found in this project; showing the latest completed run.")).toBeTruthy();
    const reports = screen.getAllByRole("link", { name: "Download report" });
    expect(reports.map((a) => a.getAttribute("href"))).toEqual(reports.map(() => "/report/p1/r1"));
  });

  it("shows the next step for an empty project", async () => {
    renderApp(fakePorts(), { hash: "#/projects/p1" });
    const region = await screen.findByRole("region", { name: "Next step" });
    expect(within(region).getByRole("link", { name: /Add evidence/ }).getAttribute("href")).toBe("#/projects/p1/input");
  });

  it("shows a server error with a way back", async () => {
    renderApp(fakePorts(), { hash: "#/projects/nope" });
    expect(await screen.findByRole("alert")).toBeTruthy();
    expect(screen.getByText("project not found")).toBeTruthy();
  });
});

describe("field feedback and readiness guards", () => {
  it("focuses the missing project name, connects descriptions and keeps summary links on the route", async () => {
    const ports = fakePorts(fakeState({ projects: [] }));
    renderApp(ports);
    fireEvent.click(await screen.findByRole("button", { name: "Create" }));
    const name = screen.getByLabelText("Project name");
    await waitFor(() => expect(document.activeElement).toBe(name));
    expect(name.getAttribute("aria-required")).toBe("true");
    expect(name.getAttribute("aria-invalid")).toBe("true");
    for (const id of name.getAttribute("aria-describedby")?.split(" ") ?? []) expect(document.getElementById(id)).not.toBeNull();
    fireEvent.click(within(screen.getByRole("alert")).getByRole("link"));
    expect(window.location.hash).toBe("#/");
    expect(document.activeElement).toBe(name);
    expect(ports.projects.create).not.toHaveBeenCalled();
    fireEvent.input(name, { target: { value: "Study" } });
    expect(name.getAttribute("aria-invalid")).toBe("false");
  });

  it("marks source and content required, title optional and prevents whitespace-only content", async () => {
    const ports = fakePorts();
    renderApp(ports, { hash: "#/projects/p1/input" });
    const content = await screen.findByLabelText("Content");
    expect(screen.getByLabelText("Source type").getAttribute("aria-required")).toBe("true");
    expect(screen.getByLabelText("Title").hasAttribute("required")).toBe(false);
    fireEvent.input(content, { target: { value: "   " } });
    fireEvent.click(screen.getByRole("button", { name: "Add document" }));
    await waitFor(() => expect(document.activeElement).toBe(content));
    expect(content.getAttribute("aria-invalid")).toBe("true");
    expect(ports.evidence.addText).not.toHaveBeenCalled();
  });

  it("required-later model settings do not prevent saving an unconfigured server", async () => {
    const ports = fakePorts();
    renderApp(ports, { hash: "#/settings" });
    const model = await screen.findByLabelText("Model");
    expect(model.hasAttribute("required")).toBe(false);
    expect(model.getAttribute("aria-required")).toBe("false");
    expect(document.getElementById("settings-model-requirement")?.textContent).toBe("Required later");
  });

  it("cannot submit or retry a text-only project without a model", async () => {
    const ports = fakePorts(fakeState({ documents: [{ id: "d1", projectId: "p1", source: "web", title: "", content: "x", metadata: {}, createdAt: "" }], runs: [{ id: "r1", projectId: "p1", status: "failed", progress: 0, createdAt: "" }] }));
    renderApp(ports, { hash: "#/projects/p1/analysis" });
    const start = await screen.findByRole("button", { name: "Run analysis" });
    expect((start as HTMLButtonElement).disabled).toBe(true);
    fireEvent.submit(start.closest("form") as HTMLFormElement);
    fireEvent.click(screen.getByRole("button", { name: "Retry with the same settings" }));
    expect(ports.analysis.start).not.toHaveBeenCalled();
  });
});

describe("terminology (#141)", () => {
  const done = { id: "ana_1234abcd", projectId: "p1", status: "completed", progress: 100, createdAt: "", finishedAt: "2026-01-02T00:00:00Z", metrics: { provenance: { mode: "deterministic", ruleVersion: "v1" } } };

  it("names a run in Japanese and keeps its raw ID out of headings but inspectable", async () => {
    const { container } = renderApp(fakePorts(fakeState({ runs: [done] })), { hash: "#/projects/p1/findings?run=ana_1234abcd", locale: "ja" });
    expect(await screen.findByRole("heading", { name: "表示中の実行のインサイト" })).toBeTruthy();
    expect(screen.getByText(/に完了した実行$/)).toBeTruthy();
    expect(screen.getByLabelText("結果を表示する実行")).toBeTruthy();
    for (const heading of screen.getAllByRole("heading")) expect(heading.textContent).not.toContain("ana_1234abcd");
    expect(container.querySelector("[data-run-id]")?.getAttribute("data-run-id")).toBe("ana_1234abcd");
    expect(screen.getByText("ana_1234abcd").tagName).toBe("CODE");
    expect(container.textContent).not.toMatch(/\bRun\b/);
  });

  it("labels customer-research sources without internal versioning and keeps their option values", async () => {
    const ports = fakePorts(fakeState({ documents: [{ id: "d1", projectId: "p1", source: "interview", title: "Memo", content: "x", metadata: {}, createdAt: "" }] }));
    const withLegacy = formats.map((f) => (f.kind === "documents" ? { ...f, sourceTypes: ["document", "web", "interview", "job_posting", "future_kind"] } : f));
    ports.system.importFormats.mockResolvedValue(withLegacy);
    const { container } = renderApp(ports, { hash: "#/projects/p1/input", locale: "ja" });
    const select = await screen.findByLabelText("ソースの種類") as HTMLSelectElement;
    const groups = [...select.querySelectorAll("optgroup")].map((g) => [g.label, [...g.querySelectorAll("option")].map((o) => o.value)]);
    expect(groups).toEqual([["一般的な資料", ["document", "web", "future_kind"]], ["顧客・市場調査向け", ["interview", "job_posting"]]]);
    expect(within(select).getByRole("option", { name: "future_kind" })).toBeTruthy();
    expect(screen.getAllByText("インタビュー").length).toBeGreaterThan(0);
    expect(container.textContent).not.toMatch(/旧形式|legacy/);
  });

  it("explains the accepted inputs in one sentence instead of a label-like line", async () => {
    const { container } = renderApp(fakePorts(), { hash: "#/projects/p1/input", locale: "ja" });
    expect(await screen.findByText(/^取り込めるのは、下に示す CSV 形式/)).toBeTruthy();
    expect(container.textContent).not.toContain("対応している入力:");
  });

  const insight = {
    id: "i1", projectId: "p1", analysisId: "ana_1234abcd", title: "Finding", observation: "", statedNeed: "", latentNeed: "A hypothesis", hypothesis: "",
    jtbd: "  ", expectation: "", surprisingFact: "", rationale: "", interpretation: "Reading", alternativeInterpretation: "Other reading",
    productOpportunity: "", monetizationAngle: "", confidence: 0.4, qualityFlags: [], createdAt: "", evidence: [], patterns: [],
  };

  it("hides empty customer-research insight fields and shows other missing fields as not recorded", async () => {
    const ports = fakePorts();
    ports.results.insight.mockResolvedValue(insight);
    const { container } = renderApp(ports, { hash: "#/insights/i1", locale: "ja" });
    const details = (await screen.findByRole("heading", { name: "詳細" })).closest("section") ?? container;
    expect(within(details as HTMLElement).queryByText(/表明されたニーズ|JTBD|プロダクト機会|収益化/)).toBeNull();
    expect(within(details as HTMLElement).getByText("未記録")).toBeTruthy();
    expect([...container.querySelectorAll("dd")].map((d) => d.textContent)).not.toContain("-");
  });

  it("shows a customer-research insight field when it has a value", async () => {
    const ports = fakePorts();
    ports.results.insight.mockResolvedValue({ ...insight, statedNeed: "Faster refunds" });
    renderApp(ports, { hash: "#/insights/i1" });
    expect(await screen.findByText("Stated need (customer-research field)")).toBeTruthy();
    expect(screen.getByText("Faster refunds")).toBeTruthy();
    expect(screen.queryByText(/Job to be done/)).toBeNull();
  });

  it("keeps unreadable settings explicitly unknown in Japanese, never ready", async () => {
    const ports = fakePorts(fakeState({ documents: [{ id: "d1", projectId: "p1", source: "web", title: "", content: "x", metadata: {}, createdAt: "" }] }));
    ports.settings.get.mockRejectedValue(new Error("unavailable"));
    const { container } = renderApp(ports, { hash: "#/projects/p1/analysis", locale: "ja" });
    expect(await screen.findByText("設定を取得できないため、状態は不明です。開始時にサーバーが確認します。")).toBeTruthy();
    expect(container.querySelector("[data-readiness]")?.getAttribute("data-readiness")).toBe("unknown");
    expect(container.textContent).not.toContain("実行可能");
    expect(container.textContent).not.toContain("UNKNOWN");
  });
});
