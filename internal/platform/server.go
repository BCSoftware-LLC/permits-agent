package platform

import (
	"bytes"
	"context"
	"crypto/subtle"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/BCSoftware-LLC/permits-agent/internal/mcpserver"
	mcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

//go:embed web/*
var webFiles embed.FS

type Credential struct {
	Tenant                string   `json:"tenant"`
	TokenHash             string   `json:"token_hash"`
	Scopes                []string `json:"scopes"`
	MonthlyRequestLimit   int      `json:"monthly_request_limit"`
	MonthlyModelCallLimit int      `json:"monthly_model_call_limit"`
}
type Config struct {
	Agent       *AgentConfig
	Origin      string
	Credentials []Credential
	Store       *Store
}
type identityKey struct{}
type requestState struct {
	credential Credential
	failed     bool
}
type App struct {
	config  Config
	mcp     *server.MCPServer
	handler http.Handler
}

func TokenHash(token string) string { return digest([]byte(token)) }
func New(cfg Config) (*App, error) {
	if err := cfg.Agent.validate(); err != nil {
		return nil, err
	}
	u, err := url.Parse(cfg.Origin)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" && u.Path != "/" {
		return nil, fmt.Errorf("origin must be an absolute origin without path")
	}
	if u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1")) {
		return nil, fmt.Errorf("origin must use HTTPS except on localhost")
	}
	cfg.Origin = strings.TrimSuffix(cfg.Origin, "/")
	if cfg.Store == nil || len(cfg.Credentials) == 0 {
		return nil, fmt.Errorf("database and at least one credential required")
	}
	seen := map[string]bool{}
	limits := map[string]int{}
	modelLimits := map[string]int{}
	for _, c := range cfg.Credentials {
		if len(c.Tenant) == 0 || len(c.Tenant) > 100 || len(c.TokenHash) != 64 || strings.Trim(c.TokenHash, "0123456789abcdef") != "" || c.MonthlyRequestLimit < 1 || c.MonthlyModelCallLimit < 0 || len(c.Scopes) == 0 {
			return nil, fmt.Errorf("invalid credential configuration")
		}
		if seen[c.TokenHash] {
			return nil, fmt.Errorf("duplicate credential")
		}
		seen[c.TokenHash] = true
		if old, ok := limits[c.Tenant]; ok && old != c.MonthlyRequestLimit {
			return nil, fmt.Errorf("tenant credentials must share one request limit")
		}
		limits[c.Tenant] = c.MonthlyRequestLimit
		if old, ok := modelLimits[c.Tenant]; ok && old != c.MonthlyModelCallLimit {
			return nil, fmt.Errorf("tenant credentials must share one model call limit")
		}
		modelLimits[c.Tenant] = c.MonthlyModelCallLimit
		for _, scope := range c.Scopes {
			if scope != "research" && scope != "cases:read" && scope != "cases:write" && scope != "agent:read" {
				return nil, fmt.Errorf("invalid credential scope")
			}
		}
	}
	a := &App{config: cfg, mcp: mcpserver.New()}
	a.mcp.DeleteTools("refresh_data", "license_history")
	a.mcp.DeleteResources("abc://stats", "abc://history", "abc://license-types", "abc://fees")
	a.addTools()
	for _, name := range append(mcpserver.ToolNames(), platformToolNames...) {
		t := a.mcp.GetTool(name)
		if t == nil {
			continue
		}
		original := t.Handler
		a.mcp.AddTool(t.Tool, func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			state, ok := ctx.Value(identityKey{}).(*requestState)
			if !ok {
				return mcp.NewToolResultError("authentication required"), nil
			}
			scope := "research"
			if strings.HasPrefix(r.Params.Name, "case_") {
				scope = "cases:read"
			}
			if r.Params.Name == "case_create" || r.Params.Name == "case_add_note" || r.Params.Name == "case_update_intake" {
				scope = "cases:write"
			}
			if !hasScope(state.credential, scope) {
				state.failed = true
				return mcp.NewToolResultError("credential does not grant " + scope), nil
			}
			args := r.GetArguments()
			if r.Params.Arguments != nil && args == nil {
				state.failed = true
				return mcp.NewToolResultError("arguments must be a JSON object"), nil
			}
			if args == nil {
				args = map[string]any{}
			}
			if _, isABC := abcToolNames[r.Params.Name]; isABC {
				// Hosted queries are bounded and read from operator-refreshed caches.
				if v, exists := args["limit"]; exists {
					n, ok := v.(float64)
					if !ok || n < 1 || n > 200 || n != float64(int(n)) {
						state.failed = true
						return mcp.NewToolResultError("hosted limit must be an integer from 1 to 200"), nil
					}
				}
				if v, exists := args["offset"]; exists {
					n, ok := v.(float64)
					if !ok || n < 0 || n > 100000 || n != float64(int(n)) {
						state.failed = true
						return mcp.NewToolResultError("hosted offset must be an integer from 0 to 100000"), nil
					}
				}
				if referenceTools[r.Params.Name] {
					if off, exists := args["offline"]; exists {
						if _, valid := off.(bool); !valid {
							state.failed = true
							return mcp.NewToolResultError("offline must be a boolean"), nil
						}
					}
					args["offline"] = true
				}
				r.Params.Arguments = args
			}
			result, err := original(ctx, r)
			if err != nil || result != nil && result.IsError {
				state.failed = true
			}
			return result, err
		})
	}
	mux := http.NewServeMux()
	mcpHTTP := server.NewStreamableHTTPServer(a.mcp, server.WithStateLess(true), server.WithDisableStreaming(true), server.WithHTTPContextFunc(func(ctx context.Context, r *http.Request) context.Context {
		return context.WithValue(ctx, identityKey{}, r.Context().Value(identityKey{}))
	}))
	mux.Handle("/mcp", a.auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.Header().Set("Allow", "POST")
			writeJSON(w, 405, map[string]string{"error": "POST required; server uses stateless JSON responses"})
			return
		}
		mcpHTTP.ServeHTTP(w, r)
	})))
	mux.Handle("/api/call", a.auth(http.HandlerFunc(a.call)))
	mux.Handle("/api/agent", a.auth(http.HandlerFunc(a.agent)))
	mux.Handle("/api/usage", a.auth(http.HandlerFunc(a.usage)))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, map[string]string{"status": "ok"}) })
	mux.HandleFunc("GET /api/example", func(w http.ResponseWriter, r *http.Request) {
		i := Intake{BusinessName: "Example neighborhood café", BusinessType: "Restaurant", City: "Los Angeles", County: "Los Angeles", Jurisdiction: "unknown", Activities: []string{"food", "alcohol_retail", "employees"}}
		c := Case{ID: "example", Version: 1, Intake: i, Notes: []Note{}}
		writeJSON(w, 200, map[string]any{"case": c, "plan": BuildPlan(i), "fictional": true})
	})
	mux.HandleFunc("GET /api/catalog", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"sources": Sources, "status": "discovery_entry_points", "state": "CA"})
	})
	mux.HandleFunc("GET /.well-known/permits-agent.json", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"name": "Permits Agent", "mcp_url": cfg.Origin + "/mcp", "api_url": cfg.Origin + "/api/call", "authentication": "Bearer token; operator provisioned, no OAuth discovery", "owner_assistant_enabled": cfg.Agent != nil, "capabilities": []string{"abc_public_data", "california_research_checklist", "tenant_cases", "draft_brief", "review_reminder"}, "limitations": []string{"No automatic submission, signature, payment, email or calendar write", "No complete jurisdiction coverage or verified permit determinations", "Owner assistant requires explicit model configuration and allowance"}})
	})
	assets, _ := fs.Sub(webFiles, "web")
	mux.Handle("/", http.FileServer(http.FS(assets)))
	a.handler = a.security(mux)
	return a, nil
}

