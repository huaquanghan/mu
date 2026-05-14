#!/usr/bin/env bash
set -euo pipefail

REPO="huaquanghan/mu"
INSTALL_DIR="${MU_INSTALL_DIR:-/usr/local/bin}"
BINARY="mu"
TMP_DIR=""

trap '[[ -n "$TMP_DIR" ]] && rm -rf "$TMP_DIR"' EXIT

main() {
  echo "→ Fetching latest release..."
  local latest
  latest="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | cut -d'"' -f4)"

  if [[ -z "$latest" ]]; then
    echo "Error: could not determine latest release" >&2
    exit 1
  fi

  TMP_DIR="$(mktemp -d)"
  local url="https://github.com/${REPO}/releases/download/${latest}/${BINARY}"

  echo "→ Downloading ${BINARY} ${latest}..."
  curl -fsSL -o "${TMP_DIR}/${BINARY}" "$url"
  chmod +x "${TMP_DIR}/${BINARY}"

  echo "→ Installing to ${INSTALL_DIR}/${BINARY}..."
  sudo install -Dm755 "${TMP_DIR}/${BINARY}" "${INSTALL_DIR}/${BINARY}"

  echo "✨ mu ${latest} installed! Run: mu --help"
}

main "$@"
