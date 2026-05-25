.PHONY: solomon build install cursor-stop cursor-build cursor-bundle clean-cursor-bundle clean-temp-exe

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

LDFLAGS := -s -w

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
