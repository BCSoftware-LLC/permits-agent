package platform

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"time"

	mcp "github.com/mark3labs/mcp-go/mcp"
)

// ClientIdentity is operator registration, never inferred from a request header
// or MCP clientInfo. It identifies the credential's intended use, not the actual
// human/model behind a call. Use opaque IDs, not names or email addresses.
type ClientIdentity struct {
	ID          string `json:"id"`
	ActorKind   string `json:"actor_kind"`
	AgentFamily string `json:"agent_family,omitempty"`
}

var clientSlug = regexp.MustCompile(`^[a-z0-9][a-z0-9_.-]{0,63}$`)

func clientIdentity(c *ClientIdentity) ClientIdentity {
	if c == nil {
		return ClientIdentity{"unknown", "unknown", "unknown"}
	}
	v := *c
	if v.AgentFamily == "" {
		v.AgentFamily = "unknown"
	}
	return v
}
func validateClient(c *ClientIdentity) error {
	if c == nil {
		return nil
	}
	if !clientSlug.MatchString(c.ID) || c.ID == "unknown" {
		return fmt.Errorf("client id must be an opaque lowercase identifier, not unknown")
	}
	if c.ActorKind != "human" && c.ActorKind != "agent" && c.ActorKind != "service" && c.ActorKind != "unknown" {
		return fmt.Errorf("client actor_kind must be human, agent, service or unknown")
	}
	if c.AgentFamily != "" && (!clientSlug.MatchString(c.AgentFamily) || c.ActorKind != "agent") {
		return fmt.Errorf("agent_family requires an agent client and an opaque identifier")
	}
	return nil
}

type AnalyticsTotals struct {
	Requests        int `json:"requests"`
	RequestFailures int `json:"request_failures_or_incomplete"`
	ToolCalls       int `json:"tool_calls"`
	ToolFailures    int `json:"tool_failures_or_incomplete"`
	ModelCalls      int `json:"model_calls"`
	ModelIncomplete int `json:"model_calls_without_usage"`
	InputTokens     int `json:"input_tokens"`
	OutputTokens    int `json:"output_tokens"`
}
type AnalyticsRow struct {
	Kind         string `json:"kind"`
	ClientID     string `json:"client_id"`
	ActorKind    string `json:"actor_kind"`
	AgentFamily  string `json:"agent_family"`
	Channel      string `json:"channel"`
	Operation    string `json:"operation"`
	Executor     string `json:"executor"`
	Day          string `json:"day_utc"`
	Count        int    `json:"count"`
	Failed       int    `json:"failed_or_incomplete"`
	InputTokens  int    `json:"input_tokens"`
	OutputTokens int    `json:"output_tokens"`
	DurationMS   int64  `json:"duration_ms_total"`
}
type AnalyticsReport struct {
	Scope       string          `json:"scope"`
	WindowStart string          `json:"window_start"`
	WindowEnd   string          `json:"window_end"`
	Totals      AnalyticsTotals `json:"totals"`
	Rows        []AnalyticsRow  `json:"rows"`
	Truncated   bool            `json:"rows_truncated"`
	Notes       []string        `json:"notes"`
}

func (a *App) observeTool(name string, next func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, r mcp.CallToolRequest) (result *mcp.CallToolResult, callErr error) {
		state, ok := ctx.Value(identityKey{}).(*requestState)
		if !ok {
			return mcp.NewToolResultError("authentication required"), nil
		}
		executor := "client"
		if state.operation == "/api/agent" {
			executor = "owner_assistant"
		}
		id := newID()
		start := time.Now()
		_, err := a.config.Store.db.ExecContext(ctx, "INSERT INTO tool_usage VALUES(?,?,?,?,?,?,?,?)", id, state.requestID, state.credential.Tenant, name, executor, now(), "started", 0)
		if err != nil {
			state.failed = true
			return mcp.NewToolResultError("tool accounting unavailable; operation not started"), nil
		}
		defer func() {
			outcome := "ok"
			if callErr != nil || result == nil || result.IsError {
				outcome = "error"
			}
			finishCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_, err := a.config.Store.db.ExecContext(finishCtx, "UPDATE tool_usage SET outcome=?,duration_ms=? WHERE id=?", outcome, time.Since(start).Milliseconds(), id)
			if err != nil {
				state.failed = true
				result = mcp.NewToolResultError("Tool outcome could not be recorded. The operation may have completed; check case state before retrying.")
			}
		}()
		return next(ctx, r)
	}
}

