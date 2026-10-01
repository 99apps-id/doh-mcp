package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResolveDoH(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if got := request.URL.Query().Get("name"); got != "example.com" {
			t.Errorf("name = %q", got)
		}
		if got := request.URL.Query().Get("type"); got != "1" {
			t.Errorf("type = %q, want 1 for A", got)
		}
		writer.Header().Set("Content-Type", "application/dns-json")
		fmt.Fprint(writer, `{"Status":0,"Answer":[{"name":"example.com","type":1,"TTL":120,"data":"93.184.216.34"}]}`)
	}))
	defer server.Close()

	records, err := resolveDoH(context.Background(), server.URL, "example.com", "A")
	if err != nil {
		t.Fatalf("resolveDoH: %v", err)
	}
	if len(records) != 1 || records[0].Data != "93.184.216.34" || records[0].Type != "A" || records[0].TTL != 120 {
		t.Fatalf("records = %+v", records)
	}
}

func TestResolveDoHReportsNXDOMAIN(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		fmt.Fprint(writer, `{"Status":3}`)
	}))
	defer server.Close()
	if _, err := resolveDoH(context.Background(), server.URL, "nope.invalid", "A"); err == nil {
		t.Errorf("an NXDOMAIN status must be an error")
	}
}

func TestResolveRejectsUnknownType(t *testing.T) {
	if _, err := resolveDoH(context.Background(), "cloudflare", "example.com", "BOGUS"); err == nil {
		t.Errorf("an unknown record type must fail")
	}
}

func TestResolverEndpoint(t *testing.T) {
	if endpoint, err := resolverEndpoint("cloudflare"); err != nil || !strings.Contains(endpoint, "cloudflare-dns.com") {
		t.Errorf("cloudflare endpoint = %q err=%v", endpoint, err)
	}
	if endpoint, err := resolverEndpoint("https://dns.example/dns-query"); err != nil || endpoint != "https://dns.example/dns-query" {
		t.Errorf("custom endpoint = %q err=%v", endpoint, err)
	}
	if _, err := resolverEndpoint("bogus"); err == nil {
		t.Errorf("an unknown resolver must fail")
	}
}

func TestCompareVerdict(t *testing.T) {
	cases := []struct {
		system    []string
		systemErr error
		doh       []string
		dohErr    error
		want      string
	}{
		{systemErr: fmt.Errorf("no such host"), doh: []string{"1.2.3.4"}, want: "blocked or poisoned"},
		{system: []string{"1.2.3.4"}, doh: []string{"1.2.3.4"}, want: "agree"},
		{system: []string{"1.2.3.4"}, doh: []string{"5.6.7.8"}, want: "disagree"},
		{system: []string{"1.2.3.4"}, doh: nil, want: "hosts entry"},
		{want: "did not resolve"},
	}
	for _, testCase := range cases {
		got := compareVerdict(testCase.system, testCase.systemErr, testCase.doh, testCase.dohErr)
		if !strings.Contains(got, testCase.want) {
			t.Errorf("compareVerdict(%v,%v,%v,%v) = %q, want %q", testCase.system, testCase.systemErr, testCase.doh, testCase.dohErr, got, testCase.want)
		}
	}
}
