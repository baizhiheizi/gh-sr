---
network:
  allowed:
    - defaults
    - go

jobs:
  runs-on: [self-hosted, Linux, agentic]
  setup:
    steps:
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'
---
