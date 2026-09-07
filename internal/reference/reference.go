// Package reference exposes source-backed California ABC public reference data.
package reference

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	htmlnode "golang.org/x/net/html"
)

const (
	cacheTTL      = 24 * time.Hour
	ua            = "abc-agent/0.1 (read-only public-reference-client)"
	maxFetchBytes = 5 << 20
)

var sourceURLs = map[string]string{
	"types":                   "https://www.abc.ca.gov/licensing/license-types/",
	"forms":                   "https://www.abc.ca.gov/licensing/license-forms/",
	"fees":                    "https://www.abc.ca.gov/licensing/license-fees/",
	"news":                    "https://www.abc.ca.gov/feed/",
	"advisories":              "https://www.abc.ca.gov/industry-advisories/",
	"advisories_page":         "https://www.abc.ca.gov/industry-advisories/",
	"new_license_application": "https://www.abc.ca.gov/licensing/license-forms/new-license-application/",
	"person_transfer":         "https://www.abc.ca.gov/licensing/transfer-or-change-a-license/person-to-person-transfer/",
	"requirements":            "https://www.abc.ca.gov/licensing/apply-for-a-new-license/license-application-requirements/",
	"caterers_permit":         "https://www.abc.ca.gov/licensing/license-forms/caterers-permit/",
	"event_authorization":     "https://www.abc.ca.gov/licensing/license-forms/event-authorization/",
}

type Provenance struct {
	Source  string `json:"source"`
	Fetched string `json:"fetched,omitempty"`
	Stale   bool   `json:"stale"`
	Offline bool   `json:"offline"`
	Caveat  string `json:"caveat"`
}

type TypeEntry struct{ Code, Description string }
type typeEntryJSON struct {
	Code        string `json:"code"`
	Description string `json:"description"`
}

func (t TypeEntry) MarshalJSON() ([]byte, error) { return json.Marshal(typeEntryJSON(t)) }

type TypesResult struct {
	Types      []TypeEntry `json:"types"`
	Provenance Provenance  `json:"provenance"`
}
type Form struct {
	Number  string `json:"number"`
	Title   string `json:"title"`
	Revised string `json:"revised"`
	URL     string `json:"url"`
}
type FormsResult struct {
	Forms      []Form     `json:"forms"`
	Provenance Provenance `json:"provenance"`
}
type Surcharge struct {
	Name      string `json:"name"`
	Purpose   string `json:"purpose"`
	Amount    string `json:"amount"`
	AppliesTo string `json:"applies_to"`
}
type FeesResult struct {
	Surcharges      []Surcharge `json:"surcharges"`
	PageTextExcerpt string      `json:"page_text_excerpt"`
	Provenance      Provenance  `json:"provenance"`
}
type NewsItem struct {
	Feed      string `json:"feed"`
	Title     string `json:"title"`
	Link      string `json:"link"`
	Published string `json:"published"`
	Summary   string `json:"summary"`
}
type NewsResult struct {
	Feed       string     `json:"feed"`
	Items      []NewsItem `json:"items"`
	Total      int        `json:"total"`
	Provenance Provenance `json:"provenance"`
}
type RequirementForm struct {
	Number  string  `json:"number"`
	Title   *string `json:"title"`
	URL     *string `json:"url"`
	InIndex bool    `json:"in_index"`
}
type RequirementsResult struct {
	LicenseType      string            `json:"license_type"`
	TypeName         string            `json:"type_name"`
	Action           string            `json:"action"`
	Forms            []RequirementForm `json:"forms"`
	ConditionalForms []RequirementForm `json:"conditional_forms"`
	Documents        []string          `json:"documents"`
	Notes            []string          `json:"notes"`
	Source           string            `json:"source"`
	SourceReviewed   string            `json:"source_reviewed"`
	MappedActions    []string          `json:"mapped_actions"`
	Provenance       Provenance        `json:"provenance"`
}

type cacheBlob[T any] struct {
	Fetched time.Time `json:"fetched"`
	Data    T         `json:"data"`
}

type ValidationError struct{ Message string }

func (e ValidationError) Error() string { return e.Message }
func IsValidationError(err error) bool  { var v ValidationError; return errors.As(err, &v) }

