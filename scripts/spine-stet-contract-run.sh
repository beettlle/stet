#!/bin/bash
# Spine contract step: review the lane worker's diff with stet and fail only on actionable findings.
# Usage: scripts/spine-stet-contract-run.sh [lenient|default|strict]
# Run from the lane worktree root, after scripts/spine-worktree-setup.sh opened the session.
# SPINE_STET_NO_AUTO_FINISH=1 keeps the session open after a review with no threshold findings.
#
# Gate policy: the contract fails only when `stet list --min-severity --min-confidence` shows
# findings at or above the threshold. Defaults: severity warning, confidence 0.8; override with
# SPINE_STET_MIN_SEVERITY (one of error, warning, info, nitpick) and SPINE_STET_MIN_CONFIDENCE
# (0 to 1; validated by `stet list`, whose non-zero exit fails the script). Active findings below
# the threshold are reported to stderr but do not block the lane.
set -euo pipefail

strictness="${1:-default}"
case "$strictness" in
	lenient|default|strict) ;;
	*)
		echo "stet: invalid strictness '$strictness' (expected lenient, default, or strict)" >&2
		exit 2
		;;
esac

min_severity="${SPINE_STET_MIN_SEVERITY:-warning}"
min_confidence="${SPINE_STET_MIN_CONFIDENCE:-0.8}"
case "$min_severity" in
	error|warning|info|nitpick) ;;
	*)
		echo "stet: invalid SPINE_STET_MIN_SEVERITY '$min_severity' (expected error, warning, info, or nitpick)" >&2
		exit 2
		;;
esac

if [[ ! -f .review/session.json ]]; then
	if [[ ! -f .review/spine-stet-baseline.ref ]]; then
		echo "stet: no session and missing .review/spine-stet-baseline.ref (did worktreeSetupHook run?)" >&2
		exit 1
	fi
	baseline="$(tr -d '[:space:]' < .review/spine-stet-baseline.ref)"
	stet start "$baseline" --allow-dirty --strictness "$strictness" --quiet
fi

# No --auto-finish-zero: a non-zero finding count must leave the session open for triage.
set +e
stet run --strictness "$strictness" --quiet
run_exit=$?
set -e
if [[ "$run_exit" -ne 0 ]]; then
	echo "stet: run failed (exit $run_exit); check 'stet doctor' and that LM Studio / OpenJev are up" >&2
	exit "$run_exit"
fi

# stet list prints one line per active (non-dismissed) finding; stet status counts dismissed ones too.
# The gating list hides findings below the threshold; the full list is for triage output.
if ! gating_list="$(stet list --min-severity "$min_severity" --min-confidence "$min_confidence")"; then
	echo "stet: 'stet list' failed; cannot verify active findings" >&2
	exit 1
fi
if ! active_list="$(stet list)"; then
	echo "stet: 'stet list' failed; cannot verify active findings" >&2
	exit 1
fi
gating_count="$(printf '%s' "$gating_list" | grep -c . || true)"
active_count="$(printf '%s' "$active_list" | grep -c . || true)"

if [[ "$gating_count" -gt 0 ]]; then
	{
		echo "stet: contract review found ${gating_count} finding(s) at or above the ${min_severity}/${min_confidence} threshold."
		echo "Full list of active findings (all severities, for triage):"
		printf '%s\n' "$active_list"
	} >&2
	cat >&2 <<EOF

Triage, then re-run the contract:

  stet list
  stet dismiss <id> <reason>

Dismiss reasons: false_positive, already_correct, wrong_suggestion, out_of_scope.
Fix real defects in this task's diff. If a finding exposes a stet reviewer bug (bad prompt,
parser, critic), dismiss it with a reason and record it under Discoveries in STATUS.md.
Never dismiss a real defect without fixing it.
EOF
	exit 1
fi

if [[ "$active_count" -gt 0 ]]; then
	echo "stet: ${active_count} finding(s) below the ${min_severity}/${min_confidence} threshold did not block this lane" >&2
fi

if [[ "${SPINE_STET_NO_AUTO_FINISH:-}" == "1" ]]; then
	exit 0
fi

if ! stet finish >/dev/null 2>&1; then
	echo "stet: finish failed after zero-finding review" >&2
	exit 1
fi
exit 0
