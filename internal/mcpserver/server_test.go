package mcpserver

import (
	"bufio"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestToolNamesExposeCoreReadOnlySurface(t *testing.T) {
	got := ToolNames()
	want := []string{"search_licenses", "get_license", "pending_applications", "expiring_licenses", "licenses_at_address", "overdue_licenses", "status_overview", "licenses_in_area", "license_stats", "license_type_description", "search_forms", "license_requirements", "latest_news", "fee_surcharges", "refresh_data", "license_statuses", "licenses_by_area", "license_types", "abc_forms", "abc_fees", "abc_news", "abc_requirements", "license_history"}
	if len(got) != len(want) {
		t.Fatalf("tool count = %d, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("tool[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
func TestToolResultJSONUsesPortableTextForArrays(t *testing.T) {
	result, err := toolResultJSON([]map[string]string{{"status": "PEND"}})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	if strings.Contains(text, "structuredContent") || !strings.Contains(text, `\"status\":\"PEND\"`) {
		t.Fatalf("unexpected MCP result: %s", text)
	}
}
func TestArgumentHelpersApplyDefaults(t *testing.T) {
	args := map[string]any{"limit": float64(25), "query": "  abc  ", "mail": true}
	if got := intArg(args, "limit", 100); got != 25 {
		t.Fatalf("limit = %d, want 25", got)
	}
	if got := stringArg(args, "query"); got != "  abc  " {
		t.Fatalf("query = %q, want preserved whitespace", got)
	}
	if !boolArg(args, "mail") || intArg(args, "missing", 100) != 100 {
		t.Fatal("argument defaults were not preserved")
	}
}
func TestStdioProtocolInitializeAndListTools(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "abc-agent-mcp")
	build := exec.Command("go", "build", "-o", bin, "./cmd/abc-agent-mcp")
	build.Dir = "../.."
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	cmd := exec.Command(bin)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Process.Kill()
	enc := json.NewEncoder(stdin)
	_ = enc.Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{"protocolVersion": "2024-11-05", "capabilities": map[string]any{}, "clientInfo": map[string]string{"name": "test", "version": "0"}}})
	_ = enc.Encode(map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized", "params": map[string]any{}})
	_ = enc.Encode(map[string]any{"jsonrpc": "2.0", "id": 2, "method": "tools/list", "params": map[string]any{}})
	_ = enc.Encode(map[string]any{"jsonrpc": "2.0", "id": 3, "method": "resources/list", "params": map[string]any{}})
	_ = enc.Encode(map[string]any{"jsonrpc": "2.0", "id": 4, "method": "tools/call", "params": map[string]any{"name": "search_licenses", "arguments": map[string]any{"query": "x", "limit": -1}}})
	scan := bufio.NewScanner(stdout)
	found := false
	for scan.Scan() {
		var msg map[string]any
		if json.Unmarshal(scan.Bytes(), &msg) != nil {
			continue
		}
		if msg["id"] == float64(2) {
			tools := msg["result"].(map[string]any)["tools"].([]any)
			for _, x := range tools {
				if x.(map[string]any)["name"] == "abc_fees" {
					found = true
				}
			}
			break
		}
	}
	if !found {
		t.Fatal("tools/list did not include abc_fees")
	}
}
