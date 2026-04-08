---
title: "Introduction"
weight: 2
---

# Introduction

## The Problem

Self-hosted GitHub Actions runners give you more control over your CI/CD infrastructure — but managing them across multiple machines is tedious. To add a new runner you must SSH into each machine, download the runner software, configure it, start the process, and remember which machine has what. Monitoring, updating, and restarting runners means touching every machine individually. As the number of runners and machines grows, this overhead becomes unmanageable.

## What is gh wm?

**gh wm** is a CLI control plane that manages self-hosted GitHub Actions runners on remote machines from a single host — typically your laptop. Instead of SSHing into each machine, you describe your infrastructure in a YAML file and run commands like `gh wm setup`, `gh wm up`, and `gh wm status` to manage everything from one place.

gh wm handles:

- **Installation** — Downloads and configures the GitHub Actions runner software on each host
- **Lifecycle** — Starts, stops, and restarts runner instances (as native processes or Docker containers)
- **Monitoring** — Pulls real-time status and logs from each host and the GitHub API
- **Autostart** — Installs system services so runners survive reboots

## How It Works

```
Your laptop (control plane)          Runner hosts
                                    ──────────────
  gh wm CLI ──────────────────────────►  Mac mini
        │                              Linux VPS
        │                              Windows PC
        │                              localhost
        │
        ▼
  ~/.gh-wm/runners.yml  (your config)
  ~/.gh-wm/env          (GitHub PAT)
```

1. **Config** — You describe hosts and runners in `~/.gh-wm/runners.yml`
2. **Connect** — gh wm reaches each host over SSH (or runs locally with `addr: local`)
3. **Install** — `gh wm setup` downloads the runner software, configures it with your GitHub PAT, and (for Docker mode) pulls the runner image
4. **Manage** — `gh wm up` / `gh wm down` start and stop runner instances as processes or containers
5. **Monitor** — `gh wm status` and `gh wm dashboard` show running state from each host alongside GitHub's online/offline view

```
  gh wm dashboard                              [r]efresh  [?]help  [q]uit

  INSTANCE      HOST         REPO                MODE    LOCAL    GITHUB
  ─────────────────────────────────────────────────────────────────────────
  runner-1      mac-mini     org/backend         native  running  online
  runner-2      linux-vps    org/frontend        docker  running  busy
  runner-3      win-pc       org/infra           native  stopped  offline

  [enter] runner actions  [g] global menu  [f] filter  [j/k] navigate
```

Only the machine where gh wm runs needs the gh wm binary. Target hosts only need SSH access and (for Docker mode) a working Docker installation.

## Key Concepts

### Control plane vs execution plane

gh wm is a **control plane only**. The gh wm CLI runs on your machine and issues commands. The actual runner process or container always runs on the **target host** defined in your config — not on the machine where gh wm executes.

### Hosts

A **host** is a machine (physical or virtual) where runners run. You specify it in `runners.yml` as an address — either `addr: local` to run runners on the same machine as gh wm, or `user@hostname` for an SSH connection.

### Runners

A **runner** is a declared runner instance in your config. Each runner has a name, a target host, an OS type, and a mode. You can run multiple instances of the same runner with `count` — for example, `count: 4` creates `myrunner-1`, `myrunner-2`, `myrunner-3`, and `myrunner-4` on the same host.

### Docker vs native mode

Runners can run as **native** processes or inside **Docker** containers:

- **native** — Runner software installed directly on the host filesystem; process managed by a shell script and PID file
- **docker** — Runner runs in a container (`ghcr.io/actions/actions-runner:latest`); container managed by Docker with `--restart unless-stopped`

Linux defaults to **docker** if you don't specify. Windows and macOS default to **native**.

### Config and secrets

- **`~/.gh-wm/runners.yml`** — Declares all hosts and runners (OS, mode, count, labels, etc.)
- **`~/.gh-wm/env`** — Contains environment variables, most importantly your GitHub PAT (`GHR_TOKEN`)

### Instances

When `count` is greater than 1, gh wm creates multiple instances of a runner on the same host, each with its own directory (native) or container (docker). Instances are named `<runner-name>-1`, `<runner-name>-2`, and so on.
