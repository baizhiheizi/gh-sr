FROM ubuntu:22.04

ARG RUNNER_VERSION=latest
ARG TARGETARCH

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y \
    curl \
    ca-certificates \
    jq \
    git \
    sudo \
    && rm -rf /var/lib/apt/lists/*

RUN useradd -m -s /bin/bash runner \
    && echo "runner ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers

WORKDIR /home/runner

RUN set -e; \
    ARCH=$([ "$TARGETARCH" = "arm64" ] && echo "arm64" || echo "x64"); \
    if [ "$RUNNER_VERSION" = "latest" ]; then \
      VER=$(curl -s https://api.github.com/repos/actions/runner/releases/latest \
            | jq -r '.tag_name | ltrimstr("v")'); \
    else \
      VER="$RUNNER_VERSION"; \
    fi; \
    echo "Installing runner v${VER} (${ARCH})..."; \
    curl -fsSL "https://github.com/actions/runner/releases/download/v${VER}/actions-runner-linux-${ARCH}-${VER}.tar.gz" \
      | tar xz; \
    ./bin/installdependencies.sh

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

USER runner
ENTRYPOINT ["/entrypoint.sh"]
