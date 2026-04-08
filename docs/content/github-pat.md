---
title: "Authentication"
weight: 30
---

# Authentication

**gh wm** needs a GitHub token to manage self-hosted runners via the REST API. It tries multiple sources in this order:

1. **`github.pat`** in `runners.yml` (supports `env:VAR_NAME` to read from environment)
2. **`GITHUB_PAT`** or **`GITHUB_TOKEN`** environment variable
3. **`gh` CLI** — if you have the [GitHub CLI](https://cli.github.com/) installed and authenticated, gh wm reads its stored token automatically

The simplest setup is `gh auth login` — no env files or PAT fields needed.

## Required permissions

The GitHub user or token must have **admin** access to every repository listed under `runners[].repo` in your config; the REST API requires that for self-hosted runner management.

**gh wm** uses the token for:

| Operation | REST (summary) | Used for |
| --- | --- | --- |
| Start / register runners | `POST /repos/{owner}/{repo}/actions/runners/registration-token` | Native and Docker runner startup |
| Stop / remove runners (native) | `POST .../actions/runners/remove-token` | Native runner removal |
| Dashboard / status | `GET .../actions/runners` | Match runner names; online / offline / busy |
| `gh wm cleanup` | `DELETE .../actions/runners/{runner_id}` | Remove offline runners from GitHub |

Fetching the latest runner package version uses the public `actions/runner` releases API and does not require extra token permissions beyond a valid request.

## Option 1: gh CLI (easiest)

Install [gh](https://cli.github.com/) and log in:

```bash
gh auth login
```

That is it. **gh wm** reads the token from gh's config automatically. This also handles token refresh and supports GitHub Enterprise Server (`gh auth login --hostname enterprise.example.com`).

Run `gh wm doctor` to verify the token was found:

```
OK    [local       ] GitHub token: from gh CLI (gh auth login)
```

## Option 2: Fine-grained personal access token (recommended for CI/automation)

1. Go to **GitHub → Settings → Developer settings → Personal access tokens → Fine-grained tokens**.
2. Under **Repository access**, include every `owner/repo` you configure in `runners`.
3. Under **Permissions → Repository permissions**, set **Administration** to **Read and write**. That level covers listing runners, creating registration and removal tokens, and deleting runners, as defined in GitHub's [repository permissions for "Administration"](https://docs.github.com/en/rest/authentication/permissions-required-for-fine-grained-personal-access-tokens#repository-permissions-for-administration). See also [REST API endpoints for self-hosted runners](https://docs.github.com/en/rest/actions/self-hosted-runners).

Then provide the token to gh wm via one of:

**`~/.gh-wm/env` file** (keeps secrets out of the YAML):

```bash
# ~/.gh-wm/env
GITHUB_PAT=github_pat_...
```

Pair with `github.pat: env:GITHUB_PAT` in `runners.yml`.

**Shell environment:**

```bash
export GITHUB_PAT=github_pat_...
```

## Option 3: Personal access token (classic)

If you use a **classic** token instead, GitHub documents the **`repo`** scope for repository-level runner endpoints (for example [create a registration token for a repository](https://docs.github.com/en/rest/actions/self-hosted-runners#create-a-registration-token-for-a-repository)).

If you see **403** responses or an empty registration token, confirm **Administration** read/write on a fine-grained token (and that the token includes the target repositories), or the **`repo`** scope on a classic token.

## Troubleshooting

Run `gh wm doctor` to check which token source is active and verify API access. The output shows:

```
OK    [local       ] GitHub token: from PAT (config or environment)
```

or

```
OK    [local       ] GitHub token: from gh CLI (gh auth login)
```

If no token is found, you will see:

```
FAIL  [local       ] GitHub token: not found; set github.pat in runners.yml, export GITHUB_PAT, or run `gh auth login`
```
