---
title: "Installation"
weight: 10
---

# Installation

## Install as a GitHub CLI extension (recommended)

Requires [GitHub CLI](https://cli.github.com/) (`gh`).

```bash
gh extension install an-lee/gh-sr
```

After installing, you invoke the tool as **`gh sr`**. Create config with **`gh sr init`** (see below).

## Install with Go

```bash
go install github.com/an-lee/gh-sr/cmd/gh-sr@latest
```

This installs a binary named **`gh-sr`** on your `PATH`. Use **`gh sr`** by installing the extension as above, or run `gh-sr` directly.

`go install` does not create config files. After installing, run:

```bash
gh sr init
```

This creates `~/.gh-sr/runners.yml` from a template and `~/.gh-sr/env` (optional dotenv for other tooling). Then edit the config (or use `gh sr config edit`), run **`gh auth login`**, then `gh sr doctor`. gh sr uses the GitHub CLI token only; see [Authentication](authentication.md).

## Build from source

This repository is [an-lee/gh-sr](https://github.com/an-lee/gh-sr); the Go module path is `github.com/an-lee/gh-sr`.

```bash
git clone https://github.com/an-lee/gh-sr.git
cd gh-sr
go build -o gh-sr ./cmd/gh-sr/
```

On Windows without GNU Make, use `go build` as above; Go writes `gh-sr.exe` when you pass `-o gh-sr.exe`.

## Makefile (optional)

Requires [GNU Make](https://www.gnu.org/software/make/) and the same Go version as in `go.mod` / CI (see [Prerequisites](#prerequisites)). Works on Linux and macOS, and on Windows in a Unix-like environment (for example WSL2 or MSYS2) where `make`, `rm`, and a POSIX shell are available. The `install` target requires [GitHub CLI](https://cli.github.com/) (`gh`) and runs `gh extension install` so **`gh sr`** uses the binary built in the repository root (see `gh extension install --help`).

| Target | Description |
|--------|-------------|
| `make` / `make build` | Build `./cmd/gh-sr` into `gh-sr` (or `gh-sr.exe` on Windows when `OS` is `Windows_NT`). |
| `make test` | Run `go test ./... -race -count=1` (same as CI). |
| `make vet` | Run `go vet ./...`. |
| `make check` | Run `vet` then `test`. |
| `make clean` | Remove the built binary in the repo root. |
| `make install` | Build, then `gh extension install --force .` (register this repo as the `gh sr` extension). |
| `make uninstall` | `gh extension remove gh-sr`. |

To use the checked-in example at `config/runners.yml` while hacking on this repo, point the CLI at it explicitly, for example `export GH_SR_CONFIG="$PWD/config/runners.yml"` or `gh sr -c config/runners.yml status`.

## Prerequisites

**On your laptop:**

- Go 1.25+ (for building; matches `go.mod` and CI)
- SSH key-based access to remote runner hosts (not needed for `addr: local` hosts)

**On runner hosts:**

- **Linux** — Docker installed (for `mode: docker`) or just a shell (for `mode: native`)
- **macOS** — `curl` available (pre-installed). For `mode: docker`, install Docker Desktop, OrbStack, or Colima and ensure `docker` works in the same environment as your SSH session (gh sr does not auto-install Docker on macOS).

For Linux `sudo`/SSH user setup, OpenSSH on Windows, and per-OS host preparation, see [Host setup](host-setup.md).
