package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestRunFetchUsesDoH(t *testing.T) {
	page := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "text/html")
		fmt.Fprint(writer, "<html><body><h1>Hello</h1><p>World</p><script>x()</script></body></html>")
	}))
	defer page.Close()
	parsed, _ := url.Parse(page.URL)

	doh := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		fmt.Fprint(writer, `{"Status":0,"Answer":[{"name":"example.test","type":1,"TTL":60,"data":"127.0.0.1"}]}`)
	}))
	defer doh.Close()

	text, err := runFetch(context.Background(), map[string]any{
		"url":      "http://example.test:" + parsed.Port() + "/",
		"resolver": doh.URL,
	})
	if err != nil {
		t.Fatalf("runFetch: %v", err)
	}
	for _, want := range []string{"Hello", "World", "Fetched via DoH", "200 OK"} {
		if !strings.Contains(text, want) {
			t.Errorf("output is missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "x()") {
		t.Errorf("the script body should be stripped:\n%s", text)
	}
}

func TestRunFetchRefusesMetadata(t *testing.T) {
	if _, err := runFetch(context.Background(), map[string]any{"url": "http://169.254.169.254/latest/meta-data/"}); err == nil {
		t.Errorf("a metadata address must be refused")
	}
}

func TestHTMLToTextCollapses(t *testing.T) {
	got := htmlToText("<div>a</div>\n<p>b &amp; c</p><!-- x --><style>y</style>")
	if got != "a\nb & c" {
		t.Errorf("htmlToText = %q", got)
	}
}
