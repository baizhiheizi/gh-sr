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

`profile: agentic` always runs in **container mode** (privileged Docker-in-Docker): each instance is isolated and every job runs from a pristine inner state, so multiple concurrent agentic jobs are safe on one machine. The host only needs Docker (with privileged-container support); gh-aw, AWF, DNS, and tooling are baked into the image `gh sr` builds. Use `count: N` for N concurrent jobs — no MCP port or label juggling. See the [Agentic Workflows guide]({{< ref "agentic-workflows" >}}) and [host setup docs](../host-setup.md#github-agentic-workflows-gh-aw) for details.

> Native-mode agentic runners are no longer supported (`profile: agentic` + `runner_mode: native` is rejected), and the `agentic_mcp_ports` / `gh-sr-mcp-<port>` scheme has been removed — container mode isolates the MCP gateway port per runner automatically.

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
