package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"sync"
)

// protocolVersion is the MCP revision this server implements. A client that
// asks for another version still gets a well-formed reply naming this one.
const protocolVersion = "2025-06-18"

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type server struct {
	out *bufio.Writer
	mu  sync.Mutex
}

// serve reads newline-delimited JSON-RPC messages and answers them. A line that
// is not valid JSON is dropped, as the transport requires.
func (s *server) serve(in io.Reader) error {
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var req request
		if err := json.Unmarshal(line, &req); err != nil {
			continue
		}
		s.handle(req)
	}
	return scanner.Err()
}

func (s *server) handle(req request) {
	// A message without an id is a notification: handled, never answered.
	if len(req.ID) == 0 {
		return
	}
	switch req.Method {
	case "initialize":
		s.send(response{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{
			"protocolVersion": protocolVersion,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "doh-mcp", "version": version},
		}})
	case "ping":
		s.send(response{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{}})
	case "tools/list":
		s.send(response{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{"tools": toolSchemas()}})
	case "tools/call":
		s.handleToolCall(req)
	default:
		s.send(response{JSONRPC: "2.0", ID: req.ID, Error: &rpcError{Code: -32601, Message: "method not found: " + req.Method}})
	}
}

func (s *server) handleToolCall(req request) {
	var params struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		s.send(response{JSONRPC: "2.0", ID: req.ID, Error: &rpcError{Code: -32602, Message: "invalid params"}})
		return
	}
	tool, ok := toolByName(params.Name)
	if !ok {
		s.send(response{JSONRPC: "2.0", ID: req.ID, Error: &rpcError{Code: -32602, Message: "unknown tool: " + params.Name}})
		return
	}
	text, err := tool.Run(context.Background(), params.Arguments)
	if err != nil {
		s.send(response{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{
			"content": []map[string]any{{"type": "text", "text": err.Error()}},
			"isError": true,
		}})
		return
	}
	s.send(response{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{
		"content": []map[string]any{{"type": "text", "text": text}},
	}})
}

func (s *server) send(v any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	s.out.Write(data)
	s.out.WriteByte('\n')
	s.out.Flush()
}

// tool is one MCP tool.
type tool struct {
	Name        string
	Description string
	Schema      map[string]any
	Run         func(ctx context.Context, args map[string]any) (string, error)
}

func tools() []tool {
	return []tool{
		{
			Name:        "doh_resolve",
			Description: "Resolve a DNS name through a DNS-over-HTTPS resolver and return the records. Works where the system resolver is blocked or poisoned, and needs no API key.",
			Schema: object(map[string]any{
				"name":     strProp("The DNS name to resolve, such as example.com."),
				"type":     strProp("Record type: A, AAAA, CNAME, MX, TXT, NS, SOA, PTR, SRV or CAA. Default A."),
				"resolver": strProp("cloudflare (default), google, quad9, adguard or an https:// DoH endpoint URL."),
			}, "name"),
			Run: runResolve,
		},
		{
			Name:        "doh_compare",
			Description: "Resolve a name with the system resolver and through DoH and report whether they agree, which detects a DNS block or poisoning.",
			Schema: object(map[string]any{
				"name":     strProp("The DNS name to compare."),
				"resolver": strProp("cloudflare (default), google, quad9, adguard or an https:// DoH endpoint URL."),
			}, "name"),
			Run: runCompare,
		},
		{
			Name:        "doh_fetch",
			Description: "Fetch an http or https URL, resolving its host through DoH and connecting to the resolved address, so a DNS block does not stop the read. HTML is reduced to text.",
			Schema: object(map[string]any{
				"url":       strProp("Absolute http or https URL."),
				"max_chars": prop("integer", "Maximum characters to return. Default 20000."),
				"resolver":  strProp("cloudflare (default), google, quad9, adguard or an https:// DoH endpoint URL."),
			}, "url"),
			Run: runFetch,
		},
	}
}

func toolByName(name string) (tool, bool) {
	for _, candidate := range tools() {
		if candidate.Name == name {
			return candidate, true
		}
	}
	return tool{}, false
}

func toolSchemas() []map[string]any {
	defs := tools()
	schemas := make([]map[string]any, 0, len(defs))
	for _, def := range defs {
		schemas = append(schemas, map[string]any{
			"name":        def.Name,
			"description": def.Description,
			"inputSchema": def.Schema,
		})
	}
	return schemas
}

func object(properties map[string]any, required ...string) map[string]any {
	schema := map[string]any{"type": "object", "properties": properties}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func strProp(description string) map[string]any { return prop("string", description) }

func prop(kind, description string) map[string]any {
	return map[string]any{"type": kind, "description": description}
}
