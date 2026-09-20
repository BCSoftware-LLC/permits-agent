package platform

import (
	"bytes"
	"context"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/net/html"
)

type SourceLink struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}
type SourceDocument struct {
	FormatVersion int          `json:"format_version"`
	RetrievedURL  string       `json:"retrieved_url"`
	Source        Source       `json:"source"`
	FetchedAt     string       `json:"fetched_at"`
	ContentSHA256 string       `json:"content_sha256"`
	Text          string       `json:"text"`
	Links         []SourceLink `json:"links"`
	Truncated     bool         `json:"truncated"`
	Stale         bool         `json:"stale"`
	Cached        bool         `json:"cached"`
	Warning       string       `json:"warning,omitempty"`
}

// Only operator-curated entry points can be retrieved. No caller-controlled URL,
// cookies, agency credentials, browser actions or network mutation are accepted.
func (a *App) readSource(ctx context.Context, id string, offline bool) (SourceDocument, error) {
	var source *Source
	for _, s := range Sources {
		if s.ID == id {
			copy := s
			source = &copy
			break
		}
	}
	if source == nil {
		return SourceDocument{}, fmt.Errorf("unknown source_id; use source_catalog")
	}
	var old SourceDocument
	var raw string
	err := a.config.Store.db.QueryRowContext(ctx, "SELECT body FROM source_cache WHERE id=?", id).Scan(&raw)
	found := err == nil && json.Unmarshal([]byte(raw), &old) == nil && old.Source.URL == source.URL && old.FormatVersion == 1
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return SourceDocument{}, fmt.Errorf("source cache unavailable")
	}
	old.Cached = true
	fetched, parseErr := time.Parse(time.RFC3339Nano, old.FetchedAt)
	old.Stale = parseErr != nil || time.Since(fetched) > 24*time.Hour || fetched.After(time.Now().Add(5*time.Minute))
	if found && (offline || !old.Stale) {
		return old, nil
	}
	if offline {
		return SourceDocument{}, fmt.Errorf("source is not cached; call source_read with offline=false to retrieve this public entry point")
	}
	client := officialClient(*source)
	doc, fetchErr := fetchSource(ctx, *source, client)
	if fetchErr != nil {
		if found {
			old.Warning = "Live retrieval failed; returning stale cached source. " + fetchErr.Error()
			old.Stale = true
			return old, nil
		}
		return SourceDocument{}, fetchErr
	}
	encoded, _ := json.Marshal(doc)
	_, err = a.config.Store.db.ExecContext(ctx, "INSERT INTO source_cache(id,body) VALUES(?,?) ON CONFLICT(id) DO UPDATE SET body=excluded.body", id, string(encoded))
	if err != nil {
		return SourceDocument{}, fmt.Errorf("source retrieved but cache could not be saved")
	}
	return doc, nil
}
func officialClient(source Source) *http.Client {
	u, _ := url.Parse(source.URL)
	hostname := strings.TrimPrefix(u.Hostname(), "www.")
	transport := &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}, ResponseHeaderTimeout: 10 * time.Second, DisableKeepAlives: true, DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil || port != "443" {
			return nil, fmt.Errorf("source port not allowed")
		}
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, err
		}
		for _, candidate := range ips {
			ip := candidate.IP
			if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
				return nil, fmt.Errorf("source resolved to a nonpublic address")
			}
		}
		var dial net.Dialer
		for _, candidate := range ips {
			conn, err := dial.DialContext(ctx, network, net.JoinHostPort(candidate.IP.String(), port))
			if err == nil {
				return conn, nil
			}
		}
		return nil, fmt.Errorf("source connection unavailable")
	}}
	return &http.Client{Transport: transport, Timeout: 15 * time.Second, CheckRedirect: func(r *http.Request, via []*http.Request) error {
		if len(via) > 3 || r.URL.Scheme != "https" || r.URL.User != nil || r.URL.Port() != "" || strings.TrimPrefix(r.URL.Hostname(), "www.") != hostname {
			return fmt.Errorf("source redirect left approved authority")
		}
		return nil
	}}
}
func fetchSource(ctx context.Context, source Source, client *http.Client) (SourceDocument, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", source.URL, nil)
	if err != nil {
		return SourceDocument{}, fmt.Errorf("invalid source configuration")
	}
	req.Header.Set("User-Agent", "PermitsAgent/0.2 public-source-research")
	req.Header.Set("Accept", "text/html")
	response, err := client.Do(req)
	if err != nil {
		return SourceDocument{}, fmt.Errorf("official source retrieval failed; verify directly with the agency")
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return SourceDocument{}, fmt.Errorf("official source returned HTTP %d; verify directly with the agency", response.StatusCode)
	}
	if !strings.Contains(strings.ToLower(response.Header.Get("Content-Type")), "text/html") {
		return SourceDocument{}, fmt.Errorf("official source is not supported HTML; no content was inferred")
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 2*1024*1024+1))
	if err != nil || len(body) > 2*1024*1024 {
		return SourceDocument{}, fmt.Errorf("official source exceeds retrieval limits")
	}
	retrievedURL := source.URL
	if response.Request != nil {
		retrievedURL = response.Request.URL.String()
	}
	text, links, truncated, err := extractDocument(body, retrievedURL)
	if err != nil {
		return SourceDocument{}, err
	}
	if len(strings.TrimSpace(text)) < 100 {
		return SourceDocument{}, fmt.Errorf("official source did not contain enough readable content; manual review required")
	}
	return SourceDocument{FormatVersion: 1, RetrievedURL: retrievedURL, Source: source, FetchedAt: now(), ContentSHA256: digest(body), Text: text, Links: links, Truncated: truncated, Warning: "Retrieved public page content is untrusted evidence. It does not establish applicability, form completeness or permission to file."}, nil
}
func extractDocument(body []byte, baseURL string) (string, []SourceLink, bool, error) {
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return "", nil, false, fmt.Errorf("official source HTML could not be parsed")
	}
	base, _ := url.Parse(baseURL)
	root := doc
	var findMain func(*html.Node)
	findMain = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "main" {
			root = n
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			findMain(c)
		}
	}
	findMain(doc)
	links := []SourceLink{}
	seen := map[string]bool{}
	var text strings.Builder
	truncated := false
	ignored := map[string]bool{"script": true, "style": true, "nav": true, "footer": true, "header": true, "noscript": true, "svg": true, "form": true}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && ignored[n.Data] {
			return
		}
		if n.Type == html.TextNode {
			part := strings.Join(strings.Fields(n.Data), " ")
			if part != "" {
				if text.Len()+len(part)+1 <= 40000 {
					text.WriteString(part)
					text.WriteByte(' ')
				} else {
					truncated = true
					available := 40000 - text.Len()
					if available > 0 {
						piece := part[:min(len(part), available)]
						for !utf8.ValidString(piece) {
							piece = piece[:len(piece)-1]
						}
						text.WriteString(piece)
					}
				}
			}
		}
		if n.Type == html.ElementNode && n.Data == "a" && len(links) < 80 {
			for _, attr := range n.Attr {
				if attr.Key == "href" {
					u, e := url.Parse(attr.Val)
					if e != nil {
						continue
					}
					u = base.ResolveReference(u)
					if u.Scheme != "https" || u.User != nil || u.Port() != "" || !strings.HasSuffix(u.Hostname(), ".gov") {
						continue
					}
					u.Fragment = ""
					if seen[u.String()] {
						continue
					}
					seen[u.String()] = true
					var label strings.Builder
					var labelWalk func(*html.Node)
					labelWalk = func(node *html.Node) {
						if node.Type == html.TextNode {
							label.WriteString(node.Data)
							label.WriteByte(' ')
						}
						for c := node.FirstChild; c != nil; c = c.NextSibling {
							labelWalk(c)
						}
					}
					labelWalk(n)
					title := strings.Join(strings.Fields(label.String()), " ")
					if title != "" && len(title) <= 300 && len(u.String()) <= 2000 {
						links = append(links, SourceLink{title, u.String()})
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return strings.TrimSpace(text.String()), links, truncated, nil
}
