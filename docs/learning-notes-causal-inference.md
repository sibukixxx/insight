# Causal Inference Learning Notes

This note records the causal-inference concepts being learned alongside the development of Insight Lab. It is a learning note, not a claim that Insight Lab currently implements all of these methods.

## 1. Why causal inference matters to Insight Lab

Insight Lab should distinguish between what was directly observed, what is merely associated, what might explain the observation, and what can actually be supported as causal.

> Observing that A and B occur together does not establish that changing A would change B.

LLMs can generate plausible explanations that sound causal even when source data supports only association. Insight therefore needs application-side guardrails rather than relying on model wording.

## 2. Correlation, confounding, and intervention

A classic example is ice-cream sales and heatstroke. Both increase on hot days, but banning ice cream would not be expected to reduce heatstroke.

```text
        Hot weather
        /         \
       v           v
Ice-cream sales  Heatstroke
```

Hot weather is a common cause: a confounder.

Observational analysis often asks about `P(Y | X)`. Causal inference asks what would happen under an intervention, conceptually `P(Y | do(X))`. Insight must not claim intervention effects merely because an LLM can describe a causal story.

## 3. DAGs and variable roles

A Directed Acyclic Graph represents causal assumptions, not automatically discovered truth.

```text
Confounder: X <- C -> Y
Mediator:   X -> M -> Y
Collider:   X -> C <- Y
```

Practical rules:

- confounder: common cause; genuine backdoor paths may need blocking;
- mediator: mechanism from X to Y; adjustment depends on whether the estimand is total or direct effect;
- collider: common effect; conditioning can create bias.

In shorthand:

> 交絡は塞ぎたい。媒介は目的次第。Collider は不用意に触らない。

Do not control for every available variable. The causal role follows the assumed graph, not the column name.

## 4. Backdoor paths

For target `X -> Y`, a path such as:

```text
X <- C -> Y
```

is a backdoor path. If it remains open, observed association can mix the proposed effect of X with C. Adjustment-set selection must block genuine problematic paths without accidentally conditioning on mediators or colliders.

## 5. Potential outcomes and counterfactuals

For treatment X:

```text
Y(1) = outcome if treatment is applied
Y(0) = outcome if treatment is not applied
```

Individual causal effect is conceptually `Y(1) - Y(0)`, but both worlds cannot be observed for the same unit at the same time. This is the fundamental problem of causal inference.

A common population estimand is:

```text
ATE = E[Y(1)] - E[Y(0)]
```

The difficult part is not arithmetic; it is establishing a valid comparison and defensible assumptions.

Randomization illustrates this principle because treatment and control groups can be comparable in expectation on pre-treatment factors.

## 6. Prediction is not intervention

Prediction asks:

> Who is likely to churn?

Causal inference asks:

> Which action would reduce churn?

Therefore:

```text
Prediction != Intervention Effect
```

A highly accurate predictive model does not by itself identify an effective intervention.

## 7. Association, intervention, counterfactual

```text
Association    -> What happens when X is observed?
Intervention   -> What would happen if X were deliberately changed?
Counterfactual -> What would have happened for this case if X had differed?
```

These questions require increasingly strong assumptions and evidence.

## 8. Difference-in-Differences intuition

A useful synthetic example:

```text
Treated group:  2024 = 100, 2025 = 130  -> +30
Control group:  2024 = 200, 2025 = 240  -> +40
```

A naive reading of the treated group alone might attribute `+30` to treatment. But the control group rose by `+40` over the same period.

The simple DiD contrast is:

```text
(130 - 100) - (240 - 200)
= 30 - 40
= -10
```

Under the required assumptions, the treated group changed by 10 fewer units than the comparison trend. This does **not** justify claiming that the treatment caused +30.

The crucial assumption is parallel trends: absent treatment, the treated group would have followed a comparable trend to the control group. A single pre-treatment period cannot meaningfully validate that assumption. DiD is therefore a research design with assumptions, not a magic subtraction formula.

