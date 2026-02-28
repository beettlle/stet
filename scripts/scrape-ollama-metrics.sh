#!/usr/bin/env bash
# scrape-ollama-metrics.sh: scrape Ollama Prometheus /metrics in a loop for experiment H2.
# When Ollama does not expose /metrics (e.g. versions before the metrics endpoint was added),
# the script uses process-memory fallback and records RSS (and optionally VSZ) so experiments
# can still observe memory growth over time. The output file may be in Prometheus format or
# in CSV-style "process_memory" format depending on the response from /metrics.
#
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

# Probe once to choose mode: Prometheus /metrics or process-memory fallback.
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "${OLLAMA_URL}/metrics" 2>/dev/null || echo "000")
if [ "$HTTP_CODE" = "200" ]; then
  MODE="prometheus"
else
  MODE="process"
  echo "Ollama /metrics not available (HTTP ${HTTP_CODE}), using process memory fallback." >&2
fi

START=$(date +%s)
PROCESS_HEADER_DONE=0

while true; do
  TS=$(date -u +%Y-%m-%dT%H:%M:%SZ)

  if [ "$MODE" = "prometheus" ]; then
    echo "# timestamp $TS" >> "$OUTPUT"
    curl -s "${OLLAMA_URL}/metrics" >> "$OUTPUT" 2>/dev/null || true
    echo "" >> "$OUTPUT"
  else
    # Process mode: record RSS and VSZ of the first Ollama process (portable: pgrep + ps).
    RSS_BYTES=0
    VSZ_BYTES=0
    PID=$(pgrep -f ollama 2>/dev/null | head -1)
    if [ -n "$PID" ]; then
      # ps -o rss=,vsz= -p PID: RSS and VSZ in KB on macOS and Linux.
      STATS=$(ps -o rss=,vsz= -p "$PID" 2>/dev/null | head -1)
      if [ -n "$STATS" ]; then
        RSS_KB=$(echo "$STATS" | awk '{print $1}')
        VSZ_KB=$(echo "$STATS" | awk '{print $2}')
        [ -n "$RSS_KB" ] && [ "$RSS_KB" -ge 0 ] 2>/dev/null && RSS_BYTES=$((RSS_KB * 1024))
        [ -n "$VSZ_KB" ] && [ "$VSZ_KB" -ge 0 ] 2>/dev/null && VSZ_BYTES=$((VSZ_KB * 1024))
      fi
    fi
    if [ "$PROCESS_HEADER_DONE" -eq 0 ]; then
      echo "# mode=process_memory" >> "$OUTPUT"
      echo "# timestamp_iso,rss_bytes,vsz_bytes" >> "$OUTPUT"
      PROCESS_HEADER_DONE=1
    fi
    echo "${TS},${RSS_BYTES},${VSZ_BYTES}" >> "$OUTPUT"
  fi

  if [ -n "$DURATION" ]; then
    NOW=$(date +%s)
    ELAPSED=$((NOW - START))
    if [ "$ELAPSED" -ge "$DURATION" ]; then
      break
    fi
  fi

  sleep "$INTERVAL"
done
