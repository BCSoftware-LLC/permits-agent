// Package mcpserver exposes the local ABC mirror over MCP stdio.
package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/BCSoftware-LLC/permits-agent/internal/abc"
	"github.com/BCSoftware-LLC/permits-agent/internal/reference"
	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

var Version = "0.1.0"

var toolNames = []string{
	"search_licenses", "get_license", "pending_applications", "expiring_licenses", "licenses_at_address",
	"overdue_licenses", "status_overview", "licenses_in_area", "license_stats", "license_type_description",
	"search_forms", "license_requirements", "latest_news", "fee_surcharges", "refresh_data",
	"license_statuses", "licenses_by_area", "license_types", "abc_forms", "abc_fees", "abc_news", "abc_requirements", "license_history",
}

// ToolNames returns the stable ordered MCP tool surface.
func ToolNames() []string { return append([]string(nil), toolNames...) }

// New creates the ABC MCP server and registers its tools.
func New() *server.MCPServer {
	s := server.NewMCPServer(
		"abc-agent",
		Version,
		server.WithToolCapabilities(false),
	)
	s.AddResource(mcplib.NewResource("abc://stats", "ABC mirror stats"), func(ctx context.Context, request mcplib.ReadResourceRequest) ([]mcplib.ResourceContents, error) {
		store, err := abc.OpenDefault()
		if err != nil {
			return nil, err
		}
		defer store.Close()
		v, err := store.Statuses(ctx, time.Now())
		if err != nil {
			return nil, err
		}
		b, _ := json.Marshal(v)
		return []mcplib.ResourceContents{mcplib.TextResourceContents{URI: "abc://stats", MIMEType: "application/json", Text: string(b)}}, nil
	})
	s.AddResource(mcplib.NewResource("abc://history", "ABC mirror history"), func(ctx context.Context, request mcplib.ReadResourceRequest) ([]mcplib.ResourceContents, error) {
		store, err := abc.OpenDefault()
		if err != nil {
			return nil, err
		}
		defer store.Close()
		v, err := store.HistoryDiff(ctx)
		if err != nil {
			return nil, err
		}
		b, _ := json.Marshal(v)
		return []mcplib.ResourceContents{mcplib.TextResourceContents{URI: "abc://history", MIMEType: "application/json", Text: string(b)}}, nil
	})
	s.AddResource(mcplib.NewResource("abc://license-types", "ABC license type reference"), func(ctx context.Context, request mcplib.ReadResourceRequest) ([]mcplib.ResourceContents, error) {
		v, err := reference.Types(ctx, "", true)
		if err != nil {
			return nil, err
		}
		b, _ := json.Marshal(v)
		return []mcplib.ResourceContents{mcplib.TextResourceContents{URI: "abc://license-types", MIMEType: "application/json", Text: string(b)}}, nil
	})
	s.AddResource(mcplib.NewResource("abc://fees", "ABC fees reference"), func(ctx context.Context, request mcplib.ReadResourceRequest) ([]mcplib.ResourceContents, error) {
		v, err := reference.Fees(ctx, true)
		if err != nil {
			return nil, err
		}
		b, _ := json.Marshal(v)
		return []mcplib.ResourceContents{mcplib.TextResourceContents{URI: "abc://fees", MIMEType: "application/json", Text: string(b)}}, nil
	})
	readonly := []mcplib.ToolOption{
		mcplib.WithReadOnlyHintAnnotation(true),
		mcplib.WithDestructiveHintAnnotation(false),
		mcplib.WithIdempotentHintAnnotation(true),
		mcplib.WithOpenWorldHintAnnotation(false),
	}

	s.AddTool(
		mcplib.NewTool("license_stats",
			append([]mcplib.ToolOption{mcplib.WithDescription("Aggregate official status and license-type counts from the existing local ABC mirror")}, readonly...)...,
		),
		withStore(func(ctx context.Context, store *abc.Store, _ map[string]any) (any, error) {
			return store.Stats(ctx)
		}),
	)

	s.AddTool(
		mcplib.NewTool("licenses_at_address",
			append([]mcplib.ToolOption{
				mcplib.WithDescription("Return every license/application tied to a premises-address fragment"),
				mcplib.WithString("fragment", mcplib.Required(), mcplib.Description("Street, street number, city, or ZIP fragment")),
				mcplib.WithString("status", mcplib.Description("Official ABC status filter")),
				mcplib.WithString("license_type", mcplib.Description("License type code")),
				mcplib.WithString("lic_or_app", mcplib.Description("LIC or APP")),
				mcplib.WithString("expire_year", mcplib.Description("Four-digit expiration year")),
				mcplib.WithBoolean("match_mail", mcplib.Description("Also match mailing addresses")),
				mcplib.WithNumber("limit", mcplib.Description("Maximum rows, default 100")),
				mcplib.WithNumber("offset", mcplib.Description("Rows to skip")),
			}, readonly...)...,
		),
		withStore(func(ctx context.Context, store *abc.Store, args map[string]any) (any, error) {
			fragment := stringArg(args, "fragment")
			if fragment == "" {
				return nil, fmt.Errorf("fragment is required")
			}
			return store.Address(ctx, fragment, abc.QueryOptions{
				Limit: intArg(args, "limit", 100), Offset: intArg(args, "offset", 0), Status: stringArg(args, "status"),
				Type: stringArg(args, "license_type"), LicOrApp: stringArg(args, "lic_or_app"),
				ExpireYear: stringArg(args, "expire_year"), MatchMail: boolArg(args, "match_mail"),
			})
		}),
	)

	s.AddTool(
		mcplib.NewTool("get_license",
			append([]mcplib.ToolOption{
				mcplib.WithDescription("Return all license-type rows for one ABC file number"),
				mcplib.WithString("file_number", mcplib.Required(), mcplib.Description("ABC file number, preserving leading zeros")),
			}, readonly...)...,
		),
		withStore(func(ctx context.Context, store *abc.Store, args map[string]any) (any, error) {
			fileNumber := stringArg(args, "file_number")
			if fileNumber == "" {
				return nil, fmt.Errorf("file_number is required")
			}
			return store.Get(ctx, fileNumber)
		}),
	)

	s.AddTool(
		mcplib.NewTool("pending_applications",
			append([]mcplib.ToolOption{
				mcplib.WithDescription("Return official PEND application rows, optionally filtered by premises area"),
				mcplib.WithString("zip", mcplib.Description("Premises ZIP prefix")),
				mcplib.WithString("city", mcplib.Description("Premises city prefix")),
				mcplib.WithString("county", mcplib.Description("Premises county prefix")),
				mcplib.WithNumber("limit", mcplib.Description("Maximum rows, default 100")),
				mcplib.WithNumber("offset", mcplib.Description("Rows to skip")),
			}, readonly...)...,
		),
		withStore(func(ctx context.Context, store *abc.Store, args map[string]any) (any, error) {
			return store.Pending(ctx, abc.AreaFilter{
				ZIP: stringArg(args, "zip"), City: stringArg(args, "city"),
				County: stringArg(args, "county"), Limit: intArg(args, "limit", 100), Offset: intArg(args, "offset", 0),
			})
		}),
	)

	s.AddTool(
		mcplib.NewTool("overdue_licenses",
			append([]mcplib.ToolOption{
				mcplib.WithDescription("Derived signal: rows still officially ACTIVE whose expiration date is past; not an official revocation status"),
				mcplib.WithString("zip", mcplib.Description("Premises ZIP prefix")),
				mcplib.WithString("city", mcplib.Description("Premises city prefix")),
				mcplib.WithString("county", mcplib.Description("Premises county prefix")),
				mcplib.WithString("district", mcplib.Description("ABC district prefix")),
				mcplib.WithNumber("min_days", mcplib.Description("Minimum days past expiration")),
				mcplib.WithNumber("limit", mcplib.Description("Maximum rows, default 100")),
				mcplib.WithNumber("offset", mcplib.Description("Rows to skip")),
			}, readonly...)...,
		),
		withStore(func(ctx context.Context, store *abc.Store, args map[string]any) (any, error) {
			return store.Overdue(ctx, abc.AreaFilter{
				ZIP: stringArg(args, "zip"), City: stringArg(args, "city"),
				County: stringArg(args, "county"), District: stringArg(args, "district"),
				MinDays: intArg(args, "min_days", intArg(args, "min_days_past", 0)), Limit: intArg(args, "limit", 100), Offset: intArg(args, "offset", 0),
			}, time.Now())
		}),
	)

	s.AddTool(
		mcplib.NewTool("expiring_licenses",
			append([]mcplib.ToolOption{
				mcplib.WithDescription("Active licenses expiring today through N days"),
				mcplib.WithString("zip"), mcplib.WithString("city"), mcplib.WithString("county"), mcplib.WithNumber("days"), mcplib.WithNumber("limit"), mcplib.WithNumber("offset"),
			}, readonly...)...),
		withStore(func(ctx context.Context, store *abc.Store, args map[string]any) (any, error) {
			return store.Expiring(ctx, abc.AreaFilter{ZIP: stringArg(args, "zip"), City: stringArg(args, "city"), County: stringArg(args, "county"), Limit: intArg(args, "limit", 100), Offset: intArg(args, "offset", 0)}, intArg(args, "days", 90), time.Now())
		}),
	)
	s.AddTool(mcplib.NewTool("status_overview", append([]mcplib.ToolOption{mcplib.WithDescription("Official status vocabulary and observed export counts")}, readonly...)...), withStore(func(ctx context.Context, store *abc.Store, args map[string]any) (any, error) {
		return store.Statuses(ctx, time.Now())
	}))
	s.AddTool(mcplib.NewTool("license_statuses", append([]mcplib.ToolOption{mcplib.WithDescription("Alias for status_overview")}, readonly...)...), withStore(func(ctx context.Context, store *abc.Store, args map[string]any) (any, error) {
		return store.Statuses(ctx, time.Now())
	}))
	s.AddTool(
		mcplib.NewTool("licenses_in_area",
			append([]mcplib.ToolOption{mcplib.WithDescription("List licenses/applications in one area"), mcplib.WithString("zip"), mcplib.WithString("city"), mcplib.WithString("county"), mcplib.WithString("district"), mcplib.WithString("status"), mcplib.WithString("license_type"), mcplib.WithString("lic_or_app"), mcplib.WithString("expire_year"), mcplib.WithNumber("limit"), mcplib.WithNumber("offset")}, readonly...)...),
		withStore(func(ctx context.Context, store *abc.Store, args map[string]any) (any, error) {
			return store.ByArea(ctx, abc.AreaFilter{ZIP: stringArg(args, "zip"), City: stringArg(args, "city"), County: stringArg(args, "county"), District: stringArg(args, "district"), Limit: intArg(args, "limit", 200), Offset: intArg(args, "offset", 0)}, abc.QueryOptions{Status: stringArg(args, "status"), Type: stringArg(args, "license_type"), LicOrApp: stringArg(args, "lic_or_app"), ExpireYear: stringArg(args, "expire_year")})
		}),
	)
	s.AddTool(
		mcplib.NewTool("licenses_by_area",
			append([]mcplib.ToolOption{mcplib.WithDescription("Alias for licenses_in_area"), mcplib.WithString("zip"), mcplib.WithString("city"), mcplib.WithString("county"), mcplib.WithString("district"), mcplib.WithString("status"), mcplib.WithString("license_type"), mcplib.WithString("lic_or_app"), mcplib.WithString("expire_year"), mcplib.WithNumber("limit"), mcplib.WithNumber("offset")}, readonly...)...),
		withStore(func(ctx context.Context, store *abc.Store, args map[string]any) (any, error) {
			return store.ByArea(ctx, abc.AreaFilter{ZIP: stringArg(args, "zip"), City: stringArg(args, "city"), County: stringArg(args, "county"), District: stringArg(args, "district"), Limit: intArg(args, "limit", 200), Offset: intArg(args, "offset", 0)}, abc.QueryOptions{Status: stringArg(args, "status"), Type: stringArg(args, "license_type"), LicOrApp: stringArg(args, "lic_or_app"), ExpireYear: stringArg(args, "expire_year")})
		}),
	)

	s.AddTool(
		mcplib.NewTool("search_licenses",
			append([]mcplib.ToolOption{
				mcplib.WithDescription("Search licensee name, DBA, or file number in the local ABC mirror"),
				mcplib.WithString("query", mcplib.Required(), mcplib.Description("Search text")),
				mcplib.WithString("status", mcplib.Description("Official ABC status filter")),
				mcplib.WithString("county", mcplib.Description("Premises county")),
				mcplib.WithString("city", mcplib.Description("Premises city")),
				mcplib.WithString("license_type", mcplib.Description("License type code")),
				mcplib.WithString("lic_or_app", mcplib.Description("LIC or APP")),
				mcplib.WithString("expire_year", mcplib.Description("Four-digit expiration year")),
				mcplib.WithNumber("limit", mcplib.Description("Maximum rows, default 50")),
				mcplib.WithNumber("offset", mcplib.Description("Rows to skip")),
			}, readonly...)...,
		),
		withStore(func(ctx context.Context, store *abc.Store, args map[string]any) (any, error) {
			query := stringArg(args, "query")
			if query == "" {
				return nil, fmt.Errorf("query is required")
			}
			return store.Search(ctx, query, abc.QueryOptions{
				Limit: intArg(args, "limit", 50), Offset: intArg(args, "offset", 0), Status: stringArg(args, "status"),
				County: stringArg(args, "county"), City: stringArg(args, "city"),
				Type: stringArg(args, "license_type"), LicOrApp: stringArg(args, "lic_or_app"),
				ExpireYear: stringArg(args, "expire_year"),
			})
		}),
	)

	s.AddTool(
		mcplib.NewTool("refresh_data",
			mcplib.WithDescription("Download the official ABC daily export and atomically rebuild the local SQLite mirror"),
			mcplib.WithBoolean("force", mcplib.Description("Force network refresh; default true")),
			mcplib.WithReadOnlyHintAnnotation(false),
			mcplib.WithDestructiveHintAnnotation(false),
			mcplib.WithIdempotentHintAnnotation(true),
			mcplib.WithOpenWorldHintAnnotation(true),
		),
		func(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
			if err := validateArgs(request); err != nil {
				return mcplib.NewToolResultError(err.Error()), nil
			}
			force := true
			if v, ok := request.GetArguments()["force"].(bool); ok {
				force = v
			}
			store, info, err := abc.EnsureData(ctx, force)
			if err != nil {
				return mcplib.NewToolResultError(err.Error()), nil
			}
			defer store.Close()
			return toolResultJSON(info)
		},
	)

	for _, spec := range []struct{ name, desc string }{
		{"license_types", "ABC license type reference"}, {"license_type_description", "Describe one ABC license type"}, {"abc_forms", "ABC forms reference"}, {"search_forms", "Search ABC forms"}, {"abc_fees", "ABC fee reference"}, {"fee_surcharges", "ABC fee surcharges"}, {"abc_news", "ABC news feed"}, {"latest_news", "ABC latest news"}, {"abc_requirements", "ABC requirements guidance"}, {"license_requirements", "ABC requirements guidance"},
	} {
		name := spec.name
		desc := spec.desc
		s.AddTool(referenceTool(name, desc), func(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
			if err := validateArgs(request); err != nil {
				return mcplib.NewToolResultError(err.Error()), nil
			}
			args := request.GetArguments()
			off := boolArg(args, "offline")
			var v any
			var err error
			switch name {
			case "license_types", "license_type_description":
				if name == "license_type_description" && stringArg(args, "code") == "" {
					return mcplib.NewToolResultError("code is required"), nil
				}
				v, err = reference.Types(ctx, stringArg(args, "code"), off)
			case "abc_forms":
				v, err = reference.Forms(ctx, stringArg(args, "search"), off)
			case "search_forms":
				q := stringArg(args, "query")
				if q == "" {
					q = stringArg(args, "search")
				}
				v, err = reference.Forms(ctx, q, off)
			case "abc_fees", "fee_surcharges":
				v, err = reference.Fees(ctx, off)
			case "abc_news", "latest_news":
				v, err = reference.News(ctx, stringArg(args, "feed"), intArg(args, "limit", 10), off)
			case "abc_requirements":
				v, err = reference.Requirements(ctx, stringArg(args, "code"), stringArg(args, "action"), off)
			case "license_requirements":
				code := stringArg(args, "license_type")
				if code == "" {
					code = stringArg(args, "code")
				}
				v, err = reference.Requirements(ctx, code, stringArg(args, "action"), off)
			}
			if err != nil {
				return mcplib.NewToolResultError(err.Error()), nil
			}
			return referenceResult(name, v)
		})
	}
	s.AddTool(mcplib.NewTool("license_history", append(append([]mcplib.ToolOption{}, readonly...), mcplib.WithDescription("ABC mirror history snapshot and diff"), mcplib.WithReadOnlyHintAnnotation(false))...), withStore(func(ctx context.Context, store *abc.Store, args map[string]any) (any, error) {
		snap, err := store.HistorySnapshot(ctx)
		if err != nil {
			return nil, err
		}
		diff, err := store.HistoryDiff(ctx)
		if err != nil {
			return nil, err
		}
		return map[string]any{"snapshot": snap, "diff": diff}, nil
	}))
	return s
}