## 9. Insight Lab methodology

```text
Data
  -> Observation
  -> Expectation
  -> Expectation Mismatch / Surprise
  -> Competing Hypotheses
  -> Supporting Evidence + Counter-Evidence
  -> Candidate Causal Structures
  -> Confounders / Mediators / Colliders
  -> Missing Evidence / Falsification
  -> Validation Status
  -> Identification Status
  -> Insight Candidate
```

When available data cannot identify a causal effect, the scientifically useful result is:

```text
NOT_IDENTIFIED
```

rather than a fabricated causal confidence score.

An LLM-generated DAG remains a candidate causal structure. Model-generated expectations and confounders remain hypotheses until grounded. Confidence/evidence-quality scores are not probabilities that a causal claim is true.

## 10. LLM scope and determinism

The intended boundary is:

```text
LLM:
- candidate generation
- competing explanations
- natural-language assistance

Application / deterministic rules:
- schema validation
- provenance
- grounding invariants
- causal-status guards
- validation/identification constraints
- deterministic ordering and artifact construction where feasible
```

The LLM must not be the final authority for causal certification, confidence semantics, or validation status.

Observation, Evidence, Counter Evidence, Confounders, Falsification Criteria, and Identification must not become final solely because the model emitted them.

For reproducibility, prefer normalized inputs, structured output/schema validation, deterministic sorting, stable IDs, low/zero-temperature-equivalent configuration where supported, and explicit model/rule/analysis versions. Model/system-prompt/rule changes should define a new reproducibility boundary.

If the LLM is unavailable, deterministic extraction/comparison/validation/status logic should remain usable where the operation does not intrinsically require generation.

## 11. Competing hypotheses

A plausible story is not enough. A Surprise should be examined through genuinely competing explanations.

```text
Surprise
  -> PRIMARY hypothesis
  -> COMPETING hypothesis
  -> COMPETING hypothesis
  -> ...
```

Each hypothesis should be evaluated independently for supporting evidence, counter-evidence, missing evidence, validation, and identification. Evidence must not leak between candidates merely because they share a HypothesisSet.

Too few independent explanations is itself a quality problem. The goal is not to make the model pick a winner prematurely; it is to expose which explanations survive the available evidence and what would distinguish them.

## 12. Counter-evidence and falsification

Counter-evidence and falsification criteria are different:

- counter-evidence: evidence already observed that weakens a hypothesis;
- falsification criterion: a future or additional observation that would weaken the hypothesis if found.

The system must not present an invented falsification condition as if it had already been observed.

Repeated dogfooding failures should drive the roadmap. For example, if real cases repeatedly produce weak counter-evidence, improving counter-evidence retrieval/evaluation becomes a higher priority than adding unrelated features.

## 13. Research designs are candidates until executed

Insight may recommend that a question could benefit from designs such as:

- randomized experiment;
- natural experiment;
- Difference-in-Differences;
- Regression Discontinuity;
- Instrumental Variables;
- matching / propensity methods.

A suggested design is not an executed design and does not upgrade a claim to `CAUSALLY_SUPPORTED`.

## 14. From theory to dogfooding

The immediate development question is no longer simply "can Insight produce a report?" The important test is whether it improves research.

Dogfooding should use multiple, meaningfully different datasets rather than optimizing Insight around one source. A company/public-data source can be one case, but Insight must remain the primary generic research engine.

A useful loop is:

```text
Research Question
-> Dataset
-> Observation
-> Expectation
-> Surprise
-> Competing Hypotheses
-> Evidence / Counter-Evidence
-> Missing Evidence
-> What data is needed next?
-> Additional research/data
-> Re-analysis
-> Human evaluation
```

The process should not force a causal conclusion. `NOT_IDENTIFIED` can be a successful result when the system correctly explains why the evidence is insufficient and what would reduce uncertainty.

## 15. Dogfooding acceptance criterion

The practical pass/fail criterion agreed during development is:

