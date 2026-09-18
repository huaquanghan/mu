#!/usr/bin/env bash
# parity-diff.sh — run every captured golden command against the Go oracle
# (bin/mu, built via `make build` so ldflags match the shipped artifact) and
# the Rust port (target/release/mu), diffing stdout, stderr, and exit code.
#
# JSON commands are compared via `jq -S` with all numbers normalized to 0 and
# digits inside strings replaced — live metrics (cpu%, bytes, health, rates)
# are volatile; structure, key names, finding order, and string content are
# the parity contract. Text commands are compared after symmetric sed
# normalization of sizes/percentages/scores/numbers. Help and version output
# is byte-exact. Exits nonzero if any command diverges.
set -u -o pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

GO_BIN="bin/mu"
RS_BIN="target/x86_64-unknown-linux-musl/release/mu"
TIMEOUT="${MU_PARITY_TIMEOUT:-300}"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

# The Go tree was removed at cutover — running this in-tree would build the
# Rust binary twice and compare it against itself. The script is preserved
# for provenance and re-verification; to run it again, overlay the Go tree
# from the pre-removal commit into a scratch copy of this repo:
#   cp -a . /tmp/mu-parity && cd /tmp/mu-parity &&
#   git archive HEAD cmd internal go.mod go.sum | tar -x
# then run scripts/parity-diff.sh there (its Makefile must build the Go
# oracle — restore the pre-cutover Makefile too, or point GO_BIN at a Go
# build you produce yourself).
if [ ! -f go.mod ] || [ ! -d cmd ]; then
    echo "error: Go oracle sources are gone (no go.mod/cmd/)." >&2
    echo "       Restore them into a scratch copy to re-verify — see the" >&2
    echo "       header comment in this script." >&2
    exit 2
fi

echo "==> Building Go oracle (make build — production ldflags)"
make build >/dev/null || { echo "error: make build failed" >&2; exit 2; }
echo "==> Building Rust port (cargo build --release --target x86_64-unknown-linux-musl)"
cargo build --release --target x86_64-unknown-linux-musl >/dev/null || { echo "error: cargo build --release failed" >&2; exit 2; }
[ -x "$RS_BIN" ] || { echo "error: missing $RS_BIN" >&2; exit 2; }

# Same command list as scripts/capture-golden.sh.
NAMES=(help help-audit help-clean help-uninstall help-optimize help-status \
       clean-dry-run optimize-dry-run audit-report audit-json status-json version)
ARGS=("--help" "audit --help" "clean --help" "uninstall --help" "optimize --help" \
      "status --help" "clean --dry-run" "optimize --dry-run" "audit --report" \
      "audit --json" "status" "--version")
EXACT=(help help-audit help-clean help-uninstall help-optimize help-status version)
JSON=(audit-json status-json)

in_list() { local x; for x in "${@:2}"; do [ "$1" = "$x" ] && return 0; done; return 1; }

# Volatile-field normalization for text output.
norm_text() {
	sed -E \
		-e 's/[0-9]+(\.[0-9]+)?[[:space:]]*(B|KB|MB|GB|TB|KiB|MiB|GiB|TiB)/SIZE/g' \
		-e 's/[0-9]+(\.[0-9]+)?%/PCT/g' \
		-e 's/[0-9]+\/100/SCORE/g' \
		-e 's/[0-9]+(\.[0-9]+)?/N/g' "$1"
}

# JSON: numbers → 0, digits inside strings → N (health "(57/100)" lives in a
# detail string, so walk() on numbers alone is not enough).
norm_json() {
	jq -S 'walk(if type == "number" then 0
	           elif type == "string" then gsub("[0-9]+(\\.[0-9]+)?"; "N")
	           else . end)' "$1"
}

fails=0
for i in "${!NAMES[@]}"; do
	name="${NAMES[$i]}"
	# shellcheck disable=SC2086
	args=${ARGS[$i]}

	timeout "$TIMEOUT" "$GO_BIN" $args >"$WORK/$name.go.out" 2>"$WORK/$name.go.err" </dev/null
	go_code=$?
	timeout "$TIMEOUT" "$RS_BIN" $args >"$WORK/$name.rs.out" 2>"$WORK/$name.rs.err" </dev/null
	rs_code=$?

	status="PASS"
	[ "$go_code" = "$rs_code" ] || status="FAIL exit go=$go_code rust=$rs_code"

	if [ "$status" = "PASS" ]; then
		if in_list "$name" "${JSON[@]}"; then
			norm_json "$WORK/$name.go.out" >"$WORK/$name.go.n" || { echo "error: jq failed on $name go stdout" >&2; exit 2; }
			norm_json "$WORK/$name.rs.out" >"$WORK/$name.rs.n" || { echo "error: jq failed on $name rust stdout" >&2; exit 2; }
		elif in_list "$name" "${EXACT[@]}"; then
			cp "$WORK/$name.go.out" "$WORK/$name.go.n"
			cp "$WORK/$name.rs.out" "$WORK/$name.rs.n"
		else
			norm_text "$WORK/$name.go.out" >"$WORK/$name.go.n"
			norm_text "$WORK/$name.rs.out" >"$WORK/$name.rs.n"
		fi
		if ! diff -u "$WORK/$name.go.n" "$WORK/$name.rs.n" >"$WORK/$name.diff"; then
			status="FAIL stdout"
		elif ! diff -u "$WORK/$name.go.err" "$WORK/$name.rs.err" >"$WORK/$name.err.diff"; then
			status="FAIL stderr"
		fi
	fi

	if [ "$status" = "PASS" ]; then
		printf '  %-18s %s (exit %s)\n' "$name" "$status" "$go_code"
	else
		printf '  %-18s %s\n' "$name" "$status"
		fails=$((fails + 1))
		for d in "$WORK/$name.diff" "$WORK/$name.err.diff"; do
			[ -s "$d" ] && { echo "    --- $(basename "$d") ---"; head -40 "$d" | sed 's/^/    /'; }
		done
	fi
done

echo
if [ "$fails" -eq 0 ]; then
	echo "==> parity-diff: all ${#NAMES[@]} commands identical (modulo volatile fields)"
	exit 0
else
	echo "==> parity-diff: $fails of ${#NAMES[@]} commands diverged" >&2
	exit 1
fi
