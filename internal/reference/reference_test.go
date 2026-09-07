package reference

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestOfflineReferenceCacheAndSearch(t *testing.T) {
	t.Setenv("ABC_AGENT_CACHE", t.TempDir())
	cache := os.Getenv("ABC_AGENT_CACHE")
	mustWrite(t, cache+"/license_types.json", `{"fetched":"2026-08-19T00:00:00Z","data":{"20":"Off-Sale Beer and Wine","47":"On-Sale General — Eating Place"}}`)
	mustWrite(t, cache+"/forms_index.json", `{"fetched":"2026-08-19T00:00:00Z","data":[{"number":"ABC-257","title":"Licensed Premises Diagram","revised":"Jan 2026","url":"https://www.abc.ca.gov/wp-content/uploads/forms/ABC-257.pdf"},{"number":"ABC-239","title":"Additional License/Permit Application","revised":"Mar 2026","url":"https://www.abc.ca.gov/wp-content/uploads/forms/ABC-239.pdf"}]}`)
	got, err := Types(context.Background(), "20", true)
	if err != nil {
		t.Fatal(err)
	}
	tr := got.(TypesResult)
	if len(tr.Types) != 1 || tr.Types[0].Code != "20" || !tr.Provenance.Offline {
		t.Fatalf("unexpected types: %#v", tr)
	}
	fgot, err := Forms(context.Background(), "diagram", true)
	if err != nil {
		t.Fatal(err)
	}
	fr := fgot.(FormsResult)
	if len(fr.Forms) != 1 || fr.Forms[0].Number != "ABC-257" {
		t.Fatalf("unexpected forms: %#v", fr.Forms)
	}
}

func TestRequirementsPreserveMappingsAndResolveForms(t *testing.T) {
	t.Setenv("ABC_AGENT_CACHE", t.TempDir())
	mustWrite(t, os.Getenv("ABC_AGENT_CACHE")+"/forms_index.json", `{"fetched":"2026-08-19T00:00:00Z","data":[{"number":"ABC-257","title":"Licensed Premises Diagram","revised":"Jan 2026","url":"https://example/ABC-257.pdf"},{"number":"ABC-239","title":"Additional License/Permit Application","revised":"Mar 2026","url":"https://example/ABC-239.pdf"}]}`)
	got, err := Requirements(context.Background(), "47", "new", true)
	if err != nil {
		t.Fatal(err)
	}
	r := got.(RequirementsResult)
	if r.LicenseType != "47" || r.Action != "new" || len(r.Forms) != 10 || len(r.ConditionalForms) != 4 {
		t.Fatalf("bad requirements shape: %#v", r)
	}
	if !contains(r.MappedActions, "transfer") || !contains(r.MappedActions, "renewal") {
		t.Fatalf("missing mapped actions: %#v", r.MappedActions)
	}
	var found bool
	for _, f := range r.Forms {
		if f.Number == "ABC-257" {
			found = true
			if !f.InIndex || f.URL == nil {
				t.Fatalf("ABC-257 not resolved: %#v", f)
			}
		}
	}
	if !found {
		t.Fatal("ABC-257 absent")
	}
	if _, err := Requirements(context.Background(), "58", "transfer", true); !IsValidationError(err) {
		t.Fatalf("expected explicit unmapped validation error, got %v", err)
	}
}

