# Causal Inference Learning Notes

This note records the causal-inference concepts being learned alongside the development of Insight Lab. It is a learning note, not a claim that Insight Lab currently implements all of these methods.

## 1. Why causal inference matters to Insight Lab

Insight Lab should distinguish between:

- what was directly observed,
- what is merely associated,
- what might explain the observation,
- and what can actually be supported as causal.

A useful principle is:

> Observing that A and B occur together does not establish that changing A would change B.

This distinction is especially important for LLM-assisted analysis because language models can easily generate plausible explanations that sound causal even when the source data only supports association.

## 2. Correlation is not causation

A classic example is ice-cream sales and heatstroke. Both increase on hot days, but banning ice cream would not be expected to reduce heatstroke.

A plausible causal structure is:

```text
        Hot weather
        /         \
       v           v
Ice-cream sales  Heatstroke
```

Hot weather is a common cause of both variables. This is an example of **confounding**.

For Insight Lab, the analogous question is not only:

> Did X and Y appear together?

but also:

> What other variable could explain both X and Y?

## 3. Observation versus intervention

Ordinary observational analysis often asks about:

```text
P(Y | X)
```

For example:

> Among people who published specialist content, how often did customer demand increase?

Causal inference asks a different question:

```text
P(Y | do(X))
```

Conceptually:

> What would happen to Y if we intervened and changed X while leaving the relevant causal system otherwise comparable?

The `do(X)` notation is associated with structural causal models. Insight Lab should not claim to estimate `P(Y | do(X))` merely because an LLM can describe a possible causal story.

## 4. Running example: specialist content and customer demand

Consider a synthetic example inspired by an unusual customer-behavior story.

A service worker has a legal-education background. Instead of publishing the kind of promotional content normally expected in that industry, the worker regularly publishes detailed legal commentary. Customers working in legal professions subsequently begin mentioning those articles and requesting that worker.

Let:

```text
X = publishing legal/specialist content
Y = requests from customers in legal professions
```

An observed sequence such as:

```text
X occurs
↓
Y increases
```

is interesting, but it does not by itself justify:

```text
X causes Y
```

Possible explanations include:

1. shared intellectual interest,
2. differentiation from competitors,
3. perceived expertise or intelligence,
4. search/SEO discovery,
5. increased social-media exposure,
6. an unrelated time trend,
7. pre-existing popularity.

The value of the observation is therefore not that it proves a causal effect. Its value is that it creates a **surprising observation worth explaining and testing**.

## 5. Confounders

Suppose social-media exposure (`S`) affects both specialist-content publication and customer requests.

```text
       S
      / \
     v   v
     X   Y
```

If we also suspect `X -> Y`, the graph becomes:

```text
       S
      / \
     v   v
     X -> Y
```

The path:

```text
X <- S -> Y
```

is a backdoor path. A naive comparison between X and Y can mix the effect of X with the effect of S.

This is why causal inference is not simply a matter of putting every available variable into a statistical model. The assumed causal structure matters.

## 6. DAGs

A **Directed Acyclic Graph (DAG)** represents causal assumptions using directed arrows.

Example:

```text
Advertising -> Awareness -> Purchase
```

A DAG is not automatically discovered truth. It is a representation of assumptions that can be inspected, challenged, and combined with data.

For Insight Lab, a future causal hypothesis should therefore be treated as something like:

```text
candidate causal structure
+ supporting evidence
+ counter-evidence
+ possible confounders
+ missing evidence
```

rather than as an automatically established causal graph.

## 7. Potential outcomes and counterfactuals

Another major framework describes causal effects using potential outcomes.

For a treatment `X`:

```text
Y(1) = outcome if treatment is applied
Y(0) = outcome if treatment is not applied
```

The individual causal effect would conceptually be:

```text
Y(1) - Y(0)
```

The fundamental problem is that, for the same unit at the same time, we cannot observe both worlds.

If a person publishes specialist content, we observe the outcome in that world. We cannot simultaneously observe what would have happened to the identical person under identical conditions had they not published it.

This missing counterfactual is the fundamental problem of causal inference.

## 8. Average Treatment Effect (ATE)

Across a population, a common estimand is the Average Treatment Effect:

```text
ATE = E[Y(1)] - E[Y(0)]
```

For example, if a valid design estimates:

```text
E[Y(1)] = 0.60
E[Y(0)] = 0.40
```

then:

```text
ATE = 0.20
```

This means an estimated average increase of 20 percentage points, not that every individual improves by 20 points.

