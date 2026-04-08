# ghr

**ghr** is a CLI for managing self-hosted [GitHub Actions](https://docs.github.com/en/actions) runners across many machines from one place—usually your laptop. It connects over **SSH** (or runs **locally** with `addr: local`), installs and starts runners, and gives you a **terminal UI** for day-to-day operations.

**Repository:** [github.com/an-lee/ghr](https://github.com/an-lee/ghr)  
**Go module:** `github.com/an-lee/ghr` (used by `go install` and imports).

## Features

- One set of commands for **Linux**, **macOS**, and **Windows** runners (`setup`, `up`, `down`, `status`, `logs`, `service`, …).
- **Declarative YAML** for hosts and runners; auth via `gh auth login` or PAT.
- **Docker or native** runners per row, with sensible OS defaults and overrides.
- **Multi-host** from a single config; optional **TUI** dashboard (`ghr` or `ghr dashboard` on a TTY).

## Dashboard

```
  ghr dashboard                              [r]efresh  [?]help  [q]uit

  INSTANCE      HOST         REPO                MODE    LOCAL    GITHUB
  ─────────────────────────────────────────────────────────────────────────
  runner-1      mac-mini     org/backend         native  running  online
  runner-2      linux-vps    org/frontend        docker  running  busy
  runner-3      win-pc       org/infra           native  stopped  offline

  [enter] runner actions  [g] global menu  [f] filter  [j/k] navigate
```

## Documentation

**Full documentation (user guide, architecture, config reference):**  
[https://an-lee.github.io/ghr/](https://an-lee.github.io/ghr/)

The Markdown sources live under [`docs/`](docs/) in this repo and are built with [Hugo](https://gohugo.io/) (Hugo Book theme).

## Install

```bash
go install github.com/an-lee/ghr/cmd/ghr@latest
```

Requires a recent **Go** toolchain (see `go.mod` / CI for the exact version).

## Quick start

```bash
gh auth login           # easiest auth — or set GITHUB_PAT in ~/.ghr/env
ghr init --quick        # interactive: prompts for repo + host, auto-detects everything else
ghr up                  # auto-setup + start (all in one)
ghr status
```

Or step by step:

```bash
ghr init                            # create ~/.ghr with template config
ghr add host my-vps root@10.0.0.1  # add a host (os/arch auto-detected)
ghr add runner ci --repo owner/repo --host my-vps  # add a runner (labels auto-generated)
ghr up                              # auto-setup + start
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
# from the repository root:
git clone https://github.com/alex-shpak/hugo-book docs/themes/book --depth 1
cd docs && hugo server
```

Build (also run in CI):

```bash
cd docs && hugo --minify
```
