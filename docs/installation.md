# Installation

## Install with Go

```bash
go install github.com/an-lee/ghr/cmd/ghr@latest
```

`go install` does not create config files. After installing, run:

```bash
ghr init
```

This creates `~/.ghr/runners.yml` from a template and `~/.ghr/env` for secrets (optional). Then edit those files (or use `ghr config edit` / `ghr config edit-env`), run `ghr config validate` and `ghr doctor`, and you are ready to use `ghr status` and other commands.

## Build from source

This repository is [an-lee/gh-runners](https://github.com/an-lee/gh-runners); the Go module path is `github.com/an-lee/ghr`.

```bash
git clone https://github.com/an-lee/gh-runners.git
cd gh-runners
go build -o ghr ./cmd/ghr/
```

On Windows without GNU Make, use `go build` as above; Go writes `ghr.exe` when you pass `-o ghr`.

## Makefile (optional)

Requires [GNU Make](https://www.gnu.org/software/make/) and the same Go version as in `go.mod` / CI (see [Prerequisites](#prerequisites)). Works on Linux and macOS, and on Windows in a Unix-like environment (for example WSL2 or MSYS2) where `make`, `rm`, and a POSIX shell are available. The `install` target uses the `install` utility from coreutils and is intended for Unix-like systems only, not plain cmd.exe or PowerShell.

| Target | Description |
|--------|-------------|
| `make` / `make build` | Build `./cmd/ghr` into `ghr` (or `ghr.exe` on Windows when `OS` is `Windows_NT`). |
| `make test` | Run `go test ./... -race -count=1` (same as CI). |
| `make vet` | Run `go vet ./...`. |
| `make check` | Run `vet` then `test`. |
| `make clean` | Remove the built binary in the repo root. |
| `make install` | Install the binary to `$(PREFIX)/bin` (default `PREFIX=/usr/local`). Set `DESTDIR` for staged installs (packaging). |

To use the checked-in example at `config/runners.yml` while hacking on this repo, point the CLI at it explicitly, for example `export GHR_CONFIG="$PWD/config/runners.yml"` or `ghr -c config/runners.yml status`.

## Prerequisites

**On your laptop:**

- Go 1.25+ (for building; matches `go.mod` and CI)
- SSH key-based access to remote runner hosts (not needed for `addr: local` hosts)

**On runner hosts:**

- **Linux** — Docker installed (for `mode: docker`) or just a shell (for `mode: native`)
- **macOS** — `curl` available (pre-installed). For `mode: docker`, install Docker Desktop, OrbStack, or Colima and ensure `docker` works in the same environment as your SSH session (ghr does not auto-install Docker on macOS).

For Linux `sudo`/SSH user setup, OpenSSH on Windows, and per-OS host preparation, see [Host setup](host-setup.md).
