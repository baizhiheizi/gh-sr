# GitHub Workflow Manager (`gh wm`)

**GitHub Workflow Manager** is a [GitHub CLI](https://cli.github.com/) extension for managing self-hosted [GitHub Actions](https://docs.github.com/en/actions) runners across many machines from one place—usually your laptop. It connects over **SSH** (or runs **locally** with `addr: local`), installs and starts runners, and gives you a **terminal UI** for day-to-day operations.

**Repository:** [github.com/an-lee/gh-wm](https://github.com/an-lee/gh-wm)  
**Go module:** `github.com/an-lee/gh-wm` (used by `go install` and imports).

## Features

- One set of commands for **Linux**, **macOS**, and **Windows** runners (`setup`, `up`, `down`, `status`, `logs`, `service`, …).
- **Declarative YAML** for hosts and runners; auth via `gh auth login` or PAT.
- **Docker or native** runners per row, with sensible OS defaults and overrides.
- **Multi-host** from a single config; optional **TUI** dashboard (`gh wm` or `gh wm dashboard` on a TTY).

## Dashboard

```
  gh wm dashboard                              [r]efresh  [?]help  [q]uit

  INSTANCE      HOST         REPO                MODE    LOCAL    GITHUB
  ─────────────────────────────────────────────────────────────────────────
  runner-1      mac-mini     org/backend         native  running  online
  runner-2      linux-vps    org/frontend        docker  running  busy
  runner-3      win-pc       org/infra           native  stopped  offline

  [enter] runner actions  [g] global menu  [f] filter  [j/k] navigate
```

## Documentation

**Full documentation (user guide, architecture, config reference):**  
[https://an-lee.github.io/gh-wm/](https://an-lee.github.io/gh-wm/)

The Markdown sources live under [`docs/`](docs/) in this repo and are built with [Hugo](https://gohugo.io/) (Hugo Book theme).

## Install

Install as a GitHub CLI extension (recommended):

```bash
gh extension install an-lee/gh-wm
```

Then use **`gh wm`** (for example `gh wm status`). Requires [GitHub CLI](https://cli.github.com/) (`gh`) and a recent `gh` version.

From source with Go:

```bash
go install github.com/an-lee/gh-wm/cmd/gh-wm@latest
```

The resulting binary is named `gh-wm`. To invoke it as **`gh wm`**, install the extension as above, or run `gh-wm` directly if it is on your `PATH`.

Requires a recent **Go** toolchain when building from source (see `go.mod` / CI for the exact version).

### Migrating from the old `ghr` CLI

If you previously used **`ghr`** with config under **`~/.ghr`** and **`GHR_CONFIG`**, move your data and update your environment:

```bash
mv ~/.ghr ~/.gh-wm
# If you used GHR_CONFIG, set GH_WM_CONFIG to the same path instead.
```

## Quick start

```bash
gh auth login           # easiest auth — or set GITHUB_PAT in ~/.gh-wm/env
gh wm init --quick        # interactive: prompts for repo + host, auto-detects everything else
gh wm up                  # auto-setup + start (all in one)
gh wm status
```

Or step by step:

```bash
gh wm init                            # create ~/.gh-wm with template config
gh wm add host my-vps root@10.0.0.1  # add a host (os/arch auto-detected)
gh wm add runner ci --repo owner/repo --host my-vps  # add a runner (labels auto-generated)
gh wm up                              # auto-setup + start
```

Then run `gh wm` on a terminal for the interactive dashboard, or use `gh wm status`, `gh wm logs <name>`, etc.

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
