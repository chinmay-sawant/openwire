.PHONY: build test vet lint run clean

GO ?= go
export GOTOOLCHAIN ?= go1.26.4

BIN_DIR := bin
BIN := $(BIN_DIR)/openwire

build:
	@mkdir -p $(BIN_DIR)
	$(GO) build -o $(BIN) ./cmd/openwire

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

lint: vet

run:
	$(GO) run ./cmd/openwire start --demo

clean:
	rm -f $(BIN)
