# Insight Lab

[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-1.25%2B-00ADD8.svg)](go.mod)

Insight Lab is a local-first research tool that finds hidden customer needs in interviews, reviews, support conversations, and other text.

Every insight is linked to verified source quotes, counter-evidence, and an application-calculated confidence score. The reasoning trail—from an expected behavior, through a surprising deviation, to a hypothesis—remains visible and auditable.

## Quick start

Requirements: Go 1.25 or later and an OpenAI-compatible API.

```bash
make build-demo
./bin/insight-lab-demo --demo
```

Insight Lab opens at `http://127.0.0.1:8787`. Add your API base URL, model, and API key on the Settings page, or pass `--base-url`, `--model`, and `--api-key`.

## How to use it

1. Open the included fictional demo or create a project.
2. Paste text or import a CSV with `id,source,title,content` columns.
3. Run the analysis.
4. Review each insight's evidence, counter-evidence, reasoning trail, confidence, and quality warnings.
5. Download the result as a Markdown report.

Reports are also available through the API:

```bash
curl -o report.md http://127.0.0.1:8787/api/projects/<projectID>/report.md
```

Input may be written in any language supported by your configured model. Insight Lab asks the model to keep generated analysis in the input language; the interface and top-level project documentation are in English.

## Build and test

```bash
make build       # production build without demo data
make build-demo  # build with the fictional demo dataset
make vet
make test
```

The default build never embeds the demo dataset. Run `make cross-compile` to build production and demo binaries for macOS, Linux, and Windows.

To evaluate output quality with a real model:

```bash
INSIGHT_LAB_API_KEY=sk-... INSIGHT_LAB_MODEL=gpt-5 make eval-demo
```

## Documentation

- [Contributing guide](CONTRIBUTING.md)
- [Security policy](SECURITY.md)
- [Code of Conduct](CODE_OF_CONDUCT.md)

## Privacy

Project data is stored locally. Only text required for analysis is sent to the configured AI provider. Review that provider's data-handling policy before processing confidential or personal information.

## License

Copyright 2026 Yuichi Takada. Licensed under the [Apache License 2.0](LICENSE).

Third-party Go modules are not relicensed by this project. Their own license terms continue to apply; see `go.mod` and `go.sum` for the exact dependency set.