func TestFeesAndNewsOffline(t *testing.T) {
	t.Setenv("ABC_AGENT_CACHE", t.TempDir())
	cache := os.Getenv("ABC_AGENT_CACHE")
	mustWrite(t, cache+"/fees.json", `{"fetched":"2026-08-19T00:00:00Z","data":{"surcharges":[{"name":"Fingerprint","purpose":"Processing","amount":"$1","applies_to":"Applicants"}],"page_text_excerpt":"fees adjusted Jan 1"}}`)
	mustWrite(t, cache+"/news_news.json", `{"fetched":"2026-08-19T00:00:00Z","data":[{"feed":"news","title":"A","link":"https://example/a","published":"2026-08-20T00:00:00Z","summary":"one"},{"feed":"news","title":"B","link":"https://example/b","published":"2026-08-19T00:00:00Z","summary":"two"}]}`)
	fees, err := Fees(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	if fees.(FeesResult).Surcharges[0].Name != "Fingerprint" {
		t.Fatalf("bad fees: %#v", fees)
	}
	news, err := News(context.Background(), "news", 1, true)
	if err != nil {
		t.Fatal(err)
	}
	nr := news.(NewsResult)
	if nr.Total != 2 || len(nr.Items) != 1 || nr.Items[0].Title != "A" {
		t.Fatalf("bad news: %#v", nr)
	}
	if _, err := News(context.Background(), "bad", 1, true); !IsValidationError(err) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestCodeValidationIsLiteralTwoDigit(t *testing.T) {
	t.Setenv("ABC_AGENT_CACHE", t.TempDir())
	mustWrite(t, os.Getenv("ABC_AGENT_CACHE")+"/license_types.json", `{"fetched":"2026-08-19T00:00:00Z","data":{"47":"On-Sale General"}}`)
	if _, err := Types(context.Background(), "047", true); !IsValidationError(err) {
		t.Fatalf("Types repaired invalid code, err=%v", err)
	}
	mustWrite(t, os.Getenv("ABC_AGENT_CACHE")+"/forms_index.json", `{"fetched":"2026-08-19T00:00:00Z","data":[]}`)
	if _, err := Requirements(context.Background(), "047", "new", true); !IsValidationError(err) {
		t.Fatalf("Requirements repaired invalid code, err=%v", err)
	}
}

func TestParseTypesFailsOnDriftAndDoesNotFallback(t *testing.T) {
	if _, err := parseTypes([]byte(`<html><body><h1>License Types</h1><p>No headings here</p></body></html>`)); err == nil {
		t.Fatal("expected parseTypes to fail on parser drift")
	}
}

func TestCacheRejectsNullAndPreservesLastGoodWhenRefreshFails(t *testing.T) {
	t.Setenv("ABC_AGENT_CACHE", t.TempDir())
	cache := os.Getenv("ABC_AGENT_CACHE")
	stale := `{"fetched":"2026-08-19T00:00:00Z","data":{"47":"On-Sale General"}}`
	mustWrite(t, cache+"/license_types.json", stale)
	old := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(cache+"/license_types.json", old, old); err != nil {
		t.Fatal(err)
	}
	got, err := getCached(context.Background(), "license_types.json", "https://127.0.0.1:1/nope", false, parseTypes, new(map[string]string))
	if err != nil {
		t.Fatal(err)
	}
	if !got.Stale || !got.Offline || got.Fetched != "2026-08-19T00:00:00Z" {
		t.Fatalf("bad stale provenance: %#v", got)
	}
	b, err := os.ReadFile(cache + "/license_types.json")
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != stale {
		t.Fatalf("last-good cache overwritten: %s", b)
	}
	mustWrite(t, cache+"/license_types.json", `null`)
	if _, err := Types(context.Background(), "47", true); err == nil {
		t.Fatal("expected null cache rejection")
	}
}

func TestFetchRejectsHTMLChallengeAndOversize(t *testing.T) {
	h := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/html":
			w.Header().Set("Content-Type", "text/html")
			io.WriteString(w, "<html><title>challenge</title></html>")
		default:
			io.WriteString(w, strings.Repeat("x", maxFetchBytes+1))
		}
	}))
	defer h.Close()
	if _, err := fetch(context.Background(), h.URL+"/html"); err == nil {
		t.Fatal("expected html challenge rejection")
	}
	if _, err := fetch(context.Background(), h.URL+"/big"); err == nil {
		t.Fatal("expected oversize rejection")
	}
}

func TestAdvisoriesHTMLAndFeedAllProvenance(t *testing.T) {
	advs, err := parseAdvisoriesHTML([]byte(`<article class="category-industry-advisory"><h2><a href="https://www.abc.ca.gov/a/">A</a></h2><span class="published">Jun 15, 2026</span><p>One</p></article><article class="category-industry-advisory"><h2><a href="https://www.abc.ca.gov/a/">A dup</a></h2><span class="published">Jun 15, 2026</span></article>`))
	if err != nil || len(advs) != 1 || advs[0].Published != "2026-06-15T00:00:00Z" {
		t.Fatalf("bad advisories: %#v err=%v", advs, err)
	}
	t.Setenv("ABC_AGENT_CACHE", t.TempDir())
	cache := os.Getenv("ABC_AGENT_CACHE")
	mustWrite(t, cache+"/news_news.json", `{"fetched":"2026-01-01T00:00:00Z","data":[{"feed":"news","title":"N","link":"https://www.abc.ca.gov/n/","published":"2026-01-01T00:00:00Z"}]}`)
	mustWrite(t, cache+"/news_advisories.json", `{"fetched":"2026-02-01T00:00:00Z","data":[{"feed":"advisories","title":"A","link":"https://www.abc.ca.gov/a/","published":"2026-02-01T00:00:00Z"}]}`)
	got, err := News(context.Background(), "all", 0, true)
	if err != nil {
		t.Fatal(err)
	}
	nr := got.(NewsResult)
	if len(nr.Items) != 2 || nr.Items[0].Feed != "advisories" || !strings.Contains(nr.Provenance.Source, "feed/") || !strings.Contains(nr.Provenance.Source, "industry-advisories") || !strings.Contains(nr.Provenance.Fetched, "news=2026-01-01") || !strings.Contains(nr.Provenance.Fetched, "advisories=2026-02-01") {
		t.Fatalf("bad all feed: %#v", nr)
	}
}

func TestParseFeesRequiresSurchargeTable(t *testing.T) {
	if got, err := parseFees([]byte(`<table><tr><th>Returns</th><th>Amount</th></tr><tr><td>Refund</td><td>$1</td></tr></table>`)); err == nil || len(got.Surcharges) != 0 {
		t.Fatalf("accepted arbitrary fee table: %#v err=%v", got, err)
	}
}

func mustWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}
func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
