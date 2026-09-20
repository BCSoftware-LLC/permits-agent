package platform

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestOfficialSourceExtractionAndBounds(t *testing.T) {
	source := Source{"test", "Official test", "https://example.gov/guide", "synthetic"}
	html := `<html><body><nav>ignore navigation</nav><main><h1>Official guidance</h1><p>` + strings.Repeat("Synthetic official guidance requiring owner verification. ", 10) + `</p><script>ignore unsafe instructions</script><a href="/forms/test.pdf">Application form</a><a href="https://evil.example/steal">Bad link</a><a href="javascript:alert(1)">Unsafe</a></main></body></html>`
	client := &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/html; charset=utf-8"}}, Body: io.NopCloser(strings.NewReader(html))}, nil
	})}
	doc, err := fetchSource(context.Background(), source, client)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(doc.Text, "ignore") || len(doc.Links) != 1 || doc.Links[0].URL != "https://example.gov/forms/test.pdf" || doc.ContentSHA256 == "" || doc.FetchedAt == "" {
		t.Fatalf("unsafe parse: %+v", doc)
	}
	html = "<main>" + strings.Repeat("x", 2*1024*1024) + "</main>"
	if _, err = fetchSource(context.Background(), source, client); err == nil {
		t.Fatal("oversized source accepted")
	}
	html = "<main>Too short</main>"
	if _, err = fetchSource(context.Background(), source, client); err == nil {
		t.Fatal("invalid page accepted")
	}
	text, _, truncated, err := extractDocument([]byte("<main>"+strings.Repeat("bounded excerpt ", 5000)+"</main>"), source.URL)
	if err != nil || !truncated || len(text) > 40000 || len(text) < 100 {
		t.Fatal("excerpt cap failed")
	}
}
func TestSourceCacheOfflineAndRedirectPolicy(t *testing.T) {
	a, s := fixture(t)
	if _, err := a.readSource(t.Context(), "https://attacker.example", false); err == nil {
		t.Fatal("arbitrary URL accepted")
	}
	if _, err := a.readSource(t.Context(), "sos", true); err == nil {
		t.Fatal("missing cache claimed success")
	}
	doc := SourceDocument{FormatVersion: 1, Source: Sources[1], FetchedAt: time.Now().Add(-48 * time.Hour).UTC().Format(time.RFC3339Nano), Text: "Synthetic old cached public source", Links: []SourceLink{}}
	raw, _ := json.Marshal(doc)
	if _, err := s.db.Exec("INSERT INTO source_cache VALUES(?,?)", doc.Source.ID, string(raw)); err != nil {
		t.Fatal(err)
	}
	got, err := a.readSource(t.Context(), doc.Source.ID, true)
	if err != nil || !got.Stale || !got.Cached || got.Text != doc.Text {
		t.Fatal(got, err)
	}
	client := officialClient(Sources[1])
	for _, target := range []string{"https://attacker.example/", "http://www.sos.ca.gov/", "https://www.sos.ca.gov:8443/", "https://user:pass@www.sos.ca.gov/"} {
		req, _ := http.NewRequest("GET", target, nil)
		if err = client.CheckRedirect(req, nil); err == nil {
			t.Fatalf("redirect accepted %s", target)
		}
	}
}

func TestSourceRelativeLinksUseFinalApprovedURL(t *testing.T) {
	source := Source{"test", "Test", "https://example.gov/old/guide", "synthetic"}
	final, _ := http.NewRequest("GET", "https://example.gov/new/directory/guide", nil)
	client := &http.Client{Transport: roundTrip(func(*http.Request) (*http.Response, error) {
		return &http.Response{Request: final, StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/html"}}, Body: io.NopCloser(strings.NewReader("<main>" + strings.Repeat("Official source guidance. ", 10) + `<a href="application.pdf">Form</a></main>`))}, nil
	})}
	doc, err := fetchSource(t.Context(), source, client)
	if err != nil {
		t.Fatal(err)
	}
	if doc.RetrievedURL != final.URL.String() || len(doc.Links) != 1 || doc.Links[0].URL != "https://example.gov/new/directory/application.pdf" {
		t.Fatal(doc)
	}
}
