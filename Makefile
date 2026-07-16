.PHONY: solomon build install test check-docs loc-chart cursor-stop cursor-build cursor-bundle cursor-proxy-build cursor-proxy-test cursor-proxy-test-clean clean-cursor-proxy clean-cursor-bundle clean-temp-exe

GOOS := $(shell go env GOOS)
ifeq ($(GOOS),windows)
OUT ?= solomon.exe
INSTALL_NAME := solomon.exe
else
OUT ?= solomon
INSTALL_NAME := solomon
endif

BIN_DIR ?= $(shell go env GOPATH)/bin
INSTALL_BIN := $(BIN_DIR)/$(INSTALL_NAME)

export CGO_ENABLED := 0

ifeq ($(GOOS),windows)
VERSION ?= $(shell git describe --tags --abbrev=0 --match "v*" 2>NUL || git describe --tags --abbrev=0 --match 'v*' 2>NUL || echo dev)
COMMIT ?= $(shell git rev-parse --short HEAD 2>NUL || echo unknown)
else
VERSION ?= $(shell git describe --tags --abbrev=0 --match "v*" 2>/dev/null || git describe --tags --abbrev=0 --match 'v*' 2>/dev/null || echo dev)
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
endif
LDFLAGS := -s -w -X github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands.version=$(VERSION) -X github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands.commit=$(COMMIT)

BUILD_FLAGS := -trimpath -ldflags="$(LDFLAGS)"

CURSOR_BUNDLER := go run scripts/cursor_bundler.go
CURSOR_PROXY_DIR := integrations/cursor

ifeq ($(GOOS),windows)
FIX_TTY =
define INSTALL_STEP
	@echo.
	@echo -- $(1) --
	@echo     $(2)
	@$(2)
endef
else
FIX_TTY = stty sane opost onlcr icanon echo 2>/dev/null || true;
define INSTALL_STEP
	@$(FIX_TTY)
	@echo ""
	@echo "── $(1) ──"
	@echo "    $$ $(2)"
	@$(2)
	@$(FIX_TTY)
endef
endif

cursor-stop:
	@$(FIX_TTY)
	$(CURSOR_BUNDLER) stop
	@$(FIX_TTY)

# Build the Cursor proxy sidecar (TypeScript -> dist/index.js).
cursor-proxy-build:
	npm --prefix $(CURSOR_PROXY_DIR) run build

# Run the Cursor proxy TypeScript unit tests.
cursor-proxy-test:
	npm --prefix $(CURSOR_PROXY_DIR) test

# Run the Cursor proxy tests and clean up generated artifacts afterwards.
# Cleanup runs even if tests fail, while preserving the test exit code.
cursor-proxy-test-clean:
	@$(MAKE) cursor-proxy-test; status=$$?; $(MAKE) clean-cursor-proxy; exit $$status

# Remove generated Cursor proxy artifacts (test bundle dir + runtime guard dir).
clean-cursor-proxy:
ifeq ($(GOOS),windows)
	-cmd /C "if exist integrations\cursor\.test rmdir /S /Q integrations\cursor\.test"
	-cmd /C "if exist integrations\cursor\.solomon-cursor-guard rmdir /S /Q integrations\cursor\.solomon-cursor-guard"
else
	-rm -rf $(CURSOR_PROXY_DIR)/.test $(CURSOR_PROXY_DIR)/.solomon-cursor-guard
endif

cursor-build: cursor-stop
	$(CURSOR_BUNDLER) build

cursor-bundle: cursor-build
	$(CURSOR_BUNDLER) bundle

solomon build: cursor-bundle
	go build $(BUILD_FLAGS) -o $(OUT) ./cmd/solomon

test: cursor-bundle
	go test ./... -count=1

check-docs:
	go run scripts/check_doc_paths.go
	go run scripts/check_package_index.go

loc-chart:
	go run scripts/loc_chart.go scripts/loc_chart_render.go

ifneq (,$(wildcard ./.env))
include .env
export
endif

# Full reinstall: stop sidecar, rebuild Cursor proxy + embed bundle, install solomon, deploy ~/.solomon integration.
install:
	@$(FIX_TTY)
	@echo ""
	@echo "=== Solomon install ($(VERSION)) ==="
	$(call INSTALL_STEP,1/6 Stop Cursor sidecar,$(CURSOR_BUNDLER) stop)
	$(call INSTALL_STEP,2/6 Build Cursor proxy (TypeScript),$(CURSOR_BUNDLER) build --force)
	$(call INSTALL_STEP,3/6 Prepare embedded Cursor bundle,$(CURSOR_BUNDLER) bundle)
	$(call INSTALL_STEP,4/6 Install solomon binary,go install $(BUILD_FLAGS) ./cmd/solomon)
	$(call INSTALL_STEP,5/6 Install prompt templates,$(INSTALL_BIN) templates install)
	$(call INSTALL_STEP,6/6 Deploy Cursor integration,$(CURSOR_BUNDLER) install)
	@$(FIX_TTY)
	@echo ""
	@echo "solomon -> $(INSTALL_BIN)"
	@echo "=== Done ==="

clean-cursor-bundle:
ifeq ($(GOOS),windows)
	-cmd /C "if exist internal\integrations\cursor\bundle rmdir /S /Q internal\integrations\cursor\bundle"
else
	-rm -rf internal/integrations/cursor/bundle
endif

clean-temp-exe:
ifeq ($(GOOS),windows)
	-cmd /C "if exist solomon.exe~ del /F /Q solomon.exe~"
else
	-rm -f solomon.exe~
endif
