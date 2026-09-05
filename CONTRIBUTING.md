# Contributing to Insight Lab

Issues and pull requests are welcome.

## Setup

You need Go 1.25 or later. CGO is not required.

```bash
git clone https://github.com/sibukixxx/insight.git
cd insight
make build-demo
./bin/insight-lab-demo --demo
```

## Before opening a pull request

```bash
make vet
make test
```

- Keep each change focused on one concern.
- Add or update tests before implementation when practical.
- Use real internal components and replace only external dependencies, such as the LLM API, with test doubles.
- Use descriptive test names, for example `TestExtractInsightsShouldReturnEmptyWhenNoObservations`.
- Include a screenshot or verification steps for UI changes.
- Clearly identify breaking API, CLI, or database changes.

Use [Conventional Commits](https://www.conventionalcommits.org/) with an English subject:

```text
<type>: <subject>
```

Add schema changes as forward-only migrations. Never edit or delete a migration that has been merged.

Report vulnerabilities privately as described in [SECURITY.md](SECURITY.md). By participating, you agree to the [Code of Conduct](CODE_OF_CONDUCT.md). Contributions are licensed under the project's [Apache License 2.0](LICENSE).
