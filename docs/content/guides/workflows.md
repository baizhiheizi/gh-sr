---
title: "Workflows"
weight: 10
---

# Using runners in workflows

Reference runners by label in your workflow files:

```yaml
jobs:
  build-linux:
    runs-on: [self-hosted, Linux, X64]

  build-mac:
    runs-on: [self-hosted, macOS, ARM64]

  build-win:
    runs-on: [self-hosted, Windows, X64]
```

Labels must match what you configure under `runners[].labels` in [Configuration](../configuration.md).

## GitHub Agentic Workflows (gh-aw)

gh sr has first-class support for [GitHub Agentic Workflows](https://github.github.com/gh-aw/). Use `profile: agentic` to configure a runner with all the prerequisites gh-aw needs:

```yaml
runners:
  - name: aw-runner
    repo: owner/repo
    host: vps-1
    profile: agentic
    count: 2
```

This automatically sets docker mode, host networking, `NET_ADMIN` capability, and adds an `agentic` label. See the [host setup docs](../host-setup.md#github-agentic-workflows-gh-aw) for details.

### Native Linux (`mode: native`)

Alternatively, use a **native** Linux runner and add **`agentic`** to `labels` yourself (no `profile: agentic`). The machine still needs **Docker** for gh-aw’s containers; the runner user needs **non-interactive `sudo`** for the Agent Workflow Firewall. See [Native Linux runners and sudo](../host-setup.md#native-linux-runners-and-sudo-gh-aw).

```yaml
runners:
  - name: aw-native
    repo: owner/repo
    host: vps-1
    mode: native
    labels: [self-hosted, Linux, X64, agentic]
```

Reference the runner in your agentic workflow Markdown frontmatter:

```yaml
---
on: issues
runs-on: [self-hosted, Linux, X64, agentic]
engine: copilot
---
Triage this issue.
```

### Organization-level runners

gh-aw supports `runs-on: { group: my-group, labels: [...] }` for targeting runner groups. Register an org-level runner with gh sr:

```yaml
runners:
  - name: org-aw-runner
    org: my-org
    group: my-runner-group
    host: vps-1
    profile: agentic
    count: 4
```

### Ephemeral runners

For security isolation between agentic runs, use `ephemeral: true` so each runner handles one job and then deregisters:

```yaml
runners:
  - name: aw-ephemeral
    repo: owner/repo
    host: vps-1
    profile: agentic
    ephemeral: true
    count: 4
```