The arithmetic is easy. The difficult question is whether the design and assumptions allow `E[Y(1)]` and `E[Y(0)]` to be estimated meaningfully.

## 9. Randomized experiments

Randomized controlled trials are powerful because random assignment can make treatment and control groups comparable on both observed and, in expectation, unobserved pre-treatment factors.

In a simple randomized experiment, an effect estimate may be just:

```text
mean(Y | treatment) - mean(Y | control)
```

The statistical calculation can therefore be simple while the research design is the important part.

This leads to an important learning principle:

> In causal inference, the hardest problem is often not calculation but deciding whether a comparison is valid.

## 10. Prediction versus causal inference

Prediction asks:

> Who is likely to churn?

Causal inference asks:

> Which action would reduce churn?

A machine-learning model might accurately predict that a customer has an 80% probability of churn without telling us whether a discount, onboarding intervention, support call, or product change would reduce that probability.

Therefore:

```text
Prediction != Intervention Effect
```

This distinction is highly relevant to Insight Lab. Finding a pattern is not the same as finding an action that would change the outcome.

## 11. Association, intervention, counterfactual

A useful conceptual progression is:

### Association

What tends to happen when X is observed?

```text
P(Y | X)
```

### Intervention

What would happen if X were deliberately changed?

```text
P(Y | do(X))
```

### Counterfactual

For a specific observed case, what would have happened if X had been different?

These questions require increasingly strong assumptions and evidence.

## 12. Connection to Insight Lab's methodology

The current direction for Insight Lab can be summarized as:

```text
Data
  ↓
Observation
  ↓
Expectation
  ↓
Expectation Violation / Surprise
  ↓
Competing Hypotheses
  ↓
Supporting Evidence + Counter-Evidence
  ↓
Possible Confounders
  ↓
Falsification Criteria
  ↓
Validation Status
  ↓
Insight Candidate
```

Causal inference adds an important guardrail:

```text
Observation
    ↓
Association
    ↓
Candidate causal hypothesis
    ↓
Possible causal structures
    ↓
Confounders / alternative explanations
    ↓
Required evidence
    ↓
Causal status
```

When the available data cannot identify a causal effect, the scientifically useful result is:

```text
CAUSAL STATUS: NOT IDENTIFIED
```

rather than a fabricated causal confidence score.

## 13. Relationship to abductive reasoning

Causal inference is not the whole Insight Lab methodology.

A useful separation is:

```text
Surprise detection
    ↓
Abduction
"What could explain this?"
    ↓
Competing hypotheses
    ↓
Causal reasoning
"Would changing X actually change Y?"
    ↓
Evidence / falsification
```

Abduction helps generate explanations. Causal inference helps determine what would be required to distinguish association from intervention effects. Falsification asks what observations should weaken a hypothesis.

These roles should remain separate in the implementation.

## 14. Important scientific guardrails

Insight Lab should avoid the following claims unless the data and research design genuinely support them:

- correlation proves causation,
- temporal order alone proves causation,
- an LLM-generated explanation is evidence,
- an LLM-generated DAG is the true causal graph,
- a numerical confidence score is a probability that a causal hypothesis is true,
- adding more control variables always improves a causal estimate.

The system should instead expose assumptions and uncertainty.

## 15. Concepts to study next

The next learning topics are:

1. DAGs in more detail
2. confounders
3. colliders
4. mediators
5. backdoor paths and the backdoor criterion
6. adjustment sets
7. potential outcomes and identification
8. propensity scores / matching
9. instrumental variables
10. regression discontinuity designs
11. difference-in-differences
12. heterogeneous treatment effects / CATE
13. causal forests
14. double/debiased machine learning
15. causal discovery
16. Bayesian approaches to uncertainty and causal modeling

The immediate next step should be **confounder vs collider vs mediator**, because this explains why controlling for every available variable can make a causal analysis worse rather than better.

## 16. Current takeaway

The key lesson so far is:

> Causal inference is less about producing a sophisticated number and more about making explicit what must be assumed before data can justify a statement about what would happen under an intervention.

For Insight Lab, this means the goal should not be to make the LLM sound more certain. The goal should be to make the reasoning trail more inspectable:

```text
What did we observe?
What did we expect?
Why is the difference surprising?
What explanations compete?
What causal relationship is being proposed?
What could confound it?
What evidence supports or contradicts it?
What would falsify it?
What additional data would be required?
What can we currently claim — and what can we not claim?
```

This is the scientific boundary that should guide future causal features in Insight Lab.
