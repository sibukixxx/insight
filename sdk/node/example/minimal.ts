// The smallest end-to-end use of the TypeScript SDK: create an opaque
// subject, add evidence, run an analysis and print what it observed.
//
//   node example/minimal.ts http://127.0.0.1:8787
//
// Research runs (hypotheses, gaps, data requirements) additionally need an
// engine with a model configured.
import { InsightClient } from "../src/index.ts";

const client = new InsightClient({ baseUrl: process.argv[2] ?? "http://127.0.0.1:8787" });

const info = await client.getEngine();
console.log(`engine ${info.engine.version} (commit ${info.engine.commit}), contract ${info.contractVersion}`);

const subject = await client.createSubject({ idempotencyKey: "example-subject-node", subject: { namespace: "example", id: "city-inquiries-node" } });
await client.addEvidence(subject.subjectId, {
  idempotencyKey: "example-evidence-node",
  documents: [
    { externalRef: "jan", source: "dataset", content: "Dataset observation: 2026-01 Example City inquiries = 120.", metadata: { record_count: "120", period: "2026-01", event_type: "inquiry", location: "Example City" } },
    { externalRef: "feb", source: "dataset", content: "Dataset observation: 2026-02 Example City inquiries = 150.", metadata: { record_count: "150", period: "2026-02", event_type: "inquiry", location: "Example City" } },
  ],
});
const started = await client.startAnalysis(subject.subjectId, { idempotencyKey: "example-analysis-node" });
const run = await client.waitForAnalysis(subject.subjectId, started.analysisId);
console.log(`run ${run.analysisId} ${run.status} (${run.executionMode}), input ${run.inputFingerprint}`);
const results = await client.getAnalysisResults(subject.subjectId, run.analysisId);
for (const o of results.observations) console.log(`- [${o.externalRef}] ${o.quote}`);
