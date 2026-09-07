package reference

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func mockOfficial(t *testing.T, body string) {
	t.Helper()
	old := http.DefaultTransport
	http.DefaultTransport = transportFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/html"}}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = old })
}
func TestPublicReferencesRejectEmptyParserDrift(t *testing.T) {
	t.Setenv("ABC_AGENT_CACHE", t.TempDir())
	mockOfficial(t, "<html><body>Temporarily unavailable</body></html>")
	if _, err := Forms(context.Background(), "", false); err == nil {
		t.Fatal("Forms accepted empty parser drift")
	}
	if _, err := News(context.Background(), "news", 0, false); err == nil {
		t.Fatal("News accepted HTML as empty RSS")
	}
}
func TestOfficialRedirectCannotReachLoopback(t *testing.T) {
	old := http.DefaultTransport
	defer func() { http.DefaultTransport = old }()
	localHit := false
	http.DefaultTransport = transportFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "www.abc.ca.gov" {
			localHit = true
			return nil, fmt.Errorf("unexpected loopback")
		}
		return &http.Response{StatusCode: 302, Header: http.Header{"Location": []string{"http://127.0.0.1:9876/private"}}, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
	})
	_, err := fetch(context.Background(), sourceURLs["forms"])
	if err == nil || localHit {
		t.Fatalf("redirect reached private host: hit=%v err=%v", localHit, err)
	}
}
func TestReferenceCacheRejectsFutureAndEmptyPayload(t *testing.T) {
	t.Setenv("ABC_AGENT_CACHE", t.TempDir())
	for _, body := range []string{`{"fetched":"2999-01-01T00:00:00Z","data":{"47":"test"}}`, `{"fetched":"2026-08-19T00:00:00Z","data":{}}`} {
		mustWrite(t, filepath.Join(os.Getenv("ABC_AGENT_CACHE"), "license_types.json"), body)
		if _, err := Types(context.Background(), "", true); err == nil {
			t.Fatal("invalid cache accepted", body)
		}
	}
}
func TestOfficialTypesFinalDescriptionStopsAtFooter(t *testing.T) {
	t.Setenv("ABC_AGENT_CACHE", t.TempDir())
	mockOfficial(t, `<h3>99 - Official type</h3><div class="et_pb_toggle_content">A valid description.</div><footer>Privacy Policy Contact Newsletter</footer>`)
	got, err := Types(context.Background(), "99", false)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got.(TypesResult).Types[0].Description, "Privacy Policy") {
		t.Fatal("footer included in official type description")
	}
}
func TestMirrorReferenceCacheRecordedTime(t *testing.T) {
	t.Setenv("ABC_AGENT_CACHE", t.TempDir())
	stamp := time.Now().Add(-48 * time.Hour).UTC().Format(time.RFC3339)
	mustWrite(t, filepath.Join(os.Getenv("ABC_AGENT_CACHE"), "license_types.json"), fmt.Sprintf(`{"fetched":%q,"data":{"47":"test"}}`, stamp))
	got, err := Types(context.Background(), "47", true)
	if err != nil || !got.(TypesResult).Provenance.Stale {
		t.Fatalf("mtime overrides fetched: %v %v", got, err)
	}
}
