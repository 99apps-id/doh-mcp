package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

const fallbackResolver = "cloudflare"

// dohResolvers are the named JSON-API resolvers.
var dohResolvers = map[string]string{
	"cloudflare": "https://cloudflare-dns.com/dns-query",
	"google":     "https://dns.google/resolve",
	"quad9":      "https://dns.quad9.net/dns-query",
	"adguard":    "https://dns.adguard-dns.com/dns-query",
}

// recordNumber maps a record type name to its DNS type number.
var recordNumber = map[string]int{
	"A": 1, "NS": 2, "CNAME": 5, "SOA": 6, "PTR": 12, "MX": 15, "TXT": 16, "AAAA": 28, "SRV": 33, "CAA": 257,
}

var recordName = func() map[int]string {
	out := make(map[int]string, len(recordNumber))
	for name, number := range recordNumber {
		out[number] = name
	}
	return out
}()

type record struct {
	Name string `json:"name"`
	Type string `json:"type"`
	TTL  int    `json:"ttl"`
	Data string `json:"data"`
}

// dohClient is the shared client for the resolver API.
var dohClient = &http.Client{Timeout: 10 * time.Second}

func defaultResolverValue() string {
	if value := strings.TrimSpace(os.Getenv("DOH_DEFAULT_RESOLVER")); value != "" {
		return value
	}
	return fallbackResolver
}

func resolverEndpoint(name string) (string, error) {
	key := strings.ToLower(strings.TrimSpace(name))
	if key == "" {
		key = defaultResolverValue()
	}
	if endpoint, ok := dohResolvers[key]; ok {
		return endpoint, nil
	}
	if strings.HasPrefix(key, "https://") || strings.HasPrefix(key, "http://") {
		return key, nil
	}
	return "", fmt.Errorf("unknown resolver %q (use cloudflare, google, quad9, adguard or an https URL)", name)
}

// resolveDoH resolves a name through a DoH resolver and returns its records.
func resolveDoH(ctx context.Context, resolver, name, recordType string) ([]record, error) {
	endpoint, err := resolverEndpoint(resolver)
	if err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	number, ok := recordNumber[strings.ToUpper(strings.TrimSpace(recordType))]
	if !ok {
		return nil, fmt.Errorf("unsupported record type %q", recordType)
	}
	link := endpoint + "?name=" + url.QueryEscape(name) + "&type=" + strconv.Itoa(number)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("accept", "application/dns-json")
	response, err := dohClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("resolver %s: %w", endpoint, err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("resolver returned %s", response.Status)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Status int `json:"Status"`
		Answer []struct {
			Name string `json:"name"`
			Type int    `json:"type"`
			TTL  int    `json:"TTL"`
			Data string `json:"data"`
		} `json:"Answer"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("read resolver response: %v", err)
	}
	if len(parsed.Answer) == 0 {
		if parsed.Status != 0 {
			return nil, fmt.Errorf("name did not resolve (DNS status %d)", parsed.Status)
		}
		return nil, nil
	}
	records := make([]record, 0, len(parsed.Answer))
	for _, answer := range parsed.Answer {
		records = append(records, record{Name: answer.Name, Type: typeName(answer.Type), TTL: answer.TTL, Data: answer.Data})
	}
	return records, nil
}

func typeName(number int) string {
	if name, ok := recordName[number]; ok {
		return name
	}
	return strconv.Itoa(number)
}

func runResolve(ctx context.Context, args map[string]any) (string, error) {
	name := argString(args, "name")
	recordType := argString(args, "type")
	if strings.TrimSpace(recordType) == "" {
		recordType = "A"
	}
	resolver := argString(args, "resolver")
	records, err := resolveDoH(ctx, resolver, name, recordType)
	if err != nil {
		return "", err
	}
	if resolver == "" {
		resolver = defaultResolverValue()
	}
	payload := map[string]any{
		"name":     strings.TrimSpace(name),
		"type":     strings.ToUpper(recordType),
		"resolver": resolver,
		"records":  records,
		"count":    len(records),
	}
	return marshalPretty(payload)
}

// runCompare resolves a name both ways and reports whether they agree, which
// is how a DNS block or poisoning shows up.
func runCompare(ctx context.Context, args map[string]any) (string, error) {
	name := strings.TrimSpace(argString(args, "name"))
	if name == "" {
		return "", fmt.Errorf("name is required")
	}
	resolver := argString(args, "resolver")
	if resolver == "" {
		resolver = defaultResolverValue()
	}

	system, systemErr := net.DefaultResolver.LookupHost(ctx, name)
	dohRecords, dohErr := resolveDoH(ctx, resolver, name, "A")
	var dohAddresses []string
	for _, entry := range dohRecords {
		if entry.Type == "A" || entry.Type == "AAAA" {
			dohAddresses = append(dohAddresses, entry.Data)
		}
	}

	verdict := compareVerdict(system, systemErr, dohAddresses, dohErr)
	payload := map[string]any{
		"name":     name,
		"resolver": resolver,
		"verdict":  verdict,
		"system": map[string]any{
			"addresses": sortedCopy(system),
			"error":     errText(systemErr),
		},
		"doh": map[string]any{
			"addresses": sortedCopy(dohAddresses),
			"error":     errText(dohErr),
		},
	}
	return marshalPretty(payload)
}

func compareVerdict(system []string, systemErr error, doh []string, dohErr error) string {
	switch {
	case systemErr != nil && dohErr != nil:
		return "both resolvers failed"
	case systemErr != nil && len(doh) > 0:
		return "the system resolver failed but DoH resolved the name; DNS is blocked or poisoned"
	case len(system) > 0 && dohErr == nil && len(doh) == 0:
		return "the system resolver answered but DoH returned nothing; a local override or hosts entry"
	case len(system) == 0 && len(doh) == 0:
		return "the name did not resolve"
	case !sameSet(system, doh):
		return "the resolvers disagree; a block, a hijack or an anycast difference"
	default:
		return "the resolvers agree"
	}
}

func sameSet(a, b []string) bool {
	aset := map[string]bool{}
	for _, value := range a {
		aset[value] = true
	}
	for _, value := range b {
		if !aset[value] {
			return false
		}
	}
	return len(a) == len(b)
}

func sortedCopy(values []string) []string {
	out := append([]string(nil), values...)
	sort.Strings(out)
	return out
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func marshalPretty(value any) (string, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func argString(args map[string]any, key string) string {
	if args == nil {
		return ""
	}
	value, ok := args[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return fmt.Sprintf("%v", typed)
	}
}

func argInt(args map[string]any, key string, fallback, min, max int) int {
	if args == nil {
		return fallback
	}
	value, ok := args[key]
	if !ok {
		return fallback
	}
	number := 0
	switch typed := value.(type) {
	case float64:
		number = int(typed)
	case int:
		number = typed
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		if err != nil {
			return fallback
		}
		number = parsed
	default:
		return fallback
	}
	if number < min {
		return min
	}
	if number > max {
		return max
	}
	return number
}
