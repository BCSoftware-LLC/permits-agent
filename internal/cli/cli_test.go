package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BCSoftware-LLC/permits-agent/internal/abc"
)

const fixtureCSV = "\ufeff\"Updated Tuesday 25th of August 2026 03:50:22 AM\"\n\"License Type\",\"File Number\",\"Lic or App\",\"Type Status\",\"Type Orig Iss Date\",\"Expir Date\",\"Fee Codes\",\"Dup Counts\",\"Master Ind\",\"Term in # of Months\",\"Geo Code\",District,\"Primary Name\",\"Prem Addr 1\",\" Prem Addr 2\",\"Prem City\",\" Prem State\",\"Prem Zip\",\"DBA Name\",\"Mail Addr 1\",\"Mail Addr 2\",\"Mail City\",\"Mail State\",\"Mail Zip\",\"Prem County\",\"Prem Census Tract #\"\n47,00677768,LIC,ACTIVE,01-JAN-2020,31-DEC-2099,,,,12,1900,04,\"EXAMPLE OWNER\",\"6417 SELMA AVE\",\" \",HOLLYWOOD,CA,90028,\"EXAMPLE DBA\",,,,,,LOS ANGELES,\n21,00700000,APP,PEND,\" \",\" \",,,,12,1900,04,\"PENDING OWNER\",\"100 TEST ST\",\" \",HOLLYWOOD,CA,90028,\"PENDING DBA\",,,,,,LOS ANGELES,\n"

func buildCLI(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "abc-agent")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/abc-agent")
	cmd.Dir = "../.."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	cache := filepath.Join(dir, "cache")
	csv := filepath.Join(cache, "ABC-DailyDataExport.csv")
	if err := os.MkdirAll(cache, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(csv, []byte(fixtureCSV), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := abc.ImportCSV(context.Background(), csv, filepath.Join(cache, "abc.sqlite")); err != nil {
		t.Fatal(err)
	}
	return bin, cache
}
func runCLI(t *testing.T, bin, cache string, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = append(os.Environ(), "ABC_AGENT_CACHE="+cache)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ee, ok := err.(*exec.ExitError); ok {
		return stdout.String(), stderr.String(), ee.ExitCode()
	}
	if err != nil {
		t.Fatal(err)
	}
	return stdout.String(), stderr.String(), 0
}
func TestRealBinaryJSONAndValidation(t *testing.T) {
	bin, cache := buildCLI(t)
	out, errout, code := runCLI(t, bin, cache, "search", "EXAMPLE", "--json")
	if code != 0 {
		t.Fatalf("code %d err %q", code, errout)
	}
	var rows []map[string]any
	if err := json.Unmarshal([]byte(out), &rows); err != nil || len(rows) != 1 {
		t.Fatalf("bad json %v %s", err, out)
	}
	_, errout, code = runCLI(t, bin, cache, "search", "EXAMPLE", "--limit", "-1", "--json")
	if code != 2 || !strings.Contains(errout, "limit") {
		t.Fatalf("want usage code 2 validation, got %d %q", code, errout)
	}
}
func TestReferenceOfflineCommandNeedsNoMirror(t *testing.T) {
	bin, _ := buildCLI(t)
	cache := t.TempDir()
	if err := os.WriteFile(filepath.Join(cache, "fees.json"), []byte(`{"fetched":"2026-08-25T00:00:00Z","data":{"surcharges":[],"page_text_excerpt":"cached"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	out, errout, code := runCLI(t, bin, cache, "fees", "--offline", "--json")
	if code != 0 || !json.Valid([]byte(out)) {
		t.Fatalf("bad offline fees code=%d err=%s out=%s", code, errout, out)
	}
}

func TestEveryCommandHelpDoesNotPanic(t *testing.T) {
	bin, cache := buildCLI(t)
	commands := []string{"refresh", "stats", "address", "get", "search", "area", "pending", "expiring", "overdue", "statuses", "types", "forms", "fees", "requirements", "news", "digest", "history-snapshot", "history-diff", "history", "doctor", "mcp", "serve", "version"}
	for _, name := range commands {
		_, errout, code := runCLI(t, bin, cache, name, "--help")
		if code != 0 || strings.Contains(strings.ToLower(errout), "panic") {
			t.Fatalf("%s --help code=%d stderr=%q", name, code, errout)
		}
	}
}

func TestInvalidInputsReturnUsageBeforeCacheOpen(t *testing.T) {
	bin, _ := buildCLI(t)
	missingCache := filepath.Join(t.TempDir(), "missing")
	cases := [][]string{{"get", "677768"}, {"expiring", "--days", "-1"}, {"overdue", "--min-days", "-1"}, {"types", "47", "extra"}, {"refresh", "extra"}, {"nope"}}
	for _, args := range cases {
		_, errout, code := runCLI(t, bin, missingCache, args...)
		if code != 2 || strings.Contains(strings.ToLower(errout), "panic") {
			t.Fatalf("%v code=%d stderr=%q", args, code, errout)
		}
	}
}
