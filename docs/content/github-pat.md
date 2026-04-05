---
title: "GitHub PAT"
weight: 30
---

# GitHub personal access token

The GitHub user that creates the token must have **admin** access to every repository listed under `runners[].repo` in your config; the REST API requires that for self-hosted runner management.

**ghr** uses the PAT against the GitHub API for:

| Operation | REST (summary) | Used for |
| --- | --- | --- |
| Start / register runners | `POST /repos/{owner}/{repo}/actions/runners/registration-token` | Native and Docker runner startup |
| Stop / remove runners (native) | `POST .../actions/runners/remove-token` | Native runner removal |
| Dashboard / status | `GET .../actions/runners` | Match runner names; online / offline / busy |
| `ghr cleanup` | `DELETE .../actions/runners/{runner_id}` | Remove offline runners from GitHub |

Fetching the latest runner package version uses the public `actions/runner` releases API and does not require extra token permissions beyond a valid request.

## Fine-grained personal access token (recommended)

1. Go to **GitHub → Settings → Developer settings → Personal access tokens → Fine-grained tokens**.
2. Under **Repository access**, include every `owner/repo` you configure in `runners`.
3. Under **Permissions → Repository permissions**, set **Administration** to **Read and write**. That level covers listing runners, creating registration and removal tokens, and deleting runners, as defined in GitHub’s [repository permissions for “Administration”](https://docs.github.com/en/rest/authentication/permissions-required-for-fine-grained-personal-access-tokens#repository-permissions-for-administration). See also [REST API endpoints for self-hosted runners](https://docs.github.com/en/rest/actions/self-hosted-runners).

## Personal access token (classic)

If you use a **classic** token instead, GitHub documents the **`repo`** scope for repository-level runner endpoints (for example [create a registration token for a repository](https://docs.github.com/en/rest/actions/self-hosted-runners#create-a-registration-token-for-a-repository)).

If you see **403** responses or an empty registration token, confirm **Administration** read/write on a fine-grained token (and that the token includes the target repositories), or the **`repo`** scope on a classic token.

## Set up environment

Either put the token in `~/.ghr/env`:

```bash
# ~/.ghr/env
GITHUB_PAT=github_pat_...
```

Or export it in your shell:

```bash
export GITHUB_PAT=github_pat_...
```

Pair this with `github.pat: env:GITHUB_PAT` in `runners.yml` (see [Configuration](configuration.md)).
