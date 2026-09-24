# Insight Lab Documentation

This directory contains three different kinds of documentation. Keeping them separate is intentional.

## Start here

| Document | Purpose |
| --- | --- |
| [Project scope](project-scope.md) | What belongs in the public OSS project and what does not |
| [BYO-Evidence boundary](byo-evidence-boundary.md) | Insight Lab reasons over provided evidence and never fetches it; how missing evidence leaves and re-enters the loop |
| [Project status](project-status.md) | What is implemented now, current limitations, and the next validation phase |
| [Causal reasoning semantics](causal-reasoning.md) | Runtime contract for causal claims, statuses, evidence, and guardrails |
| [Evaluation](evaluation/README.md) | How to run and inspect repeatable model-backed evaluation |
| [Public Engine Contract](public-engine-contract.md) | HTTP/JSON boundary for downstream consumers, Go and TypeScript SDKs, conformance fixtures |

## Contracts and workflows

- [Analytical Artifact contract](analytical-artifact-contract.md) — provider-neutral import of deterministic calculations.
- [Research loop](research-loop.md) — append-only research iterations over completed analyses.
- [Publication workflow](publication-workflow.md) — human review and approved research artifacts.
- [Corporate-event dogfooding](dogfooding-corporate-events.md) — reproducible dataset run from an external CSV.

## Architecture and implementation

- [Detailed design](detailed-design.md) — historical v1 design (pre BYO-Evidence / Research Loop). Kept for record; see `project-status.md` and `byo-evidence-boundary.md` for the current architecture.
- [Implementation plan](implementation-plan.md) — implementation history and planned phases. Treat current code and `project-status.md` as authoritative when they differ from older planning notes.
- [Design review](design-review.md) — historical design review and rationale.

## Learning material

- [Causal inference learning notes](learning-notes-causal-inference.md) — educational notes for understanding causal concepts used by the project. This is not the runtime contract.

The runtime contract is [causal-reasoning.md](causal-reasoning.md).

## Evaluation artifacts

`docs/evaluation/` contains evaluation instructions and generated outputs from model-backed dogfooding. Generated results are evidence about system behavior for a particular dataset/model/run; they are not universal quality claims.

## Documentation principles

1. **Code and tests define behavior.** Documentation must not claim capabilities that the current implementation does not provide.
2. **Observation and inference remain separate.** Documentation examples must not turn association into causation.
3. **Unknown is a valid result.** `NOT_IDENTIFIED` and `INSUFFICIENT_EVIDENCE` are useful outcomes, not failures to be hidden.
4. **Public OSS and commercial decision logic remain separate.** Reusable research mechanics belong here; customer-specific commercial judgment does not.
5. **Planning documents are not status documents.** Use `project-status.md` for the current state.

## Language

The canonical top-level README and implementation documentation are currently maintained in English. A Japanese project overview is available at [`../README.ja.md`](../README.ja.md).

Contributions improving translations are welcome as long as semantics stay aligned with the canonical implementation documentation.
