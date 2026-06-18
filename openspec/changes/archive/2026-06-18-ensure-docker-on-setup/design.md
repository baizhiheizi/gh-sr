## Context

Container-mode setup (`internal/runner/container.go` → `setupContainer`) assumes Docker is already installed and accessible. Doctor checks (`agentic.ValidateContainerPrereqs`) validate `docker --version`, `docker info`, and `--privileged` support, but setup does not call them or auto-remediate.

Documentation in `docs/content/host-setup.md` already describes Docker auto-install via get.docker.com and the `sudo -n` privilege model, but the implementation is missing.

Native setup (`setupNative`) already bootstraps missing tools (curl/tar via apt/yum/apk with `sudoPrelude()`). This change applies the same pattern for Docker on Linux container-mode hosts.

**Option B (chosen):** After a fresh Docker install, stop setup early and tell the user to re-run `gh sr setup`. A new gh-sr invocation opens fresh SSH sessions where the SSH user's updated `docker` group membership is active. No `sudo docker` fallback for ongoing operations.

## Goals / Non-Goals

**Goals:**

- Detect missing Docker before any `docker build` / `docker create` commands run
- Auto-install Docker on Linux via `curl -fsSL https://get.docker.com | sh` when CLI is absent
- Add SSH user to `docker` group after install; exit with clear re-run instructions (Option B)
- Start stopped daemon via `systemctl enable --now docker` when CLI exists but daemon is down
- Reuse existing `sudoPrelude()` / `host.SSHUser()` conventions
- Keep behavior scoped to Linux + container mode (`runner_mode: container`, `profile: agentic`)

**Non-Goals:**

- Auto-install Docker on macOS or Windows
- Upgrade existing Docker installations (get.docker.com is install-only; do not re-run if CLI present)
- `sudo docker` fallback for setup or runtime commands
- Installing Sysbox or configuring rootless Docker
- Changing inner DinD behavior inside the runner image

## Decisions

### 1. Hook point: start of `setupContainer`

**Decision:** Call `EnsureHostDocker(h, w)` as the first step in `setupContainer`, before runner version resolution or image build.

**Rationale:** Single entry point for all container-mode setup paths (`Setup`, `EnsureSetup`, `RebuildImage` partial paths). Matches "prerequisite before work" ordering.

**Alternative considered:** Call from `ops.Setup` per-host before any runner setup — rejected because `EnsureSetup` during `gh sr up` also needs the check and `setupContainer` is the natural container-mode gate.

### 2. Detection: two-step check

**Decision:**

1. `docker --version 2>/dev/null` — must contain `Docker version`
2. `docker info >/dev/null 2>&1` — daemon reachable with current user's permissions

**Rationale:** Aligns with `ValidateContainerPrereqs`. Catches CLI-missing vs daemon-down vs permission-denied as distinct cases.

**Alternative considered:** Reuse `host.DetectDockerAvailable()` only — rejected because it uses `docker info` alone and doesn't distinguish "not installed" from "permission denied" for remediation messaging.

### 3. Install script: get.docker.com

**Decision:** `curl -fsSL https://get.docker.com | $SUDO sh` after `sudoPrelude()` and curl bootstrap.

**Rationale:** Already documented in host-setup.md; distro-agnostic; matches user request.

**Alternative considered:** `apt-get install docker.io` — rejected as less portable and inconsistent with docs.

### 4. Post-install UX: Option B (stop + re-run)

**Decision:** After fresh install + `usermod -aG docker <ssh-user>` + `systemctl enable --now docker`, return a sentinel error (e.g. `ErrDockerGroupPending`) that causes setup to exit **before** image build with message:

```
Docker installed and <user> added to the docker group.
Re-run: gh sr setup [<runner-names...>]
```

**Rationale:** Avoids mid-setup permission-denied failures. Re-running gh-sr from the laptop opens new SSH sessions with updated group membership — user does not need manual SSH reconnect.

**Alternative considered:** Continue with `sudo docker` — rejected (user chose Option B).

### 5. Do not re-run install script when CLI exists

**Decision:** If `docker --version` succeeds, never invoke get.docker.com — only attempt daemon start (`systemctl enable --now docker`) if `docker info` fails.

**Rationale:** get.docker.com explicitly warns against using it to upgrade existing installs.

### 6. SSH user for usermod

**Decision:** Use `h.SSHUser()` from `hosts.*.addr`. If empty (bare hostname / local), skip usermod and rely on root or existing group membership; document in error if still permission-denied.

**Rationale:** Matches how gh-sr identifies the remote login user elsewhere.

### 7. New file: `internal/runner/docker.go`

**Decision:** Implement `EnsureHostDocker(h *host.Host, w io.Writer) error` in a dedicated file with testable outcome types.

**Rationale:** Keeps `container.go` focused; mirrors separation of native/container concerns; easy to unit test with mock executor.

### 8. Doctor remediation alignment

**Decision:** Update `ValidateContainerPrereqs` docker-cli failure remediation to mention `gh sr setup` will auto-install on Linux, and that a second run may be needed after group membership.

**Rationale:** Keeps doctor and setup messaging consistent; doctor remains read-only (no auto-install during doctor).

## Risks / Trade-offs

| Risk | Mitigation |
|---|---|
| Fresh install requires two setup runs | Clear, early exit message; docs updated |
| get.docker.com needs curl | Bootstrap curl/tar like native setup before install |
| Install requires sudo -n | Existing `sudoPrelude()` fails fast with doctor hint |
| get.docker.com not for production | Documented trade-off; same as current docs promise |
| Parallel hosts: N/A per host | Within-host runners are sequential in `runPerHostParallel` |
| Root SSH: usermod unnecessary | Skip usermod when `id -u` is 0 or SSHUser empty |
| Install succeeds but privileged check still fails later | Unchanged — doctor/setup already fail on `--privileged`; out of scope |

## Migration Plan

- No config migration required
- Existing hosts with Docker installed: no behavior change (detection passes, proceed normally)
- Hosts without Docker: first `gh sr setup` installs; second completes setup
- Rollback: revert code; manually installed Docker hosts unaffected

## Open Questions

- **Exit code on group-pending:** Exit 0 with message vs exit 1 with message? Recommend exit 0 (install succeeded; user action is expected) — decide during implementation.
- **Localized output:** English-only messages match existing gh-sr convention — no i18n needed.
