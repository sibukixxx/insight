# Insight Lab Go SDK

Thin, dependency-free Go client for the [Public Engine Contract v1](../../docs/public-engine-contract.md). It holds no research logic and never imports Insight's internal packages.

```sh
go get github.com/sibukixxx/insight/sdk/go
```

```go
client := insight.NewClient("http://127.0.0.1:8787")
subject, err := client.CreateSubject(ctx, insight.CreateSubjectRequest{
	IdempotencyKey: "my-subject",
	Subject:        insight.SubjectRef{Namespace: "my-app", ID: "item-42"},
})
// AddEvidence -> StartAnalysis -> WaitForAnalysis -> GetAnalysisResults
// CreateResearchRun -> AppendIteration -> GetResearchRun
```

A runnable example lives in [`example/minimal`](example/minimal/main.go):

```sh
go run ./example/minimal -engine http://127.0.0.1:8787
```

- Errors are `*insight.Error` with a contract `Code`. `errors.Is(err, &insight.Error{Code: insight.CodeNotFound})` matches on the code. `CodeUnavailable` means the engine could not be reached.
- The client fills `ContractVersion` and a random `IdempotencyKey` when they are empty. Pass your own key and reuse it on retry to get the first result back.
- `ResearchResult.View()` decodes the research artifact fields the contract promises.
- Package `conformance` runs the shared fixtures in `contracts/public-engine/v1/fixtures` against a live engine.
