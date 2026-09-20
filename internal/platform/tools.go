package platform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	mcp "github.com/mark3labs/mcp-go/mcp"
)

var platformToolNames = []string{"source_catalog", "source_read", "plan_business", "case_create", "case_update_intake", "case_list", "case_get", "case_add_note", "case_export", "case_reminder"}

func (a *App) addTools() {
	definitions := []struct {
		name, description string
		options           []mcp.ToolOption
	}{
		{"source_catalog", "List operator-curated official discovery entry points with source_id and stated coverage. Listing is not a verified requirement.", nil},
		{"source_read", "Read an official entry point by source_id with timestamp, fingerprint and links. Uses a 24-hour cache or explicit stale fallback. offline=true prevents network retrieval. No arbitrary URLs.", []mcp.ToolOption{mcp.WithString("source_id", mcp.Required(), mcp.Description("An id returned by source_catalog")), mcp.WithBoolean("offline", mcp.DefaultBool(false))}},
		{"plan_business", "Build a California research checklist from declared premises and activities. No verified legal determination.", []mcp.ToolOption{intakeOption()}},
		{"case_create", "Save a tenant-scoped preparation case. Supply a unique request_key; identical retries return the same case.", []mcp.ToolOption{intakeOption(), mcp.WithString("request_key", mcp.Required(), mcp.MinLength(8), mcp.MaxLength(100))}},
		{"case_update_intake", "Update business intake using the current case version; rebuilds research checklist without treating owner declarations as verified.", []mcp.ToolOption{mcp.WithString("case_id", mcp.Required()), mcp.WithNumber("version", mcp.Required(), mcp.Min(1), mcp.MultipleOf(1)), intakeOption()}},
		{"case_list", "List tenant case summaries with pagination; excludes notes. Continue with offset plus limit until a shorter page is returned.", []mcp.ToolOption{mcp.WithNumber("limit", mcp.Min(1), mcp.Max(100), mcp.MultipleOf(1), mcp.DefaultNumber(50)), mcp.WithNumber("offset", mcp.Min(0), mcp.Max(1000000), mcp.MultipleOf(1), mcp.DefaultNumber(0))}},
		{"case_get", "Read a tenant case and its research checklist.", []mcp.ToolOption{mcp.WithString("case_id", mcp.Required())}},
		{"case_add_note", "Append user-reported evidence or an answer to a case, using its current version. Notes never imply agency verification.", []mcp.ToolOption{mcp.WithString("case_id", mcp.Required()), mcp.WithNumber("version", mcp.Required(), mcp.Min(1), mcp.MultipleOf(1)), mcp.WithString("kind", mcp.Required(), mcp.Enum("research", "owner_answer", "agency_receipt")), mcp.WithString("text", mcp.Required()), mcp.WithString("source_url")}},
		{"case_export", "Generate a Markdown preparation worksheet and unsent correspondence draft. Not a filled government form or submission.", []mcp.ToolOption{mcp.WithString("case_id", mcp.Required())}},
		{"case_reminder", "Generate an ICS download for an owner-selected review date. Does not schedule an appointment or infer an agency deadline.", []mcp.ToolOption{mcp.WithString("case_id", mcp.Required()), mcp.WithString("date", mcp.Required(), mcp.Pattern(`^\d{4}-\d{2}-\d{2}$`), mcp.Description("Owner-selected review date YYYY-MM-DD, not an agency deadline"))}},
	}
	for _, d := range definitions {
		opts := append([]mcp.ToolOption{mcp.WithDescription(d.description), mcp.WithSchemaAdditionalProperties(false), mcp.WithReadOnlyHintAnnotation(d.name != "case_create" && d.name != "case_add_note" && d.name != "case_update_intake"), mcp.WithDestructiveHintAnnotation(false), mcp.WithOpenWorldHintAnnotation(d.name == "source_read")}, d.options...)
		a.mcp.AddTool(mcp.NewTool(d.name, opts...), func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			v, err := a.execute(ctx, r.Params.Name, r.GetArguments())
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			raw, err := json.Marshal(v)
			if err != nil {
				return nil, err
			}
			res := mcp.NewToolResultText(string(raw))
			res.StructuredContent = map[string]any{"data": v}
			return res, nil
		})
	}
}
func arguments(args map[string]any, target any) error {
	if args == nil {
		args = map[string]any{}
	}
	b, err := json.Marshal(args)
	if err != nil {
		return fmt.Errorf("invalid arguments")
	}
	return decode(strings.NewReader(string(b)), target)
}
func (a *App) execute(ctx context.Context, name string, args map[string]any) (any, error) {
	c := ctx.Value(identityKey{}).(*requestState).credential
	switch name {
	case "source_catalog":
		if len(args) > 0 {
			return nil, fmt.Errorf("source_catalog takes no arguments")
		}
		return Sources, nil
	case "source_read":
		var in struct {
			SourceID string `json:"source_id"`
			Offline  bool   `json:"offline"`
		}
		if err := arguments(args, &in); err != nil {
			return nil, err
		}
		return a.readSource(ctx, in.SourceID, in.Offline)
	case "plan_business", "case_create":
		var in struct {
			Intake     Intake `json:"intake"`
			RequestKey string `json:"request_key"`
		}
		if err := arguments(args, &in); err != nil {
			return nil, err
		}
		if err := in.Intake.Validate(); err != nil {
			return nil, err
		}
		if name == "plan_business" {
			return BuildPlan(in.Intake), nil
		}
		return a.config.Store.Create(ctx, c.Tenant, in.RequestKey, in.Intake)
	case "case_list":
		in := struct {
			Limit  int `json:"limit"`
			Offset int `json:"offset"`
		}{Limit: 50}
		if err := arguments(args, &in); err != nil {
			return nil, err
		}
		return a.config.Store.List(ctx, c.Tenant, in.Limit, in.Offset)
	case "case_update_intake":
		var in struct {
			ID      string `json:"case_id"`
			Version int    `json:"version"`
			Intake  Intake `json:"intake"`
		}
		if err := arguments(args, &in); err != nil {
			return nil, err
		}
		if !validID(in.ID) {
			return nil, ErrNotFound
		}
		return a.config.Store.UpdateIntake(ctx, c.Tenant, in.ID, in.Version, in.Intake)
	case "case_add_note":
		var in struct {
			ID        string `json:"case_id"`
			Version   int    `json:"version"`
			Kind      string `json:"kind"`
			Text      string `json:"text"`
			SourceURL string `json:"source_url"`
		}
		if err := arguments(args, &in); err != nil {
			return nil, err
		}
		if !validID(in.ID) {
			return nil, ErrNotFound
		}
		return a.config.Store.AddNote(ctx, c.Tenant, in.ID, in.Version, Note{Kind: in.Kind, Text: in.Text, SourceURL: in.SourceURL})
	default:
		var in struct {
			ID   string `json:"case_id"`
			Date string `json:"date"`
		}
		if err := arguments(args, &in); err != nil {
			return nil, err
		}
		if !validID(in.ID) {
			return nil, ErrNotFound
		}
		saved, err := a.config.Store.Get(ctx, c.Tenant, in.ID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return nil, err
			}
			return nil, fmt.Errorf("case unavailable")
		}
		switch name {
		case "case_get":
			return map[string]any{"case": saved, "plan": BuildPlan(saved.Intake)}, nil
		case "case_export":
			return map[string]string{"filename": "permits-agent-" + saved.ID + ".md", "mime_type": "text/markdown", "content": Packet(saved)}, nil
		case "case_reminder":
			body, err := calendar(saved, in.Date)
			if err != nil {
				return nil, err
			}
			return map[string]string{"filename": "permit-review-" + saved.ID + ".ics", "mime_type": "text/calendar", "content": body}, nil
		}
	}
	return nil, fmt.Errorf("unknown tool")
}
func validID(id string) bool { return len(id) == 32 && strings.Trim(id, "0123456789abcdef") == "" }

