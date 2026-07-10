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

.PHONY: all build test bench bench-save coverage coverage-html vet fmt tidy ci check clean install uninstall

all: build

build:
	go build -ldflags "-X main.version=$(GIT_TAG)" -o $(BINARY) $(CMD_DIR)/

test:
	go test ./... -race -count=1

bench:
	go test ./... -run='^$$' -bench=. -benchmem -count=3

# bench-save runs `make bench` and tees the output to a timestamped file under
# bench-results/. Pair with `scripts/benchstat` to diff two snapshots:
#
#   make bench-save    # → bench-results/bench-<UTC-stamp>.txt (current state)
#   # ...edit code...
#   make bench-save    # → bench-results/bench-<UTC-stamp>.txt (after change)
#   go run scripts/benchstat bench-results/old.txt bench-results/new.txt
#
# Override BENCH_COUNT to change the sample size (default 3, matches `make bench`).
# Override BENCH_RESULTS_DIR to point at a different snapshot directory.
BENCH_COUNT ?= 3
BENCH_RESULTS_DIR ?= bench-results
bench-save:
	@mkdir -p $(BENCH_RESULTS_DIR)
	@stamp=$$(date -u +%Y%m%dT%H%M%SZ); \
	file=$(BENCH_RESULTS_DIR)/bench-$$stamp.txt; \
	echo "Writing benchmark snapshot to $$file"; \
	go test ./... -run='^$$' -bench=. -benchmem -count=$(BENCH_COUNT) | tee $$file

# coverage runs the full test suite with coverage and prints the per-package
# summary plus the project total. Coverage data goes to coverage.out (gitignored
# convention; add it to your local .gitignore if you keep the file around).
# Use `make coverage-html` to open an annotated HTML report in your browser.
COVERAGE_FILE := coverage.out
coverage:
	go test ./... -coverprofile=$(COVERAGE_FILE) -covermode=atomic
	@echo
	@echo "=== Project total ==="
	@go tool cover -func=$(COVERAGE_FILE) | tail -1

coverage-html: coverage
	go tool cover -html=$(COVERAGE_FILE) -o coverage.html
	@echo "Wrote coverage.html — open it in a browser to drill into per-line coverage."

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