func (a *App) analytics(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		w.Header().Set("Allow", "GET")
		writeJSON(w, 405, map[string]string{"error": "GET required"})
		return
	}
	c := r.Context().Value(identityKey{}).(*requestState).credential
	if !hasScope(c, "analytics:read") && !hasScope(c, "analytics:all") {
		writeJSON(w, 403, map[string]string{"error": "analytics access required"})
		return
	}
	q := r.URL.Query()
	for k, v := range q {
		if (k != "days" && k != "scope") || len(v) != 1 {
			writeJSON(w, 400, map[string]string{"error": "only one days and scope value supported"})
			return
		}
	}
	scope := q.Get("scope")
	if scope == "" {
		scope = "tenant"
	}
	if scope != "tenant" && scope != "platform" {
		writeJSON(w, 400, map[string]string{"error": "scope must be tenant or platform"})
		return
	}
	if scope == "platform" && !hasScope(c, "analytics:all") {
		writeJSON(w, 403, map[string]string{"error": "platform analytics access required"})
		return
	}
	days := 30
	if raw, ok := q["days"]; ok {
		n, err := strconv.Atoi(raw[0])
		if err != nil || n < 1 || n > 90 {
			writeJSON(w, 400, map[string]string{"error": "days must be 1 to 90"})
			return
		}
		days = n
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	report, err := a.config.Store.Analytics(ctx, c.Tenant, scope, days)
	if err != nil {
		writeJSON(w, 503, map[string]string{"error": "analytics unavailable"})
		return
	}
	writeJSON(w, 200, report)
}

// One read transaction keeps totals and grouped details consistent. Request,
// tool and model events are separate populations; joins cannot multiply counts.
func (s *Store) Analytics(ctx context.Context, tenant, scope string, days int) (AnalyticsReport, error) {
	end := time.Now().UTC()
	start := end.AddDate(0, 0, -days)
	report := AnalyticsReport{Scope: scope, WindowStart: start.Format(time.RFC3339Nano), WindowEnd: end.Format(time.RFC3339Nano), Rows: []AnalyticsRow{}, Notes: []string{
		"Only this hosted service's admitted authenticated requests are measured. Public visits, denied admission and local connector runs are not included.",
		"Identity is operator-registered credential purpose, not proof of a human or model. Shared credentials and historical records remain ambiguous; headers cannot label a client.",
		"Requests include protocol and usage calls but exclude analytics views. Tool calls count handler attempts, not filings, successful workflows or unique users.",
		"Model usage covers this service's configured owner assistant only; external agents' model tokens and costs are not observable here.",
		"Incomplete events remain visible. Grouped rows are limited to 2000; totals cover the full window. No prompts, tool arguments, outputs, IP addresses or contact details are recorded in these metrics.",
	}}
	if scope != "tenant" && scope != "platform" || days < 1 || days > 90 {
		return report, fmt.Errorf("invalid analytics scope/window")
	}
	cte, args := analyticsQuery(tenant, scope, start, end)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return report, err
	}
	defer tx.Rollback()
	err = tx.QueryRowContext(ctx, cte+`SELECT
 COALESCE(SUM(kind='request'),0),COALESCE(SUM(CASE WHEN kind='request' THEN failed ELSE 0 END),0),
 COALESCE(SUM(kind='tool'),0),COALESCE(SUM(CASE WHEN kind='tool' THEN failed ELSE 0 END),0),
 COALESCE(SUM(kind='model'),0),COALESCE(SUM(CASE WHEN kind='model' THEN failed ELSE 0 END),0),
 COALESCE(SUM(input_tokens),0),COALESCE(SUM(output_tokens),0) FROM filtered`, args...).Scan(&report.Totals.Requests, &report.Totals.RequestFailures, &report.Totals.ToolCalls, &report.Totals.ToolFailures, &report.Totals.ModelCalls, &report.Totals.ModelIncomplete, &report.Totals.InputTokens, &report.Totals.OutputTokens)
	if err != nil {
		return report, err
	}
	rows, err := tx.QueryContext(ctx, cte+`SELECT kind,client_id,actor_kind,agent_family,channel,operation,executor,substr(created_at,1,10),COUNT(*),SUM(failed),SUM(input_tokens),SUM(output_tokens),SUM(duration_ms)
 FROM filtered GROUP BY kind,client_id,actor_kind,agent_family,channel,operation,executor,substr(created_at,1,10)
 ORDER BY substr(created_at,1,10) DESC,kind,client_id,actor_kind,agent_family,channel,operation,executor LIMIT 2001`, args...)
	if err != nil {
		return report, err
	}
	for rows.Next() {
		var row AnalyticsRow
		if err = rows.Scan(&row.Kind, &row.ClientID, &row.ActorKind, &row.AgentFamily, &row.Channel, &row.Operation, &row.Executor, &row.Day, &row.Count, &row.Failed, &row.InputTokens, &row.OutputTokens, &row.DurationMS); err != nil {
			rows.Close()
			return report, err
		}
		if len(report.Rows) == 2000 {
			report.Truncated = true
			break
		}
		report.Rows = append(report.Rows, row)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return report, err
	}
	return report, tx.Commit()
}

