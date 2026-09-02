#!/usr/bin/env bash
# Run the bundled demo dataset through a real LLM and record the result
# under docs/evaluation/. This is the "golden" check for prompt or model
# changes: it produces the evaluation metrics (Trace-backed / Quality
# Flagged rates included), every insight with its abduction chain and
# quality flags, and every trace/pattern, as JSON plus a Markdown summary.
#
# Required:
#   INSIGHT_LAB_API_KEY    API key for the OpenAI-compatible endpoint
#   INSIGHT_LAB_MODEL      model name (e.g. gpt-5, claude-sonnet-5)
# Optional:
#   INSIGHT_LAB_BASE_URL   default https://api.openai.com/v1
#   EVAL_OUT               output dir (default docs/evaluation/<date>-<model>)
#   EVAL_PORT              default 8797
#   EVAL_TIMEOUT           seconds to wait for the analysis (default 1800)
#
# The API key is passed to the binary on its command line only and is
# never written to the output directory.
set -euo pipefail

: "${INSIGHT_LAB_API_KEY:?set INSIGHT_LAB_API_KEY}"
: "${INSIGHT_LAB_MODEL:?set INSIGHT_LAB_MODEL}"
BASE_URL="${INSIGHT_LAB_BASE_URL:-https://api.openai.com/v1}"
PORT="${EVAL_PORT:-8797}"
TIMEOUT="${EVAL_TIMEOUT:-1800}"
MODEL_SLUG="$(printf '%s' "$INSIGHT_LAB_MODEL" | tr '/:' '--')"
OUT="${EVAL_OUT:-docs/evaluation/$(date +%Y%m%d)-${MODEL_SLUG}}"
API="http://127.0.0.1:${PORT}/api"
PROJECT="demo-invoicing-saas"

cd "$(dirname "$0")/.."
make build-demo >/dev/null

tmp="$(mktemp -d)"
./bin/insight-lab-demo --demo --no-browser --port "$PORT" --db "$tmp/eval.db" \
  --base-url "$BASE_URL" --model "$INSIGHT_LAB_MODEL" --api-key "$INSIGHT_LAB_API_KEY" \
  >"$tmp/app.log" 2>&1 &
pid=$!
trap 'kill "$pid" 2>/dev/null || true' EXIT

for _ in $(seq 1 30); do
  curl -sf "$API/health" >/dev/null 2>&1 && break
  sleep 0.5
done
curl -sf "$API/health" >/dev/null || { echo "server did not start:"; cat "$tmp/app.log"; exit 1; }

echo "connection test ..."
curl -sf -X POST "$API/settings/test" || { echo; echo "LLM connection test failed"; exit 1; }
echo

echo "running analysis with $INSIGHT_LAB_MODEL ..."
curl -sf -X POST "$API/projects/$PROJECT/analysis" >/dev/null
start=$(date +%s)
while :; do
  status="$(curl -sf "$API/projects/$PROJECT/analyses" | python3 -c 'import sys,json;a=json.load(sys.stdin)[0];print(a["status"],a.get("currentStep",""),a.get("progress",0),a.get("error",""))')"
  printf '\r  %s        ' "$status"
  case "$status" in
    completed*) echo; break ;;
    failed*) echo; echo "analysis failed: $status"; exit 1 ;;
  esac
  if (( $(date +%s) - start > TIMEOUT )); then echo; echo "timed out"; exit 1; fi
  sleep 2
done

mkdir -p "$OUT"
curl -sf "$API/projects/$PROJECT/evaluation" | python3 -m json.tool >"$OUT/metrics.json"
curl -sf "$API/projects/$PROJECT/patterns" | python3 -m json.tool >"$OUT/patterns.json"
curl -sf "$API/projects/$PROJECT/insights" \
  | python3 -c 'import sys,json;[print(i["id"]) for i in json.load(sys.stdin)]' \
  | while read -r id; do curl -sf "$API/insights/$id"; echo; done \
  | python3 -c 'import sys,json;print(json.dumps([json.loads(l) for l in sys.stdin if l.strip()],ensure_ascii=False,indent=2))' >"$OUT/insights.json"

python3 scripts/eval-summary.py "$OUT" "$INSIGHT_LAB_MODEL" "$BASE_URL" >"$OUT/summary.md"
echo "wrote $OUT/{metrics.json,patterns.json,insights.json,summary.md}"