func withStore(fn func(context.Context, *abc.Store, map[string]any) (any, error)) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		if err := validateArgs(request); err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		store, err := abc.OpenDefault()
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		defer store.Close()
		value, err := fn(ctx, store, request.GetArguments())
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		result, err := toolResultJSON(value)
		if err == nil {
			attachProvenance(result, value, store.Provenance(ctx))
		}
		return result, err
	}
}

func attachProvenance(result *mcplib.CallToolResult, value any, provenance any) {
	result.Meta = &mcplib.Meta{AdditionalFields: map[string]any{"provenance": provenance}}
	result.StructuredContent = map[string]any{"data": value, "provenance": provenance}
}

func referenceResult(name string, value any) (*mcplib.CallToolResult, error) {
	var payload, provenance any
	switch name {
	case "license_type_description":
		r := value.(reference.TypesResult)
		if len(r.Types) != 1 {
			return mcplib.NewToolResultError("license type not found"), nil
		}
		result := mcplib.NewToolResultText(r.Types[0].Description)
		attachProvenance(result, r.Types[0].Description, r.Provenance)
		return result, nil
	case "search_forms":
		r := value.(reference.FormsResult)
		payload = r.Forms
		provenance = r.Provenance
	case "latest_news":
		r := value.(reference.NewsResult)
		payload = r.Items
		provenance = r.Provenance
	case "fee_surcharges":
		r := value.(reference.FeesResult)
		payload = r.Surcharges
		provenance = r.Provenance
	default:
		return toolResultJSON(value)
	}
	result, err := toolResultJSON(payload)
	if err == nil {
		attachProvenance(result, payload, provenance)
	}
	return result, err
}

