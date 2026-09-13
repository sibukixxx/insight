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

```text
        Hot weather
        /         \
       v           v
Ice-cream sales  Heatstroke
```

Hot weather is a common cause of both variables. This is an example of **confounding**.

## 3. Observation versus intervention

Observational analysis often asks about `P(Y | X)`. Causal inference asks what would happen under an intervention, conceptually `P(Y | do(X))`.

Insight Lab should not claim to estimate intervention effects merely because an LLM can describe a possible causal story.

## 4. Running example: specialist content and customer demand

Consider a synthetic example: a service worker with legal education publishes detailed legal commentary instead of the promotional content normally expected in that industry. Customers working in legal professions subsequently begin mentioning those articles and requesting that worker.

```text
X = publishing legal/specialist content
Y = requests from customers in legal professions
```

The sequence `X occurs -> Y increases` is interesting but does not establish `X causes Y`.

Possible explanations include shared intellectual interest, differentiation, perceived expertise, search/SEO discovery, social-media exposure, an unrelated time trend, and pre-existing popularity. The observation is therefore a surprising fact worth explaining and testing, not causal proof.

## 5. DAGs

A **Directed Acyclic Graph (DAG)** represents causal assumptions using directed arrows.

```text
Advertising -> Awareness -> Purchase
```

A DAG is not automatically discovered truth. It is a representation of assumptions that can be inspected, challenged, and combined with data.

The role of a variable is determined by the arrows in the causal graph, not by the variable's name.

## 6. Confounders

A confounder is a common cause of the treatment/exposure and outcome.

```text
X <- C -> Y
```

For example, if social-media popularity affects both specialist-content publication and customer requests:

```text
Specialist content <- Social-media popularity -> Customer requests
```

then the path through social-media popularity can make X and Y appear related even when part of the association comes from C. This is a **backdoor path**.

For estimating the causal effect of X on Y, a genuine pre-treatment confounder is generally a variable we want to account for or block, subject to the assumptions of the design.

## 7. Mediators

A mediator lies on a causal pathway from X to Y.

```text
X -> M -> Y
```

Example:

```text
Specialist content
    -> perceived intellectual expertise
    -> customer requests
```

Whether to adjust for a mediator depends on the estimand.

If the question is the **total effect** of specialist content on customer requests, adjusting away the mediator can remove part of the very effect we want to measure. If the question is a **direct effect** that excludes the mediated pathway, mediation requires a different analysis and stronger assumptions.

Therefore a mediator is not simply another control variable.

## 8. Colliders

A collider is a variable caused by two other variables.

```text
X -> C <- Y
```

Example:

```text
Specialist content -> Goes viral <- Pre-existing popularity
```

If we condition on or select only cases where `Goes viral = true`, we can create a statistical relationship between specialist content and pre-existing popularity even when no such causal relationship existed before conditioning.

This is **collider bias**.

A useful medical-style example is:

```text
Smoking -> Hospitalization <- Infection
```

Analyzing only hospitalized people can induce a misleading association between smoking and infection because hospitalization is a common effect of both.

The practical lesson is important: **controlling for every available variable can make causal analysis worse**.

## 9. Confounder vs mediator vs collider

For a target causal relationship `X -> Y`:

```text
Confounder: X <- C -> Y
Mediator:   X -> M -> Y
Collider:   X -> C <- Y
```

A compact memory rule is:

- **Confounder:** a common cause of X and Y; usually something we want to block.
- **Mediator:** a mechanism through which X reaches Y; treatment depends on whether we want total or direct effect.
- **Collider:** a common effect; generally do not condition on it without a specific causal reason.

In Japanese shorthand:

> 交絡は塞ぎたい。媒介は目的次第。Collider は不用意に触らない。

## 10. Backdoor paths

Suppose the target is:

```text
Specialist content -> Customer requests
```

but social-media popularity creates:

```text
Specialist content <- Social-media popularity -> Customer requests
```

The second route enters X through an arrow pointing into X, so it is a backdoor path. If it remains open, the observed X-Y association can mix the proposed effect of X with the influence of social-media popularity.

The next learning step is the **backdoor criterion**: how to choose an adjustment set that blocks problematic backdoor paths without accidentally conditioning on mediators or colliders.

## 11. Potential outcomes and counterfactuals

For treatment X:

```text
Y(1) = outcome if treatment is applied
Y(0) = outcome if treatment is not applied
```

The individual causal effect is conceptually `Y(1) - Y(0)`. We cannot observe both worlds for the same unit at the same time. This missing counterfactual is the fundamental problem of causal inference.

## 12. Average Treatment Effect (ATE)

A common population estimand is:

