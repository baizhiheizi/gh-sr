# Registry Pull-Through Mirror

gh-sr can deploy a Docker registry pull-through mirror on every Linux host, so container runners' inner dockerd pulls Docker Hub images from the host instead of every runner independently downloading the same layers over the WAN.

The server is the stock [`registry:2`](https://docs.docker.com/registry/deploying/) image with `REGISTRY_PROXY_REMOTEURL` — a pull-through cache: the first pull of a tag fetches it from Docker Hub and stores it; every later pull is served from host disk at LAN speed.

## Topology

- **One `gh-sr-registry` container per Linux host**, shared by all container-mode runners on that host.
- **Wiring**: the runner container receives `GH_SR_REGISTRY_MIRROR` at `docker create`; the image entrypoint writes it into the inner dockerd's `/etc/docker/daemon.json` (`registry-mirrors`) before the single dockerd start. All Docker Hub pulls then go through the mirror; dockerd falls back to Docker Hub directly when the mirror is unreachable.
- **Scope — Docker Hub only.** dockerd's `registry-mirrors` mechanism redirects `docker.io` pulls transparently; there is no per-registry equivalent for `ghcr.io` (the gh-aw stack) or other registries. Those images keep pulling directly from their registry — the inner Docker image-layer cache (preserved across jobs and rebuilds) already deduplicates them per runner.
- **Binding**: like the cache server, the mirror publishes on the **docker0 gateway IP** so only containers on the host can reach it. Hosts with a default-deny INPUT firewall need one allow rule; `gh sr doctor` probes the mirror URL **from inside a runner container** and prints the exact rule when blocked.

## Configuration (runners.yml)

The whole `registry_mirror:` section is optional; a per-host mirror is deployed automatically (`enabled` defaults to `true`) whenever `gh sr setup` / `gh sr up` / `gh sr update` / `gh sr rebuild` run.

```yaml
registry_mirror:
  enabled: true                    # default true; set false to pull from Docker Hub directly
  port: 5577                       # host-side published port (built-in default; override on collision)
  bind_addr: ""                    # host-side bind; empty = docker0 gateway IP; 0.0.0.0 = all interfaces
  storage_path: ~/.gh-sr/registry  # host directory for mirrored layers ($HOME expansion supported)
  upstream: https://registry-1.docker.io   # registry the mirror pulls through
  image: registry:2                # pin a digest for reproducible deploys
  url_override: http://10.0.0.5:5577       # escape hatch for exotic topologies; must include scheme
```

## CLI

```bash
gh sr registry status             # per-host: running/healthy, URL, storage usage
gh sr registry deploy             # explicit deploy / upgrade (idempotent)
gh sr registry remove             # stop and remove the mirror container (runners keep running)
gh sr registry remove --purge-data  # also delete the storage directory (all mirrored layers)
```

`gh sr registry remove` is the only uninstall path — removing a runner never removes the mirror.

## Health checks

`gh sr doctor` (on hosts with container-mode runners) verifies the mirror when enabled:

- **FAIL** — the container exists but `/v2/` is not healthy: check `docker logs gh-sr-registry`;
- **FAIL** — host-side health is fine but the URL is **not reachable from a runner container** (default-deny INPUT firewall): the check prints the exact allow rule to run;
- **WARN** — enabled but not deployed: run `gh sr registry deploy`;
- **WARN** — bound to `0.0.0.0`: set `registry_mirror.bind_addr`;
- **WARN** — a runner container predates the mirror (no `GH_SR_REGISTRY_MIRROR` env, baked at create): recreate with `gh sr rebuild <name>`.

`gh sr disk usage` includes the per-host mirror storage directory as a `gh-sr-registry` row.

## When this pays off

Every runner container on the host has its own inner Docker image cache (preserved across jobs), so an image is normally downloaded once per runner. The mirror collapses that to **once per host**:

- new pools and pool scale-outs (N runners × the same base images),
- image tag updates across a 12-runner pool (each ~GB-scale image downloads once instead of N times),
- any workflow pulling job service images (`postgres`, `redis`, ...) on runners that haven't seen the tag yet.

On metered WANs (per-GB proxies, LTE/5G uplinks) that is the difference between N× and 1× the bytes for every image refresh.

## Measuring the savings

Two signals are available without any external traffic tracker:

- **`gh sr disk usage`** — the `gh-sr-registry` row (and `gh-sr-registry-<name>` rows for extra mirrors) is the mirrored-layer total; its growth between two points in time is the WAN traffic the mirror absorbed. The `volume` rows show the shared cache volumes' growth the same way.
- **Mirror access logs** — `docker logs gh-sr-registry-ghcr` prints an access log line per request; after a `gh sr images sync`, every `GET /v2/…` served there (rather than a fresh upstream fetch in `gh sr registry status`'s storage growth) is a pull the runners did not make independently. To verify the "fetched at most once per host" acceptance criterion, clear the mirror's storage (`gh sr registry remove --purge-data` + redeploy), run one sync, and confirm the storage grows by exactly one copy of the prefetch set no matter how many runners the host has.
