import { fireEvent, screen, waitFor, within } from "@testing-library/preact";
import { describe, expect, it } from "vitest";
import { fakePorts, fakeState } from "./fakePorts";
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

  it("offers header-only templates for every server-reported format", async () => {
    renderApp(fakePorts(), { hash: "#/projects/p1/input" });
    const links = await screen.findAllByRole("link", { name: /Download template/ });
    expect(links.map((a) => a.getAttribute("href"))).toEqual(["/template/documents", "/template/analysis"]);
  });
});

describe("analysis", () => {
  const failed = { id: "r1", projectId: "p1", status: "failed", progress: 30, createdAt: "", error: "the LLM is not configured", researchQuestion: "why?", reasoningProfile: "CUSTOMER_INSIGHT" };

  it("blocks a run without evidence and points to the input page", async () => {
    renderApp(fakePorts(), { hash: "#/projects/p1/analysis" });
    expect(await screen.findByText("No evidence yet. Add text or import a CSV first.")).toBeTruthy();
    expect((screen.getByRole("button", { name: "Run analysis" }) as HTMLButtonElement).disabled).toBe(true);
  });

  it("warns when no model is configured, shows the failure and retries with the same settings", async () => {
    const ports = fakePorts(fakeState({ documents: [{ id: "d1", projectId: "p1", source: "web", title: "", content: "x", metadata: {}, createdAt: "" }], runs: [failed] }));
    renderApp(ports, { hash: "#/projects/p1/analysis" });
    expect(await screen.findByText(/No model is configured and there are no dataset documents/)).toBeTruthy();
    expect(screen.getByText("Failed: the LLM is not configured")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Retry with the same settings" }));
    await waitFor(() => expect(ports.analysis.start).toHaveBeenCalledWith("p1", { researchQuestion: "why?", reasoningProfile: "CUSTOMER_INSIGHT", outputLocale: "" }));
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
