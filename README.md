# doh-mcp

A small Model Context Protocol server for DNS-over-HTTPS. It resolves names and
fetches URLs through a DoH resolver, so an agent keeps working behind an ISP DNS
block or a poisoned resolver. No API key. A single Go binary, CGO-free.

An ISP commonly returns NXDOMAIN for a whole domain (this is how DuckDuckGo and
some developer sites are blocked in some countries) while the site is fine
everywhere else. A browser escapes that by resolving over HTTPS; this server
gives an MCP client the same path.

## Tools

| Tool | What it does |
| --- | --- |
| `doh_resolve` | Resolve a name (A, AAAA, CNAME, MX, TXT, NS, SOA, PTR, SRV, CAA) through a DoH resolver and return the records with TTL. |
| `doh_compare` | Resolve a name with the system resolver and through DoH and report whether they agree, which detects a block, a hijack or a local override. |
| `doh_fetch` | Fetch an http(s) URL, resolving its host through DoH and dialing the resolved address, so a DNS block does not stop the read. HTML is reduced to text. |

Resolvers: `cloudflare` (default), `google`, `quad9`, `adguard`, or any DoH
endpoint URL that answers the JSON API (`Accept: application/dns-json`).

## Build

```sh
go build -o doh-mcp .
go install github.com/99apps-id/doh-mcp@latest
```

## Use with any MCP client

The server speaks MCP over stdio and needs no key, so any MCP client can run it.
The easiest path is `npx`, which downloads the right binary on first use:

```json
{ "command": "npx", "args": ["-y", "@99apps-id/doh-mcp"] }
```

Or install the binary directly:

```sh
go install github.com/99apps-id/doh-mcp@latest
# or download doh-mcp_<version>_<os>_<arch> from Releases
```

Add `"-resolver", "cloudflare"` (or `google`, `quad9`, `adguard`) after the
package name to pick a resolver.

### Termixgo

`mcpServers` in `~/.termixgo/config.json`:

```json
{ "name": "doh", "command": "npx", "args": ["-y", "@99apps-id/doh-mcp"] }
```

Tools appear as `mcp_doh__doh_resolve`, `mcp_doh__doh_compare`, `mcp_doh__doh_fetch`.

### Hermes

`mcp_servers` in `~/.hermes/config.yaml`:

```yaml
mcp_servers:
  doh:
    command: npx
    args: ["-y", "@99apps-id/doh-mcp"]
```

### OpenClaw

```sh
openclaw mcp add doh --command npx --arg -y --arg @99apps-id/doh-mcp
openclaw mcp doctor doh --probe
```

### VS Code (GitHub Copilot, Cline, Continue)

VS Code uses `servers` in `.vscode/mcp.json`:

```json
{
  "servers": {
    "doh": { "type": "stdio", "command": "npx", "args": ["-y", "@99apps-id/doh-mcp"] }
  }
}
```

### Claude Desktop, Cursor, Windsurf, Cline, Zed

These use `mcpServers`:

```json
{
  "mcpServers": {
    "doh": { "command": "npx", "args": ["-y", "@99apps-id/doh-mcp"] }
  }
}
```

### Docker

```sh
docker build -t doh-mcp .
# point the client at: docker run -i --rm doh-mcp
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
