.DEFAULT_GOAL := help

# Bauen und Testen, lokal und in CI mit denselben Targets und Flags. Persönliche
# Abläufe (release, sichern) stehen in k-playbook-local/Makefile.

BINARY := kephalaion
PKG := ./cmd/kephalaion
BUILDINFO := github.com/kascada/kephalaion/internal/buildinfo
DIST_DIR := dist
SUMS_FILE := SHA256SUMS
RELEASE_TARGETS := linux-amd64 linux-arm64 darwin-amd64 darwin-arm64
# Verzeichnisse mit Go-Code des Projekts. Nicht `.`: darunter liegt die
# k-playbook-Installation mit eigenem Go-Code, der hier nicht geprüft wird.
GO_DIRS := cmd internal

# Die Version ist der Git-Tag; ohne Angabe entsteht ein dev build. Der
# Release-Workflow ruft `make dist VERSION=<tag>`.
VERSION ?= dev
# Mit = statt := : git und go laufen erst, wenn ein Target den Wert braucht,
# und nicht schon bei `make help`.
# --long: auch auf einem getaggten Commit steht der Hash dabei (v0.1.0-0-g<sha>).
COMMIT ?= $(shell git describe --tags --long --always --dirty 2>/dev/null)
GO_TOOLCHAIN = $(shell awk '$$1 == "toolchain" { print $$2 }' go.mod)
HOST_TARGET = $(shell go env GOOS)-$(shell go env GOARCH)

# -trimpath und CGO_ENABLED=0 machen das Binary unabhängig vom Bau-Rechner;
# -buildvcs=false, weil der Commit ausdrücklich per -ldflags kommt.
LDFLAGS = -s -w -X $(BUILDINFO).Version=$(VERSION) -X $(BUILDINFO).Commit=$(COMMIT)

.PHONY: help build test check check-toolchain dist dist-host dev-install clean

help: ## Zeigt diese Hilfe an
	@echo "Targets:"
	@echo ""
	@grep -hE '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
	  awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "Parameter:"
	@echo "  VERSION=v0.1.0   Version im Binary, sonst dev"
	@echo ""

define build_binaries
	@mkdir -p "$(DIST_DIR)"
	@set -eu; \
	for target in $(1); do \
	  os="$${target%-*}"; \
	  arch="$${target#*-}"; \
	  output="$(DIST_DIR)/$(BINARY)-$${os}-$${arch}"; \
	  echo "Baue $$output ($(VERSION))"; \
	  CGO_ENABLED=0 GOOS="$$os" GOARCH="$$arch" \
	    go build -trimpath -buildvcs=false -ldflags="$(LDFLAGS)" -o "$$output" "$(PKG)"; \
	done
endef

build: dist-host ## Alias für dist-host

dist-host: ## Baut nur das Binary dieser Plattform nach ./dist/
	$(call build_binaries,$(HOST_TARGET))

# dist räumt vorher auf: SHA256SUMS soll genau die vier Binaries dieses Laufs
# decken, keine Reste eines früheren.
dist: ## Baut alle vier Plattformen nach ./dist/ und schreibt SHA256SUMS
	@rm -rf "$(DIST_DIR)"
	$(call build_binaries,$(RELEASE_TARGETS))
	@set -eu; \
	  if command -v sha256sum >/dev/null 2>&1; then \
	    checksum() { sha256sum "$$@"; }; \
	  else \
	    checksum() { shasum -a 256 "$$@"; }; \
	  fi; \
	  cd "$(DIST_DIR)"; \
	  for target in $(RELEASE_TARGETS); do \
	    checksum "$(BINARY)-$$target"; \
	  done > "$(SUMS_FILE)"; \
	  echo "Geschrieben: $(DIST_DIR)/$(SUMS_FILE)"

test: ## Führt die Tests aus
	go test ./...

# install.sh bekommt hier nur die Syntaxprüfung; shellcheck läuft in CI.
check: ## gofmt-Prüfung, go vet, Tests und Syntax von install.sh
	sh -n install.sh
	@set -eu; \
	  unformatted="$$(gofmt -l $(GO_DIRS))"; \
	  if [ -n "$$unformatted" ]; then \
	    printf 'gofmt: diese Dateien sind nicht formatiert:\n%s\n' "$$unformatted" >&2; \
	    exit 1; \
	  fi
	go vet ./...
	go test ./...

# CI ruft das vor dem Bauen: mit GOTOOLCHAIN=local soll eine abweichende
# Toolchain den Lauf scheitern lassen, statt still anders zu bauen.
check-toolchain: ## Prüft, ob die Toolchain aus go.mod läuft
	@set -eu; \
	  want="$(GO_TOOLCHAIN)"; \
	  test -n "$$want" || { printf 'In go.mod fehlt die toolchain-Zeile.\n' >&2; exit 1; }; \
	  have="$$(go env GOVERSION)"; \
	  test "$$have" = "$$want" || { \
	    printf 'Go %s läuft, verlangt ist %s (go.mod, toolchain).\n' "$$have" "$$want" >&2; \
	    exit 1; \
	  }; \
	  echo "Toolchain: $$have"

dev-install: dist-host ## Baut diese Plattform und ersetzt ~/.local/bin/kephalaion
	@set -eu; \
	  binary="$(DIST_DIR)/$(BINARY)-$(HOST_TARGET)"; \
	  target="$$HOME/.local/bin/$(BINARY)"; \
	  mkdir -p "$${target%/*}"; \
	  command -p install -m 755 "$$binary" "$$target.tmp"; \
	  mv -f "$$target.tmp" "$$target"; \
	  printf 'Installiert: %s\n' "$$target"

clean: ## Entfernt ./dist/
	rm -rf "$(DIST_DIR)"
