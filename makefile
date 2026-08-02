.PHONY: build test test-live vet lint run clean

GO ?= go
export GOTOOLCHAIN ?= go1.26.4

BIN_DIR := bin
BIN := $(BIN_DIR)/openwire

build:
	@mkdir -p $(BIN_DIR)
	$(GO) build -o $(BIN) ./cmd/openwire

test:
	$(GO) test ./...

# P5.14: prove AF_PACKET live capture with capabilities (requires Docker).
test-live:
	docker run --rm --cap-add=NET_RAW --cap-add=NET_ADMIN --net=host \
		-v "$(CURDIR)":/src -w /src golang:1.26.4 \
		go test -v ./internal/capture/ -run 'TestLinuxEngineLiveCapture|TestCanOpenCapture' -count=1

vet:
	$(GO) vet ./...

lint: vet

run:
	$(GO) run ./cmd/openwire start --demo

clean:
	rm -f $(BIN)
