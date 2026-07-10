BINARY    := mu
MODULE    := github.com/huaquanghan/mu
CMD       := ./cmd/mu
BUILD_DIR := ./bin
INSTALL_DIR := $(HOME)/.local/bin
VERSION   ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS   := -s -w -X $(MODULE)/cmd/mu/cli.Version=$(VERSION)

.PHONY: build install install-local uninstall test test-verbose test-race smoke clean lint release deps checksums

# ── Build ────────────────────────────────────────────────────────────────────

build:
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY) $(CMD)
	@echo "Built $(BUILD_DIR)/$(BINARY) ($(shell du -sh $(BUILD_DIR)/$(BINARY) | cut -f1))"

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
	go test ./... -count=1

test-verbose:
	go test ./... -count=1 -v

test-race:
	go test ./... -count=1 -race

# Smoke test: runs the non-destructive flags against the live system.
# Does not require YES confirmation or sudo.
smoke: build
	@echo "=== mu --help ==="
	@$(BUILD_DIR)/$(BINARY) --help
	@echo ""
	@echo "=== mu clean --dry-run ==="
	@$(BUILD_DIR)/$(BINARY) clean --dry-run
	@echo ""
	@echo "=== mu optimize --dry-run ==="
	@$(BUILD_DIR)/$(BINARY) optimize --dry-run
	@echo ""
	@echo "=== mu audit --report ==="
	@$(BUILD_DIR)/$(BINARY) audit --report || true
	@echo ""
	@echo "=== mu status (JSON mode) ==="
	@$(BUILD_DIR)/$(BINARY) status | python3 -m json.tool --no-ensure-ascii | head -20
	@echo ""
	@echo "=== Binary size ==="
	@du -sh $(BUILD_DIR)/$(BINARY)
	@echo "=== All smoke checks passed ==="

# ── Dev ───────────────────────────────────────────────────────────────────────

lint:
	golangci-lint run ./...

clean:
	rm -rf $(BUILD_DIR)

deps:
	go get github.com/spf13/cobra@latest
	go get github.com/charmbracelet/bubbletea@latest
	go get github.com/charmbracelet/lipgloss@latest
	go get github.com/charmbracelet/bubbles@latest
	go mod tidy

# checksums.txt for GitHub release assets (required by scripts/install.sh).
# After tagging a release binary, attach both bin/mu and bin/checksums.txt.
checksums: build
	cd $(BUILD_DIR) && sha256sum $(BINARY) > checksums.txt
	@echo "Wrote $(BUILD_DIR)/checksums.txt:"
	@cat $(BUILD_DIR)/checksums.txt

release:
	goreleaser release --clean

PREFIX ?= /usr/local
