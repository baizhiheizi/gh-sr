# gh-sr — self-hosted GitHub Actions runners (GitHub CLI extension)
# https://github.com/an-lee/gh-sr
#
# Development: `make install` builds the binary and runs `gh extension install`
# so `gh sr` uses the executable in this repository (see `gh extension install --help`).

ifeq ($(OS),Windows_NT)
BINARY := gh-sr.exe
else
BINARY := gh-sr
endif

CMD_DIR := ./cmd/gh-sr

GIT_TAG := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: all build test bench vet fmt tidy ci check clean install uninstall

all: build

build:
	go build -ldflags "-X main.version=$(GIT_TAG)" -o $(BINARY) $(CMD_DIR)/

test:
	go test ./... -race -count=1

bench:
	go test ./... -run='^$$' -bench=. -benchmem -count=3

vet:
	go vet ./...

# fmt lists gofmt-clean violations without mutating files (mirrors the
# CI "Format" step in .github/workflows/ci.yml). Use `gofmt -w` to apply.
fmt:
	@output=$$(gofmt -l .); \
	if [ -n "$$output" ]; then \
		echo "The following files are not gofmt-clean:"; \
		echo "$$output"; \
		exit 1; \
	fi

tidy:
	go mod tidy

# ci is the local equivalent of .github/workflows/ci.yml's test job so
# contributors can verify the green-CI surface before pushing.
ci: vet fmt test

check: ci

clean:
	rm -f $(BINARY)

# Build, then register this checkout with GitHub CLI (symlink under ~/.local/share/gh/extensions).
# Requires `gh` on PATH. Re-run after cloning; rebuilds pick up without reinstalling.
install: build
	gh extension install --force .

uninstall:
	gh extension remove gh-sr
