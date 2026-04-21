# gh-sr agentic runner container image
#
# Each runner instance runs in its own container, providing:
#   - isolated /tmp/gh-aw (per-container filesystem)
#   - isolated dockerd and iptables (per-container network namespace)
#   - isolated MCP gateway on port 80 (no port labeling required)
#
# Build: docker build -t gh-sr/agentic-runner:<version> .
# Run:   see internal/runner/container.go

FROM ubuntu:24.04

ENV DEBIAN_FRONTEND=noninteractive

# Core system dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
        ca-certificates \
        curl \
        gnupg \
        lsb-release \
        sudo \
        iptables \
        dnsmasq \
        dnsutils \
        jq \
        git \
        tar \
        unzip \
        libicu-dev \
        iproute2 \
        uidmap \
        imagemagick \
        libvips \
        libyaml-0-2 \
        libsodium-dev \
    && rm -rf /var/lib/apt/lists/*

# Docker CE (for DinD — the inner dockerd that gh-aw uses)
RUN install -m 0755 -d /etc/apt/keyrings \
    && curl -fsSL https://download.docker.com/linux/ubuntu/gpg \
        | gpg --dearmor -o /etc/apt/keyrings/docker.gpg \
    && chmod a+r /etc/apt/keyrings/docker.gpg \
    && echo \
       "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] \
        https://download.docker.com/linux/ubuntu \
        $(lsb_release -cs) stable" \
       > /etc/apt/sources.list.d/docker.list \
    && apt-get update \
    && apt-get install -y --no-install-recommends \
        docker-ce \
        docker-ce-cli \
        containerd.io \
        docker-buildx-plugin \
        docker-compose-plugin \
    && rm -rf /var/lib/apt/lists/*

# GitHub CLI (gh) — required by gh-aw
RUN curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg \
        | dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg \
    && chmod go+r /usr/share/keyrings/githubcli-archive-keyring.gpg \
    && echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] \
             https://cli.github.com/packages stable main" \
        > /etc/apt/sources.list.d/github-cli.list \
    && apt-get update \
    && apt-get install -y --no-install-recommends gh \
    && rm -rf /var/lib/apt/lists/*

# runner user (non-root, but in docker group so it can talk to the inner dockerd)
# Full passwordless sudo is required: AWF installs binaries to system paths
# (/usr/local/lib/awf/) and manipulates iptables rules at job runtime.
# The container is already --privileged and single-purpose, so this is safe.
RUN useradd --create-home --shell /bin/bash runner \
    && usermod -aG docker runner \
    && echo "runner ALL=(ALL) NOPASSWD: ALL" \
       > /etc/sudoers.d/runner-nopasswd \
    && chmod 0440 /etc/sudoers.d/runner-nopasswd

# /opt/hostedtoolcache — gh-aw agent containers look here for pre-installed tools
RUN mkdir -p /opt/hostedtoolcache && chown runner:runner /opt/hostedtoolcache

# Actions runner binary — installed at image build time so startup is fast.
# RUNNER_VERSION is injected by `docker build --build-arg RUNNER_VERSION=<ver>`.
ARG RUNNER_VERSION
ARG RUNNER_ARCH=x64
RUN if [ -z "$RUNNER_VERSION" ]; then \
        echo "ERROR: RUNNER_VERSION build-arg is required" >&2; exit 1; \
    fi \
    && mkdir -p /home/runner/actions-runner \
    && curl -fsSL \
       "https://github.com/actions/runner/releases/download/v${RUNNER_VERSION}/actions-runner-linux-${RUNNER_ARCH}-${RUNNER_VERSION}.tar.gz" \
       | tar -xz -C /home/runner/actions-runner \
    && chown -R runner:runner /home/runner/actions-runner \
    && /home/runner/actions-runner/bin/installdependencies.sh

# gh-aw CLI extension
RUN su - runner -c "curl -sL https://raw.githubusercontent.com/github/gh-aw/main/install-gh-aw.sh | bash"

# Runner state directory (bind-mounted from host at runtime)
RUN mkdir -p /runner-state && chown runner:runner /runner-state

# Docker wrapper: transparently patches gh-aw-mcpg images so they start
# correctly under `--network host`. See docker-wrapper.sh header for details.
# /usr/local/bin comes before /usr/bin in the default PATH, so this wrapper
# shadows the real docker CLI (at /usr/bin/docker) for all users.
COPY docker-wrapper.sh /usr/local/bin/docker
RUN chmod +x /usr/local/bin/docker

# Entrypoint
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
