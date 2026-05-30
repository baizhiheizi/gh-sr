---
engine:
  id: colipot
  env:
    COPILOT_PROVIDER_BASE_URL: "https://api.minimaxi.com/anthropic"
    COPILOT_MODEL: "MiniMax-M2.7"
    COPILOT_PROVIDER_API_KEY: ${{ secrets.MINIMAX_API_KEY }}
    COPILOT_PROVIDER_TYPE: anthropic
network:
  allowed:
    - defaults
    - api.minimaxi.com
---