var abcToolNames = func() map[string]bool {
	m := map[string]bool{}
	for _, n := range mcpserver.ToolNames() {
		m[n] = true
	}
	return m
}()
var referenceTools = map[string]bool{"license_type_description": true, "license_types": true, "search_forms": true, "abc_forms": true, "license_requirements": true, "abc_requirements": true, "latest_news": true, "abc_news": true, "fee_surcharges": true, "abc_fees": true}

func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) { a.handler.ServeHTTP(w, r) }
func (a *App) security(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		if strings.HasPrefix(a.config.Origin, "https:") {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000")
		}
		if r.Header.Get("Origin") != "" && r.Header.Get("Origin") != a.config.Origin {
			writeJSON(w, 403, map[string]string{"error": "origin not allowed"})
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/mcp" {
			w.Header().Set("Cache-Control", "no-store")
		}
		r.Body = http.MaxBytesReader(w, r.Body, 65536)
		next.ServeHTTP(w, r)
	})
}
func (a *App) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		hash := TokenHash(token)
		var credential *Credential
		if strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") && len(token) >= 32 && len(token) <= 512 {
			for _, c := range a.config.Credentials {
				if subtle.ConstantTimeCompare([]byte(c.TokenHash), []byte(hash)) == 1 {
					copy := c
					credential = &copy
				}
			}
		}
		if credential == nil {
			w.Header().Set("WWW-Authenticate", `Bearer realm="Permits Agent"`)
			writeJSON(w, 401, map[string]string{"error": "valid access key required"})
			return
		}
		id, err := a.config.Store.Reserve(r.Context(), credential.Tenant, r.URL.Path, credential.MonthlyRequestLimit)
		if err != nil {
			if errors.Is(err, ErrQuota) {
				w.Header().Set("Retry-After", "60")
				writeJSON(w, 429, map[string]string{"error": err.Error()})
			} else {
				writeJSON(w, 503, map[string]string{"error": "usage accounting unavailable"})
			}
			return
		}
		state := &requestState{credential: *credential}
		ctx := context.WithValue(r.Context(), identityKey{}, state)
		w.Header().Set("X-Request-ID", id)
		// Persist an incomplete event first. A crash cannot silently lose consumption.
		recorder := &statusWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(recorder, r.WithContext(ctx))
		outcome := "ok"
		if recorder.status >= 400 || state.failed || recorder.protocolError() {
			outcome = "error"
		}
		finishCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err = a.config.Store.Finish(finishCtx, id, outcome); err != nil {
			slog.Error("usage completion failed", "request_id", id)
		}
	})
}