```text
ATE = E[Y(1)] - E[Y(0)]
```

The arithmetic is simple; the difficult part is establishing a research design and assumptions that make the comparison meaningful.

## 13. Randomized experiments

Random assignment can make treatment and control groups comparable, in expectation, on pre-treatment factors. This illustrates an important principle:

> In causal inference, the hardest problem is often not calculation but deciding whether a comparison is valid.

## 14. Prediction versus causal inference

Prediction asks:

> Who is likely to churn?

Causal inference asks:

> Which action would reduce churn?

Therefore:

```text
Prediction != Intervention Effect
```

A highly accurate predictive model does not by itself tell us which intervention will change the outcome.

## 15. Association, intervention, counterfactual

A useful progression is:

```text
Association    -> What happens when X is observed?
Intervention   -> What would happen if X were deliberately changed?
Counterfactual -> For this case, what would have happened if X had been different?
```

These questions require increasingly strong assumptions and evidence.

## 16. Connection to Insight Lab's methodology

```text
Data
  -> Observation
  -> Expectation
  -> Expectation Violation / Surprise
  -> Competing Hypotheses
  -> Supporting Evidence + Counter-Evidence
  -> Possible Confounders
  -> Falsification Criteria
  -> Validation Status
  -> Insight Candidate
```

Causal inference adds a guardrail:

```text
Observation
  -> Association
  -> Candidate causal hypothesis
  -> Candidate DAG / causal structures
  -> Variable roles
       - confounder
       - mediator
       - collider
  -> Alternative explanations
  -> Required evidence / identification strategy
  -> Causal status
```

When available data cannot identify a causal effect, the scientifically useful result is:

```text
CAUSAL STATUS: NOT IDENTIFIED
```

rather than a fabricated causal confidence score.

An LLM-generated DAG must remain a **candidate causal structure**, not be presented as the true causal graph.

## 17. Relationship to abductive reasoning

```text
Surprise detection
    -> Abduction: "What could explain this?"
    -> Competing hypotheses
    -> Causal reasoning: "Would changing X actually change Y?"
    -> Evidence / falsification
```

Abduction generates explanations. Causal inference asks what evidence/design is required to distinguish association from intervention effects. Falsification asks what observations should weaken a hypothesis.

## 18. Important scientific guardrails

Avoid the following unless the data and research design genuinely support them:

- correlation proves causation,
- temporal order alone proves causation,
- an LLM-generated explanation is evidence,
- an LLM-generated DAG is the true causal graph,
- a numerical confidence score is a probability that a causal hypothesis is true,
- adding more control variables always improves a causal estimate,
- conditioning on a post-treatment variable is automatically safe.

## 19. From theory to dogfooding

The next stage should deliberately move from toy examples to reproducible public-data case studies.

For each case:

1. define the question and target causal relationship X -> Y,
2. record what is directly observed,
3. draw a small candidate DAG before fitting a model,
4. classify important variables as pre-treatment confounder, mediator, collider, or unresolved,
5. identify backdoor paths,
6. state what adjustment seems justified and why,
7. search for alternative explanations and counter-evidence,
8. run the simplest useful analysis,
9. distinguish the numerical result from the causal claim,
10. state what remains `NOT IDENTIFIED`,
11. document what additional data or design would strengthen identification.

The objective is not to force a surprising causal conclusion. A high-quality case study may conclude that a popular causal interpretation cannot be identified from the available public data.

## 20. Concepts to study next

1. backdoor criterion and adjustment sets
2. practical DAG exercises using real public datasets
3. potential outcomes and identification
4. propensity scores / matching
5. instrumental variables
6. regression discontinuity designs
7. difference-in-differences
8. heterogeneous treatment effects / CATE
9. causal forests
10. double/debiased machine learning
11. causal discovery
12. Bayesian approaches to uncertainty and causal modeling

The immediate priority is now **practice**: use real public data to make the distinction between observation, causal assumptions, adjustment, and identification concrete. Study additional theory when a real case requires it.

## 21. Current takeaway

Causal inference is less about producing a sophisticated number and more about making explicit what must be assumed before data can justify a statement about what would happen under an intervention.

The current practical rules are:

```text
Do not start with regression.
Start with the causal question.
Draw the DAG.
Do not control for everything.
Block genuine backdoor paths.
Treat mediators according to the estimand.
Do not casually condition on colliders.
Separate association from intervention claims.
Say NOT IDENTIFIED when the design cannot support causality.
```

For Insight Lab, the goal is not to make the LLM sound more certain. The goal is to make assumptions, competing explanations, evidence, counter-evidence, and the boundary of what can be claimed inspectable.