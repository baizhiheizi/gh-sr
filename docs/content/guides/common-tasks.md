---
title: "Common Tasks"
weight: 20
---

# Common tasks

## Add a new host

1. Add an entry under `hosts` in your resolved config file (`~/.gh-sr/runners.yml` by default)
2. Ensure SSH key-based access works: `ssh user@host true`
3. Add runner entries referencing the new host
4. Run `gh sr setup && gh sr up`

## Scale up

Change `count` in your runners YAML, then:

```bash
gh sr setup   # configures new instances
gh sr up      # starts them
```

## Update runner version

```bash
gh sr update
```

This removes existing runners, downloads the latest runner binary, reconfigures, and starts them.

## Clean up ghost runners

```bash
gh sr cleanup
```
