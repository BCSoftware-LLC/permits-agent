package mcpserver

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/BCSoftware-LLC/permits-agent/internal/abc"
)

func TestStdioMCPProtocolSurfaceToolsResourcesAndValidation(t *testing.T) {
	bin, cache := buildProtocolFixture(t)
	c := startProtocolServer(t, bin, cache)

	init := c.request(t, "initialize", map[string]any{"protocolVersion": "2024-11-05", "capabilities": map[string]any{}, "clientInfo": map[string]any{"name": "protocol-test", "version": "0"}})
	mustHave(t, init, "protocolVersion")
	c.notify(t, "notifications/initialized", map[string]any{})

	toolsResult := c.request(t, "tools/list", map[string]any{})
	tools := asSlice(t, toolsResult["tools"])
	var names []string
	toolByName := map[string]map[string]any{}
	for _, raw := range tools {
		m := asMap(t, raw)
		name := m["name"].(string)
		names = append(names, name)
		toolByName[name] = m
	}
	want := []string{"search_licenses", "get_license", "pending_applications", "expiring_licenses", "licenses_at_address", "overdue_licenses", "status_overview", "licenses_in_area", "license_stats", "license_type_description", "search_forms", "license_requirements", "latest_news", "fee_surcharges", "refresh_data", "license_statuses", "licenses_by_area", "license_types", "abc_forms", "abc_fees", "abc_news", "abc_requirements", "license_history"}
	gotSorted, wantSorted := append([]string(nil), names...), append([]string(nil), want...)
	sort.Strings(gotSorted)
	sort.Strings(wantSorted)
	if !reflect.DeepEqual(gotSorted, wantSorted) {
		t.Fatalf("tools/list names = %#v, want set %#v", names, want)
	}
	referenceTools := map[string]bool{"license_type_description": true, "search_forms": true, "license_requirements": true, "latest_news": true, "fee_surcharges": true, "license_types": true, "abc_forms": true, "abc_fees": true, "abc_news": true, "abc_requirements": true}
	for _, name := range names {
		ann := asMap(t, toolByName[name]["annotations"])
		if name == "refresh_data" {
			if ann["openWorldHint"] != true || ann["destructiveHint"] != false || ann["idempotentHint"] != true {
				t.Fatalf("refresh_data annotations = %#v", ann)
			}
			props := asMap(t, asMap(t, toolByName[name]["inputSchema"])["properties"])
			if _, ok := props["force"]; !ok {
				t.Errorf("refresh_data toolArgs accepts force but inputSchema omits it: %#v", props)
			}
			continue
		}
		if name == "license_history" && ann["readOnlyHint"] == true {
			t.Errorf("license_history is annotated readonly despite local history snapshot writes: %#v", ann)
		}
		if referenceTools[name] && ann["openWorldHint"] != true {
			t.Errorf("%s is annotated closed-world despite reference fetch capability: %#v", name, ann)
		}
		if !referenceTools[name] && name != "license_history" && (ann["readOnlyHint"] != true || ann["destructiveHint"] != false || ann["idempotentHint"] != true || ann["openWorldHint"] != false) {
			t.Fatalf("%s annotations = %#v", name, ann)
		}
	}

	resources := asSlice(t, c.request(t, "resources/list", map[string]any{})["resources"])
	var uris []string
	for _, raw := range resources {
		uris = append(uris, asMap(t, raw)["uri"].(string))
	}
	wantURIs := []string{"abc://stats", "abc://history", "abc://license-types", "abc://fees"}
	gotURIs, sortedWantURIs := append([]string(nil), uris...), append([]string(nil), wantURIs...)
	sort.Strings(gotURIs)
	sort.Strings(sortedWantURIs)
	if !reflect.DeepEqual(gotURIs, sortedWantURIs) {
		t.Fatalf("resources/list uris = %#v", uris)
	}
	for _, uri := range uris {
		res := c.request(t, "resources/read", map[string]any{"uri": uri})
		contents := asSlice(t, res["contents"])
		if len(contents) != 1 {
			t.Fatalf("%s contents len=%d", uri, len(contents))
		}
		text, _ := asMap(t, contents[0])["text"].(string)
		if !json.Valid([]byte(text)) {
			t.Fatalf("%s resource returned non-json text: %q", uri, text)
		}
	}

	validArgs := map[string]map[string]any{
		"search_licenses":      {"query": "OWNER", "limit": 2, "offset": 0, "status": "ACTIVE", "county": "LOS ANGELES", "city": "HOLLYWOOD", "license_type": "47", "lic_or_app": "LIC", "expire_year": futureYear()},
		"get_license":          {"file_number": "00677768"},
		"pending_applications": {"city": "HOLLYWOOD", "limit": 2, "offset": 0},
		"expiring_licenses":    {"city": "HOLLYWOOD", "days": 36500, "limit": 2, "offset": 0},
		"licenses_at_address":  {"fragment": "SELMA", "match_mail": true, "limit": 2, "offset": 0},
		"overdue_licenses":     {"city": "HOLLYWOOD", "min_days": 0, "limit": 2, "offset": 0},
		"status_overview":      {}, "licenses_in_area": {"city": "HOLLYWOOD", "limit": 2, "offset": 0}, "license_stats": {},
		"license_type_description": {"code": "47"}, "search_forms": {"query": "ABC-217", "offline": true},
		"license_requirements": {"license_type": "47", "action": "new", "offline": true}, "latest_news": {"feed": "all", "limit": 2, "offline": true},
		"fee_surcharges": {"offline": true}, "license_statuses": {}, "licenses_by_area": {"city": "HOLLYWOOD", "limit": 2, "offset": 0},
		"license_types": {"code": "47", "offline": true}, "abc_forms": {"search": "ABC-217", "offline": true}, "abc_fees": {"offline": true},
		"abc_news": {"feed": "news", "limit": 1, "offline": true}, "abc_requirements": {"code": "47", "action": "transfer", "offline": true}, "license_history": {},
	}
	for _, name := range names {
		if name == "refresh_data" {
			continue
		}
		got := callToolJSON(t, c, name, validArgs[name], false)
		if got == nil {
			t.Fatalf("%s returned nil JSON", name)
		}
	}

	// Every record result is self-contained even when the mirror is stale or empty.
	for _, query := range []string{"OWNER", "NO-MATCH-UNLIKELY"} {
		r := c.request(t, "tools/call", map[string]any{"name": "search_licenses", "arguments": map[string]any{"query": query}})
		structured := asMap(t, r["structuredContent"])
		p := asMap(t, structured["provenance"])
		if p["source_url"] != abc.ExportURL || p["export_date"] == "" || p["stale"] != true {
			t.Fatalf("missing stale source evidence: %#v", p)
		}
		mustHave(t, structured, "data")
	}
	// Legacy single-type tool must not silently return the whole reference.
	assertToolError(t, c, "license_type_description", map[string]any{"offline": true})
	required := asSlice(t, asMap(t, toolByName["license_type_description"]["inputSchema"])["required"])
	if !reflect.DeepEqual(required, []any{"code"}) {
		t.Errorf("required schema=%v", required)
	}
	// Explicit data semantics: multi-row get, filters, zero limit, and offset.
	if res := c.request(t, "tools/call", map[string]any{"name": "license_type_description", "arguments": map[string]any{"code": "47", "offline": true}}); res["isError"] == true {
		t.Errorf("license_type_description rejects schema-declared offline arg: %#v", res)
	}
	if rows := callToolJSON(t, c, "get_license", map[string]any{"file_number": "00677768"}, false).([]any); len(rows) != 2 {
		t.Fatalf("get_license multi-row len=%d", len(rows))
	}
	if rows := callToolJSON(t, c, "licenses_in_area", map[string]any{"city": "HOLLYWOOD", "status": "PEND", "limit": 10}, false).([]any); len(rows) != 1 || asMap(t, rows[0])["status"] != "PEND" {
		t.Fatalf("filter result=%#v", rows)
	}
	if rows := callToolJSON(t, c, "search_licenses", map[string]any{"query": "OWNER", "limit": 0}, false).([]any); len(rows) != 3 {
		t.Errorf("zero limit len=%d", len(rows))
	}
	if rows := callToolJSON(t, c, "search_licenses", map[string]any{"query": "OWNER", "limit": 10, "offset": 1}, false).([]any); len(rows) != 2 {
		t.Errorf("offset len=%d", len(rows))
	}

	badValues := []any{"x", true, 1.5, -1, float64(9223372036854775808), 1e100, nil}
	for _, bad := range badValues {
		assertToolError(t, c, "search_licenses", map[string]any{"query": "OWNER", "limit": bad})
	}
	for _, bad := range []any{true, 1, nil} {
		assertToolError(t, c, "search_licenses", map[string]any{"query": bad})
	}
	for _, bad := range []any{"true", 1, nil} {
		assertToolError(t, c, "licenses_at_address", map[string]any{"fragment": "SELMA", "match_mail": bad})
	}
	assertToolError(t, c, "search_licenses", map[string]any{"query": "OWNER", "unknown": "x"})
	assertToolError(t, c, "search_licenses", nil)

	missing := startProtocolServer(t, bin, filepath.Join(t.TempDir(), "missing-cache"))
	assertToolError(t, missing, "license_stats", map[string]any{})
	assertToolError(t, missing, "license_types", map[string]any{"offline": true})
}

