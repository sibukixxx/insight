# Deployment

Issue #138, following [ADR 0001](adr/0001-standalone-first-portable-deployment.md). Insight runs as one process with SQLite and a data directory. Docker is one way to package it, never a requirement: the local binary (`make build`, then `./bin/insight-lab`) remains fully supported.

## Default container

```sh
docker compose up -d        # builds the image and starts Insight only
open http://127.0.0.1:8787
docker compose logs -f insight
docker compose down         # stops; the data volume stays
```

`compose.yaml` starts **one** service. It creates no database, broker or other container, and has no `depends_on`. All state lives in the named volume `insight-data` mounted at `/data`:

| Path in the volume | Content |
|---|---|
| `/data/insight.db` (+ `-wal`, `-shm`, `.lock`) | Research and coordinator state |
| `/data/ingest/` | Staged large-CSV uploads and their error exports |
| `/data/heavy/` | HEAVY partition state, only with the PROCESS override |

> **`docker compose down -v` deletes the volume and all research data.** Back it up first.

The image (`Dockerfile`) is a static Go binary on Alpine, running as the unprivileged user `insight` (uid 65532). It embeds the committed Reference Web, so building it needs neither Node nor network access beyond the Go module download. The health check runs `insight-lab health`, which probes `/api/health`.

### Security

Insight has **no built-in authentication**; the browser API only rejects non-loopback `Origin` headers. The container listens on all of its interfaces, and `compose.yaml` publishes port 8787 on the host's loopback address only. Expose it further only behind an authenticating reverse proxy that terminates TLS.

## Configuration

Precedence is: command-line flag, then environment variable, then built-in default. There is no configuration file.

| Setting | Flag | Environment | Default |
|---|---|---|---|
| Database | `-db` | — | OS data directory (`/data/insight.db` in the image) |
| Listen address / port | `-host`, `-port` | — | `127.0.0.1:8787` (`0.0.0.0:8787` in the image) |
| Model API key | `-api-key` | `INSIGHT_LAB_API_KEY` | empty: deterministic analysis only |
| Model / base URL | `-model`, `-base-url` | `INSIGHT_LAB_MODEL`, `INSIGHT_LAB_BASE_URL` | empty |
| Raw input root | `-input-root` | `INSIGHT_LAB_INPUT_ROOT` | disabled |
| HEAVY state | `-heavy-dir` | `INSIGHT_LAB_HEAVY_DIR` | disabled |
| HEAVY placement | `-runtime local\|process` | `INSIGHT_LAB_RUNTIME` | `local` |
| Ingest staging | `-ingest-dir`, `-ingest-max-bytes`, `-ingest-max-staged-bytes`, `-ingest-max-rows` | `INSIGHT_LAB_INGEST_DIR` | `ingest/` beside the database; 2 GiB; 8 GiB; 10M rows |
| Extra models | `-allowed-models` | `INSIGHT_LAB_ALLOWED_MODELS` | none |

Copy `deploy/insight.env.example` to `.env` next to `compose.yaml` to set the model connection; the example contains no working secret. Invalid combinations fail at start-up with the reason (for example `-runtime process` without `-heavy-dir` and `-input-root`, an unknown `-runtime`, or a second process on the same database: `ENGINE_IN_USE`). Nothing falls back silently.

## Optional setups

These are examples, never started by default.

| File | Adds | Needs |
|---|---|---|
| `deploy/compose.process.yaml` | HEAVY preparation with PROCESS workers (#134) over raw files in `./input` (read-only) | Nothing beyond the default |

```sh
docker compose -f compose.yaml -f deploy/compose.process.yaml up -d
```

External state stores (#137) and remote workers with a broker and shared artifact store (#135) are not implemented; no example is shipped for them until they are.

## Operations

- **Lifecycle.** `docker compose stop` sends SIGTERM; Insight stops accepting requests, finishes shutdown within the 30-second grace period and exits 0. A run that was still `running` is marked INTERRUPTED on the next start and can be retried; queued runs resume when their configuration still matches (see [architecture](architecture.md#analysis-lifecycle-133)).
- **Headless commands on the volume.** Stop the server first (one process owns one database), then for example `docker compose run --rm insight status -db /data/insight.db -subject S`.
- **Backup.** Stop the service, then archive the volume, e.g. `docker run --rm -v <project>_insight-data:/data -v "$PWD":/backup alpine tar czf /backup/insight-data.tgz -C /data .`, and start the service again.
- **Upgrade.** Back up, rebuild or pull the new image, `docker compose up -d`. Migrations are forward-only and run at start-up.
- **Rollback.** An older image cannot open a database a newer one migrated. Roll back by restoring the backup taken before the upgrade together with the older image.
- **Smoke test.** `make docker-smoke` builds the image, starts the default Compose project, checks that it becomes healthy and runs no other service, and checks that data survives recreating the container.

## Where it can run

See the [support matrix](adr/0001-standalone-first-portable-deployment.md#support-matrix). In short: any host or VM, on-premises or in a cloud (AWS, Google Cloud, Azure, others), that runs the binary or this container with a **persistent** volume and one long-running process. The operator provides the host, the volume and its backups, TLS and authentication in front, and optionally a model endpoint. Only the local binary and this container are verified here; provider-specific services are not. Serverless sandboxes without a persistent volume, including Cloudflare Workers, cannot host the coordinator.
