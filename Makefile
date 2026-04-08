# gh-sr — self-hosted GitHub Actions runners (GitHub CLI extension)
# https://github.com/an-lee/gh-sr

ifeq ($(OS),Windows_NT)
BINARY := gh-sr.exe
else
BINARY := gh-sr
endif

CMD_DIR := ./cmd/gh-sr

GIT_TAG := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

PREFIX ?= /usr/local

.PHONY: all build test vet check clean install

all: build

build:
	go build -ldflags "-X main.version=$(GIT_TAG)" -o $(BINARY) $(CMD_DIR)/

test:
	go test ./... -race -count=1

vet:
	go vet ./...

check: vet test

clean:
	rm -f $(BINARY)

# Unix-like systems only (requires coreutils install); not for plain cmd.exe / PowerShell.
install: build
	install -d "$(DESTDIR)$(PREFIX)/bin"
	install -m 755 "$(BINARY)" "$(DESTDIR)$(PREFIX)/bin/$(BINARY)"
