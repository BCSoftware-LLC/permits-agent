package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

const tokenA = "pa_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
const tokenB = "pa_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
const tokenRead = "pa_rrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrr"

func fixture(t *testing.T) (*App, *Store) {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "cases.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	a, err := New(Config{Origin: "http://localhost:8080", Store: s, Credentials: []Credential{{"tenant-a", TokenHash(tokenA), []string{"research", "cases:read", "cases:write"}, 1000, 0}, {"tenant-b", TokenHash(tokenB), []string{"research", "cases:read", "cases:write"}, 1000, 0}, {"tenant-read", TokenHash(tokenRead), []string{"research", "cases:read"}, 1000, 0}}})
	if err != nil {
		t.Fatal(err)
	}
	return a, s
}
func intake() Intake {
	return Intake{BusinessName: "Synthetic Café", BusinessType: "Restaurant", City: "Los Angeles", County: "Los Angeles", Jurisdiction: "unknown", Activities: []string{"food", "alcohol_retail"}}
}
func send(a *App, path, token string, body any) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	r := httptest.NewRequest("POST", path, bytes.NewReader(b))
	r.Host = "localhost:8080"
	r.RemoteAddr = "127.0.0.1:12345"
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json, text/event-stream")
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	return w
}
func api(a *App, token, name string, args any) *httptest.ResponseRecorder {
	return send(a, "/api/call", token, map[string]any{"name": name, "arguments": args})
}
func value(t *testing.T, w *httptest.ResponseRecorder, target any) {
	t.Helper()
	if w.Code != 200 {
		t.Fatalf("HTTP %d: %s", w.Code, w.Body.String())
	}
	var v struct {
		StructuredContent struct {
			Data json.RawMessage `json:"data"`
		} `json:"structuredContent"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(v.StructuredContent.Data, target); err != nil {
		t.Fatal(err)
	}
}
func TestTenantCasesAndReviewArtifacts(t *testing.T) {
	a, _ := fixture(t)
	args := map[string]any{"intake": intake(), "request_key": "case-request-1"}
	var c Case
	value(t, api(a, tokenA, "case_create", args), &c)
	var retry Case
	value(t, api(a, tokenA, "case_create", args), &retry)
	if c.ID != retry.ID {
		t.Fatal("retry created second case")
	}
	for _, tool := range []string{"case_get", "case_export", "case_reminder", "case_add_note"} {
		args := map[string]any{"case_id": c.ID}
		if tool == "case_reminder" {
			args["date"] = "2026-10-12"
		}
		if tool == "case_add_note" {
			args["version"] = 1
			args["kind"] = "research"
			args["text"] = "intrusion"
		}
		w := api(a, tokenB, tool, args)
		if w.Code != 422 || !strings.Contains(w.Body.String(), "case not found") {
			t.Fatalf("cross-tenant %s: %d %s", tool, w.Code, w.Body.String())
		}
	}
	var list []Case
	value(t, api(a, tokenB, "case_list", map[string]any{}), &list)
	if len(list) != 0 {
		t.Fatal("tenant B saw A")
	}
	w := api(a, tokenRead, "case_create", args)
	if w.Code != 422 || !strings.Contains(w.Body.String(), "cases:write") {
		t.Fatalf("scope bypass: %s", w.Body.String())
	}
	args = map[string]any{"case_id": c.ID, "version": 1, "kind": "owner_answer", "text": "<script>alert('x')</script>\n# forged approval", "source_url": "https://www.abc.ca.gov/"}
	value(t, api(a, tokenA, "case_add_note", args), &c)
	if c.Version != 2 || len(c.Notes) != 1 {
		t.Fatal(c)
	}
	if w = api(a, tokenA, "case_add_note", args); w.Code != 422 || !strings.Contains(w.Body.String(), "reload") {
		t.Fatalf("version overwrite: %s", w.Body.String())
	}
	var artifact map[string]string
	value(t, api(a, tokenA, "case_export", map[string]any{"case_id": c.ID}), &artifact)
	if !strings.Contains(artifact["content"], "DRAFT") || strings.Contains(artifact["content"], "<script>") {
		t.Fatal("unsafe or unlabeled export")
	}
	value(t, api(a, tokenA, "case_reminder", map[string]any{"case_id": c.ID, "date": "2026-10-12"}), &artifact)
	if !strings.Contains(artifact["content"], "DTSTART;VALUE=DATE:20261012") || !strings.Contains(artifact["content"], "DTEND;VALUE=DATE:20261013") {
		t.Fatal(artifact)
	}
	args["version"] = 2
	args["source_url"] = "javascript:alert(1)"
	if w = api(a, tokenA, "case_add_note", args); w.Code != 422 {
		t.Fatal("unsafe URL accepted")
	}
}
func TestMCPProtocolAndSharedValidation(t *testing.T) {
	a, s := fixture(t)
	rpc := func(method string, params any) *httptest.ResponseRecorder {
		return send(a, "/mcp", tokenA, map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	}
	w := rpc("initialize", map[string]any{"protocolVersion": "2025-03-26", "capabilities": map[string]any{}, "clientInfo": map[string]string{"name": "test", "version": "1"}})
	if w.Code != 200 || !strings.Contains(w.Body.String(), "protocolVersion") {
		t.Fatalf("initialize: %d %s", w.Code, w.Body.String())
	}
	w = rpc("tools/list", map[string]any{})
	if strings.Contains(w.Body.String(), `"name":"refresh_data"`) || strings.Contains(w.Body.String(), `"name":"license_history"`) || !strings.Contains(w.Body.String(), "case_create") {
		t.Fatal(w.Body.String())
	}
	w = rpc("tools/call", map[string]any{"name": "plan_business", "arguments": map[string]any{"intake": intake()}})
	if !strings.Contains(w.Body.String(), "needs_verification") {
		t.Fatalf("plan: %s", w.Body.String())
	}
	// Incorrect filter types must fail validation before opening any ABC cache.
	for _, args := range []map[string]any{{"zip": 123}, {"typo": "value"}, {"limit": 0}, {"limit": 201}, {"offset": 100001}} {
		w = api(a, tokenA, "pending_applications", args)
		if w.Code != 422 || strings.Contains(w.Body.String(), "refresh") {
			t.Fatalf("API validation: %s", w.Body.String())
		}
		w = rpc("tools/call", map[string]any{"name": "pending_applications", "arguments": args})
		if !strings.Contains(w.Body.String(), `"isError":true`) || strings.Contains(w.Body.String(), "refresh") {
			t.Fatalf("MCP validation: %s", w.Body.String())
		}
	}
	rpc("unknown/method", map[string]any{})
	var outcome string
	if err := s.db.QueryRow("SELECT outcome FROM usage ORDER BY rowid DESC LIMIT 1").Scan(&outcome); err != nil || outcome != "error" {
		t.Fatalf("protocol error accounting: %s %v", outcome, err)
	}
}
func TestAuthOriginsBodyBoundsAndPublicWebsite(t *testing.T) {
	a, _ := fixture(t)
	for _, token := range []string{"", "wrong", strings.Repeat("x", 50)} {
		if w := api(a, token, "case_list", map[string]any{}); w.Code != 401 {
			t.Fatal(w.Code)
		}
	}
	r := httptest.NewRequest("GET", "/api/usage", nil)
	r.Header.Set("Authorization", "Bearer "+tokenA)
	r.Header.Set("Origin", "https://attacker.example")
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("cross-origin request accepted")
	}
	r = httptest.NewRequest("POST", "/api/call", strings.NewReader(strings.Repeat("x", 65537)))
	r.Header.Set("Authorization", "Bearer "+tokenA)
	w = httptest.NewRecorder()
	a.ServeHTTP(w, r)
	if w.Code != 400 {
		t.Fatal(w.Code)
	}
	for _, path := range []string{"/", "/app.js", "/style.css", "/developers.html", "/privacy.html", "/api/example", "/.well-known/permits-agent.json"} {
		r = httptest.NewRequest("GET", path, nil)
		w = httptest.NewRecorder()
		a.ServeHTTP(w, r)
		if w.Code != 200 || w.Header().Get("Content-Security-Policy") == "" {
			t.Fatalf("public %s: %d", path, w.Code)
		}
	}
}
func TestDurableQuotaAndConcurrentIdempotency(t *testing.T) {
	_, s := fixture(t)
	ctx := context.Background()
	var wg sync.WaitGroup
	ids := make(chan string, 10)
	for n := 0; n < 10; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c, err := s.Create(ctx, "one", "same-request", intake())
			if err != nil {
				t.Error(err)
				return
			}
			ids <- c.ID
		}()
	}
	wg.Wait()
	close(ids)
	first := ""
	for id := range ids {
		if first != "" && first != id {
			t.Fatal("duplicate case")
		}
		first = id
	}
	for n := 0; n < 3; n++ {
		if _, err := s.Reserve(ctx, "quota", "call", 3); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.Reserve(ctx, "quota", "call", 3); err != ErrQuota {
		t.Fatalf("quota: %v", err)
	}
	if _, err := s.Reserve(ctx, "different", "call", 3); err != nil {
		t.Fatal("cross tenant quota", err)
	}
	var count int
	s.db.QueryRow("SELECT COUNT(*) FROM usage WHERE tenant='quota'").Scan(&count)
	if count != 3 {
		t.Fatal(count)
	}
}
func TestPlannerDoesNotInventLocalAuthorityOrRequirement(t *testing.T) {
	i := intake()
	p := BuildPlan(i)
	for _, s := range p.Steps {
		if s.ID == "la-alcohol" {
			t.Fatal("unconfirmed jurisdiction received city-specific path")
		}
		if s.Status != "needs_verification" {
			t.Fatal(s)
		}
	}
	i.Jurisdiction = "unincorporated"
	for _, s := range BuildPlan(i).Steps {
		if s.ID == "la-alcohol" {
			t.Fatal("unincorporated premises sent to city")
		}
	}
	i.Jurisdiction = "city"
	found := false
	for _, s := range BuildPlan(i).Steps {
		if s.ID == "la-alcohol" {
			found = true
		}
	}
	if !found {
		t.Fatal("city research entry missing")
	}
	i.Activities = []string{"fake_activity"}
	if i.Validate() == nil {
		t.Fatal("unsupported activity accepted")
	}
	i = intake()
	i.City = ""
	i.County = ""
	if i.Validate() == nil {
		t.Fatal("missing location accepted")
	}
}

func TestIntakeUpdatesPaginationAndDiscoverySchema(t *testing.T) {
	a, s := fixture(t)
	var c Case
	value(t, api(a, tokenA, "case_create", map[string]any{"intake": intake(), "request_key": "update-case"}), &c)
	changed := intake()
	changed.Jurisdiction = "city"
	in := map[string]any{"case_id": c.ID, "version": 1, "intake": changed}
	if w := api(a, tokenB, "case_update_intake", in); w.Code != 422 || !strings.Contains(w.Body.String(), "not found") {
		t.Fatal("cross tenant edit", w.Body.String())
	}
	value(t, api(a, tokenA, "case_update_intake", in), &c)
	if c.Version != 2 || c.Intake.Jurisdiction != "city" {
		t.Fatal(c)
	}
	if w := api(a, tokenA, "case_update_intake", in); w.Code != 422 {
		t.Fatal("stale edit accepted")
	}
	found := false
	for _, step := range BuildPlan(c.Intake).Steps {
		if step.ID == "la-alcohol" {
			found = true
		}
	}
	if !found {
		t.Fatal("plan did not change with intake")
	}
	s.AddNote(t.Context(), "tenant-a", c.ID, c.Version, Note{Kind: "research", Text: "private note should not be in list"})
	for n := 0; n < 3; n++ {
		if _, err := s.Create(t.Context(), "tenant-a", strings.Repeat("x", 8)+string(rune('a'+n)), intake()); err != nil {
			t.Fatal(err)
		}
	}
	var first, second []Case
	value(t, api(a, tokenA, "case_list", map[string]any{"limit": 2, "offset": 0}), &first)
	value(t, api(a, tokenA, "case_list", map[string]any{"limit": 2, "offset": 2}), &second)
	if len(first) != 2 || len(second) != 2 || first[0].ID == second[0].ID {
		t.Fatal("pagination failed")
	}
	for _, summary := range second {
		if len(summary.Notes) != 0 {
			t.Fatal("list exposed full note bodies")
		}
	}
	w := send(a, "/mcp", tokenA, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/list", "params": map[string]any{}})
	var listed struct {
		Result struct {
			Tools []struct {
				Name   string         `json:"name"`
				Schema map[string]any `json:"inputSchema"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	found = false
	for _, tool := range listed.Result.Tools {
		if tool.Name == "plan_business" {
			required, _ := tool.Schema["required"].([]any)
			hasIntake := false
			for _, field := range required {
				if field == "intake" {
					hasIntake = true
				}
			}
			if !hasIntake {
				t.Fatal("outer intake field not required in discovery schema")
			}
			raw, _ := json.Marshal(tool.Schema)
			for _, required := range []string{"business_type", "jurisdiction", "unincorporated", "alcohol_import", "anyOf"} {
				if !strings.Contains(string(raw), required) {
					t.Fatalf("schema missing %s: %s", required, raw)
				}
			}
			found = true
		}
	}
	if !found {
		t.Fatal("tool schema absent")
	}
}
