---
title: "Organization Runners"
weight: 25
---

# Organization-level self-hosted runners

By default, gh-sr examples use **repo-scoped** runners: each runner registers against one `owner/repo` and only that repository's workflows can use it.

**Org-scoped** runners register once against a GitHub organization and can serve workflows from **any repository** in that org (subject to runner group access policies). This is the recommended model when you operate many repos under one org and want a shared CI pool instead of duplicating runner config per repo.

## Repo vs org scope

| | Repo-scoped | Org-scoped |
|---|---|---|
| Config field | `repo: owner/repo` | `org: my-org` |
| Registration URL | `https://github.com/owner/repo` | `https://github.com/my-org` |
| Who can use it | That repo only | Any org repo allowed by the runner group |
| Name uniqueness | Per repository | **Org-wide** |
| Runner groups | Not available | Optional `group:` field |

## Configuration example

```yaml
runners:
  - name: myorg-ci
    org: my-org
    group: ci-pool          # optional; omit for Default group
    host: vps-1
    count: 4
    labels: [self-hosted, Linux, X64]
    runner_mode: container
```

Add via CLI:

```bash
gh sr runner add myorg-ci --org my-org --group ci-pool --host vps-1 --count 4
```

Then `gh sr setup myorg-ci` and `gh sr up myorg-ci`.

## Runner groups

Runner groups control **which repositories** in your org may use a set of runners. They are created in **GitHub org settings** (Settings → Actions → Runner groups), not by gh-sr.

- When `group:` is omitted, the runner joins the org's **Default** group.
- When `group: ci-pool` is set, gh-sr passes `--runnergroup ci-pool` during registration.
- By default, **public repositories cannot use org runners** unless the group policy allows it.
- Org owners configure whether a group is available to all private repos or a selected list.

Create the group in GitHub before referencing it in `runners.yml`.

## Workflow targeting

Workflows in any allowed org repo can target your runners by **labels** or **runner group**:

```yaml
# Match any org runner with these labels
jobs:
  build:
    runs-on: [self-hosted, Linux, X64]

# Match any runner in the group
jobs:
  build:
    runs-on:
      group: ci-pool

# Runner must be in the group AND have all labels
jobs:
  build:
    runs-on:
      group: ci-pool
      labels: [self-hosted, Linux, X64]
```

## Authentication

Org-scoped runners require **organization-level** permissions, not just repo admin:

- Org owner, or
- Classic PAT with `admin:org` scope, or
- Fine-grained PAT with org self-hosted runner permissions

Run `gh auth login` with sufficient scopes. See [Authentication](../authentication.md) for API paths and troubleshooting. `gh sr doctor` reports org API access and hints when permissions are insufficient.

## Security considerations

Org runners have a broader blast radius than repo runners:

- Any repository allowed by the runner group can schedule jobs on your machine.
- Workflows run with access to that repo's secrets (and org secrets where applicable).
- Review runner group policies before allowing public repos or wide org access.
- Consider `ephemeral: true` for org pools where you want one-job-then-deregister behavior.

## Migrating from per-repo to org-scoped runners

You cannot convert an existing repo registration in place — register a new org-scoped runner and retire the old ones.

1. **Create a runner group** in GitHub org settings (recommended).
2. **Add an org-scoped runner** to `runners.yml` (or `gh sr runner add ... --org ...`).
3. **Set up and start** — `gh sr setup <name>` then `gh sr up <name>`.
4. **Update workflows** in org repos to target the org runner labels or group.
5. **Stop old repo runners** — `gh sr down <old-name>` on each per-repo entry.
6. **Clean up GitHub** — `gh sr cleanup` to remove offline repo-scoped registrations.
7. **Remove old config** — delete per-repo runner blocks from `runners.yml`.

Org and repo runners can coexist during transition.

## Status display

`gh sr status` and the TUI show org-scoped targets as `org:<name>` and append `group=<name>` when configured, for example `org:my-org group=ci-pool`.