func intakeOption() mcp.ToolOption {
	props := map[string]any{}
	for _, name := range []string{"business_name", "business_type", "address", "city", "county"} {
		props[name] = map[string]any{"type": "string", "maxLength": 300}
	}
	props["business_type"] = map[string]any{"type": "string", "minLength": 1, "maxLength": 300, "description": "Declared business activity or industry"}
	props["jurisdiction"] = map[string]any{"type": "string", "enum": []string{"city", "unincorporated", "unknown"}, "description": "Owner-declared parcel jurisdiction; mailing city is not proof"}
	props["activities"] = map[string]any{"type": "array", "maxItems": 9, "items": map[string]any{"type": "string", "enum": []string{"alcohol_retail", "alcohol_production", "alcohol_wholesale", "alcohol_import", "food", "employees", "construction", "retail", "other"}}}
	option := mcp.WithObject("intake", mcp.Properties(props), mcp.AdditionalProperties(false), mcp.Description("California premises intake; at least city or county is required. Leave unknown optional fields blank."), func(schema map[string]any) {
		schema["required"] = []string{"business_type", "jurisdiction"}
		schema["anyOf"] = []any{map[string]any{"required": []string{"city"}, "properties": map[string]any{"city": map[string]any{"minLength": 1}}}, map[string]any{"required": []string{"county"}, "properties": map[string]any{"county": map[string]any{"minLength": 1}}}}
	})
	return func(tool *mcp.Tool) {
		option(tool)
		tool.InputSchema.Required = append(tool.InputSchema.Required, "intake")
	}
}