func TestRefreshDataDeclaredExternalAndDoesNotCrashOffline(t *testing.T) {
	bin, cache := buildProtocolFixture(t)
	c := startProtocolServer(t, bin, cache)
	_ = c.request(t, "initialize", map[string]any{"protocolVersion": "2024-11-05", "capabilities": map[string]any{}, "clientInfo": map[string]any{"name": "protocol-test", "version": "0"}})
	c.notify(t, "notifications/initialized", map[string]any{})
	tools := asSlice(t, c.request(t, "tools/list", map[string]any{})["tools"])
	for _, raw := range tools {
		m := asMap(t, raw)
		if m["name"] == "refresh_data" {
			ann := asMap(t, m["annotations"])
			if ann["openWorldHint"] != true {
				t.Fatalf("refresh_data must declare external access: %#v", ann)
			}
		}
	}
	result := asMap(t, callToolJSON(t, c, "refresh_data", map[string]any{"force": false}, false))
	if result["refreshed"] != false {
		t.Fatalf("force false did not reuse fresh cache: %#v", result)
	}
	assertToolError(t, c, "refresh_data", map[string]any{"force": true})
}

type protocolClient struct {
	t    *testing.T
	cmd  *exec.Cmd
	enc  *json.Encoder
	scan *bufio.Scanner
	next int
}

