---
engine:
  id: claude
  env:
    ANTHROPIC_BASE_URL: "https://api.minimaxi.com/anthropic"
    API_TIMEOUT_MS: "3000000"
    CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC: "1"
    ANTHROPIC_MODEL: "MiniMax-M2.7"
    ANTHROPIC_SMALL_FAST_MODEL: "MiniMax-M2.7"
    ANTHROPIC_DEFAULT_SONNET_MODEL: "MiniMax-M2.7"
    ANTHROPIC_DEFAULT_OPUS_MODEL: "MiniMax-M2.7"
    ANTHROPIC_DEFAULT_HAIKU_MODEL: "MiniMax-M2.7"
network:
  allowed:
    - defaults
    - api.minimaxi.com
mcp-servers:
  mixnimax:
    container: "ghcr.io/astral-sh/uv:python3.12-alpine"
    entrypoint: "uvx"
    entrypointArgs: ["minimax-coding-plan-mcp", "-y"]
    allowed: ["*"]
    env:
      MINIMAX_API_KEY: ${{ secrets.ANTHROPIC_API_KEY }}
      MINIMAX_API_HOST: https://api.minimaxi.com
---
