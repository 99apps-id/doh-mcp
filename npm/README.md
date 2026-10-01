# doh-mcp (npm)

An MCP server for DNS-over-HTTPS: resolve names and fetch URLs through DoH, so
an agent keeps working behind a DNS block. This package downloads the prebuilt
Go binary for your platform on first run and launches it, so any MCP client can
use it with `npx` and no manual install.

```json
{ "command": "npx", "args": ["-y", "doh-mcp"] }
```

See the main project for tools and configuration:
https://github.com/99apps-id/doh-mcp
