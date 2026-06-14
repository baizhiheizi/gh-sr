---
title: "Authentication"
weight: 30
aliases:
  - /github-pat/
---

# Authentication

**gh sr** uses the [GitHub CLI](https://cli.github.com/) for GitHub API authentication only. Install **gh** and sign in:

```bash
gh auth login
```

**gh sr** reads the token from gh's stored credentials (via the same mechanism as other `gh` extensions). This also handles token refresh. For GitHub Enterprise Server, use `gh auth login --hostname enterprise.example.com` (API hostname support in gh sr follows **gh**; the default is `github.com`).

Do not set **`github.pat`** in `runners.yml` or **`GITHUB_PAT`** / **`GITHUB_TOKEN`** for gh sr — those paths are removed. If your config still contains `github.pat`, loading will fail with an error until you remove it.

## Required permissions

The authenticated GitHub user needs permissions matching the runner scope in your config:

| Scope | Who can manage runners |
|---|---|
| **Repo-scoped** (`runners[].repo`) | User with **admin** access to that repository |
| **Org-scoped** (`runners[].org`) | Org **owner**, or token with **`admin:org`** scope (classic PAT), or equivalent fine-grained org permission for self-hosted runners |

See [Organization runners](guides/org-runners.md) for org-level setup and runner groups.

**gh sr** uses the token for:

| Operation | REST (repo-scoped) | REST (org-scoped) | Used for |
| --- | --- | --- | --- |
| Start / register runners | `POST /repos/{owner}/{repo}/actions/runners/registration-token` | `POST /orgs/{org}/actions/runners/registration-token` | Native and container runner startup |
| Stop / remove runners (native) | `POST .../actions/runners/remove-token` | `POST /orgs/{org}/actions/runners/remove-token` | Native runner removal |
| Dashboard / status | `GET .../actions/runners` | `GET /orgs/{org}/actions/runners` | Match runner names; online / offline / busy |
| `gh sr cleanup` | `DELETE .../actions/runners/{runner_id}` | `DELETE /orgs/{org}/actions/runners/{runner_id}` | Remove offline runners from GitHub |

Fetching the latest runner package version uses the public `actions/runner` releases API and does not require extra token permissions beyond a valid request.

### Troubleshooting 403 errors

**Repo-scoped runners:** confirm your account has **Administration** access on the target repositories. See GitHub's [repository permissions for Administration](https://docs.github.com/en/rest/authentication/permissions-required-for-fine-grained-personal-access-tokens#repository-permissions-for-administration).

**Org-scoped runners:** confirm you are an org owner or re-authenticate with `admin:org` scope:

```bash
gh auth login --scopes admin:org
```

`gh sr doctor` checks both repo and org API access and hints when org permissions are missing.

## Verify

Run `gh sr doctor` to confirm a token was found:

```
OK    [local       ] GitHub token: from gh CLI (gh auth login)
```

For org-scoped runners, doctor also reports:

```
OK    [github      ] org my-org: list runners OK (3 registered)
```

If gh is not logged in:

```
FAIL  [local       ] GitHub token: not found; run `gh auth login`
```
