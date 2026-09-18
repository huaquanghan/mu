BINARY    := mu
BUILD_DIR := ./bin
# musl produces a fully static binary — the Go build's `CGO_ENABLED=0`
# equivalent. Requires `rustup target add x86_64-unknown-linux-musl`.
RS_TARGET := x86_64-unknown-linux-musl
RS_BINARY := ./target/$(RS_TARGET)/release/$(BINARY)
INSTALL_DIR := $(HOME)/.local/bin
SHELL     := /bin/bash
.SHELLFLAGS := -eu -o pipefail -c

.PHONY: build install install-local uninstall test test-verbose smoke run clean lint fmt release checksums harness-bootstrap harness-init

# ── Build ────────────────────────────────────────────────────────────────────

# cargo release build (static musl + stripped), copied to bin/mu
# (install/checksums/smoke consume bin/mu; version comes from build.rs
# `git describe` — same as the old -X ldflag).
build:
	cargo build --release --target $(RS_TARGET)
	@mkdir -p $(BUILD_DIR)
	install -m755 $(RS_BINARY) $(BUILD_DIR)/$(BINARY)
	@echo "Built $(BUILD_DIR)/$(BINARY) ($$(du -sh $(BUILD_DIR)/$(BINARY) | cut -f1))"

# ── Install ───────────────────────────────────────────────────────────────────

# System-wide install (requires sudo)
install: build
	install -Dm755 $(BUILD_DIR)/$(BINARY) $(DESTDIR)$(PREFIX)/bin/$(BINARY)
	@echo "Installed to $(DESTDIR)$(PREFIX)/bin/$(BINARY)"

# Local install to ~/.local/bin (no sudo needed)
install-local: build
	@mkdir -p $(INSTALL_DIR)
	install -m755 $(BUILD_DIR)/$(BINARY) $(INSTALL_DIR)/$(BINARY)
	@echo "Installed to $(INSTALL_DIR)/$(BINARY)"
	@echo "Make sure $(INSTALL_DIR) is in your PATH."

uninstall:
	rm -f $(INSTALL_DIR)/$(BINARY) $(DESTDIR)$(PREFIX)/bin/$(BINARY)
	@echo "Uninstalled mu"

# ── Test ─────────────────────────────────────────────────────────────────────

test:
	cargo test

test-verbose:
	cargo test -- --nocapture

# Smoke test: runs the non-destructive flags against the live system.
# Does not require YES confirmation or sudo.
smoke: build
	@echo "=== mu --help ==="
	@$(BUILD_DIR)/$(BINARY) --help
	@echo ""
	@echo "=== mu clean --dry-run ==="
	@$(BUILD_DIR)/$(BINARY) clean --dry-run | cat
	@echo ""
	@echo "=== mu optimize --dry-run ==="
	@$(BUILD_DIR)/$(BINARY) optimize --dry-run
	@echo ""
	@echo "=== mu audit --report ==="
	@code=0; $(BUILD_DIR)/$(BINARY) audit --report || code=$$?; \
		if [ "$$code" -gt 2 ]; then echo "unexpected audit exit code: $$code" >&2; exit "$$code"; fi
	@echo ""
	@echo "=== mu status (JSON mode) ==="
	@$(BUILD_DIR)/$(BINARY) status | python3 -m json.tool --no-ensure-ascii
	@echo ""
	@echo "=== Binary size ==="
	@du -sh $(BUILD_DIR)/$(BINARY)
	@echo "=== All smoke checks passed ==="

# ── Dev ───────────────────────────────────────────────────────────────────────

run: build
	@$(BUILD_DIR)/$(BINARY)

lint:
	cargo clippy --all-targets -- -D warnings

fmt:
	cargo fmt --check

clean:
	rm -rf $(BUILD_DIR)
	cargo clean

# checksums.txt for GitHub release assets (required by scripts/install.sh).
# After tagging a release binary, attach both bin/mu and bin/checksums.txt.
checksums: build
	cd $(BUILD_DIR) && sha256sum $(BINARY) > checksums.txt
	@echo "Wrote $(BUILD_DIR)/checksums.txt:"
	@cat $(BUILD_DIR)/checksums.txt

# Cargo-compatible release: tag first (`git tag vX.Y.Z`), then this builds the
# release binary, writes checksums.txt, and publishes both assets via gh.
# The tag check runs before the build so a missing tag fails fast.
# scripts/install.sh consumes exactly this artifact pair (mu + checksums.txt).
release:
	@tag=$$(git describe --tags --exact-match 2>/dev/null) || { \
		echo "error: HEAD is not an exact tag — run 'git tag vX.Y.Z' first" >&2; exit 1; }; \
	$(MAKE) checksums; \
	gh release create "$$tag" $(BUILD_DIR)/$(BINARY) $(BUILD_DIR)/checksums.txt \
		--title "$$tag" --generate-notes

harness-bootstrap:
	./scripts/harness-bootstrap.sh

harness-init: harness-bootstrap
	./scripts/bin/harness-cli init
	./scripts/bin/harness-cli import brownfield

PREFIX ?= /usr/local
