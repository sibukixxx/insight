# Remote runtime (DISTRIBUTED) — experimental PoC

Issue #135, under [ADR 0001](adr/0001-standalone-first-portable-deployment.md). DISTRIBUTED placement sends HEAVY partitions to workers on other hosts through a broker. It is **opt-in and experimental**: the default build contains no broker adapter, and nothing in the standalone path depends on it.

## Design

```text
coordinator (insight-lab serve -runtime distributed -broker URL)
  Heavy Runtime: job + partition state, retries, final merge   (-heavy-dir)
  RemoteDispatcher ──Task{token, WorkSpec}──▶ broker work queue ──▶ worker (insight-lab worker -broker URL -input-root DIR)
                   ◀──TaskResult{token, WorkResult}── results ◀──  reads its byte range from its own mount of the input root
```

- **Broker-neutral port.** `execution.Broker` has four operations: publish a task, take the next task (with an explicit ack), publish a result, subscribe to a job's results. Adapters register themselves by URL scheme; `execution.MemoryBroker` is the in-process reference used by the conformance tests.
- **What travels.** Only WorkSpecs (operation/version, input URI, byte ranges, hashes, spec) and WorkResults. Never raw bytes, API keys, code, SQL or host paths. Workers read inputs from their own read-only mount of the same immutable input root (a shared filesystem or a synchronized copy); every shard is checked against its content hash, so a stale or different copy fails instead of producing numbers.
- **Delivery.** At least once. A worker acknowledges a task only after publishing its result, so a crash redelivers the task. Redelivery is bounded (JetStream `MaxDeliver` = 5).
- **Fencing.** Each dispatch carries a fresh random token. The coordinator accepts only a result with the token it is waiting for and then checks it with `execution.Accept` (job, partition, operation, content hash, result hash). Late results of a timed-out attempt and duplicates after redelivery are ignored. Results are deterministic, so executing a task twice is harmless. There is no exactly-once claim.
- **Coordinator restart.** Partition state lives in `-heavy-dir`; a restarted coordinator republishes only unfinished partitions. Results published while no coordinator listened are lost and the partition is retried after its timeout.
- **No silent fallback.** `-runtime distributed` needs `-broker`, `-heavy-dir` and `-input-root`, and a binary built with an adapter; otherwise start-up fails (`no broker adapter for this URL in this build`). It never runs partitions locally instead.

## NATS JetStream adapter

Package `internal/execution/jetstream`, compiled only with `-tags jetstream`:

- tasks: stream `INSIGHT_WORK`, subject `insight.work`, work-queue retention, file storage; one durable pull consumer `insight-workers` shared by all workers, explicit ack, `AckWait` 5 minutes (override with `?ackWait=` on the URL), `MaxDeliver` 5;
- results: core NATS subject `insight.results.<jobId>` (not persisted, see restart above);
- credentials come from the URL or the operator's NATS setup; user info is redacted from error messages.

```sh
go build -tags jetstream -o bin/insight-lab-js ./cmd/insight-lab
docker run --rm -p 4222:4222 nats:2 -js                       # or any JetStream-enabled server
./bin/insight-lab-js worker -broker nats://127.0.0.1:4222 -input-root /shared/input -name w1 &
./bin/insight-lab-js worker -broker nats://127.0.0.1:4222 -input-root /shared/input -name w2 &
./bin/insight-lab-js serve -runtime distributed -broker nats://127.0.0.1:4222 \
  -input-root /shared/input -heavy-dir ./heavy
```

`GET /api/health` lists `DISTRIBUTED` in `capabilities.runtime.modes` and the compiled adapters in `brokers` only for such a build.

## Verification

| Scenario | Test |
|---|---|
| Two workers give the single-process aggregate | `TestRemoteWorkersProduceTheSingleProcessAggregate` |
| Worker crash before ack → redelivery | `TestRemoteTaskIsRedeliveredWhenAWorkerCrashesBeforeAcknowledging` |
| Crash after publishing, before ack → duplicate completion is harmless | `TestRemoteDuplicateCompletionAfterRedeliveryIsHarmless` |
| Late result of a timed-out attempt is ignored | `TestRemoteLateResultOfATimedOutAttemptIsIgnored` |
| Malformed WorkSpec is answered with an error, never executed or published by the coordinator | `TestRemoteWorkerAnswersAMalformedWorkSpecWithAnErrorAndNeverRunsIt` |
| Cancellation | `TestRemoteDispatchStopsWhenCancelled` |
| Coordinator restart republishes only unfinished partitions | `TestCoordinatorRestartResumesOnlyUnfinishedRemotePartitions` |
| Default build refuses DISTRIBUTED instead of running locally | `TestDistributedRuntimeNeedsABrokerAndABuildWithItsAdapter` |
| Real JetStream: two workers, one crashed worker, same aggregate | `TestJetStreamTwoWorkersProduceTheSingleProcessAggregateAndSurviveACrashedWorker` (set `INSIGHT_LAB_NATS_URL`, `-tags jetstream`) |

A manual end-to-end run (JetStream in Docker, two `worker -broker` processes, CLI HEAVY analysis with `-runtime distributed`) produced the same prepared artifact as `-runtime local`.

## Cost, limits and decision

Operating DISTRIBUTED means running and securing a JetStream cluster, sharing the input root read-only with every worker host, monitoring stuck or dead-lettered tasks, and keeping coordinator and worker binaries on compatible WorkSpec versions. Not yet provided: TLS/auth configuration beyond what a NATS URL carries, per-project quotas on the broker, dead-letter inspection, worker heartbeats (a task is redelivered only after `AckWait`), persisted results, and a shared artifact store other than a shared filesystem. Only one coordinator is supported (ADR 0001); multiple coordinators still need #137's external store.

**Decision:** keep DISTRIBUTED as an experimental, build-tagged PoC. Single-host LOCAL/PROCESS covers the workloads seen so far; roll DISTRIBUTED out for production only when a real workload shows single-host PROCESS is insufficient, and then address the gaps above first. JetStream stays a candidate adapter, not a permanent dependency; another broker can implement `execution.Broker` without touching Core.
