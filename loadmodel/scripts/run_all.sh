#!/usr/bin/env bash
# Run all k6 scenarios in sequence and collect their summaries.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
for s in smoke cached_1k voice_20 writes_50 cold_cache source_outage burst; do
  echo "==================================================="
  echo "running scenario: $s"
  echo "==================================================="
  bash scripts/run_scenario.sh "$s" || true
done
echo "all scenarios complete."
ls reports/
