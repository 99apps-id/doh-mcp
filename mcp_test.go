package main

import (
	"bufio"
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func runServer(t *testing.T, lines ...string) []string {
	t.Helper()
	input := strings.NewReader(strings.Join(lines, "\n") + "\n")
	var out bytes.Buffer
	server := &server{out: bufio.NewWriter(&out)}
	if err := server.serve(input); err != nil {
		t.Fatalf("serve: %v", err)
	}
	text := strings.TrimSpace(out.String())
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}

func TestInitializeAndToolsList(t *testing.T) {
	lines := runServer(t,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
	)
	if len(lines) != 2 {
		t.Fatalf("got %d replies: %v", len(lines), lines)
	}
	if !strings.Contains(lines[0], protocolVersion) || !strings.Contains(lines[0], "doh-mcp") {
		t.Errorf("initialize reply = %s", lines[0])
	}
	for _, want := range []string{"doh_resolve", "doh_compare", "doh_fetch"} {
		if !strings.Contains(lines[1], want) {
			t.Errorf("tools/list is missing %q: %s", want, lines[1])
		}
	}
}

func TestToolsCallResolve(t *testing.T) {
	doh := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		fmt.Fprint(writer, `{"Status":0,"Answer":[{"name":"example.com","type":1,"TTL":60,"data":"93.184.216.34"}]}`)
	}))
	defer doh.Close()

	call := fmt.Sprintf(`{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"doh_resolve","arguments":{"name":"example.com","resolver":%q}}}`, doh.URL)
	lines := runServer(t, call)
	if len(lines) != 1 {
		t.Fatalf("got %d replies: %v", len(lines), lines)
	}
	if !strings.Contains(lines[0], "93.184.216.34") || !strings.Contains(lines[0], `"id":7`) {
		t.Errorf("resolve reply = %s", lines[0])
	}
}

func TestToolsCallUnknownTool(t *testing.T) {
	lines := runServer(t, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"nope"}}`)
	if len(lines) != 1 || !strings.Contains(lines[0], "unknown tool") {
		t.Fatalf("replies = %v", lines)
	}
}

func TestUnknownMethod(t *testing.T) {
	lines := runServer(t, `{"jsonrpc":"2.0","id":4,"method":"does/not/exist"}`)
	if len(lines) != 1 || !strings.Contains(lines[0], "method not found") {
		t.Fatalf("replies = %v", lines)
	}
}

func TestMalformedLineIsDropped(t *testing.T) {
	lines := runServer(t, `not json`, `{"jsonrpc":"2.0","id":5,"method":"ping"}`)
	if len(lines) != 1 || !strings.Contains(lines[0], `"id":5`) {
		t.Fatalf("replies = %v", lines)
	}
}
