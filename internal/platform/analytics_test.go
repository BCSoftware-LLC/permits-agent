package platform

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func analyticsGet(a *App, token, query string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("GET", "/api/analytics"+query, nil)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	return w
}

func TestAnalyticsIdentityScopeAndProtocolTraffic(t *testing.T) {
	a, s := fixture(t)
	a.config.Credentials[0].Client = &ClientIdentity{ID: "muse", ActorKind: "agent", AgentFamily: "muse"}
	a.config.Credentials[0].Scopes = append(a.config.Credentials[0].Scopes, "analytics:read")
	a.config.Credentials[1].Client = &ClientIdentity{ID: "website", ActorKind: "human"}
	a.config.Credentials[1].Scopes = append(a.config.Credentials[1].Scopes, "analytics:all")
	r := httptest.NewRequest("POST", "/api/call", strings.NewReader(`{"name":"source_catalog","arguments":{}}`))
	r.Header.Set("Authorization", "Bearer "+tokenA)
	r.Header.Set("User-Agent", "ChatGPT")
	r.Header.Set("X-Agent-Family", "cursor")
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	send(a, "/mcp", tokenA, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{"protocolVersion": "2025-03-26", "capabilities": map[string]any{}, "clientInfo": map[string]any{"name": "fake-client", "version": "1"}}})
	api(a, tokenB, "source_catalog", map[string]any{})
	if w := analyticsGet(a, tokenRead, ""); w.Code != 403 {
		t.Fatal("unscoped analytics", w.Code)
	}
	if w := analyticsGet(a, tokenA, "?scope=platform"); w.Code != 403 {
		t.Fatal("tenant escaped", w.Code)
	}
	w = analyticsGet(a, tokenA, "")
	var report AnalyticsReport
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &report) != nil {
		t.Fatal(w.Code, w.Body.String())
	}
	if report.Totals.Requests != 2 || report.Totals.ToolCalls != 1 {
		t.Fatalf("protocol counted as tool or report counted itself: %+v", report.Totals)
	}
	for _, row := range report.Rows {
		if row.ClientID != "muse" || row.AgentFamily != "muse" || row.ActorKind != "agent" {
			t.Fatalf("spoofed/leaked identity: %+v", row)
		}
	}
	w = analyticsGet(a, tokenB, "?scope=platform")
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &report) != nil || report.Totals.Requests != 3 {
		t.Fatal(w.Code, w.Body.String())
	}
	for _, q := range []string{"?days=0", "?days=91", "?scope=tenant&tenant=tenant-b", "?days=1&days=2"} {
		if w := analyticsGet(a, tokenA, q); w.Code != 400 {
			t.Fatal(q, w.Code)
		}
	}
	// Historical rows cannot be assigned retroactively to Muse.
	if _, err := s.db.Exec("INSERT INTO usage VALUES('legacy','tenant-a','/api/call',?,'ok')", now()); err != nil {
		t.Fatal(err)
	}
	w = analyticsGet(a, tokenA, "")
	json.Unmarshal(w.Body.Bytes(), &report)
	found := false
	for _, row := range report.Rows {
		if row.ClientID == "unknown" {
			found = true
		}
	}
	if !found {
		t.Fatal("legacy requests falsely classified")
	}
}

func TestToolAnalyticsRecordsFailureWithoutPayload(t *testing.T) {
	a, s := fixture(t)
	a.config.Credentials[0].Scopes = append(a.config.Credentials[0].Scopes, "analytics:read")
	api(a, tokenA, "case_get", map[string]any{"case_id": "synthetic-secret-must-not-be-logged"})
	var report AnalyticsReport
	w := analyticsGet(a, tokenA, "")
	json.Unmarshal(w.Body.Bytes(), &report)
	if report.Totals.ToolCalls != 1 || report.Totals.ToolFailures != 1 {
		t.Fatal(w.Body.String())
	}
	var payload string
	if err := s.db.QueryRow("SELECT tool||outcome FROM tool_usage LIMIT 1").Scan(&payload); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(payload, "synthetic-secret") || strings.Contains(w.Body.String(), "synthetic-secret") {
		t.Fatal("payload logged")
	}
	// A failed telemetry reservation must prevent execution of a mutating tool.
	s.db.Exec("DROP TABLE tool_usage")
	api(a, tokenA, "case_create", map[string]any{"intake": intake(), "request_key": "must-not-create"})
	var count int
	s.db.QueryRow("SELECT COUNT(*) FROM cases").Scan(&count)
	if count != 0 {
		t.Fatal("tool ran without audit reservation")
	}
}

func TestClientIdentityConfiguration(t *testing.T) {
	a, _ := fixture(t)
	for _, client := range []*ClientIdentity{{ID: "muse", ActorKind: "robot"}, {ID: "", ActorKind: "agent"}, {ID: "customer@example.com", ActorKind: "human"}, {ID: "muse", ActorKind: "human", AgentFamily: "muse"}} {
		cfg := a.config
		cfg.Credentials = append([]Credential{}, cfg.Credentials...)
		cfg.Credentials[0].Client = client
		if _, err := New(cfg); err == nil {
			t.Fatalf("accepted invalid client: %+v", client)
		}
	}
}

func TestModelAnalyticsPreservesCallerAndInternalExecutor(t *testing.T) {
	a, s := fixture(t)
	client := &ClientIdentity{ID: "website", ActorKind: "human"}
	requestID, err := s.reserveAttributed(t.Context(), "tenant-a", "/api/agent", 1000, client)
	if err != nil {
		t.Fatal(err)
	}
	state := &requestState{credential: a.config.Credentials[0], requestID: requestID, operation: "/api/agent"}
	ctx := context.WithValue(t.Context(), identityKey{}, state)
	modelID, err := s.reserveModel(ctx, "tenant-a", "synthetic-model", 10)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.finishModel(ctx, modelID, "synthetic-response", 123, 45); err != nil {
		t.Fatal(err)
	}
	report, err := s.Analytics(ctx, "tenant-a", "tenant", 30)
	if err != nil {
		t.Fatal(err)
	}
	if report.Totals.ModelCalls != 1 || report.Totals.InputTokens != 123 || report.Totals.OutputTokens != 45 {
		t.Fatalf("model accounting %+v", report)
	}
	found := false
	for _, row := range report.Rows {
		if row.Kind == "model" {
			found = true
			if row.ClientID != "website" || row.ActorKind != "human" || row.Executor != "owner_assistant" {
				t.Fatalf("origin confused with executor %+v", row)
			}
		}
	}
	if !found {
		t.Fatal("model attribution missing")
	}
}

func TestAnalyticsQueriesUseBoundedHistoryIndexes(t *testing.T) {
	_, s := fixture(t)
	for _, scope := range []string{"tenant", "platform"} {
		end := time.Now().UTC()
		cte, args := analyticsQuery("tenant-a", scope, end.AddDate(0, 0, -7), end)
		rows, err := s.db.Query("EXPLAIN QUERY PLAN "+cte+"SELECT COUNT(*) FROM filtered", args...)
		if err != nil {
			t.Fatal(err)
		}
		plan := ""
		for rows.Next() {
			var id, parent, unused int
			var detail string
			if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
				t.Fatal(err)
			}
			plan += detail + "\n"
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			t.Fatal(err)
		}
		suffix := "_date"
		if scope == "tenant" {
			suffix = "_tenant_date"
		}
		for _, table := range []string{"usage", "tool_usage", "model_usage"} {
			if !strings.Contains(plan, "USING INDEX "+table+suffix) {
				t.Fatalf("%s report lost bounded %s index:\n%s", scope, table, plan)
			}
		}
	}
}
