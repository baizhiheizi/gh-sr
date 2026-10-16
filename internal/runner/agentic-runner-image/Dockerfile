# gh-sr container runner image (Docker tag: gh-sr/agentic-runner — historical name)
#
# Used for every runner_mode: container instance, with or without profile: agentic.
# Base image is the falcondev-oss fork of the official actions runner image, which
# backports the CUSTOM_ACTIONS_RESULTS_URL override (local Actions cache) and skips
# runner self-update while it is set. The base ships: Ubuntu 24.04, the runner
# binaries at /home/runner (user `runner`, uid 1001, docker group 123), a full
# static Docker bundle (dockerd/containerd/runc/docker CLI) — but NO gh, Node,
# zstd, or iptables; those are layered below. No ENTRYPOINT: /entrypoint.sh is
# ours.
#
# From gh-aw's point of view the container is a GitHub-hosted-VM equivalent:
# runner FS == inner daemon FS, default bridge network, socket reachable, and the
# default `docker` sandbox profile runs AWF rootless (no sudo, no NET_ADMIN,
# network isolation purely via inner-Docker topology). No DNS shimming, NAT
# rules, or docker-CLI shimming is baked in — inner dockerd's IPAM picks a
# non-colliding bridge subnet automatically, and gh-aw's gateway/AWF pass their
# own --add-host=host.docker.internal:host-gateway.
#
# Build: docker build --build-arg FORK_RUNNER_IMAGE=ghcr.io/falcondev-oss/actions-runner:2.338.0 -t gh-sr/agentic-runner:<version> .
# Run:   see internal/runner/container.go

ARG FORK_RUNNER_IMAGE
FROM ${FORK_RUNNER_IMAGE}

# The fork base ends with `USER runner`; every build step below needs root
# (apt-get, keyrings, chown). Runtime must stay root too: entrypoint.sh starts
# the inner dockerd and drops to the runner user itself via `su -`.
USER root

ENV DEBIAN_FRONTEND=noninteractive

