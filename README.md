# doh-mcp

MCP server for DNS-over-HTTPS (DoH). Resolves names and fetches URLs through a
DoH resolver so an agent keeps working behind an ISP DNS block or a poisoned
resolver. No API key. A single Go binary, CGO-free.

**License:** Apache-2.0

## Install

### Build from source

```bash
git clone https://github.com/99apps-id/doh-mcp.git
cd doh-mcp
go build -o doh-mcp .
```

### Download binary

Pre-built binaries are available for Linux, macOS, and Windows on the GitHub
Releases page.

## Configure

The server accepts a `-resolver` flag:

```sh
doh-mcp -resolver cloudflare
# or
doh-mcp -resolver google
# or
doh-mcp -resolver quad9
# or
doh-mcp -resolver adguard
# or
doh-mcp -resolver https://your-doh-server/dns-query
```

Available built-in resolver aliases:
- `cloudflare` (default)
- `google`
- `quad9`
- `adguard`

Or pass any DoH endpoint URL that answers the JSON API
(`Accept: application/dns-json`).

## Tools

| Tool | What it does |
| --- | --- |
| `doh_resolve` | Resolve a name (A, AAAA, CNAME, MX, TXT, NS, SOA, PTR, SRV, CAA) through a DoH resolver and return the records with TTL. |
| `doh_compare` | Resolve a name with the system resolver and through DoH and report whether they agree, which detects a block, a hijack or a local override. |
| `doh_fetch` | Fetch an http(s) URL, resolving its host through DoH and dialing the resolved address, so a DNS block does not stop the read. HTML is reduced to text. |

## Editor and Agent Compatibility

This MCP server is designed to work with any MCP client, including:

- **Termigo** - native integration via `~/.termigo/mcp.json`
- **Termixgo** - `~/.termixgo/config.json` or `mcpServers` section
- **VS Code** (with MCP extension)
- **Claude Code** (Anthropic's CLI)
- **Codex** (OpenAI's coding agent)
- **Cursor**
- **OpenCode**
- **OpenClaw**
- **Hermes**
- **9router**
- **Windsurf**
- **Zed**
- **Cline**
- Any other editor or agent that supports the Model Context Protocol (MCP) over stdio

### Generic MCP Configuration

All MCP clients that support stdio servers can use this format:

```json
{
  "mcpServers": {
    "doh": {
      "command": "doh-mcp",
      "args": ["-resolver", "cloudflare"]
    }
  }
}
```

Replace `doh-mcp` with the full path to the binary on your system:

- **Linux:** `/usr/local/bin/doh-mcp` or `$HOME/.local/bin/doh-mcp`
- **macOS:** `/usr/local/bin/doh-mcp` or `$HOME/.local/bin/doh-mcp`
- **Windows:** `C:\\Users\\<USER>\\bin\\doh-mcp.exe` or `C:\\Program Files\\doh-mcp\\doh-mcp.exe`

### Termigo

`mcpServers` in `~/.termigo/mcp.json`:

```json
{
  "mcpServers": {
    "doh": {
      "command": "doh-mcp",
      "args": ["-resolver", "cloudflare"]
    }
  }
}
```

Tools appear as `mcp_doh__doh_resolve`, `mcp_doh__doh_compare`, `mcp_doh__doh_fetch`.

### Termixgo

`mcpServers` in `~/.termixgo/config.json`:

```json
{
  "mcpServers": {
    "doh": {
      "command": "doh-mcp",
      "args": ["-resolver", "cloudflare"]
    }
  }
}
```

Or use the built-in MCP commands:

```sh
termixgo mcp add doh --command doh-mcp --arg -resolver --arg cloudflare
termixgo mcp list
```

### VS Code (GitHub Copilot, Cline, Continue)

VS Code uses `mcpServers` in `.vscode/mcp.json` (workspace) or user settings:

```json
{
  "mcpServers": {
    "doh": {
      "command": "doh-mcp",
      "args": ["-resolver", "cloudflare"]
    }
  }
}
```

### Claude Desktop / Claude Code

Claude Desktop uses `mcpServers` in `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "doh": {
      "command": "doh-mcp",
      "args": ["-resolver", "cloudflare"]
    }
  }
}
```

Claude Code reads from `~/.claude.json`:

```json
{
  "mcpServers": {
    "doh": {
      "command": "doh-mcp",
      "args": ["-resolver", "cloudflare"]
    }
  }
}
```

### Cursor

Cursor uses `mcpServers` in `~/.cursor/settings.json` or workspace `.cursor/settings.json`:

```json
{
  "mcpServers": {
    "doh": {
      "command": "doh-mcp",
      "args": ["-resolver", "cloudflare"]
    }
  }
}
```

### OpenCode

OpenCode uses `.opencode/mcp.json`:

```json
{
  "mcpServers": {
    "doh": {
      "command": "doh-mcp",
      "args": ["-resolver", "cloudflare"]
    }
  }
}
```

### OpenClaw

```sh
openclaw mcp add doh --command doh-mcp --arg -resolver --arg cloudflare
openclaw mcp doctor doh --probe
```

### Hermes

Hermes uses `mcp_servers` in `~/.hermes/config.yaml`:

```yaml
mcp_servers:
  doh:
    command: doh-mcp
    args:
      - "-resolver"
      - "cloudflare"
```

### 9router

9router uses the MCP Marketplace UI or `managedMcpServers` in the workspace config:

```json
{
  "managedMcpServers": {
    "doh": {
      "command": "doh-mcp",
      "args": ["-resolver", "cloudflare"]
    }
  }
}
```

Or use the Dashboard UI: **Plugins** -> **Add Custom MCP** -> paste the command above.

### Codex CLI

Codex CLI reads from `OPENAI_MCP_SERVERS` environment variable:

```sh
export OPENAI_MCP_SERVERS='{"doh":{"command":"doh-mcp","args":["-resolver","cloudflare"]}}'
```

Or in `.codex/config.json`:

```json
{
  "mcpServers": {
    "doh": {
      "command": "doh-mcp",
      "args": ["-resolver", "cloudflare"]
    }
  }
}
```

### Docker

```sh
docker build -t doh-mcp .
# point the client at: docker run -i --rm doh-mcp
```

Dockerfile:

```dockerfile
FROM golang:1.26-alpine AS build
WORKDIR /app
COPY . .
RUN go build -o doh-mcp .

FROM alpine:latest
RUN apk add --no-cache ca-certificates
COPY --from=build /app/doh-mcp /usr/local/bin/doh-mcp
ENTRYPOINT ["doh-mcp"]
```

## Example calls

```jsonc
// doh_resolve
{ "name": "example.com", "type": "A", "resolver": "cloudflare" }

// doh_compare
{ "name": "duckduckgo.com" }

// doh_fetch
{ "url": "https://example.com", "max_chars": 20000 }
```

`doh_compare` is the diagnostic: on a blocked network it reports
`the system resolver failed but DoH resolved the name; DNS is blocked or poisoned`.

## Security

`doh_fetch` refuses cloud metadata (`169.254.169.254`,
`metadata.google.internal`) and link-local addresses, re-checks on every
redirect, and pins the connection to the address it resolved, which also
defeats DNS rebinding. Loopback and private addresses are allowed, so a local
documentation server can be read.

## Development

```sh
gofmt -s -l .
go vet ./...
go test ./...
```

## License

Apache-2.0. See [LICENSE](LICENSE).
