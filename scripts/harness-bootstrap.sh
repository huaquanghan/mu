#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
version_file="$repo_root/scripts/harness-cli.version"
checksum_file="$repo_root/scripts/harness-cli.sha256"

if [[ ! -r "$version_file" || ! -r "$checksum_file" ]]; then
  echo "error: Harness version or checksum pin is missing" >&2
  exit 1
fi

tag="$(tr -d '[:space:]' < "$version_file")"
case "$tag" in
  harness-cli-v[0-9]*) ;;
  *)
    echo "error: invalid Harness CLI version pin: $tag" >&2
    exit 1
    ;;
esac

case "$(uname -s)" in
  Linux) os_label="linux" ;;
  Darwin) os_label="macos" ;;
  *)
    echo "error: unsupported Harness CLI platform: $(uname -s)" >&2
    exit 1
    ;;
esac

case "$(uname -m)" in
  x86_64|amd64) arch_label="x64" ;;
  arm64|aarch64) arch_label="arm64" ;;
  *)
    echo "error: unsupported Harness CLI architecture: $(uname -m)" >&2
    exit 1
    ;;
esac

platform="$os_label-$arch_label"
expected="$(awk -v platform="$platform" '$1 == platform { print $2 }' "$checksum_file")"
if [[ ! "$expected" =~ ^[0-9a-f]{64}$ ]]; then
  echo "error: no valid SHA-256 pin for $platform" >&2
  exit 1
fi

asset="harness-cli-$platform"
base_url="${HARNESS_CLI_BASE_URL:-https://github.com/hoangnb24/repository-harness/releases/download/$tag}"
destination="$repo_root/scripts/bin/harness-cli"
mkdir -p "$(dirname "$destination")"

sha256_file() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | awk '{print $1}'
  else
    echo "error: sha256sum or shasum is required" >&2
    return 1
  fi
}

if [[ -x "$destination" ]] && [[ "$(sha256_file "$destination")" == "$expected" ]]; then
  echo "Harness CLI $tag already verified at scripts/bin/harness-cli"
  exit 0
fi

temporary="$(mktemp "$repo_root/scripts/bin/.harness-cli.XXXXXX")"
cleanup() {
  rm -f "$temporary"
}
trap cleanup EXIT

if command -v curl >/dev/null 2>&1; then
  curl --fail --silent --show-error --location "$base_url/$asset" --output "$temporary"
elif command -v wget >/dev/null 2>&1; then
  wget --quiet --output-document="$temporary" "$base_url/$asset"
else
  echo "error: curl or wget is required to download Harness CLI" >&2
  exit 1
fi

actual="$(sha256_file "$temporary")"
if [[ "$actual" != "$expected" ]]; then
  echo "error: Harness CLI checksum mismatch for $platform" >&2
  echo "expected: $expected" >&2
  echo "actual:   $actual" >&2
  exit 1
fi

chmod 755 "$temporary"
expected_version="${tag#harness-cli-v}"
actual_version="$($temporary --version)"
if [[ "$actual_version" != "harness-cli $expected_version" ]]; then
  echo "error: Harness CLI version mismatch: $actual_version" >&2
  exit 1
fi

mv -f "$temporary" "$destination"
trap - EXIT
echo "Installed and verified Harness CLI $tag for $platform"
