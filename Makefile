# ghr — GitHub Actions runner manager CLI
# https://github.com/an-lee/ghr

ifeq ($(OS),Windows_NT)
BINARY := ghr.exe
else
BINARY := ghr
endif

CMD_DIR := ./cmd/ghr

PREFIX ?= /usr/local

.PHONY: all build test vet check clean install

all: build

build:
	go build -o $(BINARY) $(CMD_DIR)/

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
