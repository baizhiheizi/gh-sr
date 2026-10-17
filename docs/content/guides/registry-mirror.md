# Registry Pull-Through Mirror

gh-sr can deploy a Docker registry pull-through mirror on every Linux host, so container runners' inner dockerd pulls Docker Hub images from the host instead of every runner independently downloading the same layers over the WAN.

The server is the stock [`registry:2`](https://docs.docker.com/registry/deploying/) image with `REGISTRY_PROXY_REMOTEURL` — a pull-through cache: the first pull of a tag fetches it from Docker Hub and stores it; every later pull is served from host disk at LAN speed.

## Topology

- **One `gh-sr-registry` container per Linux host** (plus one `gh-sr-registry-<name>` per extra `mirrors:` entry), shared by all container-mode runners on that host.
- **Wiring**: the runner container receives `GH_SR_REGISTRY_MIRROR` at `docker create`; the image entrypoint writes it into the inner dockerd's `/etc/docker/daemon.json` (`registry-mirrors`) before the single dockerd start. All Docker Hub pulls then go through the mirror; dockerd falls back to Docker Hub directly when the mirror is unreachable.
- **Scope — Docker Hub only (transparently).** dockerd's `registry-mirrors` mechanism redirects `docker.io` pulls transparently; there is no per-registry equivalent for `ghcr.io` (the gh-aw stack) or other registries — and no, `hosts.toml` does not help: dockerd never reads containerd's host configuration, containerd image store or not (it builds its own registry endpoints; [moby/moby#41456](https://github.com/moby/moby/issues/41456) tracks the upstream gap). Other registries are handled by [prefetch + `gh sr images sync`](#beyond-docker-hub-ghcrio-and-friends) instead: pulled once per host through an extra mirror, then loaded into every runner's inner dockerd.
- **Binding**: like the cache server, each mirror publishes on the **docker0 gateway IP** so only containers on the host can reach it — and additionally on **`127.0.0.1`** so the host's own dockerd can pull through it for prefetch (dockerd treats loopback as an insecure registry, so the plain-HTTP mirror works without any daemon configuration). Hosts with a default-deny INPUT firewall need one allow rule; `gh sr doctor` probes the mirror URL **from inside a runner container** and prints the exact rule when blocked.

## Configuration (runners.yml)

The whole `registry_mirror:` section is optional; a per-host mirror is deployed automatically (`enabled` defaults to `true`) whenever `gh sr setup` / `gh sr up` / `gh sr update` / `gh sr rebuild` run.

```yaml
registry_mirror:
  enabled: true                    # default true; set false to pull from Docker Hub directly
  port: 5577                       # host-side published port (built-in default; override on collision)
  bind_addr: ""                    # host-side bind; empty = docker0 gateway IP + loopback; 0.0.0.0 = all interfaces
  storage_path: ~/.gh-sr/registry  # host directory for mirrored layers ($HOME expansion supported)
  upstream: https://registry-1.docker.io   # registry the mirror pulls through
  image: registry:2                # pin a digest for reproducible deploys
  url_override: http://10.0.0.5:5577       # escape hatch for exotic topologies; must include scheme
```

## Beyond Docker Hub: ghcr.io and friends

dockerd cannot be redirected to a mirror for any registry but Docker Hub, so for the gh-aw stack (which lives on `ghcr.io` and is pulled by **every** runner on a version bump) gh-sr pairs extra pull-through mirrors with an eager host-side sync:

```yaml
registry_mirror:
  mirrors:
    - name: ghcr                        # extra mirror: container gh-sr-registry-ghcr, port 5578
      upstream: https://ghcr.io
      # username: ci-bot                # optional: private GHCR (read:packages PAT)
      # password_env: GHCR_PAT          #   the PAT is read from the env var, never stored here
  prefetch:                             # images to sync once per host into every runner
    - ghcr.io/github/gh-aw-firewall/agent:latest
    - ghcr.io/github/gh-aw-firewall/squid:latest
    - ghcr.io/github/gh-aw-firewall/api-proxy:latest
    - ghcr.io/github/gh-aw-mcpg:latest
```

`gh sr images sync` (also run automatically after `gh sr up` and `gh sr rebuild`) then, per host:

1. **Pulls each prefetch image once** — through the matching mirror when one exists (`127.0.0.1:5578/github/gh-aw-mcpg:latest`, so the fetch is cached in the mirror and survives host image prunes), else directly from the upstream.
2. **Streams it into every runner's inner dockerd** (`docker save | docker exec -i <runner> docker load`), skipping runners that already have the exact image by ID.
3. Reports per-host counts (`pulled` / `loaded` / `already cached in runners`); failures are per-image and never take runners down. There is no host-side "skipped" count on purpose — `docker pull` must contact the registry to learn whether a `:latest` moved, so a host-side pull always costs a manifest request; the skip that saves real work is the per-runner one, matched by image ID.

Each image is therefore fetched from the WAN **at most once per host** — the difference between N× and 1× on every gh-aw version bump. Two caveats: the sync is *eager* (a brand-new tag benefits only after the next sync), and tag references are restored with their tags while bare digest references land their layers without a taggable name — prefer tags in the prefetch list.

The first `mirrors:` deployment recreates the existing `gh-sr-registry` container once (deploy layout v2 adds per-mirror credentials and the loopback publish); mirrored Docker Hub layers in the storage directory are preserved.

## CLI

```bash
gh sr registry status             # per-host mirror: running/healthy, URL, storage usage
gh sr registry deploy             # explicit deploy / upgrade (idempotent)
gh sr registry remove             # stop and remove the mirror containers (runners keep running)
gh sr registry remove --purge-data  # also delete the storage directories (all mirrored layers)
gh sr images sync                 # pull prefetch images once per host, load into runners
```

`gh sr registry remove` is the only uninstall path — removing a runner never removes the mirrors. `gh sr images sync` needs the containers running (`docker exec`); it also runs automatically after `gh sr up` / `gh sr rebuild`.

## Health checks

`gh sr doctor` (on hosts with container-mode runners) verifies the mirror when enabled:

- **FAIL** — the container exists but `/v2/` is not healthy: check `docker logs gh-sr-registry`;
- **FAIL** — host-side health is fine but the URL is **not reachable from a runner container** (default-deny INPUT firewall): the check prints the exact allow rule to run;
- **WARN** — enabled but not deployed: run `gh sr registry deploy`;
- **WARN** — bound to `0.0.0.0`: set `registry_mirror.bind_addr`;
- **WARN** — a prefetch-only mirror (`registry[<name>]`) is not reachable from a runner container (non-fatal — prefetch pulls run on the host);
- **WARN** — a `gh-sr-registry*` container runs on the host but is no longer in `registry_mirror.mirrors` (an entry that was renamed or deleted). `Ensure` and `gh sr registry remove` only walk the configured list, so an orphan keeps serving pulls nobody asked for and keeps its storage dir on disk; remove it with `docker rm -f <name>` (plus `rm -rf` of its storage dir);
- **WARN** — a runner container predates the mirror (no `GH_SR_REGISTRY_MIRROR` env, baked at create): recreate with `gh sr rebuild <name>`.

`gh sr disk usage` reports one row per mirror — the container name against **its own** storage directory (`gh-sr-registry` → `<storage_path>`, `gh-sr-registry-<name>` → `<storage_path>-<name>`), so no mirror's layers are attributed to another mirror's path.

## When this pays off

Every runner container on the host has its own inner Docker image cache (preserved across jobs), so an image is normally downloaded once per runner. The mirror collapses that to **once per host**:

- new pools and pool scale-outs (N runners × the same base images),
- image tag updates across a 12-runner pool (each ~GB-scale image downloads once instead of N times),
- any workflow pulling job service images (`postgres`, `redis`, ...) on runners that haven't seen the tag yet.

On metered WANs (per-GB proxies, LTE/5G uplinks) that is the difference between N× and 1× the bytes for every image refresh.
