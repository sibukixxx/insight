# Deployment conformance

Issue #139. Vendor neutrality has to be tested, not only described. This page lists what every supported topology must satisfy, which tests check it today, and which topologies are verified. None of these tests needs PostgreSQL, a broker, a cloud account or a paid model; only the container smoke test needs a Docker daemon.

## Topologies

| Topology | Status | Checked by |
|---|---|---|
| Local binary, LOCAL runtime | Verified | `go test ./...` (all packages) |
| Local binary, PROCESS runtime | Verified | `internal/cli/conformance_test.go`, `internal/execution/work_test.go` |
| Default Compose (one container, one volume) | Verified | `make docker-smoke` (#138) |
| External state store (#137) | Not implemented | — |
| Remote workers / broker (#135) | Not implemented | — |
| Provider-hosted services (AWS, Google Cloud, Azure, Cloudflare, …) | Not verified | Covered only as "a host running the binary or container with a persistent volume" (ADR 0001) |

An optional adapter is marked supported only after it ships its own run of the dimensions below and its documented limitations.

## Dimensions and current coverage

| Dimension | Requirement | Tests |
|---|---|---|
| Same result in every runtime | Same input and request give the same prepared artifact and research metrics; runtime differences are recorded as execution metadata (`runtimeMode`), never as a research difference | `TestDeploymentConformanceLocalAndProcessRuntimesProduceTheSameResearchInput` (real CLI and wiring, LOCAL vs PROCESS workers); `TestProcessAndInProcessWorkersProduceTheSameMergedAggregate`; `TestHeavyShardsAndSerializedWorkersKeepTheStandardArtifactForAnyShardSize`; conformance fixture `08-execution-profile-equivalence` |
| Shard-size invariance | Any shard size or count gives the single-pass aggregate, including quoted newlines, BOM and CRLF | `TestPlanCSVShardsMergedShardAggregatesEqualTheSinglePassForEveryShardSize` |
| No silent downgrade | An unavailable profile, runtime or input source fails with a clear error | `TestDeploymentConformanceUnavailableCapabilitiesFailInsteadOfDowngrading`; `TestParseConfigRuntimeIsLocalByDefaultAndProcessNeedsHeavyDirAndInputRoot` |
| Allowlisted work only | Unknown operation or version, tampered spec, malformed range and unsafe IDs are rejected before any byte is read | `TestWorkSpecValidationAllowsOnlyTheKnownOperationAndAnIntactSpec` |
| Stale / duplicate completion | A result for another job, partition or input, or with a broken hash, is never merged | `TestAcceptRefusesAResultForAnotherPartitionOrWithABrokenHash` |
| Bad hash / changed input | Bytes that changed after planning or registration fail the work | `TestWorkerCannotReadOutsideItsRootOrUseChangedBytes`; `TestPrepareFailsWhenRawBytesChangedAfterRegistration` |
| Cross-scope access | Workers read only their input root; ingests and analyses are project scoped | `TestWorkerCannotReadOutsideItsRootOrUseChangedBytes`; `TestFileResolverRefusesPathsOutsideRoot`; `TestIngestFileNameIsDisplayOnlyAndNeverAPath` |
| Secret redaction | Workers receive no inherited environment; snapshots and responses carry no API key | `TestProcessAndInProcessWorkersProduceTheSameMergedAggregate` (worker fails if it sees `INSIGHT_LAB_API_KEY`); `TestCreateAnalysisRecordsLabelNoteAndExecutionSnapshotWithoutSecrets` |
| Worker crash / timeout / cancel | Crash is reported and retried, a hung worker is killed, cancel stops the job | `TestProcessDispatcherReportsCrashesAndTimeouts`; `TestLocalRuntimeRetriesACrashedProcessPartitionAndCancelStopsTheJob` |
| Coordinator restart | Interrupted runs become INTERRUPTED, queued runs resume only when reproducible, interrupted ingests requeue without duplicate rows | `TestRunningAnalysisIsInterruptedNotResumedAfterACrash`; `TestQueuedAnalysisResumesAfterRestartWhenItsConfigurationIsReproducible`; `TestQueuedAnalysisNeedsRequeueAfterRestartWhenSettingsNoLongerMatch`; `TestIngestRestartRequeuesAnInterruptedIngestWithoutDuplicatingRows` |
| Crash before / after finalization | Partial work is never visible as success; a cancel requested before completion wins | `TestIngestCancelStopsQueuedAndRunningIngestsAndDiscardsTheirRows`; `TestCompletionLosesToACancelRequestedBeforeTheRunFinished` |
| Backpressure | Queues and staging are bounded and answer 503 / 507 / 413 | `TestEnqueueRejectsRunsBeyondTheQueueBound`; `TestCreateAnalysisHTTPAnswers503WhenTheQueueIsFull`; `TestIngestHTTPMapsLimitsAndInvalidRequestsToStatusCodes` |
| Idempotency | The same request resolves to the same receipt; one retry per failed run | `TestIngestSameBytesAndIdentityReturnTheSameReceiptWithoutDuplicateEvidence`; `TestRetryEnqueuesAFailedAnalysisOnceWithTheSameRequest`; Public Engine idempotency fixtures |
| One coordinator per database | A second process on the same database is refused | `TestOpenRefusesASecondEngineOnTheSameDatabaseUntilTheFirstCloses`; `TestHeadlessCommandRefusesADatabaseAnotherEngineOwns` |
| Persistence across recreation | Recreating the container keeps research state | `make docker-smoke` |
| Public contract and SDKs | Behavior over the Public Engine Contract is fixture-checked; SDK snapshots change only when the contract does | `internal/http/public_conformance_test.go` and `contracts/public-engine/v1/fixtures` |

## Not yet covered

- Remote-adapter specifics (lease expiry, fencing epochs, at-least-once redelivery, object-store outage) belong to #135 and to any external store under #137; they are listed in [persistence.md](persistence.md#requirements-for-a-future-external-state-store).
- Mixed-version WorkSpec compatibility: `insight-lab.work/v1` is the only version; a worker rejects any other. A second version will need an explicit compatibility test.
- Operating systems: CI and the tests above run on the developer and CI platforms; Windows is cross-compiled but its lock and PROCESS paths are not exercised by tests here.

## Running

```sh
make test          # includes the LOCAL/PROCESS conformance test
make docker-smoke  # container: healthy, Insight only, data survives recreation
```