type statusWriter struct {
	http.ResponseWriter
	status  int
	capture bytes.Buffer
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.capture.Len()+len(b) <= 1048576 {
		_, _ = w.capture.Write(b)
	}
	return w.ResponseWriter.Write(b)
}
func (w *statusWriter) protocolError() bool {
	var v struct {
		Error json.RawMessage `json:"error"`
	}
	return json.Unmarshal(w.capture.Bytes(), &v) == nil && len(v.Error) > 0 && string(v.Error) != "null"
}
func (w *statusWriter) WriteHeader(n int)           { w.status = n; w.ResponseWriter.WriteHeader(n) }
func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func hasScope(c Credential, scope string) bool {
	for _, s := range c.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}
func decode(r io.Reader, v any) error {
	d := json.NewDecoder(r)
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return fmt.Errorf("invalid request JSON")
	}
	if d.Decode(new(any)) != io.EOF {
		return fmt.Errorf("request must contain one JSON object")
	}
	return nil
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func (a *App) call(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.Header().Set("Allow", "POST")
		writeJSON(w, 405, map[string]string{"error": "POST required"})
		return
	}
	var in struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if err := decode(r.Body, &in); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	t := a.mcp.GetTool(in.Name)
	if t == nil {
		writeJSON(w, 404, map[string]string{"error": "unknown tool"})
		return
	}
	req := mcp.CallToolRequest{}
	req.Params.Name = in.Name
	req.Params.Arguments = in.Arguments
	res, err := t.Handler(r.Context(), req)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "tool execution failed"})
		return
	}
	status := 200
	if res.IsError {
		status = 422
	}
	writeJSON(w, status, res)
}
func (a *App) usage(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		w.Header().Set("Allow", "GET")
		writeJSON(w, 405, map[string]string{"error": "GET required"})
		return
	}
	c := r.Context().Value(identityKey{}).(*requestState).credential
	v, err := a.config.Store.Usage(r.Context(), c.Tenant)
	if err != nil {
		writeJSON(w, 503, map[string]string{"error": "usage unavailable"})
		return
	}
	v["monthly_request_limit"] = c.MonthlyRequestLimit
	v["monthly_model_call_limit"] = c.MonthlyModelCallLimit
	modelUsage, err := a.config.Store.ModelUsage(r.Context(), c.Tenant)
	if err != nil {
		writeJSON(w, 503, map[string]string{"error": "model usage unavailable"})
		return
	}
	v["model_usage"] = modelUsage
	writeJSON(w, 200, v)
}
