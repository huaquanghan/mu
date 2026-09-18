#!/usr/bin/env bash
# capture-golden.sh — capture the Go `mu` binary's machine-readable outputs
# into tests/golden/ as parity fixtures for the Rust port.
#
# HISTORICAL: the Go tree was removed at cutover, so this script cannot run
# in this checkout (go build fails). Kept for provenance — to recapture,
# run it from a pre-cutover checkout (e.g. `git worktree add /tmp/mu-oracle
# v0.2.0`). The goldens it produced are the frozen parity contract.
#
# Only read-only / non-destructive commands are captured:
#   --help, --version, clean --dry-run, optimize --dry-run,
#   audit --report, audit --json, status (piped → JSON).
# audit legitimately exits 0/1/2, so this script does NOT use `set -e`.
set -u -o pipefail

# Resolve repo root from this script's location so it works from any cwd.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

BIN="bin/mu"
GOLDEN_DIR="tests/golden"
# Generous per-command ceiling so a stuck scan cannot hang the capture.
TIMEOUT="${MU_GOLDEN_TIMEOUT:-300}"

echo "==> Building Go binary (migration oracle)"
mkdir -p bin
if ! go build -o "$BIN" ./cmd/mu; then
	echo "error: go build failed — cannot capture golden outputs" >&2
	exit 1
fi

mkdir -p "$GOLDEN_DIR"

# capture <name> <cmd...>
#   stdout → tests/golden/<name>.txt
#   stderr → tests/golden/<name>.stderr.txt
#   exit   → tests/golden/<name>.exitcode
# stdin is /dev/null so no command can block on a prompt.
capture() {
	local name="$1"
	shift
	local out="$GOLDEN_DIR/${name}.txt"
	local err="$GOLDEN_DIR/${name}.stderr.txt"
	local code_file="$GOLDEN_DIR/${name}.exitcode"

	echo "==> Capturing: $*"
	timeout "$TIMEOUT" "$@" >"$out" 2>"$err" </dev/null
	local code=$?
	printf '%d\n' "$code" >"$code_file"
	echo "    -> ${name}.txt (exit $code)"
}

# — Help output (safe for every command, including the interactive TUIs) —
capture help "$BIN" --help
for cmd in audit clean uninstall optimize status; do
	capture "help-$cmd" "$BIN" "$cmd" --help
done

# — Read-only command output —
capture clean-dry-run    "$BIN" clean --dry-run
capture optimize-dry-run "$BIN" optimize --dry-run
capture audit-report     "$BIN" audit --report
capture audit-json       "$BIN" audit --json
# status emits JSON automatically when stdout is not a TTY (redirected here).
capture status-json      "$BIN" status

# — Version (only if the flag exists) —
capture version "$BIN" --version
if [ "$(cat "$GOLDEN_DIR/version.exitcode")" != "0" ]; then
	echo "    note: --version not supported; removing version.* artifacts"
	rm -f "$GOLDEN_DIR/version.txt" "$GOLDEN_DIR/version.stderr.txt" "$GOLDEN_DIR/version.exitcode"
fi

# — Summary —
echo
echo "==> Golden files captured in $GOLDEN_DIR/:"
for f in "$GOLDEN_DIR"/*; do
	[ -f "$f" ] || continue
	printf '    %10s  %s\n' "$(wc -c <"$f")" "$f"
done

echo
echo "==> Exit codes:"
for f in "$GOLDEN_DIR"/*.exitcode; do
	[ -f "$f" ] || continue
	printf '    %-28s %s\n' "$(basename "$f" .exitcode)" "$(cat "$f")"
done

echo
echo "Done."
exit 0