func toolResultJSON(value any) (*mcplib.CallToolResult, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return mcplib.NewToolResultError(fmt.Sprintf("encoding result: %v", err)), nil
	}
	return mcplib.NewToolResultText(string(data)), nil
}

func stringArg(args map[string]any, name string) string {
	value, _ := args[name].(string)
	return value
}

func intArg(args map[string]any, name string, fallback int) int {
	switch value := args[name].(type) {
	case float64:
		return int(value)
	case int:
		return value
	}
	return fallback
}

func boolArg(args map[string]any, name string) bool {
	value, _ := args[name].(bool)
	return value
}

type argKind string

const (
	kindString argKind = "string"
	kindBool   argKind = "bool"
	kindInt    argKind = "int"
)

var toolArgs = map[string]map[string]argKind{
	"search_licenses":      {"query": kindString, "status": kindString, "county": kindString, "city": kindString, "license_type": kindString, "lic_or_app": kindString, "expire_year": kindString, "limit": kindInt, "offset": kindInt},
	"get_license":          {"file_number": kindString},
	"pending_applications": {"zip": kindString, "city": kindString, "county": kindString, "limit": kindInt, "offset": kindInt},
	"expiring_licenses":    {"zip": kindString, "city": kindString, "county": kindString, "days": kindInt, "limit": kindInt, "offset": kindInt},
	"licenses_at_address":  {"fragment": kindString, "status": kindString, "license_type": kindString, "lic_or_app": kindString, "expire_year": kindString, "match_mail": kindBool, "limit": kindInt, "offset": kindInt},
	"overdue_licenses":     {"zip": kindString, "city": kindString, "county": kindString, "district": kindString, "min_days": kindInt, "min_days_past": kindInt, "limit": kindInt, "offset": kindInt},
	"status_overview":      {}, "license_statuses": {}, "license_stats": {}, "license_history": {},
	"licenses_in_area":         {"zip": kindString, "city": kindString, "county": kindString, "district": kindString, "status": kindString, "license_type": kindString, "lic_or_app": kindString, "expire_year": kindString, "limit": kindInt, "offset": kindInt},
	"licenses_by_area":         {"zip": kindString, "city": kindString, "county": kindString, "district": kindString, "status": kindString, "license_type": kindString, "lic_or_app": kindString, "expire_year": kindString, "limit": kindInt, "offset": kindInt},
	"license_type_description": {"code": kindString, "offline": kindBool}, "license_types": {"code": kindString, "offline": kindBool},
	"search_forms": {"query": kindString, "search": kindString, "offline": kindBool}, "abc_forms": {"search": kindString, "offline": kindBool},
	"license_requirements": {"license_type": kindString, "code": kindString, "action": kindString, "offline": kindBool}, "abc_requirements": {"code": kindString, "action": kindString, "offline": kindBool},
	"latest_news": {"feed": kindString, "limit": kindInt, "offline": kindBool}, "abc_news": {"feed": kindString, "limit": kindInt, "offline": kindBool},
	"fee_surcharges": {"offline": kindBool}, "abc_fees": {"offline": kindBool}, "refresh_data": {"force": kindBool},
}

