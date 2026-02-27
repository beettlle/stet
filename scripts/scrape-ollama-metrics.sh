#!/usr/bin/env bash
# scrape-ollama-metrics.sh: scrape Ollama Prometheus /metrics in a loop for experiment H2.
# Run in background while "stet start" runs; stop when the review finishes or after -d seconds.
#
# Usage:
#   scrape-ollama-metrics.sh [ -o FILE ] [ -i SECONDS ] [ -d SECONDS ]
#
# Options:
#   -o FILE     Output file (default: ollama_metrics.txt)
#   -i SECONDS  Scrape interval in seconds (default: 60)
#   -d SECONDS  Optional duration limit; exit after this many seconds
#
# Example:
#   ./scripts/scrape-ollama-metrics.sh -o ollama_metrics.txt -i 60 &
#   stet start HEAD --context 256k --search-replace --trace 2>&1 | tee run_h2_trace.log
#   kill %1
#
set -e

OLLAMA_URL="${OLLAMA_URL:-http://localhost:11434}"
OUTPUT="ollama_metrics.txt"
INTERVAL=60
DURATION=""

while getopts "o:i:d:h" opt; do
  case "$opt" in
    o) OUTPUT="$OPTARG" ;;
    i) INTERVAL="$OPTARG" ;;
    d) DURATION="$OPTARG" ;;
    h)
      echo "Usage: $0 [ -o FILE ] [ -i SECONDS ] [ -d SECONDS ]"
      echo "  -o FILE     Output file (default: ollama_metrics.txt)"
      echo "  -i SECONDS Scrape interval (default: 60)"
      echo "  -d SECONDS Duration limit; exit after this many seconds"
      exit 0
      ;;
    *) exit 1 ;;
  esac
done

START=$(date +%s)
while true; do
  TS=$(date -u +%Y-%m-%dT%H:%M:%SZ)
  echo "# timestamp $TS" >> "$OUTPUT"
  curl -s "${OLLAMA_URL}/metrics" >> "$OUTPUT" 2>/dev/null || true
  echo "" >> "$OUTPUT"

  if [ -n "$DURATION" ]; then
    NOW=$(date +%s)
    ELAPSED=$((NOW - START))
    if [ "$ELAPSED" -ge "$DURATION" ]; then
      break
    fi
  fi

  sleep "$INTERVAL"
done