# APT mirror — this is the slowest network path in the whole build.
#
# The fork base ships the stock Ubuntu archive sources, so `apt-get update` +
# `apt-get install` go to archive.ubuntu.com / security.ubuntu.com. Measured from
# a cn-region host (x1, via the WAN proxy) that link sustained only 60–338 KB/s,
# which is 30–100x slower than the CN mirrors the toolcache bake uses (6–11 MB/s).
# The core + extra apt manifests (clang, cmake, gtk-3-dev, gstreamer-*, …) are
# several hundred MB, so this layer — not the 1.6 GB Flutter bake — dominated
# build wall time.
#
# Only the two Ubuntu archive URIs are rewritten, in place, so every other repo
# the base carries survives untouched — notably the git-core PPA
# (ppa.launchpadcontent.net), which has no CN mirror. Both the deb822 form
# (/etc/apt/sources.list.d/*.sources, what Ubuntu 24.04 noble uses) and the
# legacy /etc/apt/sources.list are handled. Passing http:// and https:// forms
# keeps this working if a future base switches one of them to https.
#
# Override with --build-arg APT_MIRROR=... (or an empty value to keep the
# upstream archives). TUNA and Aliyun serve byte-identical Release files for
# noble, so the choice is about which one is closer to the build host.
ARG APT_MIRROR=https://mirrors.tuna.tsinghua.edu.cn/ubuntu/
RUN set -eux; \
    if [ -n "${APT_MIRROR}" ]; then \
        for f in /etc/apt/sources.list /etc/apt/sources.list.d/*.list /etc/apt/sources.list.d/*.sources; do \
            [ -f "$f" ] || continue; \
            sed -i -e "s|http://archive.ubuntu.com/ubuntu/|${APT_MIRROR}|g" \
                   -e "s|http://security.ubuntu.com/ubuntu/|${APT_MIRROR}|g" \
                   -e "s|https://archive.ubuntu.com/ubuntu/|${APT_MIRROR}|g" \
                   -e "s|https://security.ubuntu.com/ubuntu/|${APT_MIRROR}|g" \
                   "$f"; \
        done; \
    fi; \
    grep -rhE '^URIs:|^deb ' /etc/apt/sources.list /etc/apt/sources.list.d/ 2>/dev/null | sort -u

# Core system dependencies (manifest: apt-packages-core.txt). iptables/iproute2
# are what the inner dockerd needs for bridge NAT and route inspection; zstd is
# the Actions-cache compression codec the runner uses for cache upload/download.
COPY apt-packages-core.txt /tmp/apt-packages-core.txt
RUN apt-get update \
    && grep -v '^[[:space:]]*#' /tmp/apt-packages-core.txt \
        | grep -v '^[[:space:]]*$' \
        | sed 's/\r$//' \
        | xargs -r apt-get install -y --no-install-recommends \
    && rm -rf /var/lib/apt/lists/*

# GitHub CLI (gh) — required by gh-aw
RUN curl -fsSL --retry 8 --retry-all-errors --retry-delay 20 --connect-timeout 20 --speed-limit 10240 --speed-time 60 --max-time 300 \
        https://cli.github.com/packages/githubcli-archive-keyring.gpg \
        | dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg \
    && chmod go+r /usr/share/keyrings/githubcli-archive-keyring.gpg \
    && echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] \
             https://cli.github.com/packages stable main" \
        > /etc/apt/sources.list.d/github-cli.list \
    && apt-get update \
    && apt-get install -y --no-install-recommends gh \
    && rm -rf /var/lib/apt/lists/*

# Node.js LTS (NodeSource) — speeds up gh-aw framework steps; the compiler also
# emits an actions/setup-node step for non-standard runners, so this is an
# accelerator, not a hard dependency.
#
# The setup script is fetched to a file and then executed, never piped straight
# into bash: a retrying curl writing into a live bash stdin can replay a partial
# script. Retry flags match the rest of the image's downloads (see below).
RUN curl -fsSL --retry 8 --retry-all-errors --retry-delay 20 --connect-timeout 20 --speed-limit 10240 --speed-time 60 --max-time 300 \
        https://deb.nodesource.com/setup_lts.x -o /tmp/nodesource_setup_lts.sh \
    && bash /tmp/nodesource_setup_lts.sh \
    && rm -f /tmp/nodesource_setup_lts.sh \
    && apt-get install -y --no-install-recommends nodejs \
    && node -v \
    && npm -v \
    && rm -rf /var/lib/apt/lists/*

# Deliberately NO baked browser. Consumer system tests download a pinned
# Chrome-for-Testing build at runtime (matching chromedriver included; runtime
# libs baked via apt-packages-core.txt, baizhiheizi/gh-sr#471). The old
# dl.google.com google-chrome step is gone: nothing consumed the binary anymore,
# and dl.google.com is TLS-reset from this host, which broke every rebuild.

# Docker Compose CLI plugin — gh-aw's rootless sandbox (AWF) starts its squid
# proxy and agent containers via `docker compose`. The fork base's static Docker
# bundle ships buildx but no compose, and the plugin lookup then fails with the
# CLI's "unknown shorthand flag: 'd' in -d". Installed into the system-wide
# plugin dir so the runner user (whom AWF runs as) finds it too. TARGETARCH is
# docker's amd64/arm64; compose release assets use Go arch names (x86_64/aarch64).
#
# This is the ONLY layer that pulls from github.com/.../releases/download, which
# redirects to release-assets.githubusercontent.com — the one GitHub host that
# TLS-resets from some runner hosts (cn-region, x1 as of 2026-10-14:
# "curl: (35) OpenSSL SSL_connect: SSL_ERROR_SYSCALL"). Since every earlier layer
# (ghcr.io, archive.ubuntu.com, cli.github.com, deb.nodesource.com) resolves
# fine, an un-retried curl here aborted an otherwise-successful ~20 min build at
# step 6/19. Same retry policy as the toolcache bake below.
ARG COMPOSE_VERSION=v5.5.0
RUN mkdir -p /usr/local/lib/docker/cli-plugins /tmp/compose \
    && case "${TARGETARCH}" in arm64) _carch=aarch64 ;; *) _carch=x86_64 ;; esac \
    && curl -fsSL --retry 8 --retry-all-errors --retry-delay 20 --connect-timeout 20 --speed-limit 10240 --speed-time 60 --max-time 600 \
        "https://github.com/docker/compose/releases/download/${COMPOSE_VERSION}/docker-compose-linux-${_carch}" \
        -o "/tmp/compose/docker-compose-linux-${_carch}" \
    && curl -fsSL --retry 8 --retry-all-errors --retry-delay 20 --connect-timeout 20 --speed-limit 10240 --speed-time 60 --max-time 300 \
        "https://github.com/docker/compose/releases/download/${COMPOSE_VERSION}/docker-compose-linux-${_carch}.sha256" \
        -o "/tmp/compose/docker-compose-linux-${_carch}.sha256" \
    && (cd /tmp/compose && sha256sum -c "docker-compose-linux-${_carch}.sha256") \
    && install -m 0755 "/tmp/compose/docker-compose-linux-${_carch}" /usr/local/lib/docker/cli-plugins/docker-compose \
    && rm -rf /tmp/compose \
    && docker compose version

# Tool cache — must NOT live under /opt/* because gh-aw's compiled AWF invocation
# (see repo-assist.lock.yml) intentionally SKIPS mounting any RUNNER_TOOL_CACHE
# whose path matches `/opt/*` to avoid leaking the rest of /opt (e.g. /opt/gh-aw
# control-plane files) into the agent container. Using /home/runner/.toolcache
# instead keeps it owned by the runner user, ephemeral per container (same lifetime
# as the prior /opt/hostedtoolcache), and — crucially — outside gh-aw's exclusion,
# so setup-* actions (Node, Go, Flutter, Python, …) land where the AWF agent can
# actually see them.
#
# Compatibility symlink: a handful of legacy actions (notably ruby/setup-ruby@v1)
# hardcode `/opt/hostedtoolcache` and deliberately ignore $RUNNER_TOOL_CACHE when
# they don't detect a self-hosted runner. Point /opt/hostedtoolcache at the new
# tool-cache root so those `mkdir -p` calls (and pre-extracted Ruby installs)
# resolve to the same writable, runner-owned location. The symlink target is
# /home/runner/.toolcache, so $RUNNER_TOOL_CACHE and gh-aw's mount guard continue
# to see the new path — the gh-aw fix from 5a8ae78 still works. (issue #405)
RUN mkdir -p /home/runner/.toolcache && chown runner:runner /home/runner/.toolcache \
    && ln -s /home/runner/.toolcache /opt/hostedtoolcache

# Optional tool-cache bake (runners.yml container_runner_image.toolcache; empty
# file = no-op). Each line is "<tarball-url> <dir> [complete] [strip]" — paths
# relative to /home/runner/.toolcache. The tarball is extracted into <dir> and a
# zero-byte marker is written at <complete> (default "<dir>.complete") —
# setup-ruby & co. treat that marker as "already installed" and skip their
# runtime download (GitHub-hosted images pre-seed rubies the same way). The
# explicit <complete> form matters for self-wrapping archives: ruby-builder
# tarballs ship a top-level x64/, so their entry is dir "Ruby/<ver>" + complete
# "Ruby/<ver>/x64.complete", yielding Ruby/<ver>/x64/bin/ruby. <strip> is the
# tar --strip-components count (default 0): strip 1 unwraps a single top-level
# version directory so the archive extracts as the bare tool dir — the
# canonical case is an Adoptium JDK tarball (jdk-<v>+<b>/) with dir
# "jdk-temurin-17" + strip 1, so consumers probing <toolcache>/jdk-temurin-17
# need not know the version. Legacy 3-field lines parse with strip 0.
# Both tar.gz and tar.xz archives are supported, picked by URL suffix (xz
# covers e.g. Flutter stable, which ships tar.xz only; xz-utils is in
# apt-packages-core.txt).
# Downloads retry hard: build egress rides the same flaky upstream that breaks
# job-time downloads, but here a failure is loud and attended (gh sr rebuild
# can simply be re-run) instead of surfacing as repeated unattended CI failures —
# the same trade-off that retired the dl.google.com chrome bake.
#
# The three timeouts are load-bearing, and the retry policy is only as good as
# them. --connect-timeout bounds TCP connect only: a connection that comes up
# and then goes dead without a FIN/RST (the documented release-asset TLS-reset
# case) leaves curl blocked in read() forever, so --retry never sees an error
# and the build hangs silently instead of failing. --speed-limit/--speed-time
# turns that dead socket into a 60s abort (curl exit 28, retried by
# --retry-all-errors), and --max-time additionally bounds a socket that is alive
# but pathologically slow — measured at ~42 KB/s from this network, which turns
# the 1.6 GB Flutter tarball alone into a ~10 h wait. Without both, a rebuild
# can wedge indefinitely on an entry it is never going to finish.
#
# Ownership: the whole tree is chowned to runner ONCE after the loop, not per
# entry. A per-entry `chown -R .../$dir` only covers the leaf — the intermediate
# parents that `mkdir -p` created as root stay root-owned, so a bake of
# "Ruby/4.0.7" left "Ruby/" unwritable by the runner user. That broke the
# runtime-install fallback this list is layered on top of: a workflow pinning a
# version the bake doesn't carry (e.g. .ruby-version 4.0.5 against a 4.0.7 bake)
# made ruby/setup-ruby mkdir a sibling version dir and die with
# `EACCES: permission denied, mkdir '/opt/hostedtoolcache/Ruby/4.0.5'` instead of
# downloading the missing version at job time. Same for the two-level node
# entries ("node/24.21.0/x64" stranded both "node/" and "node/<ver>/"), and any
# future nested dir. Chowning the root once keeps every level runner-owned, so a
# bake miss degrades to a normal runtime install (see the "Runtime install
# fallback preserved" scenario in openspec/specs/container-runner-image/spec.md).
COPY toolcache-extra.txt /tmp/toolcache-extra.txt
RUN while read -r url dir complete strip; do \
        [ -n "$url" ] || continue; \
        [ -n "$complete" ] || complete="$dir.complete"; \
        curl -fsSL --retry 8 --retry-all-errors --retry-delay 20 --connect-timeout 20 --speed-limit 10240 --speed-time 60 --max-time 1800 "$url" -o /tmp/toolcache.tar \
        && mkdir -p "/home/runner/.toolcache/$dir" "/home/runner/.toolcache/$(dirname "$complete")" \
        && case "$url" in \
             *.tar.xz) tar -xJf /tmp/toolcache.tar --strip-components=${strip:-0} -C "/home/runner/.toolcache/$dir" ;; \
             *) tar -xzf /tmp/toolcache.tar --strip-components=${strip:-0} -C "/home/runner/.toolcache/$dir" ;; \
           esac \
        && touch "/home/runner/.toolcache/$complete" \
        || exit 1; \
    done < /tmp/toolcache-extra.txt \
    && chown -R runner:runner /home/runner/.toolcache \
    && rm -f /tmp/toolcache-extra.txt /tmp/toolcache.tar

# Optional extra apt packages (from runners.yml; empty file skips this step)
COPY apt-packages-extra.txt /tmp/apt-packages-extra.txt
RUN apt-get update \
    && if [ -s /tmp/apt-packages-extra.txt ]; then \
         grep -v '^[[:space:]]*#' /tmp/apt-packages-extra.txt \
             | grep -v '^[[:space:]]*$' \
             | sed 's/\r$//' \
             | xargs -r apt-get install -y --no-install-recommends; \
       fi \
    && rm -rf /var/lib/apt/lists/*

# Shared cache volume mount points (runners.yml shared_caches): docker create
# mounts the gh-sr-npm-cache / gh-sr-runner-cache named volumes here. Baking
# the empty dirs with runner ownership makes the volume copy-up on first mount
# inherit the right owner; the entrypoint re-asserts ownership on every start
# regardless. Placed after the heavy layers so adding it never invalidates the
# apt/toolcache build cache.
RUN mkdir -p /home/runner/.npm /home/runner/.cache \
    && chown runner:runner /home/runner/.npm /home/runner/.cache

# Runner state directory (bind-mounted from host at runtime)
RUN mkdir -p /runner-state && chown runner:runner /runner-state

# Per-job reset hooks (Pillar 1): the Actions runner runs these before/after every
# job (wired via ACTIONS_RUNNER_HOOK_* in entrypoint.sh's .env). They guarantee a
# pristine inner environment per job — no leftover containers, networks, or
# /tmp/gh-aw — while preserving the image-layer cache. Stored OUTSIDE the actions
# runner application directory, as GitHub requires.
#
# awf-service-bridge.sh is the opt-in (GH_SR_AWF_SERVICE_BRIDGE) per-job waiter
# that joins the job's `services:` containers to the AWF awf-net topology
# network; job-started.sh spawns it detached, job-completed.sh kills it.
RUN mkdir -p /opt/gh-sr/hooks
COPY hooks/job-started.sh /opt/gh-sr/hooks/job-started.sh
COPY hooks/job-completed.sh /opt/gh-sr/hooks/job-completed.sh
COPY hooks/awf-service-bridge.sh /opt/gh-sr/hooks/awf-service-bridge.sh
RUN chmod +x /opt/gh-sr/hooks/job-started.sh /opt/gh-sr/hooks/job-completed.sh /opt/gh-sr/hooks/awf-service-bridge.sh

# Entrypoint
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Container starts as root (see USER root above): the entrypoint boots the inner
# dockerd before switching to the runner user for the actions runner loop.
USER root
ENTRYPOINT ["/entrypoint.sh"]
