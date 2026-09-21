# Contributing to Insight Lab

Issues and pull requests are welcome. Insight Lab is intentionally conservative about evidence and causal claims, so changes to reasoning semantics need the same care as changes to code.

## Setup

You need Go 1.25 or later. CGO is not required.

```bash
git clone <repository-url>
cd insight
make build-demo
./bin/insight-lab-demo --demo
```

## Before opening a pull request

Run:

```bash
make vet
make test
```

For changes that affect model-backed reasoning or evaluation, also run the relevant deterministic fixtures and, when you have a configured provider, the evaluation workflow described in [`docs/evaluation/`](docs/evaluation/README.md).

## Contribution guidelines

- Keep each change focused on one concern.
- Add or update tests before implementation when practical.
- Prefer deterministic application-side checks for invariants that must not depend on model behavior.
- Use real internal components in tests and replace only external dependencies, such as the LLM API, with test doubles.
- Use descriptive test names.
- Include screenshots or reproducible verification steps for UI changes.
- Clearly identify breaking API, CLI, report-schema, or database changes.
- Add schema changes as forward-only migrations. Never edit or delete a migration that has already been merged.

## Causal-reasoning changes

Read [`docs/causal-reasoning.md`](docs/causal-reasoning.md) before changing causal statuses, identification behavior, candidate causal structures, evidence semantics, or confidence handling.

A contribution must not make the system claim stronger causal knowledge merely because:

- the model used causal language;
- a confidence/evidence-quality score is high;
- supporting evidence outnumbers counter-evidence;
- two events occurred in sequence;
- a candidate research design was suggested but not executed.

When the available information does not identify a causal effect, preserving `NOT_IDENTIFIED` is correct behavior.

## Public project boundary

Read [`docs/project-scope.md`](docs/project-scope.md) before adding domain-specific integrations or decision logic.

Reusable evidence-grounded research mechanics belong in this repository. Customer-specific pricing, proposals, estimates, commercial recommendations, and private decision logic do not.

## Documentation

User-facing capability claims must match current code and tests. Planning documents should not be presented as implemented functionality.

The documentation index is [`docs/README.md`](docs/README.md), and current status is tracked in [`docs/project-status.md`](docs/project-status.md).

## Commits and pull requests

Use [Conventional Commits](https://www.conventionalcommits.org/) with an English subject:

```text
<type>: <subject>
```

In the pull request description, explain:

1. the problem being solved;
2. the behavioral change;
3. tests/evaluation performed;
4. causal or evidence-semantics impact, if any;
5. known limitations.

Report vulnerabilities privately as described in [SECURITY.md](SECURITY.md). By participating, you agree to the [Code of Conduct](CODE_OF_CONDUCT.md). Contributions are licensed under the project's [Apache License 2.0](LICENSE).
