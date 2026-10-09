.PHONY: solomon build install hot-install install-prepare install-deploy test check-docs loc-chart server-stop desktop-dev desktop-build desktop-install gui-build gui-deps cursor-stop cursor-build cursor-bundle cursor-proxy-deps cursor-proxy-build cursor-proxy-test cursor-proxy-test-clean cloak-install clean-cursor-proxy clean-cursor-bundle clean-temp-exe

GOOS := $(shell go env GOOS)
ifeq ($(GOOS),windows)
SHELL := cmd.exe
.SHELLFLAGS := /c
OUT ?= solomon.exe
INSTALL_NAME := solomon.exe
else
OUT ?= solomon
INSTALL_NAME := solomon
endif

GO_BIN_DIR := $(strip $(shell go run ./scripts/install_path))
BIN_DIR ?= $(GO_BIN_DIR)
INSTALL_BIN := $(BIN_DIR)/$(INSTALL_NAME)
export CGO_ENABLED := 0

ifeq ($(GOOS),windows)
EXACT_TAG := $(shell git describe --tags --exact-match --match "v*" 2>NUL)
BASE_TAG := $(shell git describe --tags --abbrev=0 --match "v*" 2>NUL)
WORKTREE_DIRTY := $(shell git status --porcelain 2>NUL)
VERSION ?= $(if $(EXACT_TAG),$(if $(WORKTREE_DIRTY),$(EXACT_TAG)-dev,$(EXACT_TAG)),$(if $(BASE_TAG),$(BASE_TAG)-dev,dev))
COMMIT ?= $(shell git rev-parse HEAD 2>NUL || echo unknown)
COMMIT_TIME ?= $(shell git show -s --format=%%cI HEAD 2>NUL || echo unknown)
else
EXACT_TAG := $(shell git describe --tags --exact-match --match 'v*' 2>/dev/null)
BASE_TAG := $(shell git describe --tags --abbrev=0 --match 'v*' 2>/dev/null)
WORKTREE_DIRTY := $(shell git status --porcelain 2>/dev/null)
VERSION ?= $(if $(EXACT_TAG),$(if $(WORKTREE_DIRTY),$(EXACT_TAG)-dev,$(EXACT_TAG)),$(if $(BASE_TAG),$(BASE_TAG)-dev,dev))
COMMIT ?= $(shell git rev-parse HEAD 2>/dev/null || echo unknown)
COMMIT_TIME ?= $(shell git show -s --format=%cI HEAD 2>/dev/null || echo unknown)
endif
ifeq ($(origin VERSION),file)
VERSION := $(VERSION)
endif
export VERSION
SOURCE_TREE = $(strip $(shell go run ./scripts/source_identity))
COMMIT_TREE = $(strip $(shell git rev-parse "HEAD^{tree}"))
LDFLAGS = -s -w -X github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands.version=$(VERSION) -X github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands.commit=$(COMMIT) -X github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands.commitTime=$(COMMIT_TIME) -X github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands.sourceTree=$(SOURCE_TREE) -X github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands.commitTree=$(COMMIT_TREE)

BUILD_FLAGS = -trimpath -ldflags="$(LDFLAGS)"

CURSOR_BUNDLER := go run scripts/cursor_bundler.go
CURSOR_PROXY_DIR := integrations/cursor

ifeq ($(GOOS),windows)
FIX_TTY =
PRINT_LINE = echo $(subst >,^>,$(1))
PRINT_BLANK = echo.
define INSTALL_STEP
	@echo.
	@echo -- $(1) --
	@$(2)
endef
define INSTALL_STEP_SKIPPED
	@echo.
	@echo -- $(1) --
	@echo     CloakBrowser already installed - skipped
endef
else
FIX_TTY = stty sane opost onlcr icanon echo 2>/dev/null || true;
PRINT_LINE = echo "$(1)"
PRINT_BLANK = echo ""
define INSTALL_STEP
	@$(FIX_TTY)
	@$(PRINT_BLANK)
	@echo "── $(1) ──"
	@echo "    $$ $(2)"
	@$(2)
	@$(FIX_TTY)
endef
define INSTALL_STEP_SKIPPED
	@$(FIX_TTY)
	@$(PRINT_BLANK)
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
	go run $(BUILD_FLAGS) ./cmd/solomon server stop

# Run Wails against the URL advertised by the running Solomon dev server.
desktop-dev:
	$(MAKE) gui-deps
	go run scripts/desktop_dev.go

# Build and register the native Windows application and its CLI launcher.
ifeq ($(GOOS),windows)
desktop-build: gui-deps
	powershell -NoProfile -ExecutionPolicy Bypass -File scripts/build_desktop.ps1

endif

gui-deps:
	go run ./scripts/npm_deps gui

# Stage the production UI for both the local web server and Wails.
gui-build: gui-deps
	npm --prefix gui run build
	go run scripts/gui_bundle.go

# Native Linux builds require GTK 3 and WebKitGTK 4.1 development packages.
ifeq ($(GOOS),linux)
desktop-build: gui-build
	mkdir -p gui/desktop/build/bin
	CGO_ENABLED=1 go build -tags production,webkit2_41 $(BUILD_FLAGS) -o gui/desktop/build/bin/solomon-desktop ./gui/desktop

endif

ifeq ($(GOOS),darwin)
desktop-build: gui-build
	cd gui/desktop && CGO_ENABLED=1 go run "github.com/wailsapp/wails/v2/cmd/wails@$$(go list -m -f '{{.Version}}' github.com/wailsapp/wails/v2)" build -skipbindings -nosyncgomod -m
endif

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

solomon build: cursor-bundle gui-build
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

# Prepare every artifact and dependency before interrupting the installed runtime.
install-prepare:
	@$(FIX_TTY)
	@$(PRINT_BLANK)
	@$(call PRINT_LINE,=== Prepare Solomon installation ($(VERSION)) ===)
	$(call INSTALL_STEP,1/5 Build Cursor proxy,$(CURSOR_BUNDLER) build --force)
	$(call INSTALL_STEP,2/5 Prepare embedded Cursor bundle,$(CURSOR_BUNDLER) bundle)
	$(call INSTALL_STEP,3/5 Build production GUI and native desktop,$(MAKE) gui-build desktop-build)
	$(call INSTALL_STEP,4/5 Build staged CLI,go build $(BUILD_FLAGS) -o "$(OUT)" ./cmd/solomon)
ifneq ($(CLOAK_BROWSER_READY),1)
	$(call INSTALL_STEP,5/5 Provision CloakBrowser,$(MAKE) cloak-install)
else
	$(call INSTALL_STEP_SKIPPED,5/5 Provision CloakBrowser)
	@bash -c 'source scripts/install.sh; configure_runtime_defaults' >/dev/null
endif

# Called only by the coordinator, with both installed executables backed up.
install-deploy:
	"$(INSTALL_BIN)" templates install
	$(CURSOR_BUNDLER) deploy-install "$(CURSOR_STAGE)"
	@$(FIX_TTY)
	@$(PRINT_BLANK)
	@$(call PRINT_LINE,solomon -> $(INSTALL_BIN))

# All source installations use the same runtime handoff on every platform.
install hot-install desktop-install:
	@$(FIX_TTY)
	@$(PRINT_BLANK)
	@$(call PRINT_LINE,=== Solomon hot-install ($(VERSION)) ===)
	go run ./scripts/hot_install "$(INSTALL_BIN)" "$(MAKE)" "$(BIN_DIR)"
	@$(FIX_TTY)
	@$(PRINT_BLANK)
	@$(call PRINT_LINE,=== hot-install done ===)

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
