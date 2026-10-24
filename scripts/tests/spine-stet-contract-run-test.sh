#!/bin/bash
# Tests for scripts/spine-stet-contract-run.sh (SP-073): threshold gating.
# Creates a temp directory with .review/session.json and puts a fake `stet` first on PATH;
# `list` output depends on whether --min-severity was passed. Needs no network or model.
# Exits non-zero if any case fails.
set -u

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
contract="$here/../spine-stet-contract-run.sh"
work_root="$(mktemp -d "${TMPDIR:-/tmp}/spine-stet-contract-test.XXXXXX")"
trap 'rm -rf "$work_root"' EXIT

pass=0
fail=0
ok() { pass=$((pass + 1)); echo "ok: $*"; }
bad() {
	fail=$((fail + 1))
	echo "FAIL: $*" >&2
}

[[ -f "$contract" ]] || { bad "missing contract script: $contract"; exit 1; }
bash -n "$contract" && ok "contract script parses (bash -n)" || bad "contract script has a syntax error"

# Finding lines mimic the real `stet list` format: "id  file:line  SEVERITY  message".
INFO_LINE='9f2e1ab  scripts/spine-stet-contract-run.sh:44  INFO  Naming could be clearer (confidence 0.30)'
WARN_LINE='c4d8e2f  scripts/spine-stet-contract-run.sh:52  WARNING  Possible unquoted expansion (confidence 0.90)'
NIT_LINE='b7a1d0c  scripts/spine-stet-contract-run.sh:60  NITPICK  Style nit (confidence 0.40)'

case_dir=""
# new_case NAME: fresh temp dir with .review/session.json and a fake stet on PATH.
new_case() {
	case_dir="$work_root/$1"
	mkdir -p "$case_dir/.review" "$case_dir/bin"
	printf '{"baseline_ref":"fake-baseline"}\n' > "$case_dir/.review/session.json"
	: > "$case_dir/full.txt"
	: > "$case_dir/gating.txt"
	cat > "$case_dir/bin/stet" <<'STUB'
#!/bin/bash
# Fake stet: appends each invocation's arguments to $FAKE_STET_LOG; 'list' output
# comes from $FAKE_STET_GATING when --min-severity is passed, else $FAKE_STET_FULL.
printf '%s\n' "$*" >> "${FAKE_STET_LOG:?}"
cmd="${1:-}"
if [ "$#" -gt 0 ]; then shift; fi
if [ "$cmd" = "list" ]; then
	if [[ " $* " == *" --min-severity "* ]]; then
		cat "${FAKE_STET_GATING:?}"
	else
		cat "${FAKE_STET_FULL:?}"
	fi
	exit "${FAKE_STET_LIST_EXIT:-0}"
fi
exit 0
STUB
	chmod +x "$case_dir/bin/stet"
}

run_status=""
run_out=""
# run_contract [ENV=VALUE...]: runs the contract script inside $case_dir with the fake
# stet first on PATH; captures combined output in run_out and exit code in run_status.
# Hermetic: unsets the SPINE_STET_* vars first (the spine worker exports
# SPINE_STET_NO_AUTO_FINISH=1); case-specific ENV=VALUE assignments then apply.
run_contract() {
	run_out="$(cd "$case_dir" && PATH="$case_dir/bin:$PATH" \
		FAKE_STET_LOG="$case_dir/args.log" \
		FAKE_STET_FULL="$case_dir/full.txt" \
		FAKE_STET_GATING="$case_dir/gating.txt" \
		env -u SPINE_STET_NO_AUTO_FINISH -u SPINE_STET_MIN_SEVERITY -u SPINE_STET_MIN_CONFIDENCE \
			"$@" bash "$contract" default 2>&1)"
	run_status=$?
}

expect_status() {
	if [ "$run_status" -eq "$2" ]; then
		ok "$1: exit $run_status"
	else
		bad "$1: exit $run_status, want $2; output: $run_out"
	fi
}

expect_out() {
	if printf '%s' "$run_out" | grep -qF -- "$2"; then
		ok "$1: output contains: $2"
	else
		bad "$1: output missing: $2; output: $run_out"
	fi
}

expect_not_out() {
	if printf '%s' "$run_out" | grep -qF -- "$2"; then
		bad "$1: output should not contain: $2; output: $run_out"
	else
		ok "$1: output does not contain: $2"
	fi
}

args_has() {
	if [ -s "$case_dir/args.log" ] && grep -qF -- "$2" "$case_dir/args.log"; then
		ok "$1: stet was invoked with: $2"
	else
		bad "$1: stet was not invoked with: $2; args log: $(cat "$case_dir/args.log" 2>/dev/null || echo '<absent>')"
	fi
}

args_lacks() {
	if [ -s "$case_dir/args.log" ] && grep -qF -- "$2" "$case_dir/args.log"; then
		bad "$1: stet should not have been invoked with: $2; args log: $(cat "$case_dir/args.log")"
	else
		ok "$1: stet was not invoked with: $2"
	fi
}

# (a) Only a below-threshold info finding: exit 0, below-threshold note, finish still runs.
new_case case-a
printf '%s\n' "$INFO_LINE" > "$case_dir/full.txt"
run_contract
expect_status "case-a" 0
expect_out "case-a" "1 finding(s) below the warning/0.8 threshold did not block this lane"
expect_not_out "case-a" "$INFO_LINE"
args_has "case-a" "list --min-severity warning --min-confidence 0.8"
args_has "case-a" "list"
args_has "case-a" "run --strictness default --quiet"
args_has "case-a" "finish"

# (b) Warning at 0.9: exit 1, threshold count, full list (incl. below-threshold line), triage text.
new_case case-b
printf '%s\n%s\n' "$WARN_LINE" "$INFO_LINE" > "$case_dir/full.txt"
printf '%s\n' "$WARN_LINE" > "$case_dir/gating.txt"
run_contract
expect_status "case-b" 1
expect_out "case-b" "found 1 finding(s) at or above the warning/0.8 threshold"
expect_out "case-b" "$WARN_LINE"
expect_out "case-b" "$INFO_LINE"
expect_out "case-b" "stet dismiss <id> <reason>"
args_lacks "case-b" "finish"

# (c) stet list failure: exit 1 before gating decisions.
new_case case-c
printf '%s\n' "$WARN_LINE" > "$case_dir/full.txt"
run_contract FAKE_STET_LIST_EXIT=1
expect_status "case-c" 1
expect_out "case-c" "'stet list' failed"
args_lacks "case-c" "finish"

# (d) Invalid SPINE_STET_MIN_SEVERITY: exit 2 before any stet invocation.
new_case case-d
printf '%s\n' "$INFO_LINE" > "$case_dir/full.txt"
run_contract SPINE_STET_MIN_SEVERITY=bogus
expect_status "case-d" 2
args_lacks "case-d" "list"
args_lacks "case-d" "run"

# (e) Valid env overrides are honored: info/0.5 threshold blocks an info finding.
new_case case-e
printf '%s\n%s\n' "$INFO_LINE" "$NIT_LINE" > "$case_dir/full.txt"
printf '%s\n' "$INFO_LINE" > "$case_dir/gating.txt"
run_contract SPINE_STET_MIN_SEVERITY=info SPINE_STET_MIN_CONFIDENCE=0.5
expect_status "case-e" 1
expect_out "case-e" "found 1 finding(s) at or above the info/0.5 threshold"
args_has "case-e" "list --min-severity info --min-confidence 0.5"

echo
echo "$pass passed, $fail failed"
[ "$fail" -eq 0 ]
