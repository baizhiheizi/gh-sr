---
title: "Commands"
weight: 50
---

# Commands

Running **`gh sr` with no subcommand** opens the interactive dashboard on a **TTY** (same as `gh sr dashboard`). If stdout is not a terminal (for example in a pipe or CI), gh sr prints a short hint to stderr and exits successfully; use `gh sr status` or `gh sr --help` in those environments.

```bash
gh sr                      # Open dashboard (TTY only; see above)
gh sr init [--force]       # Create ~/.gh-sr with template runners.yml and env file
gh sr doctor [--strict] [--fix]    # Check config, GitHub API, and host prerequisites
gh sr setup [names...]     # Install runner binary and configure on hosts
gh sr up [names...]        # Start runners
gh sr down [names...]      # Stop runners
gh sr restart [names...]   # Stop then start
gh sr status               # Show status table
gh sr hosts                # Show host resource usage (CPU, memory, disk, load, uptime)
gh sr logs <name>          # Show recent logs from a runner
gh sr cleanup              # Remove offline/ghost runners from GitHub
gh sr disk                 # Show per-instance disk usage under ~/.gh-sr/runners
gh sr disk prune           # Reclaim disk on idle runners (use --yes to execute)
gh sr disk schedule install   # Daily automatic disk prune on this machine
gh sr update [names...]    # Update runner binary (remove + setup + start)
gh sr rebuild [names...]   # Rebuild container runner image and restart containers (container-mode only)
gh sr service install [--system] [names...]   # Native: OS autostart (systemd / launchd / task)
gh sr service uninstall [names...]            # Remove gh sr-installed autostart
gh sr service status [names...]               # Autostart + unit state (container: policy note)
gh sr service cleanup [--dry-run]             # Remove orphan autostart units + runner dirs not in runners.yml
gh sr cache status         # Cache server state, image, health, storage per host
gh sr cache deploy         # Deploy (or start) the cache server container on hosts
gh sr cache prune          # Delete all cache entries via the management API (best-effort)
gh sr cache remove [--purge-data]  # Stop and remove the cache server container
gh sr registry status      # Mirror state, image, health, storage per host
gh sr registry deploy      # Start the pull-through mirror (idempotent)
gh sr registry remove [--purge-data]  # Stop and remove the gh-sr-registry container
gh sr config path          # Print resolved config and ~/.gh-sr/env paths
gh sr config show          # Print resolved configuration (token source summarized)
gh sr config edit          # Edit resolved runners.yml in $VISUAL / $EDITOR
gh sr config edit-env      # Edit ~/.gh-sr/env in $VISUAL / $EDITOR
gh sr config validate      # Validate config (exit 0 if OK)
gh sr dashboard            # Same as bare gh sr: launch interactive TUI dashboard
gh sr version              # Print gh sr version
```