func Types(ctx context.Context, code string, offline bool) (any, error) {
	code = strings.TrimSpace(code)
	if code != "" && !validCode(code) {
		return nil, ValidationError{"license type must be a literal two-digit code"}
	}
	var m map[string]string
	p, err := getCached(ctx, "license_types.json", sourceURLs["types"], offline, parseTypes, &m)
	if err != nil {
		return nil, err
	}
	out := make([]TypeEntry, 0, len(m))
	for k, v := range m {
		if code == "" || code == k {
			out = append(out, TypeEntry{k, v})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Code < out[j].Code })
	return TypesResult{Types: out, Provenance: p}, nil
}

func Forms(ctx context.Context, search string, offline bool) (any, error) {
	forms, p, err := loadForms(ctx, offline)
	if err != nil {
		return nil, err
	}
	q := strings.ToLower(strings.TrimSpace(search))
	if q != "" {
		f := forms[:0]
		for _, x := range forms {
			if strings.Contains(strings.ToLower(x.Number), q) || strings.Contains(strings.ToLower(x.Title), q) {
				f = append(f, x)
			}
		}
		forms = f
	}
	return FormsResult{Forms: forms, Provenance: p}, nil
}

func Fees(ctx context.Context, offline bool) (any, error) {
	var r FeesResult
	p, err := getCached(ctx, "fees.json", sourceURLs["fees"], offline, parseFees, &r)
	if err != nil {
		return nil, err
	}
	r.Provenance = p
	return r, nil
}

func News(ctx context.Context, feed string, limit int, offline bool) (any, error) {
	if limit < 0 {
		return nil, ValidationError{"limit must be nonnegative"}
	}
	feed = strings.TrimSpace(feed)
	if feed == "" {
		feed = "news"
	}
	if feed != "news" && feed != "advisories" && feed != "all" {
		return nil, ValidationError{"feed must be news, advisories, or all"}
	}
	items := []NewsItem{}
	stale := false
	off := offline
	fetched := ""
	feeds := []string{feed}
	if feed == "all" {
		feeds = []string{"news", "advisories"}
	}
	for _, f := range feeds {
		got, p, err := loadFeed(ctx, f, offline)
		if err != nil {
			return nil, err
		}
		items = append(items, got...)
		stale = stale || p.Stale
		off = off || p.Offline
		if fetched == "" {
			fetched = f + "=" + p.Fetched
		} else if feed == "all" {
			fetched += "; " + f + "=" + p.Fetched
		}
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].Published > items[j].Published })
	total := len(items)
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	source := sourceURLs[feed]
	if feed == "all" {
		source = "news=" + sourceURLs["news"] + "; advisories=" + sourceURLs["advisories"]
	}
	return NewsResult{Feed: feed, Items: items, Total: total, Provenance: prov(source, fetched, stale, off)}, nil
}

func Requirements(ctx context.Context, code, action string, offline bool) (any, error) {
	c := strings.TrimSpace(code)
	if !validCode(c) {
		return nil, ValidationError{"license type must be a literal two-digit code"}
	}
	a := strings.ToLower(strings.TrimSpace(action))
	if a == "" {
		a = "new"
	}
	if a != "new" && a != "transfer" && a != "renewal" {
		return nil, ValidationError{"action must be one of new, transfer, renewal"}
	}
	e, ok := requirements[[2]string{c, a}]
	if !ok {
		return nil, ValidationError{fmt.Sprintf("requirements unmapped for type %s action %s", c, a)}
	}
	forms, p, err := loadForms(ctx, offline)
	if err != nil {
		return nil, err
	}
	by := map[string]Form{}
	for _, f := range forms {
		by[strings.ToLower(f.Number)] = f
	}
	resolve := func(nums []string) []RequirementForm {
		out := []RequirementForm{}
		for _, n := range nums {
			if f, ok := by[strings.ToLower(n)]; ok {
				title, url := f.Title, f.URL
				out = append(out, RequirementForm{f.Number, &title, &url, true})
			} else {
				out = append(out, RequirementForm{Number: n, InIndex: false})
			}
		}
		return out
	}
	acts := mapped(c)
	return RequirementsResult{LicenseType: c, TypeName: e.Name, Action: a, Forms: resolve(e.Forms), ConditionalForms: resolve(e.Conditional), Documents: e.Documents, Notes: append(append([]string{}, e.Notes...), "Curated guidance reviewed "+e.Reviewed+" from explicit ABC source page(s); verify current requirements with ABC before filing. This is not legal advice."), Source: e.Source, SourceReviewed: e.Reviewed, MappedActions: acts, Provenance: p}, nil
}

