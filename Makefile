.PHONY: solomon build install hot-install test check-docs loc-chart server-stop desktop-dev gui-deps cursor-stop cursor-build cursor-bundle cursor-proxy-deps cursor-proxy-build cursor-proxy-test cursor-proxy-test-clean cloak-install ui-prototypes-deps ui-prototypes-dev ui-prototypes-build ui-prototypes-test clean-cursor-proxy clean-cursor-bundle clean-temp-exe

GOOS := $(shell go env GOOS)
ifeq ($(GOOS),windows)
OUT ?= solomon.exe
INSTALL_NAME := solomon.exe
else
OUT ?= solomon
INSTALL_NAME := solomon
endif

GO_BIN_DIR := $(strip $(shell go env GOBIN))
ifeq ($(GO_BIN_DIR),)
GO_BIN_DIR := $(shell go env GOPATH)/bin
endif
BIN_DIR ?= $(GO_BIN_DIR)
INSTALL_BIN := $(BIN_DIR)/$(INSTALL_NAME)
ifeq ($(GOOS),windows)
GO_INSTALL = set "GOBIN=$(BIN_DIR)" && go install
else
GO_INSTALL = GOBIN="$(BIN_DIR)" go install
endif

export CGO_ENABLED := 0

ifeq ($(GOOS),windows)
EXACT_TAG := $(shell git describe --tags --exact-match --match "v*" 2>NUL)
BASE_TAG := $(shell git describe --tags --abbrev=0 --match "v*" 2>NUL)
WORKTREE_DIRTY := $(shell git status --porcelain 2>NUL)
LATEST_RELEASE_TAG = $(strip $(shell powershell -NoProfile -Command "(Invoke-RestMethod -UseBasicParsing -Uri 'https://api.github.com/repos/SAPPHIR3-ROS3/Solomon/releases/latest').tag_name" 2>NUL))
VERSION ?= $(if $(EXACT_TAG),$(if $(WORKTREE_DIRTY),$(EXACT_TAG)-dev,$(EXACT_TAG)),$(if $(LATEST_RELEASE_TAG),$(LATEST_RELEASE_TAG)-dev,$(if $(BASE_TAG),$(BASE_TAG)-dev,dev)))
COMMIT ?= $(shell git rev-parse HEAD 2>NUL || echo unknown)
COMMIT_TIME ?= $(shell git show -s --format=%cI HEAD 2>NUL || echo unknown)
else
EXACT_TAG := $(shell git describe --tags --exact-match --match 'v*' 2>/dev/null)
BASE_TAG := $(shell git describe --tags --abbrev=0 --match 'v*' 2>/dev/null)
WORKTREE_DIRTY := $(shell git status --porcelain 2>/dev/null)
LATEST_RELEASE_TAG = $(strip $(shell curl -fsSL --max-time 5 -H 'Accept: application/vnd.github+json' -H 'User-Agent: solomon-build' 'https://api.github.com/repos/SAPPHIR3-ROS3/Solomon/releases/latest' 2>/dev/null | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p'))
VERSION ?= $(if $(EXACT_TAG),$(if $(WORKTREE_DIRTY),$(EXACT_TAG)-dev,$(EXACT_TAG)),$(if $(LATEST_RELEASE_TAG),$(LATEST_RELEASE_TAG)-dev,$(if $(BASE_TAG),$(BASE_TAG)-dev,dev)))
COMMIT ?= $(shell git rev-parse HEAD 2>/dev/null || echo unknown)
COMMIT_TIME ?= $(shell git show -s --format=%cI HEAD 2>/dev/null || echo unknown)
endif
LDFLAGS = -s -w -X github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands.version=$(VERSION) -X github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands.commit=$(COMMIT) -X github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands.commitTime=$(COMMIT_TIME)

BUILD_FLAGS = -trimpath -ldflags="$(LDFLAGS)"

CURSOR_BUNDLER := go run scripts/cursor_bundler.go
CURSOR_PROXY_DIR := integrations/cursor
UI_PROTOTYPES_DIR := ui-prototypes

ifeq ($(GOOS),windows)
FIX_TTY =
define INSTALL_STEP
	@echo.
	@echo -- $(1) --
	@echo     $(2)
	@$(2)
endef
define INSTALL_STEP_SKIPPED
	@echo.
	@echo -- $(1) --
	@echo     CloakBrowser already installed - skipped
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
define INSTALL_STEP_SKIPPED
	@$(FIX_TTY)
	@echo ""
	@echo "── $(1) ──"
	@echo "    → CloakBrowser already installed — skipped"
	@$(FIX_TTY)
endef
endif

cursor-stop:
	@$(FIX_TTY)
	$(CURSOR_BUNDLER) stop
	@$(FIX_TTY)

server-stop:
	-go run $(BUILD_FLAGS) ./cmd/solomon server stop

# Run Wails against the URL advertised by the running Solomon dev server.
desktop-dev:
	$(MAKE) gui-deps
	go run scripts/desktop_dev.go

gui-deps:
	go run ./scripts/npm_deps gui

# Build the Cursor proxy sidecar (TypeScript -> dist/index.js).
cursor-proxy-deps: cursor-stop
	go run ./scripts/npm_deps $(CURSOR_PROXY_DIR)

cursor-proxy-build: cursor-proxy-deps
	npm --prefix $(CURSOR_PROXY_DIR) run build

# Run the Cursor proxy TypeScript unit tests.
cursor-proxy-test: cursor-proxy-deps
	npm --prefix $(CURSOR_PROXY_DIR) test

# Run the Cursor proxy tests and clean up generated artifacts afterwards.
# Cleanup runs even if tests fail, while preserving the test exit code.
cursor-proxy-test-clean:
	@$(MAKE) cursor-proxy-test; status=$$?; $(MAKE) clean-cursor-proxy; exit $$status

ui-prototypes-deps:
	go run ./scripts/npm_deps $(UI_PROTOTYPES_DIR)

ui-prototypes-dev: ui-prototypes-deps
	npm --prefix $(UI_PROTOTYPES_DIR) run dev

ui-prototypes-build: ui-prototypes-deps
	npm --prefix $(UI_PROTOTYPES_DIR) run build

ui-prototypes-test: ui-prototypes-deps
	npm --prefix $(UI_PROTOTYPES_DIR) test

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

ifeq ($(GOOS),windows)
CLOAK_BROWSER_READY := 0
else
CLOAK_BROWSER_READY := $(shell bash -c 'source scripts/install.sh; cloakbrowser_ready' >/dev/null 2>&1 && echo 1)
endif

# Install the official CloakBrowser wrapper/browser and persist internal web
# runtime defaults. The standalone installers and the Makefile use the same
# implementation so hot-install cannot leave the native fallback missing.
ifeq ($(GOOS),windows)
cloak-install:
	powershell -NoProfile -ExecutionPolicy Bypass -File scripts/install.ps1 -CloakBrowserOnly
else
cloak-install:
	bash -c 'source scripts/install.sh; install_cloakbrowser; configure_runtime_defaults'
endif

solomon build: cursor-bundle
	go build $(BUILD_FLAGS) -o $(OUT) ./cmd/solomon

test: cursor-bundle ui-prototypes-test
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

# Full reinstall: stop the Solomon server and Cursor sidecar, verify GUI npm dependencies, rebuild Cursor proxy + embed bundle, install solomon, deploy ~/.solomon integration, and provision CloakBrowser.
install:
	@$(FIX_TTY)
	@echo ""
	@echo "=== Solomon install ($(VERSION)) ==="
	$(call INSTALL_STEP,1/9 Stop Solomon server,$(MAKE) server-stop)
	$(call INSTALL_STEP,2/9 Stop Cursor sidecar,$(CURSOR_BUNDLER) stop)
	$(call INSTALL_STEP,3/9 Build Cursor proxy (TypeScript),$(CURSOR_BUNDLER) build --force)
	$(call INSTALL_STEP,4/9 Prepare embedded Cursor bundle,$(CURSOR_BUNDLER) bundle)
	$(call INSTALL_STEP,5/9 Verify GUI npm dependencies,$(MAKE) gui-deps)
	$(call INSTALL_STEP,6/9 Install solomon binary,$(GO_INSTALL) $(BUILD_FLAGS) ./cmd/solomon)
	$(call INSTALL_STEP,7/9 Install prompt templates,$(INSTALL_BIN) templates install)
	$(call INSTALL_STEP,8/9 Deploy Cursor integration,$(CURSOR_BUNDLER) install)
ifneq ($(CLOAK_BROWSER_READY),1)
	$(call INSTALL_STEP,9/9 Install CloakBrowser,$(MAKE) cloak-install)
else
	$(call INSTALL_STEP_SKIPPED,9/9 Install CloakBrowser)
	@bash -c 'source scripts/install.sh; configure_runtime_defaults' >/dev/null
endif
	@$(FIX_TTY)
	@echo ""
	@echo "solomon -> $(INSTALL_BIN)"
	@echo "=== Done ==="

# Full install, then bring the local server back up. Windows starts the GUI in
# dev mode; Unix preserves the prior mode/dev directory from state.json when
# present, otherwise starts `server start dev <repo>/gui`.
# Needed because `make install` stops the server and clears state before `restart` can read it.
hot-install:
	@$(FIX_TTY)
	@echo ""
	@echo "=== Solomon hot-install ($(VERSION)) ==="
ifeq ($(GOOS),windows)
	@$(MAKE) install
	@$(INSTALL_BIN) server start dev "$(CURDIR)/gui"
else
	@STATE_FILE="$${SOLOMON_HOME:-$$HOME/.solomon}/run/server/state.json"; \
	MODE=dev; \
	DEVDIR="$(CURDIR)/gui"; \
	if [ -f "$$STATE_FILE" ]; then \
		CAPTURED_MODE=$$(sed -n 's/.*"mode"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$$STATE_FILE" | head -n1); \
		CAPTURED_DEVDIR=$$(sed -n 's/.*"dev_directory"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$$STATE_FILE" | head -n1); \
		if [ -n "$$CAPTURED_MODE" ]; then MODE=$$CAPTURED_MODE; fi; \
		if [ -n "$$CAPTURED_DEVDIR" ]; then DEVDIR=$$CAPTURED_DEVDIR; fi; \
	fi; \
	echo "Restart target: mode=$$MODE"; \
	if [ "$$MODE" = "dev" ]; then echo "Restart target: devDir=$$DEVDIR"; fi; \
	$(MAKE) install; \
	if [ "$$MODE" = "dev" ]; then \
		$(INSTALL_BIN) server start dev "$$DEVDIR"; \
	else \
		$(INSTALL_BIN) server start; \
	fi
endif
	@$(FIX_TTY)
	@echo ""
	@echo "=== hot-install done ==="

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
