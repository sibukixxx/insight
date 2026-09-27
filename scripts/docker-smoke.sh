#!/bin/sh
# Container smoke test for the default deployment (#138). Needs only a
# Docker daemon: no database, broker, model or network access beyond the
# image build.
#   1. build the image and start it from compose.yaml (Insight only)
#   2. wait until the container reports healthy
#   3. stop it, record a subject with the headless CLI on the same volume
#   4. recreate the container and check the subject survived
#   5. check that compose started no service besides Insight
set -eu
root=$(cd "$(dirname "$0")/.." && pwd)
project="insight-smoke-$$"
compose() { docker compose -p "$project" -f "$root/compose.yaml" "$@"; }
cleanup() { compose down -v --remove-orphans >/dev/null 2>&1 || true; }
trap cleanup EXIT INT TERM

wait_healthy() {
  cid=$(compose ps -q insight)
  i=0
  while [ "$(docker inspect -f '{{.State.Health.Status}}' "$cid")" != healthy ]; do
    i=$((i + 1))
    if [ "$i" -gt 60 ]; then
      echo "container did not become healthy" >&2
      docker logs "$cid" >&2
      exit 1
    fi
    sleep 1
  done
}

compose build -q
compose up -d
wait_healthy
services=$(compose ps --services | tr '\n' ' ')
[ "$services" = "insight " ] || { echo "unexpected services: $services" >&2; exit 1; }

compose stop -t 30
subject=$(compose run --rm -T insight subject create -db /data/insight.db -namespace smoke -id s1 |
  sed -n 's/.*"subjectId": *"\([^"]*\)".*/\1/p')
[ -n "$subject" ] || { echo "subject was not created" >&2; exit 1; }

compose rm -f -s >/dev/null
compose up -d
wait_healthy
compose stop -t 30
compose run --rm -T insight status -db /data/insight.db -subject "$subject" >/dev/null
echo "docker smoke: ok (subject $subject survived container recreation)"
