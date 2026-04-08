---
title: "Authentication"
weight: 30
aliases:
  - /github-pat/
---

# Authentication

**gh wm** uses the [GitHub CLI](https://cli.github.com/) for GitHub API authentication only. Install **gh** and sign in:

```bash
gh auth login
```

**gh wm** reads the token from gh's stored credentials (via the same mechanism as other `gh` extensions). This also handles token refresh. For GitHub Enterprise Server, use `gh auth login --hostname enterprise.example.com` (API hostname support in gh wm follows **gh**; the default is `github.com`).

Do not set **`github.pat`** in `runners.yml` or **`GITHUB_PAT`** / **`GITHUB_TOKEN`** for gh wm — those paths are removed. If your config still contains `github.pat`, loading will fail with an error until you remove it.

## Required permissions

The authenticated GitHub user must have **admin** access to every repository listed under `runners[].repo` in your config (and appropriate org permissions for org-level runners). The REST API requires that for self-hosted runner management.

**gh wm** uses the token for:

| Operation | REST (summary) | Used for |
| --- | --- | --- |
| Start / register runners | `POST /repos/{owner}/{repo}/actions/runners/registration-token` | Native and Docker runner startup |
| Stop / remove runners (native) | `POST .../actions/runners/remove-token` | Native runner removal |
| Dashboard / status | `GET .../actions/runners` | Match runner names; online / offline / busy |
| `gh wm cleanup` | `DELETE .../actions/runners/{runner_id}` | Remove offline runners from GitHub |

Fetching the latest runner package version uses the public `actions/runner` releases API and does not require extra token permissions beyond a valid request.

If you see **403** responses or an empty registration token, confirm your account has **Administration** access on the target repositories (see GitHub's [repository permissions for Administration](https://docs.github.com/en/rest/authentication/permissions-required-for-fine-grained-personal-access-tokens#repository-permissions-for-administration) and [self-hosted runners API](https://docs.github.com/en/rest/actions/self-hosted-runners)).

## Verify

Run `gh wm doctor` to confirm a token was found:

```
OK    [local       ] GitHub token: from gh CLI (gh auth login)
```

If gh is not logged in:

```
FAIL  [local       ] GitHub token: not found; run `gh auth login`
```
