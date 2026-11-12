.PHONY: build test test-live test-dev vet dev components config

build:
	go build -o laboratory .
	go run ./cmd/launchers

test: build
	go test ./test
	python3 test/terminal.py
	python3 test/home.py
	python3 test/mouse.py
	python3 test/tool_header.py
	python3 test/responsiveness.py
	python3 test/live.py
	$(MAKE) -C ../tui test

vet:
	go vet ./...

dev:
	go run ./cmd/tui-dev --components

components: build
	./laboratory --components

config: build
	./laboratory --init-config

test-live: build
	python3 test/live.py

test-dev: build
	go build -o tui-dev ./cmd/tui-dev
	python3 test/development.py
