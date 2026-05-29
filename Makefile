.PHONY: solomon build install test cursor-stop cursor-build cursor-bundle clean-cursor-bundle clean-temp-exe

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

cursor-stop:
	$(CURSOR_BUNDLER) stop

cursor-build: cursor-stop
	$(CURSOR_BUNDLER) build

cursor-bundle: cursor-build
	$(CURSOR_BUNDLER) bundle

solomon build: cursor-bundle
	go build $(BUILD_FLAGS) -o $(OUT) ./cmd/solomon

test: cursor-bundle
	go test ./... -count=1

# Full reinstall: stop sidecar, rebuild Cursor proxy + embed bundle, install solomon, deploy ~/.solomon integration.
install: cursor-stop
	$(CURSOR_BUNDLER) build --force
	$(CURSOR_BUNDLER) bundle
	go install $(BUILD_FLAGS) ./cmd/solomon
	$(CURSOR_BUNDLER) install
	@echo installed $(INSTALL_BIN)

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
