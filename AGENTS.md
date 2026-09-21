# Insight Lab

Evidence-first analysis engine. Keep observations, expectations, mismatches, competing hypotheses, supporting/counter evidence, limitations, and insights distinguishable and traceable.

## Commands
- `make build` — delivery build without demo data
- `make build-demo` — demo-tag build
- `make test` — normal + demo-tag tests
- `make test-golden` — golden evaluation harness
- `make vet`
- `make eval-demo` — real-LLM evaluation; requires configured API credentials

## Shared rules
- Never present an observation, anecdote, or model output as a validated insight without its evidence state and limitations.
- Delivery builds must not embed demo/sample data; preserve the build-tag separation in `internal/sampledata/`.
- Keep evidence and counter-evidence history reproducible; do not overwrite prior evidence to make a hypothesis look cleaner.
- Do not run real-LLM evaluation merely as a completion check when credentials/cost are not part of the task.

## Change-dependent checks
- Core/domain changes: `make test && make vet`.
- Insight scoring/evaluation changes: also run `make test-golden`.
- Demo-only changes: verify both normal and `demo` build paths remain separated.

## Done
- Changed analysis behavior is covered by tests or golden fixtures as applicable.
- Delivery build remains free of demo data.
- Unverified claims and checks that could not run are explicitly reported.
