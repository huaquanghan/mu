BINARY    := mu
MODULE    := github.com/huaquanghan/mu
CMD       := ./cmd/mu
BUILD_DIR := ./bin
VERSION   ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS   := -s -w -X $(MODULE)/cmd/mu/cli.Version=$(VERSION)

.PHONY: build install test lint clean release

build:
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY) $(CMD)

install: build
	install -Dm755 $(BUILD_DIR)/$(BINARY) $(DESTDIR)$(PREFIX)/bin/$(BINARY)

test:
	go test ./...

lint:
	golangci-lint run ./...

clean:
	rm -rf $(BUILD_DIR)

release:
	goreleaser release --clean

deps:
	go get github.com/spf13/cobra@latest
	go get github.com/charmbracelet/bubbletea@latest
	go get github.com/charmbracelet/lipgloss@latest
	go get github.com/charmbracelet/bubbles@latest
	go mod tidy

PREFIX ?= /usr/local
