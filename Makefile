.PHONY: build install clean test

BINARY := shanty
BUILD_DIR := .
INSTALL_DIR := $(HOME)/.local/bin

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS := -X github.com/intigraph/shanty/internal/cmd.Version=$(VERSION) \
           -X github.com/intigraph/shanty/internal/cmd.Commit=$(COMMIT) \
           -X github.com/intigraph/shanty/internal/cmd.BuildTime=$(BUILD_TIME)

build:
	go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY) ./cmd/shanty

# Install via a temp file and a rename, NOT a plain cp.
#
# cp writes THROUGH to the existing inode, and the kernel refuses that for a
# binary that is currently executing: "cp: cannot create regular file ...: Text
# file busy". On any host actually using shanty that is the normal state, not an
# edge case — the status bar re-runs `shanty seg` every few seconds in every
# pane, so `make install` fails precisely when shanty is in use.
#
# A rename swaps the directory entry instead. Already-running processes keep the
# old inode until they exit, and the next exec gets the new binary.
install: build
	@mkdir -p $(INSTALL_DIR)
	@cp $(BUILD_DIR)/$(BINARY) $(INSTALL_DIR)/.$(BINARY).new
	@mv -f $(INSTALL_DIR)/.$(BINARY).new $(INSTALL_DIR)/$(BINARY)
	@echo "Installed $(BINARY) to $(INSTALL_DIR)"

test:
	go test ./...

clean:
	rm -f $(BUILD_DIR)/$(BINARY)