func validateArgs(request mcplib.CallToolRequest) error {
	allowed, ok := toolArgs[request.Params.Name]
	if !ok {
		return nil
	}
	raw, _ := request.GetRawArguments().(json.RawMessage)
	if len(raw) == 0 {
		if request.Params.Arguments == nil {
			return nil
		}
		var err error
		raw, err = json.Marshal(request.Params.Arguments)
		if err != nil {
			return fmt.Errorf("arguments must be a JSON object")
		}
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return fmt.Errorf("arguments must be a JSON object")
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return fmt.Errorf("arguments must be a JSON object")
	}
	for k, v := range m {
		kind, ok := allowed[k]
		if !ok {
			return fmt.Errorf("unknown argument %q", k)
		}
		if bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
			return fmt.Errorf("argument %q cannot be null", k)
		}
		switch kind {
		case kindString:
			var s string
			if err := json.Unmarshal(v, &s); err != nil {
				return fmt.Errorf("argument %q must be a string", k)
			}
		case kindBool:
			var b bool
			if err := json.Unmarshal(v, &b); err != nil {
				return fmt.Errorf("argument %q must be a boolean", k)
			}
		case kindInt:
			n, err := strconv.ParseInt(string(bytes.TrimSpace(v)), 10, 64)
			if err != nil || n < 0 || n > 9007199254740991 {
				return fmt.Errorf("argument %q must be a nonnegative integer", k)
			}
		}
	}
	return nil
}

// Use the same parameter definitions for advertised schemas and validation.
func referenceTool(name, desc string) mcplib.Tool {
	opts := []mcplib.ToolOption{mcplib.WithDescription(desc), mcplib.WithReadOnlyHintAnnotation(false), mcplib.WithOpenWorldHintAnnotation(true), mcplib.WithDestructiveHintAnnotation(false), mcplib.WithIdempotentHintAnnotation(true)}
	keys := []string{}
	for k := range toolArgs[name] {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		switch toolArgs[name][k] {
		case kindString:
			if name == "license_type_description" && k == "code" {
				opts = append(opts, mcplib.WithString(k, mcplib.Required()))
			} else {
				opts = append(opts, mcplib.WithString(k))
			}
		case kindBool:
			opts = append(opts, mcplib.WithBoolean(k))
		case kindInt:
			opts = append(opts, mcplib.WithNumber(k))
		}
	}
	return mcplib.NewTool(name, opts...)
}

func Serve() error { return server.ServeStdio(New()) }
