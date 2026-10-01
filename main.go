// Command doh-mcp is a small Model Context Protocol server for DNS-over-HTTPS:
// it resolves names and fetches URLs through a DoH resolver, so a client can
// work behind an ISP DNS block or a poisoned resolver. It speaks MCP over stdio
// and needs no API key.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
)

func main() {
	resolver := flag.String("resolver", fallbackResolver, "default DoH resolver: cloudflare, google, quad9, adguard or an https URL")
	flag.Parse()
	if err := os.Setenv("DOH_DEFAULT_RESOLVER", *resolver); err != nil {
		fmt.Fprintln(os.Stderr, "doh-mcp:", err)
		os.Exit(1)
	}

	server := &server{out: bufio.NewWriter(os.Stdout)}
	if err := server.serve(os.Stdin); err != nil {
		fmt.Fprintln(os.Stderr, "doh-mcp:", err)
		os.Exit(1)
	}
}