func startProtocolServer(t *testing.T, bin, cache string) *protocolClient {
	t.Helper()
	cmd := exec.Command(bin)
	cmd.Env = append(os.Environ(), "ABC_AGENT_CACHE="+cache, "HTTPS_PROXY=http://127.0.0.1:1", "HTTP_PROXY=http://127.0.0.1:1", "NO_PROXY=127.0.0.1,localhost")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	go io.Copy(io.Discard, stderr)
	c := &protocolClient{t: t, cmd: cmd, enc: json.NewEncoder(stdin), scan: bufio.NewScanner(stdout), next: 1}
	c.scan.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		done := make(chan struct{})
		go func() { _ = cmd.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Log("mcp process wait timed out")
		}
	})
	return c
}

func (c *protocolClient) request(t *testing.T, method string, params any) map[string]any {
	t.Helper()
	c.next++
	id := c.next
	if err := c.enc.Encode(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params}); err != nil {
		t.Fatal(err)
	}
	deadline := time.After(5 * time.Second)
	for {
		ch := make(chan []byte, 1)
		go func() {
			if c.scan.Scan() {
				ch <- append([]byte(nil), c.scan.Bytes()...)
			} else {
				ch <- nil
			}
		}()
		select {
		case line := <-ch:
			if line == nil {
				t.Fatalf("server closed while waiting for %s: %v", method, c.scan.Err())
			}
			var msg map[string]any
			if err := json.Unmarshal(line, &msg); err != nil {
				t.Fatalf("bad json-rpc line %q: %v", line, err)
			}
			if msg["id"] != float64(id) {
				continue
			}
			if e, ok := msg["error"]; ok {
				t.Fatalf("%s json-rpc error: %#v", method, e)
			}
			return asMap(t, msg["result"])
		case <-deadline:
			t.Fatalf("timeout waiting for %s", method)
		}
	}
}
func (c *protocolClient) notify(t *testing.T, method string, params any) {
	t.Helper()
	if err := c.enc.Encode(map[string]any{"jsonrpc": "2.0", "method": method, "params": params}); err != nil {
		t.Fatal(err)
	}
}

func callToolJSON(t *testing.T, c *protocolClient, name string, args any, wantErr bool) any {
	t.Helper()
	res := c.request(t, "tools/call", map[string]any{"name": name, "arguments": args})
	if got, _ := res["isError"].(bool); got != wantErr {
		t.Fatalf("%s isError=%v want %v result=%#v", name, got, wantErr, res)
	}
	content := asSlice(t, res["content"])
	if len(content) != 1 {
		t.Fatalf("%s content len=%d", name, len(content))
	}
	text, _ := asMap(t, content[0])["text"].(string)
	if !wantErr && name == "license_type_description" {
		if text != "On-Sale General - Eating Place" {
			t.Fatalf("legacy description changed: %q", text)
		}
		mustHave(t, asMap(t, res["_meta"]), "provenance")
		return text
	}
	if !wantErr && !json.Valid([]byte(text)) {
		t.Fatalf("%s returned non-json text: %q", name, text)
	}
	if wantErr {
		return text
	}
	var out any
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("%s bad json: %v", name, err)
	}
	return out
}
func assertToolError(t *testing.T, c *protocolClient, name string, args any) {
	t.Helper()
	res := c.request(t, "tools/call", map[string]any{"name": name, "arguments": args})
	if got, _ := res["isError"].(bool); !got {
		t.Errorf("%s accepted invalid args %#v: %#v", name, args, res)
		return
	}
	content := asSlice(t, res["content"])
	if len(content) != 1 {
		t.Errorf("%s error content len=%d", name, len(content))
		return
	}
	text, _ := asMap(t, content[0])["text"].(string)
	if strings.Contains(strings.ToLower(text), "panic") {
		t.Errorf("%s panic error: %s", name, text)
	}
}

