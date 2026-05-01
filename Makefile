.PHONY: solomon build

GOOS := $(shell go env GOOS)
ifeq ($(GOOS),windows)
OUT := solomon.exe
else
OUT := solomon
endif

export CGO_ENABLED := 0

LDFLAGS_STATIC := -s -w

solomon build:
	go build -trimpath -ldflags="$(LDFLAGS_STATIC)" -o $(OUT) ./cmd/solomon
