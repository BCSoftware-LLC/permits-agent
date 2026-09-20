package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAllDataCommandsAtBinaryBoundary(t *testing.T) {
	bin, cache := buildCLI(t)
	fixtures := map[string]any{
		"license_types.json":   map[string]string{"47": "On-Sale General"},
		"forms_index.json":     []map[string]string{{"number": "ABC-257", "title": "Diagram", "url": "https://www.abc.ca.gov/forms/ABC-257.pdf"}},
		"fees.json":            map[string]any{"surcharges": []map[string]string{{"name": "Test surcharge", "amount": "$1"}}},
		"news_news.json":       []map[string]string{{"feed": "news", "title": "Test", "link": "https://www.abc.ca.gov/test", "published": "2026-08-25T00:00:00Z"}},
		"news_advisories.json": []map[string]string{{"feed": "advisories", "title": "Test advisory", "link": "https://www.abc.ca.gov/test-advisory", "published": "2026-08-25T00:00:00Z"}},
	}
	for file, data := range fixtures {
		b, err := json.Marshal(map[string]any{"fetched": "2026-08-25T00:00:00Z", "data": data})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(cache, file), b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	cases := [][]string{{"refresh"}, {"stats"}, {"address", "SELMA", "--status", "ACTIVE"}, {"get", "00677768"}, {"search", "EXAMPLE"}, {"area", "--city", "HOLLYWOOD"}, {"by", "--city", "HOLLYWOOD"}, {"pending"}, {"expiring", "--days", "0"}, {"overdue"}, {"statuses"}, {"types", "--offline"}, {"forms", "--offline"}, {"fees", "--offline"}, {"requirements", "47", "--action", "new", "--offline"}, {"requirements", "--type", "47", "--action", "new", "--offline"}, {"news", "--feed", "all", "--offline"}, {"digest", "--watch", "00677768"}, {"history-snapshot"}, {"history-diff"}, {"history"}, {"doctor"}}
	for _, args := range cases {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			out, errout, code := runCLI(t, bin, cache, append(args, "--json")...)
			if code != 0 || !json.Valid([]byte(out)) || strings.Contains(errout, "panic:") {
				t.Fatalf("code=%d stdout=%s stderr=%s", code, out, errout)
			}
		})
	}
}
func TestLegacyDigestAreas(t *testing.T) {
	bin, cache := buildCLI(t)
	out, errout, code := runCLI(t, bin, cache, "digest", "--zips", "90028,90069", "--counties", "LOS ANGELES", "--limit", "1", "--json")
	if code != 0 {
		t.Fatalf("%d %s", code, errout)
	}
	var result struct {
		Brief string
		Areas []struct {
			Kind         string
			Value        string
			PendingCount int `json:"pending_count"`
			Pending      []json.RawMessage
		}
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatal(err)
	}
	// The fixture has one PEND record in 90028; total must reflect it.
	if result.Brief == "" || len(result.Areas) != 3 {
		t.Fatalf("result=%s", out)
	}
	if result.Areas[0].Value != "90028" || result.Areas[0].PendingCount != 1 {
		t.Fatalf("first area=%+v", result.Areas[0])
	}
}

func TestStaleMirrorWarnsWithoutPollutingJSON(t *testing.T) {
	bin, cache := buildCLI(t)
	out, errout, code := runCLI(t, bin, cache, "stats", "--json")
	if code != 0 || !json.Valid([]byte(out)) || !strings.Contains(errout, "stale") {
		t.Fatalf("code=%d stderr=%q stdout=%s", code, errout, out)
	}
}

func TestInvalidFiltersBeforeMissingCache(t *testing.T) {
	bin, _ := buildCLI(t)
	for _, args := range [][]string{{"search", "test", "--limit", "-1"}, {"search", "test", "--type", "1"}, {"address", "test", "--status", "NOTREAL"}, {"area", "--city", "LA", "--zip", "90028"}, {"digest", "--limit", "-1"}, {"digest", "--watch", "123"}} {
		_, errout, code := runCLI(t, bin, filepath.Join(t.TempDir(), "missing"), args...)
		if code != 2 || strings.Contains(errout, "panic:") {
			t.Fatalf("%v: code%d %s", args, code, errout)
		}
	}
}
