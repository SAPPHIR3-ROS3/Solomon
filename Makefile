.PHONY: solomon build

GOOS := $(shell go env GOOS)
ifeq ($(GOOS),windows)
OUT := solomon.exe
else
OUT := solomon
endif

export CGO_ENABLED := 0

solomon build:
	go build -o $(OUT) ./cmd/solomon
