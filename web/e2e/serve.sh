#!/bin/sh
# Starts a real Insight Lab binary with an empty, throw-away database for the
# browser E2E suite. The binary is built from this checkout, so it embeds the
# current internal/web/dist (run `pnpm build` first; `make web-e2e` does).
#   serve.sh delivery <port>
#   serve.sh demo <port> <scripted-llm-port>
# The demo server is model-backed through cmd/insight-scripted-llm, a
# deterministic local test model: no real or paid LLM is ever called.
set -eu
mode=$1
port=$2
root=$(cd "$(dirname "$0")/../.." && pwd)
out="$root/bin/e2e"
data=$(mktemp -d)
mkdir -p "$out"
cd "$root"
case "$mode" in
  delivery)
    go build -o "$out/insight-lab" ./cmd/insight-lab
    exec "$out/insight-lab" serve -no-browser -port "$port" -db "$data/insight.db"
    ;;
  demo)
    llm=$3
    go build -tags demo -o "$out/insight-lab-demo" ./cmd/insight-lab
    go build -o "$out/insight-scripted-llm" ./cmd/insight-scripted-llm
    "$out/insight-scripted-llm" -addr "127.0.0.1:$llm" &
    llm_pid=$!
    trap 'kill "$llm_pid" 2>/dev/null || true' EXIT INT TERM
    "$out/insight-lab-demo" serve -no-browser -port "$port" -db "$data/insight.db" \
      -base-url "http://127.0.0.1:$llm" -model scripted-model -api-key e2e-test-key
    ;;
  *)
    echo "usage: serve.sh delivery|demo <port> [llm-port]" >&2
    exit 2
    ;;
esac