> Insight passes a dogfooding case when it surfaces at least one fact, anomaly, or relationship that the researcher had not noticed before, and then produces concrete competing explanations plus a useful next investigation that could distinguish or test them.

A run is **not** considered successful merely because it:

- summarizes the input correctly;
- restates something the researcher already knew;
- produces fluent but generic commentary;
- invents a plausible causal story;
- outputs a polished report;
- assigns a high confidence score.

The key test is discovery plus research progression.

```text
PASS
= grounded new discovery
  + meaningful competing explanations
  + actionable next evidence/research step

FAIL
= summary
  + known facts
  + generic speculation
```

The north-star evaluation question is:

> Did Insight help me discover something I did not know before the analysis?

A second important question is:

> Did Insight tell me what evidence would most efficiently reduce the remaining uncertainty?

## 16. What to do after a dogfooding result

Do not automatically add more features before evaluation.

```text
If PASS:
  preserve the case/evaluation and move toward practical use.

If FAIL:
  identify the exact failed stage in the research loop and improve that capability.
```

Examples:

```text
No useful new observation
-> improve Observation / Surprise discovery.

Only one plausible story
-> improve competing-hypothesis generation/diversity.

No serious challenge to attractive hypotheses
-> improve Counter Evidence.

Result is uncertain but gives no path forward
-> improve Missing Evidence / Next Data Requirement.

Overconfident causal language
-> strengthen identification and causal guardrails.
```

The roadmap should therefore emerge from repeated observed failures, not speculative feature accumulation.

## 17. OSS versus private dogfooding boundary

Insight Lab should remain a generic public OSS research engine.

Public OSS may include:

```text
Generic ingestion
Observation / Surprise discovery
Competing hypotheses
Evidence / Counter Evidence
Research gaps
Next-data requirements
Research iterations
Validation / Identification
Auditable reports
Synthetic/generic evaluation fixtures
```

Real dogfooding data, customer data, domain-specific commercial interpretation, pricing, recommendations, and accumulated business heuristics should not be embedded in the public Insight repository.

External data sources should normally cross a generic data boundary such as CSV/JSONL rather than becoming core dependencies.

Dogfooding should improve generic OSS capabilities when a reusable weakness is discovered, while case-specific knowledge remains outside the OSS core.

## 18. Scientific guardrails

Avoid the following unless the data and research design genuinely support them:

- correlation proves causation;
- temporal order alone proves causation;
- an LLM-generated explanation is evidence;
- an LLM-generated DAG is the true causal graph;
- numerical confidence is the probability a causal hypothesis is true;
- adding more control variables always improves a causal estimate;
- conditioning on a post-treatment variable is automatically safe;
- a candidate research design means identification has been achieved.

## 19. Concepts to study next

Theory should now be pulled by real cases rather than studied indefinitely in isolation. Likely next concepts include:

1. backdoor criterion and adjustment sets;
2. Difference-in-Differences and parallel trends in practice;
3. propensity scores / matching;
4. instrumental variables;
5. regression discontinuity;
6. heterogeneous treatment effects / CATE;
7. causal forests;
8. double/debiased machine learning;
9. causal discovery;
10. Bayesian approaches to uncertainty and causal modeling.

## 20. Current takeaway

Causal inference is less about producing a sophisticated number and more about making explicit what must be assumed before data can justify a statement about what would happen under an intervention.

For Insight Lab:

```text
Do not start with regression.
Start with the research/causal question.
Separate observation from interpretation.
Use expectations to find genuine surprises.
Generate competing explanations.
Draw candidate causal structures.
Do not control for everything.
Block genuine backdoor paths.
Treat mediators according to the estimand.
Do not casually condition on colliders.
Search for counter-evidence.
State missing evidence explicitly.
Separate association from intervention claims.
Say NOT_IDENTIFIED when the design cannot support causality.
Use dogfooding failures to decide what to build next.
```

The goal is not to make the LLM sound more certain. The goal is to make discovery, assumptions, competing explanations, evidence, counter-evidence, uncertainty, and the next research step inspectable.