#!/usr/bin/env bash
# check-coverage.sh: fail if project coverage < 77% or any file < 72%.
# Usage: check-coverage.sh coverage.out
set -e
if [ ! -f "$1" ]; then
  echo "Usage: $0 coverage.out"
  exit 1
fi
echo "Checking coverage: project >= 77%, every file >= 72%"
awk '
BEGIN { total_stmt=0; total_cov=0 }
NR==1 && /^mode:/ { next }
NF>=3 && $1 ~ /:/ && $2 ~ /^[0-9]+$/ {
  split($1, a, ":"); f=a[1]; stmt=$2+0; count=$3+0;
  if (stmt>0) {
    file_stmt[f]+=stmt; total_stmt+=stmt;
    if (count>0) { file_cov[f]+=stmt; total_cov+=stmt }
  }
}
END {
  pct = (total_stmt>0) ? (100*total_cov/total_stmt) : 0;
  if (pct < 77) { printf "FAIL: project coverage %.1f%% < 77%%\n", pct; exit 1 }
  for (f in file_stmt) {
    fpct = (file_stmt[f]>0) ? (100*file_cov[f]/file_stmt[f]) : 0;
    if (fpct < 72) { printf "FAIL: %s coverage %.1f%% < 72%%\n", f, fpct; exit 2 }
  }
  printf "PASS: project %.1f%%, all files >= 72%%\n", pct
}' "$1"