func loadForms(ctx context.Context, offline bool) ([]Form, Provenance, error) {
	var forms []Form
	p, err := getCached(ctx, "forms_index.json", sourceURLs["forms"], offline, parseForms, &forms)
	return forms, p, err
}
func loadFeed(ctx context.Context, feed string, offline bool) ([]NewsItem, Provenance, error) {
	var items []NewsItem
	p, err := getCached(ctx, "news_"+feed+".json", sourceURLs[feed], offline, func(b []byte) ([]NewsItem, error) {
		if feed == "advisories" {
			return parseAdvisoriesHTML(b)
		}
		return parseRSS(b, feed)
	}, &items)
	return items, p, err
}

func getCached[T any](ctx context.Context, name, src string, offline bool, parse func([]byte) (T, error), out *T) (Provenance, error) {
	path := filepath.Join(cacheDir(), name)
	data, fetched, valid := readCache(path, out)
	if valid && (offline || time.Since(fetched) < cacheTTL) {
		return prov(src, fetched.Format(time.RFC3339), time.Since(fetched) >= cacheTTL, offline), nil
	}
	if offline {
		return Provenance{}, fmt.Errorf("offline reference cache missing, null, stale, or unreadable at %s", path)
	}
	body, err := fetch(ctx, src)
	if err == nil {
		data, err = parse(body)
		if err == nil {
			now := time.Now().UTC()
			if err := writeCache(path, cacheBlob[T]{Fetched: now, Data: data}); err != nil {
				if valid {
					return prov(src, fetched.Format(time.RFC3339), true, true), nil
				}
				return Provenance{}, err
			}
			*out = data
			return prov(src, now.Format(time.RFC3339), false, false), nil
		}
	}
	if valid {
		return prov(src, fetched.Format(time.RFC3339), true, true), nil
	}
	return Provenance{}, err
}

func readCache[T any](path string, out *T) (T, time.Time, bool) {
	var zero T
	info, err := os.Stat(path)
	if err != nil || info.Size() > maxFetchBytes {
		return zero, time.Time{}, false
	}
	b, err := os.ReadFile(path)
	if err != nil || strings.TrimSpace(string(b)) == "null" {
		return zero, time.Time{}, false
	}
	var blob cacheBlob[T]
	if json.Unmarshal(b, &blob) != nil || blob.Fetched.IsZero() || blob.Fetched.After(time.Now().Add(5*time.Minute)) || isZeroJSON(blob.Data) {
		return zero, time.Time{}, false
	}
	*out = blob.Data
	return blob.Data, blob.Fetched.UTC(), true
}

func isZeroJSON[T any](v T) bool {
	b, err := json.Marshal(v)
	return err != nil || string(b) == "null" || string(b) == "{}" || string(b) == "[]"
}

func writeCache[T any](path string, blob cacheBlob[T]) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(blob, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, 0600); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
func cacheDir() string {
	if v := os.Getenv("ABC_AGENT_CACHE"); v != "" {
		return v
	}
	if v := os.Getenv("ABC_PP_CACHE"); v != "" {
		return v
	}
	d, _ := os.UserCacheDir()
	return filepath.Join(d, "abc-agent")
}
func prov(src, fetched string, stale, offline bool) Provenance {
	return Provenance{Source: src, Fetched: fetched, Stale: stale, Offline: offline, Caveat: "Public ABC reference data can change; cached/offline results may be stale and are not legal advice."}
}
func fetch(ctx context.Context, raw string) ([]byte, error) {
	u, err := url.Parse(raw)
	if err != nil || !((u.Scheme == "https" && u.Host == "www.abc.ca.gov") || (u.Scheme == "http" && strings.HasPrefix(u.Host, "127.0.0.1:"))) {
		return nil, fmt.Errorf("unsupported reference URL %s", raw)
	}
	c := http.Client{Timeout: 30 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if req.URL.Scheme != u.Scheme || req.URL.Host != u.Host {
			return http.ErrUseLastResponse
		}
		if len(via) > 5 {
			return http.ErrUseLastResponse
		}
		return nil
	}}
	req, err := http.NewRequestWithContext(ctx, "GET", raw, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", ua)
	r, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()
	if r.StatusCode < 200 || r.StatusCode > 299 {
		return nil, fmt.Errorf("fetch %s: %s", raw, r.Status)
	}
	b, err := io.ReadAll(io.LimitReader(r.Body, maxFetchBytes+1))
	if err != nil {
		return nil, err
	}
	if len(b) > maxFetchBytes {
		return nil, fmt.Errorf("fetch %s exceeded %d bytes", raw, maxFetchBytes)
	}
	ct := strings.ToLower(r.Header.Get("Content-Type"))
	head := strings.ToLower(string(b[:min(len(b), 2048)]))
	if strings.Contains(ct, "text/html") && strings.Contains(head, "<html") && (strings.Contains(head, "<title>challenge") || strings.Contains(head, "cloudflare") || strings.Contains(head, "attention required")) {
		return nil, fmt.Errorf("fetch %s returned html challenge", raw)
	}
	return b, nil
}