Use `gh sr add host <name> <addr>` and `gh sr add runner <name>` to append hosts and runners to your resolved `runners.yml` from the CLI; see [Add host / runner](#add-host--runner) below for flags and examples.

The dashboard includes live status, per-runner actions (setup, up, down, restart, update, logs), global actions (doctor, cleanup, show/validate/edit config and env), and host/repo filters. Press `h` to open the host metrics panel (CPU, memory, disk, load, uptime) or `?` for the full key map.

**Backward compatibility:** older releases treated `gh sr config` as “print configuration”. That is now `gh sr config show`; plain `gh sr config` lists subcommands. Older scripts that expected bare `gh sr` to print usage should use `gh sr --help` instead.

## Filters

All commands accept `--host` and `--repo` flags:

```bash
gh sr doctor --host mac-mini
gh sr status --host mac-mini
gh sr logs myapp-1 --host win-pc   # disambiguate when the same instance name exists on multiple hosts
gh sr up --repo an-lee/enjoy
gh sr down enjoy-win-1
```

## Doctor

`gh sr doctor` validates local paths, configuration, GitHub API access, and host prerequisites. Checks are scoped to runners matching `--host` / `--repo` filters:

- **Native** (`runner_mode: native`): curl/tar (or PowerShell on Windows), runner install dirs, Linux passwordless sudo when needed for setup.
- **Container** (`runner_mode: container` or `profile: agentic` on Linux): host Docker and `--privileged` support, each `gh-sr-<instance>` container, inner `dockerd`, registration, and (for agentic) inner AWF/DNS/hygiene. On non-Linux hosts, doctor reports a FAIL if container-mode runners are in scope.

By default only **FAIL** lines produce a non-zero exit. Use **`--strict`** to fail on **WARN** as well (useful in CI). Use **`--fix`** to re-run `gh sr setup` when doctor reports failures, then always re-run doctor; the final exit code reflects the post-fix result.

## Quick start sequence

```bash
# 0. Verify config, GitHub access, and hosts (optional but recommended)
gh sr doctor

# 1. Set up runners on all hosts
gh sr setup

# 2. Start everything
gh sr up

# 3. Check status
gh sr status

# 4. Launch dashboard for live monitoring (or run `gh sr` alone on a TTY)
gh sr
# gh sr dashboard   # equivalent
```

For what each lifecycle command does on the host, see [Architecture — Lifecycle commands](architecture.md#lifecycle-commands-what-they-do).

## Add host / runner

The `gh sr add` subcommands append a new entry to your resolved `runners.yml` so you do not have to edit YAML by hand. Both subcommands resolve the config via `gh sr --config` (default `~/.gh-sr/runners.yml`), append the entry, and rewrite the file in place — comments and other entries are preserved, and the file is re-serialized in canonical YAML style — then print a confirmation line to stdout.

```bash
gh sr add host vps-1 user@vps.example             # name + user@host (port optional)
gh sr add host mac-mini user@192.168.1.50:2222    # custom SSH port
gh sr add host win-pc user@10.0.0.5                # Windows hosts work the same way

gh sr add runner backend-1 --repo owner/repo --host vps-1
gh sr add runner backend-1 --repo owner/repo --host vps-1 --count 3
gh sr add runner backend-1 --repo owner/repo --host vps-1 --labels linux,x64,docker
gh sr add runner backend-1 --repo owner/repo --host vps-1 --ephemeral
gh sr add runner org-runner --org my-org --group my-group --host vps-1   # org-level + group
gh sr add runner aw-runner --repo owner/repo --host vps-1 --profile agentic
```

**Flags — `gh sr add host <name> <addr>`:** positional `<name>` and `<addr>` only. The addr accepts `user@host`, `user@host:port`, `host`, or `host:port`. `os` and `arch` are auto-detected over SSH on the next `setup` / `up` / `doctor` run.

**Flags — `gh sr add runner <name>`:**

- `--repo owner/repo` *or* `--org my-org` — exactly one is required; the runner is scoped to that target.
- `--host <name>` — required, must match an existing host entry.
- `--count <n>` — number of parallel instances, registered as `<name>-1` … `<name>-N` (default 1).
- `--labels a,b,c` — GitHub runner labels; auto-generated from the host OS/arch when omitted.
- `--ephemeral` — register as a single-job runner that GitHub deregisters after one job.
- `--group <name>` — assign the runner to a runner group (org-level runners only).
- `--profile agentic` — register as a GitHub Agentic Workflows runner; always runs in container (DinD) mode.

After either subcommand, run `gh sr doctor && gh sr setup && gh sr up` to roll the new host/runner out. For background and field-by-field reference, see the [Organization runners guide](https://an-lee.github.io/gh-sr/guides/org-runners/) (for `--org` / `--group`) and [Host setup](host-setup.md) (for the full `host:` / `runner:` schema).

## Host metrics

**`gh sr hosts`** collects and displays real-time resource usage from every configured host over SSH. All hosts are queried **concurrently**, so the command completes in roughly the time it takes to collect metrics from the slowest single host regardless of how many hosts you have configured:

```
HOST         CPU%   MEM              DISK              LOAD             UPTIME
──────────────────────────────────────────────────────────────────────────────
mac-mini      12%   3.2 / 16.0 GiB   45 / 500 GiB     0.42 0.38 0.31  5 days
linux-vps     55%   1.8 /  4.0 GiB    8 /  80 GiB     1.20 0.95 0.80  12 days
win-pc         8%   6.1 / 32.0 GiB   80 / 500 GiB     -               3 days
```

The same panel is available interactively in `gh sr dashboard` by pressing `h`.

Use `--host` to limit output to a single host:

```bash
gh sr hosts --host mac-mini
```

> **Note:** Load averages are not available on Windows and show as `-`.

## Disk usage and cleanup

Runner workspaces under `~/.gh-sr/runners/<instance>` accumulate job checkouts (`_work`), temp files (`_temp`), and — for container/agentic runners — a persistent inner Docker cache (`docker-data`). Per-job hooks wipe runtime state inside the container but **never** prune the image cache, so disk use can grow large over time. On Linux hosts the `gh sr disk usage` table also includes host-level rows for the optional services and shared caches: `gh-sr-cache` (local Actions cache server storage), one row per configured registry mirror (`gh-sr-registry`, plus `gh-sr-registry-<name>` for each extra `mirrors:` entry — each against its own storage directory), and one `volume` row per `gh-sr-*` Docker volume (the `shared_caches` volumes — npm, `~/.cache`, gradle, pub, mise, android-sdk — plus any older gh-sr volumes). Volume sizes are measured through a throwaway `busybox` container (the data under `/var/lib/docker/volumes` is only readable via the daemon; the busybox pull itself is served by the Docker Hub mirror), so watching these rows grow is the local proxy for how many WAN bytes the shared caches are saving. If that busybox helper cannot be pulled or run, the volume rows report the failure instead of claiming the volumes are empty.

```bash
gh sr disk usage                    # breakdown per host/instance
gh sr disk usage --host my-linux    # filter by host
gh sr disk prune --dry-run          # preview reclaim (default without --yes)
gh sr disk prune --yes              # clear _work/_temp on idle runners (keeps docker-data cache)
gh sr disk prune --yes --prune-cache # also reclaim inner Docker cache (slower next job)
gh sr disk prune --yes --include-orphans   # also remove dirs not in runners.yml
gh sr disk schedule install         # daily prune at 03:00 on this machine
gh sr disk schedule status
gh sr disk schedule uninstall
```

**Notes:**

- Busy runners (active job on GitHub) are always skipped.
- Default prune keeps inner Docker cache (`docker-data`) so the next agentic job does not re-pull gh-aw images. Use `--prune-cache` when you need maximum disk recovery.
- `gh sr cleanup` removes **offline GitHub registrations** only — it does not free disk. Use `gh sr disk prune` for workspace cleanup.
- `gh sr doctor` warns when any instance directory exceeds 50 GiB (including orphan dirs).
- `gh sr disk schedule install` runs on **this machine** (where you run gh), not on runner hosts. Ensure `gh` is on PATH, `gh auth login` is done, and `~/.gh-sr/env` contains any tokens needed for remote hosts. On Linux headless servers, run `loginctl enable-linger $USER` so the systemd user timer runs without an interactive login. SSH keys for remote hosts must be available to the scheduled job's user environment.

## Cache server

When `cache.enabled` is not `false` in `runners.yml`, `gh sr` deploys a per-host local GitHub Actions cache server (`falcondev-oss/github-actions-cache-server`, one `gh-sr-cache` container per Linux host) so container runners serve `actions/cache` traffic from the host instead of GitHub's shared cache service. Artifacts and other non-cache traffic still pass through to GitHub, so safe-outputs and cross-host artifact transfer keep working.

The server is deployed automatically before container runner setup/start; the `cache` subcommands exist to inspect it, reclaim storage, or uninstall it:

```bash
gh sr cache status             # State, image, health probe, du per host
gh sr cache deploy             # Start the cache server (idempotent)
gh sr cache prune              # Delete all cache entries via the management API (best-effort)
gh sr cache remove             # Stop and remove the gh-sr-cache container
gh sr cache remove --purge-data  # Also delete the storage directory (all cached data + generated management key)
```

**Notes:**

- `gh sr remove <name>` (per-runner removal) **never** deletes the cache container. `gh sr cache remove` is the only uninstall path.
- `--purge-data` deletes the storage directory and the generated management API key. Run this **before** re-deploying with a fresh key, or when retiring a host entirely.
- The cache server runs on Linux only (Windows/macOS container runners fall back to GitHub's shared cache automatically; see [Configuration — `cache:` section](configuration.md#cache--per-host-local-actions-cache-server)).
- Tune the server with `cache.port`, `cache.bind_addr`, `cache.retention_days`, `cache.max_size_bytes`, `cache.max_usage_percent`, and `cache.url_override` in `runners.yml`. `cache.image` lets you pin a specific tag or digest for reproducible deploys.

## Registry mirror

When `registry_mirror.enabled` is not `false` in `runners.yml`, `gh sr` deploys a per-host Docker registry pull-through mirror (`registry:2`, one `gh-sr-registry` container per Linux host). Container runners' inner dockerd pulls Docker Hub images through it, so every runner on the host shares one copy of each image layer instead of each downloading it over the WAN. Only Docker Hub is redirected transparently (dockerd's `registry-mirrors` mechanism has no per-registry equivalent for `ghcr.io`).

The mirror is deployed automatically before container runner setup/start; the `registry` subcommands exist to inspect or uninstall it:

```bash
gh sr registry status             # State, image, health probe, storage du per host
gh sr registry deploy             # Start the mirror (idempotent)
gh sr registry remove             # Stop and remove the gh-sr-registry container
gh sr registry remove --purge-data  # Also delete the storage directory (all mirrored layers)
```

**Notes:**

- `gh sr remove <name>` (per-runner removal) **never** deletes the mirror container. `gh sr registry remove` is the only uninstall path.
- Wiring is baked at container-create time (`GH_SR_REGISTRY_MIRROR` env → inner dockerd `daemon.json` `registry-mirrors`): run `gh sr rebuild <name>` after enabling the mirror so existing runner containers pick it up; `gh sr doctor` flags containers that predate the mirror.
- The mirror runs on Linux only. See [Configuration — `registry_mirror:` section](configuration.md#registry_mirror--per-host-docker-registry-pull-through-mirror) and the [registry mirror guide](guides/registry-mirror.md).

## Rebuild (container-mode runners)

`gh sr rebuild [names...]` rebuilds the `gh-sr/agentic-runner` Docker image from the embedded sources (the same image for every `runner_mode: container` runner, agentic or not), recreates the runner containers, and starts them. The image embeds the global `container_runner_image.extra_apt_packages` and `container_runner_image.toolcache` from your config (see the [agentic workflows guide](https://an-lee.github.io/gh-sr/guides/agentic-workflows/)).

```bash
gh sr rebuild                # Rebuild every container-mode runner
gh sr rebuild backend-1      # Rebuild a single runner
```

**When to run it:**

- You changed `container_runner_image.base_image`, the tag, or any of the embedded image sources — `doctor` flags a stale image fingerprint.
- You added or changed `container_runner_image.extra_apt_packages` / `container_runner_image.toolcache` in `runners.yml`.

**Notes:**

- Runner state (the `.runner` registration file, work directories, and Docker layer cache inside the container) is preserved across the rebuild, so runners stay registered with GitHub and do not consume a new registration token.
- `runner_mode: native` runners are skipped with a per-runner notice (no error).
- Changes that affect the container runtime (MTU, dockerd start timeout, bootstrap retry count, ...) take effect at container-create time, so a rebuild is required to pick them up.
- With `registry_mirror.prefetch` configured, the rebuild is followed by an automatic `gh sr images sync` pass — the fresh containers' inner image stores are repopulated from one host-side pull per image.

**What survives a rebuild (and what does not):**

| State | Survives? | Where it lives |
|---|---|---|
| GitHub registration (`.runner`) | ✅ | `runner-state` bind mount |
| Inner Docker layer cache (job images) | ✅ | `runner-state/docker-data` bind mount |
| Job workspaces (`_work`, `_temp`) | ✅ | `runner-state` bind mount |
| Shared caches: npm, `~/.cache`, Gradle, pub, mise, Android SDK (`shared_caches`) | ✅ | Docker named volumes (`gh-sr-npm-cache`, …) — shared by every runner on the host |
| Prefetch images (gh-aw stack, …) | ✅ reloaded | one host-side pull per image, `docker load`-ed into each runner (see [registry mirrors](guides/registry-mirror.md)) |
| `container_runner_image.toolcache` bake | ✅ | baked into the new image; sources cached on the host (below) |
| Toolcache installs beyond the bake (runtime `setup-*` downloads) | ❌ | container writable layer — re-downloaded by the first job that needs them |
| Anything else in the container's writable layer | ❌ | lost by design (that is what a rebuild means) |

**Rebuild cost on a metered WAN:** the toolcache bake is a single `RUN` layer, so any list edit, base-image bump, or builder-cache prune re-runs the whole loop. gh-sr therefore keeps a content-addressed copy of every `toolcache` tarball in `~/.gh-sr/toolcache-src/<sha256(url)[:16]>` on each host: a rebuild downloads each URL **once, ever**, and every later build reads the cached blob (the bake falls back to a direct download only when the pre-sync failed). `rm -rf ~/.gh-sr/toolcache-src` reclaims the space.