// Predicates are chosen from fixed SQL, with tenant/time values bound. Keep each
// source indexable before its joins; an OR across tenant scope forces scans.
func analyticsQuery(tenant, scope string, start, end time.Time) (string, []any) {
	args := []any{}
	predicate := func(alias string) string {
		value := alias + ".created_at>=? AND " + alias + ".created_at<?"
		args = append(args, start.Format("2006-01-02T15:04:05.000000000Z"), end.Format("2006-01-02T15:04:05.000000000Z"))
		if scope == "tenant" {
			value += " AND " + alias + ".tenant=?"
			args = append(args, tenant)
		}
		return value
	}
	cte := `WITH events AS (
 SELECT 'request' kind,u.tenant,u.created_at,COALESCE(a.client_id,'unknown') client_id,COALESCE(a.actor_kind,'unknown') actor_kind,COALESCE(a.agent_family,'unknown') agent_family,u.operation channel,u.operation operation,'client' executor,CASE WHEN u.outcome='ok' THEN 0 ELSE 1 END failed,0 input_tokens,0 output_tokens,0 duration_ms
 FROM usage u LEFT JOIN usage_attribution a ON a.request_id=u.id WHERE u.operation!='/api/analytics' AND ` + predicate("u") + `
 UNION ALL
 SELECT 'tool',t.tenant,t.created_at,COALESCE(a.client_id,'unknown'),COALESCE(a.actor_kind,'unknown'),COALESCE(a.agent_family,'unknown'),COALESCE(u.operation,'unknown'),t.tool,t.executor,CASE WHEN t.outcome='ok' THEN 0 ELSE 1 END,0,0,t.duration_ms
 FROM tool_usage t LEFT JOIN usage u ON u.id=t.request_id AND u.tenant=t.tenant LEFT JOIN usage_attribution a ON a.request_id=u.id WHERE ` + predicate("t") + `
 UNION ALL
 SELECT 'model',m.tenant,m.created_at,COALESCE(a.client_id,'unknown'),COALESCE(a.actor_kind,'unknown'),COALESCE(a.agent_family,'unknown'),COALESCE(u.operation,'unknown'),m.model,'owner_assistant',CASE WHEN m.response_id IS NULL THEN 1 ELSE 0 END,COALESCE(m.input_tokens,0),COALESCE(m.output_tokens,0),0
 FROM model_usage m LEFT JOIN model_request_link l ON l.model_id=m.id LEFT JOIN usage u ON u.id=l.request_id AND u.tenant=m.tenant LEFT JOIN usage_attribution a ON a.request_id=u.id WHERE ` + predicate("m") + `
 ), filtered AS (SELECT * FROM events) `
	return cte, args
}