func strip(s string) string {
	s = regexp.MustCompile(`(?is)<script.*?</script>|<style.*?</style>`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`(?s)<[^>]+>`).ReplaceAllString(s, " ")
	return strings.Join(strings.Fields(html.UnescapeString(s)), " ")
}
func validCode(s string) bool {
	return regexp.MustCompile(`^[0-9]{2}$`).MatchString(s)
}

func parseTypes(b []byte) (map[string]string, error) {
	root, err := htmlnode.Parse(strings.NewReader(string(b)))
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	heading := regexp.MustCompile(`^(\d{2})\s*[-–]\s*(.+)$`)
	var walk func(*htmlnode.Node)
	walk = func(n *htmlnode.Node) {
		if n.Type == htmlnode.ElementNode && n.Data == "h3" {
			m := heading.FindStringSubmatch(nodeText(n))
			if len(m) > 0 {
				desc := ""
				for s := n.NextSibling; s != nil; s = s.NextSibling {
					if s.Type == htmlnode.ElementNode && (s.Data == "h3" || s.Data == "footer") {
						break
					}
					for _, a := range s.Attr {
						if a.Key == "class" && strings.Contains(a.Val, "et_pb_toggle_content") {
							desc = nodeText(s)
						}
					}
					if desc != "" {
						break
					}
				}
				out[m[1]] = m[2]
				if desc != "" {
					out[m[1]] += " — " + desc
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	if len(out) == 0 {
		return nil, fmt.Errorf("license types parser found no entries")
	}
	return out, nil
}

func nodeText(n *htmlnode.Node) string {
	var parts []string
	var walk func(*htmlnode.Node)
	walk = func(n *htmlnode.Node) {
		if n.Type == htmlnode.ElementNode && (n.Data == "script" || n.Data == "style") {
			return
		}
		if n.Type == htmlnode.TextNode {
			parts = append(parts, n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.Join(strings.Fields(strings.Join(parts, " ")), " ")
}
func parseForms(b []byte) ([]Form, error) {
	root, err := htmlnode.Parse(strings.NewReader(string(b)))
	if err != nil {
		return nil, err
	}
	out := []Form{}
	var walk func(*htmlnode.Node)
	walk = func(n *htmlnode.Node) {
		if n.Type == htmlnode.ElementNode && n.Data == "tr" {
			var cells []*htmlnode.Node
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == htmlnode.ElementNode && (c.Data == "th" || c.Data == "td") {
					cells = append(cells, c)
				}
			}
			if len(cells) == 3 {
				var links func(*htmlnode.Node)
				links = func(c *htmlnode.Node) {
					if c.Type == htmlnode.ElementNode && c.Data == "a" {
						number := nodeText(c)
						for _, a := range c.Attr {
							if a.Key == "href" && strings.HasPrefix(number, "ABC-") {
								out = append(out, Form{Number: number, Title: nodeText(cells[2]), Revised: nodeText(cells[1]), URL: a.Val})
							}
						}
					}
					for k := c.FirstChild; k != nil; k = k.NextSibling {
						links(k)
					}
				}
				links(cells[0])
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	if len(out) == 0 {
		return nil, fmt.Errorf("forms parser found no entries")
	}
	return out, nil
}
func parseFees(b []byte) (FeesResult, error) {
	htmls := string(b)
	out := FeesResult{}
	for _, table := range regexp.MustCompile(`(?is)<table[^>]*>(.*?)</table>`).FindAllStringSubmatch(htmls, -1) {
		head := strings.ToLower(strip(table[1]))
		if !strings.Contains(head, "surcharge name") || !strings.Contains(head, "purpose") || !strings.Contains(head, "amount") {
			continue
		}
		for _, row := range regexp.MustCompile(`(?is)<tr[^>]*>(.*?)</tr>`).FindAllStringSubmatch(table[1], -1) {
			cells := regexp.MustCompile(`(?is)<t[dh][^>]*>(.*?)</t[dh]>`).FindAllStringSubmatch(row[1], -1)
			vals := []string{}
			for _, c := range cells {
				vals = append(vals, strip(c[1]))
			}
			if len(vals) >= 3 && vals[0] != "" && !strings.EqualFold(vals[0], "Surcharge Name") {
				s := Surcharge{Name: vals[0], Purpose: vals[1], Amount: vals[2]}
				if len(vals) > 3 {
					s.AppliesTo = vals[3]
				}
				out.Surcharges = append(out.Surcharges, s)
			}
		}
	}
	if len(out.Surcharges) == 0 {
		return out, fmt.Errorf("surcharge table not found")
	}
	text := strip(htmls)
	if len(text) > 3000 {
		text = text[:3000]
	}
	out.PageTextExcerpt = text
	return out, nil
}

type rss struct {
	Items []struct {
		Title, Link, PubDate, Description string `xml:",chardata"`
	} `xml:"channel>item"`
}

func parseRSS(b []byte, feed string) ([]NewsItem, error) {
	var r struct {
		Channel struct {
			Items []struct {
				Title       string `xml:"title"`
				Link        string `xml:"link"`
				PubDate     string `xml:"pubDate"`
				Description string `xml:"description"`
			} `xml:"item"`
		} `xml:"channel"`
	}
	if err := xml.Unmarshal(b, &r); err != nil {
		return nil, err
	}
	out := []NewsItem{}
	for _, it := range r.Channel.Items {
		pub := strings.TrimSpace(it.PubDate)
		if t, err := time.Parse(time.RFC1123Z, pub); err == nil {
			pub = t.Format(time.RFC3339)
		}
		sum := strip(it.Description)
		if len(sum) > 400 {
			sum = sum[:400]
		}
		out = append(out, NewsItem{feed, strings.TrimSpace(it.Title), strings.TrimSpace(it.Link), pub, sum})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("RSS parser found no items")
	}
	return out, nil
}
func parseAdvisoriesHTML(b []byte) ([]NewsItem, error) {
	seen := map[string]bool{}
	out := []NewsItem{}
	for _, a := range regexp.MustCompile(`(?is)<article[^>]*category-industry-advisory.*?</article>`).FindAllString(string(b), -1) {
		m := regexp.MustCompile(`(?is)<h[23][^>]*>\s*<a href="([^"]+)"[^>]*>(.*?)</a>`).FindStringSubmatch(a)
		if len(m) == 0 {
			m = regexp.MustCompile(`(?is)<a href="([^"]+)"[^>]*>(.*?)</a>`).FindStringSubmatch(a)
		}
		if len(m) == 0 || seen[m[1]] {
			continue
		}
		seen[m[1]] = true
		d := regexp.MustCompile(`(?is)(?:class="published"[^>]*>|<time[^>]*>)([^<]+)<`).FindStringSubmatch(a)
		pub := ""
		if len(d) > 1 {
			pub = strings.TrimSpace(d[1])
			if t, err := time.Parse("Jan 2, 2006", pub); err == nil {
				pub = t.Format(time.RFC3339)
			}
		}
		summary := strip(regexp.MustCompile(`(?is)</h[23]>`).ReplaceAllString(a, " "))
		if len(summary) > 400 {
			summary = summary[:400]
		}
		out = append(out, NewsItem{"advisories", strip(m[2]), html.UnescapeString(m[1]), pub, summary})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("advisories parser found no entries")
	}
	return out, nil
}
