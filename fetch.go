package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	scriptStyle = regexp.MustCompile(`(?is)<script\b.*?</script\s*>|<style\b.*?</style\s*>|<noscript\b.*?</noscript\s*>`)
	comment     = regexp.MustCompile(`(?s)<!--.*?-->`)
	lineBreak   = regexp.MustCompile(`(?is)<br\s*/?>|</(?:p|div|li|tr|h[1-6]|section|article|pre|blockquote)\s*>`)
	tag         = regexp.MustCompile(`(?s)<[^>]*>`)
	horizontal  = regexp.MustCompile(`[ \t\r\f\v]+`)
)

const (
	defaultMaxChars = 20000
	userAgent       = "doh-mcp/0.1 (+https://github.com/99apps-id/doh-mcp)"
)

// runFetch fetches a URL, resolving its host through DoH and dialing the
// resolved address, so a DNS block does not stop the read.
func runFetch(ctx context.Context, args map[string]any) (string, error) {
	raw := strings.TrimSpace(argString(args, "url"))
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", fmt.Errorf("url must be an absolute http or https URL")
	}
	if blockedHost(parsed.Hostname()) {
		return "", fmt.Errorf("refusing a link-local or cloud metadata address")
	}
	maxChars := argInt(args, "max_chars", defaultMaxChars, 500, 200000)
	resolver := argString(args, "resolver")
	if resolver == "" {
		resolver = defaultResolverValue()
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			Proxy:             http.ProxyFromEnvironment,
			DialContext:       dohDialer(resolver),
			ForceAttemptHTTP2: true,
			MaxIdleConns:      4,
		},
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("stopped after 5 redirects")
			}
			if blockedHost(request.URL.Hostname()) {
				return fmt.Errorf("redirect to a blocked address")
			}
			return nil
		},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("User-Agent", userAgent)
	request.Header.Set("Accept", "text/html,application/json,text/plain;q=0.9,*/*;q=0.5")
	response, err := client.Do(request)
	if err != nil {
		return "", fmt.Errorf("fetch failed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("fetch returned %s", response.Status)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 2*1024*1024))
	if err != nil {
		return "", fmt.Errorf("read body: %v", err)
	}
	text := string(body)
	if looksLikeHTML(response.Header.Get("Content-Type"), text) {
		text = htmlToText(text)
	}
	truncated := ""
	if len(text) > maxChars {
		text = clipBytes(text, maxChars)
		truncated = "\n... [truncated]"
	}
	return fmt.Sprintf("URL: %s\nStatus: %s\nFetched via DoH (%s)\n\n%s%s", raw, response.Status, resolver, text, truncated), nil
}

// dohDialer resolves every hostname through DoH and dials the resolved address.
// Because the name is resolved here, once, the connection is pinned to the
// address that was checked, which also defeats DNS rebinding.
func dohDialer(resolver string) func(ctx context.Context, network, addr string) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second}
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		var ips []string
		if net.ParseIP(host) != nil {
			ips = []string{host}
		} else {
			for _, recordType := range []string{"A", "AAAA"} {
				records, err := resolveDoH(ctx, resolver, host, recordType)
				if err != nil {
					return nil, err
				}
				for _, entry := range records {
					if net.ParseIP(entry.Data) != nil {
						ips = append(ips, entry.Data)
					}
				}
			}
		}
		if len(ips) == 0 {
			return nil, fmt.Errorf("no address for %s", host)
		}
		var lastErr error
		for _, ip := range ips {
			conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip, port))
			if err == nil {
				return conn, nil
			}
			lastErr = err
		}
		return nil, lastErr
	}
}

// blockedHost refuses cloud metadata and link-local addresses, which are never
// the page a caller meant to read.
func blockedHost(host string) bool {
	trimmed := strings.ToLower(strings.TrimSpace(host))
	if trimmed == "169.254.169.254" || trimmed == "metadata.google.internal" {
		return true
	}
	if ip := net.ParseIP(trimmed); ip != nil && ip.IsLinkLocalUnicast() {
		return true
	}
	return false
}

func looksLikeHTML(contentType, body string) bool {
	lowered := strings.ToLower(contentType)
	if strings.Contains(lowered, "html") {
		return true
	}
	head := strings.ToLower(strings.TrimSpace(body))
	return strings.HasPrefix(head, "<!doctype html") || strings.HasPrefix(head, "<html")
}

// htmlToText is a small reducer: it drops script, style and comments, strips
// tags, decodes the common entities and collapses whitespace.
func htmlToText(raw string) string {
	text := scriptStyle.ReplaceAllString(raw, " ")
	text = comment.ReplaceAllString(text, " ")
	text = lineBreak.ReplaceAllString(text, "\n")
	text = tag.ReplaceAllString(text, "")
	text = strings.NewReplacer(
		"&amp;", "&", "&lt;", "<", "&gt;", ">", "&quot;", `"`, "&#39;", "'", "&nbsp;", " ",
	).Replace(text)
	text = horizontal.ReplaceAllString(text, " ")
	lines := make([]string, 0, 64)
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	return strings.Join(lines, "\n")
}

// clipBytes trims s to at most n bytes without cutting a rune in half.
func clipBytes(s string, n int) string {
	if len(s) <= n {
		return s
	}
	cut := s[:n]
	for len(cut) > 0 && !utf8.ValidString(cut) {
		cut = cut[:len(cut)-1]
	}
	return cut
}
