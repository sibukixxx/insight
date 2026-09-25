# Reasoning profiles (#109)

A reasoning profile decides which research vocabulary the engine layers on top of its shared epistemic rules. It is one of five independent axes (see [architecture](architecture.md)); it does not replace AnalysisMode, ResearchStage, ExecutionMode or ExecutionProfile.

| Profile | Selected by | Adds |
| --- | --- | --- |
| `GENERAL_RESEARCH` | default (field omitted) | nothing — the domain-neutral, question-conditioned prompts from #107/#108 |
| `CUSTOMER_INSIGHT` | `reasoningProfile: "CUSTOMER_INSIGHT"` | stated vs latent need, behavioral traces, JTBD, optional commercial projections, customer-specific quality checks |

## What every profile shares

Profiles are prompt **extensions**, not separate pipelines. Every profile runs the same stages, schemas and temperatures, and keeps:

- grounding: claims cite source/observation ids;
- evidence and counter-evidence retrieval, alternative explanations and falsification criteria;
- causal guardrails (correlation or a single quote is not causation);
- ResearchGap, iteration and Research Artifact semantics, readiness and promotion.

A profile never relaxes these rules and never adds or removes a stage.

## CUSTOMER_INSIGHT specifics

- Hypotheses fill `statedNeed` (what customers said), `latentNeed` (what the evidence suggests; must not restate the stated need) and `jtbd` when the evidence supports them. A stated need is not proof of a latent need; at least one alternative explanation must not rely on an unmet need.
- `productOpportunity` / `monetizationAngle` are optional, untested projections. They are kept only under this profile and never drive research truth, readiness or promotion. Under `GENERAL_RESEARCH` the engine blanks them whatever the model returns.
- The stated-need echo and generic latent-need checks (`quality/v3`) run only under this profile, so generic research is never penalized by a customer-needs vocabulary.

## Selection and provenance

- The profile is always explicit. It is never inferred from the source type, subject namespace or wording.
- The execution snapshot records the request and resolution (`reasoningProfileResolution`). For a non-default profile on a model-backed run, `reasoningProfile` is part of the fingerprinted execution config and the prompt fingerprint covers the extension. `GENERAL_RESEARCH` adds nothing: its prompt fingerprint equals the one recorded before profiles existed. Its execution fingerprint shifts once, because gating the customer quality checks moved `ruleVersions.quality` to `quality/v3`.
- The input fingerprint never includes the profile. Same evidence under two profiles is an execution change, and run comparison shows a `reasoningProfile` field change.

## Where it is enforced

- `internal/service/reasoning_profile.go` — prompt extensions and commercial-projection gating.
- `internal/service/reasoning_profile_test.go` — GENERAL baseline unchanged, CUSTOMER extends the shared invariants, fingerprints, quality gating, comparison.
- `contracts/public-engine/v1/fixtures/18-reasoning-profiles.json` — model-backed conformance over the same evidence under both profiles.
