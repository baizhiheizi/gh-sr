# ghr

**ghr** is a CLI for managing self-hosted [GitHub Actions](https://docs.github.com/en/actions) runners across many machines from one place—usually your laptop. It connects over **SSH** (or runs **locally** with `addr: local`), installs and starts runners, and gives you a **terminal UI** for day-to-day operations.

**Repository:** [github.com/an-lee/ghr](https://github.com/an-lee/ghr)  
**Go module:** `github.com/an-lee/ghr` (used by `go install` and imports).

## Features

- One set of commands for **Linux**, **macOS**, and **Windows** runners (`setup`, `up`, `down`, `status`, `logs`, `service`, …).
- **Declarative YAML** for hosts and runners; secrets in `~/.ghr/env`.
- **Docker or native** runners per row, with sensible OS defaults and overrides.
- **Multi-host** from a single config; optional **TUI** dashboard (`ghr` or `ghr dashboard` on a TTY).

## Documentation

**Full documentation (user guide, architecture, config reference):**  
[https://an-lee.github.io/ghr/](https://an-lee.github.io/ghr/)

The Markdown sources live under [`docs/`](docs/) in this repo and are built with [MkDocs Material](https://squidfunk.github.io/mkdocs-material/).

## Install

```bash
go install github.com/an-lee/ghr/cmd/ghr@latest
```

Requires a recent **Go** toolchain (see `go.mod` / CI for the exact version).

## Quick start

```bash
ghr init
# Edit ~/.ghr/runners.yml and ~/.ghr/env (GitHub PAT — see docs)
ghr doctor
ghr setup
ghr up
ghr status
```

Then run `ghr` on a terminal for the interactive dashboard, or use `ghr status`, `ghr logs <name>`, etc.

## Development

```bash
make check    # vet + tests (same idea as CI)
# or: go vet ./... && go test ./... -race -count=1
```

Repository layout is described in [docs/reference/file-structure.md](docs/reference/file-structure.md).

## Publishing documentation (maintainers)

Documentation deploys to GitHub Pages via **GitHub Actions** when changes land on `main`. In the GitHub repo, enable **Settings → Pages → Build and deployment → Source: GitHub Actions** once (no branch-based `/docs` hosting required for the built site).

Local preview:

```bash
pip install -r docs/requirements.txt
mkdocs serve
```

Strict build (also run in CI):

```bash
mkdocs build --strict
```
