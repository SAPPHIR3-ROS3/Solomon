.PHONY: solomon build clean-temp-exe

GOOS := $(shell go env GOOS)
ifeq ($(GOOS),windows)
OUT ?= solomon.exe
else
OUT ?= solomon
endif

export CGO_ENABLED := 0

LDFLAGS_STATIC := -s -w

solomon build:
	go build -trimpath -ldflags="$(LDFLAGS_STATIC)" -o $(OUT) ./cmd/solomon

clean-temp-exe:
ifeq ($(GOOS),windows)
	-cmd /C "if exist solomon.exe~ del /F /Q solomon.exe~"
else
	-rm -f solomon.exe~
endif
