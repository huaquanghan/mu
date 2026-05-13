#!/usr/bin/env bash
set -euo pipefail

REPO="huaquanghan/mu"
INSTALL_DIR="${MU_INSTALL_DIR:-/usr/local/bin}"
BINARY="mu"

detect_arch() {
  case "$(uname -m)" in
    x86_64) echo "amd64" ;;
    aarch64|arm64) echo "arm64" ;;
    *) echo "unsupported arch: $(uname -m)" >&2; exit 1 ;;
  esac
}

main() {
  local arch
  arch="$(detect_arch)"
  local os_name="linux"

  echo "→ Fetching latest release..."
  local latest
  latest="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | cut -d'"' -f4)"

  if [[ -z "$latest" ]]; then
    echo "Error: could not determine latest release" >&2
    exit 1
  fi

  local url="https://github.com/${REPO}/releases/download/${latest}/${BINARY}_${os_name}_${arch}.tar.gz"
  local tmp
  tmp="$(mktemp -d)"
  trap 'rm -rf "$tmp"' EXIT

  echo "→ Downloading ${BINARY} ${latest} (${os_name}/${arch})..."
  curl -fsSL "$url" | tar -xz -C "$tmp"

  echo "→ Installing to ${INSTALL_DIR}/${BINARY}..."
  sudo install -Dm755 "${tmp}/${BINARY}" "${INSTALL_DIR}/${BINARY}"

  echo "✅  mu ${latest} installed. Run: mu --help"
}

main "$@"