func buildProtocolFixture(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "abc-agent-mcp")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/abc-agent-mcp")
	cmd.Dir = "../.."
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	cache := filepath.Join(dir, "cache")
	if err := os.MkdirAll(cache, 0700); err != nil {
		t.Fatal(err)
	}
	csvPath := filepath.Join(cache, "ABC-DailyDataExport.csv")
	if err := os.WriteFile(csvPath, []byte(protocolFixtureCSV()), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := abc.ImportCSV(context.Background(), csvPath, filepath.Join(cache, "abc.sqlite")); err != nil {
		t.Fatal(err)
	}
	writeJSON(t, filepath.Join(cache, "license_types.json"), map[string]any{"fetched": time.Now().UTC(), "data": map[string]string{"21": "Off-Sale General", "47": "On-Sale General - Eating Place"}})
	forms := []map[string]string{{"number": "ABC-217", "title": "License Application", "revised": "2026", "url": "https://www.abc.ca.gov/form217"}, {"number": "ABC-227", "title": "Notice of Intended Transfer", "revised": "2026", "url": "https://www.abc.ca.gov/form227"}}
	writeJSON(t, filepath.Join(cache, "forms_index.json"), map[string]any{"fetched": time.Now().UTC(), "data": forms})
	writeJSON(t, filepath.Join(cache, "fees.json"), map[string]any{"fetched": time.Now().UTC(), "data": map[string]any{"surcharges": []map[string]string{{"name": "test", "purpose": "fixture", "amount": "$1", "applies_to": "all"}}, "page_text_excerpt": "cached fees"}})
	news := []map[string]string{{"feed": "news", "title": "Fixture News", "link": "https://www.abc.ca.gov/news", "published": "2026-09-01", "summary": "cached"}}
	adv := []map[string]string{{"feed": "advisories", "title": "Fixture Advisory", "link": "https://www.abc.ca.gov/advisory", "published": "2026-09-02", "summary": "cached"}}
	writeJSON(t, filepath.Join(cache, "news_news.json"), map[string]any{"fetched": time.Now().UTC(), "data": news})
	writeJSON(t, filepath.Join(cache, "news_advisories.json"), map[string]any{"fetched": time.Now().UTC(), "data": adv})
	return bin, cache
}
func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
}
func protocolFixtureCSV() string {
	y := futureYear()
	return fmt.Sprintf("\ufeff\"Updated Sunday 6th of September 2026 03:50:22 AM\"\n\"License Type\",\"File Number\",\"Lic or App\",\"Type Status\",\"Type Orig Iss Date\",\"Expir Date\",\"Fee Codes\",\"Dup Counts\",\"Master Ind\",\"Term in # of Months\",\"Geo Code\",District,\"Primary Name\",\"Prem Addr 1\",\" Prem Addr 2\",\"Prem City\",\" Prem State\",\"Prem Zip\",\"DBA Name\",\"Mail Addr 1\",\"Mail Addr 2\",\"Mail City\",\"Mail State\",\"Mail Zip\",\"Prem County\",\"Prem Census Tract #\"\n47,00677768,LIC,ACTIVE,01-JAN-2020,31-DEC-%s,,,,12,1900,04,\"EXAMPLE OWNER\",\"6417 SELMA AVE\",\" \",HOLLYWOOD,CA,90028,\"EXAMPLE DBA\",,,,,,LOS ANGELES,\n21,00700000,APP,PEND,\" \",\" \",,,,12,1900,04,\"PENDING OWNER\",\"100 TEST ST\",\" \",HOLLYWOOD,CA,90028,\"PENDING DBA\",,,,,,LOS ANGELES,\n47,00677768,LIC,ACTIVE,01-JAN-2020,31-DEC-%s,,,,12,1900,04,\"EXAMPLE OWNER\",\"6417 SELMA AVE\",\" \",HOLLYWOOD,CA,90028,\"EXAMPLE DBA SECOND\",,,,,,LOS ANGELES,\n", y, y)
}
func futureYear() string { return fmt.Sprintf("%d", time.Now().Year()+1) }
func asMap(t *testing.T, v any) map[string]any {
	t.Helper()
	m, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("not object: %#v", v)
	}
	return m
}
func asSlice(t *testing.T, v any) []any {
	t.Helper()
	s, ok := v.([]any)
	if !ok {
		t.Fatalf("not array: %#v", v)
	}
	return s
}
func mustHave(t *testing.T, m map[string]any, k string) {
	t.Helper()
	if _, ok := m[k]; !ok {
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		t.Fatalf("missing %s in keys %v", k, keys)
	}
}
