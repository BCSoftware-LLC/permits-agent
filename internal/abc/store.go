// Package abc provides the read-only local mirror for California ABC public data.
package abc

import (
	"archive/zip"
	"bufio"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
	_ "time/tzdata"

	_ "modernc.org/sqlite"
)

const (
	ExportURL = "https://www.abc.ca.gov/wp-content/uploads/DailyExport-CSV.zip"
	cacheTTL  = 12 * time.Hour
	maxZip    = 200 << 20
	maxCSV    = 500 << 20
)

var laLocation = mustLoadLA()

func mustLoadLA() *time.Location {
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		panic(err)
	}
	return loc
}

type ValidationError struct{ Message string }

func (e ValidationError) Error() string { return e.Message }
func IsValidationError(err error) bool  { var v ValidationError; return errors.As(err, &v) }

func validationf(format string, args ...any) error {
	return ValidationError{Message: fmt.Sprintf(format, args...)}
}

type Record struct {
	Type       string `json:"type"`
	FileNumber string `json:"file_number"`
	LicOrApp   string `json:"lic_or_app"`
	Status     string `json:"status"`
	Name       string `json:"name"`
	DBA        string `json:"dba"`
	Address    string `json:"address"`
	County     string `json:"county"`
	District   string `json:"district"`
	Expire     string `json:"expire"`
	OrigIssue  string `json:"orig_issue"`
}

type QueryOptions struct {
	Limit      int
	Offset     int
	Status     string
	County     string
	City       string
	Type       string
	LicOrApp   string
	ExpireYear string
	MatchMail  bool
}
type AreaFilter struct {
	ZIP      string
	City     string
	County   string
	District string
	Limit    int
	Offset   int
	MinDays  int
}

type Stats struct {
	TotalRecords           int            `json:"total_records"`
	ActiveLicenses         int            `json:"active_licenses"`
	PendingApplications    int            `json:"pending_applications"`
	Surrendered            int            `json:"surrendered"`
	ByStatus               map[string]int `json:"by_status"`
	ByLicenseType          map[string]int `json:"by_license_type"`
	ApplicationsVsLicenses map[string]int `json:"applications_vs_licenses"`
	SourceURL              string         `json:"source_url,omitempty"`
	ExportDate             string         `json:"export_date,omitempty"`
	LoadedAt               string         `json:"loaded_at,omitempty"`
	Stale                  bool           `json:"stale"`
}
type ImportResult struct {
	Records    int       `json:"records"`
	ExportDate time.Time `json:"export_date"`
	Database   string    `json:"database"`
}
type SnapshotInfo struct {
	Records    int       `json:"records"`
	ExportDate time.Time `json:"export_date"`
	Database   string    `json:"database"`
	CSV        string    `json:"csv"`
	Refreshed  bool      `json:"refreshed"`
}
type Store struct {
	db          *sql.DB
	path        string
	historyPath string
}

func CachePaths() (dir, csvPath, dbPath string, err error) {
	dir = strings.TrimSpace(os.Getenv("ABC_AGENT_CACHE"))
	if dir == "" {
		dir = strings.TrimSpace(os.Getenv("ABC_PP_CACHE"))
	}
	if dir == "" {
		dir, err = os.UserCacheDir()
		if err != nil {
			return "", "", "", err
		}
		dir = filepath.Join(dir, "abc-agent")
	}
	return dir, filepath.Join(dir, "ABC-DailyDataExport.csv"), filepath.Join(dir, "abc.sqlite"), nil
}
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{db: db, path: path, historyPath: historyPathFor(path)}, nil
}
func OpenDefault() (*Store, error) {
	_, _, dbPath, err := CachePaths()
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(dbPath); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("local ABC mirror not found at %s; run abc-agent refresh first", dbPath)
		}
		return nil, err
	}
	return Open(dbPath)
}
func (s *Store) Close() error { return s.db.Close() }

func ValidateOptions(o QueryOptions) error {
	if o.Limit < 0 {
		return validationf("limit must be nonnegative")
	}
	if o.Offset < 0 {
		return validationf("offset must be nonnegative")
	}
	if o.Status != "" && !validStatus(strings.ToUpper(o.Status)) {
		return validationf("invalid status %q", o.Status)
	}
	if o.Type != "" && !regexp.MustCompile(`^\d{2}$`).MatchString(o.Type) {
		return validationf("type must be two digits")
	}
	if o.LicOrApp != "" && strings.ToUpper(o.LicOrApp) != "LIC" && strings.ToUpper(o.LicOrApp) != "APP" {
		return validationf("lic_or_app must be LIC or APP")
	}
	if o.ExpireYear != "" && !regexp.MustCompile(`^\d{4}$`).MatchString(o.ExpireYear) {
		return validationf("expire_year must be four digits")
	}
	return nil
}
func ValidateArea(a AreaFilter) error {
	if a.Limit < 0 {
		return validationf("limit must be nonnegative")
	}
	if a.Offset < 0 {
		return validationf("offset must be nonnegative")
	}
	if a.MinDays < 0 {
		return validationf("days must be nonnegative")
	}
	n := 0
	for _, v := range []string{a.ZIP, a.City, a.County, a.District} {
		if strings.TrimSpace(v) != "" {
			n++
		}
	}
	if n > 1 {
		return validationf("conflicting area selectors")
	}
	return nil
}

var statusMeanings = map[string]string{
	"ACTIVE":  "Valid and exercisable (current license).",
	"PEND":    "Pending — application under review; not yet issued.",
	"DENY":    "Denied — application rejected.",
	"INACT":   "Inactive — license not currently exercised.",
	"ISSUPD":  "Indefinite Suspension.",
	"NREN":    "Non-Renewal — renewal refused.",
	"R64B":    "Issue And Hold — issued but held (not yet in use).",
	"R65":     "Surrendered, not in use.",
	"REV":     "Revocation — license revoked.",
	"REVP":    "Revocation Pending Due To Non-Payment of Recent Renewal (auto-revocation path).",
	"REVPEN":  "Revocation Pending — disciplinary revocation action initiated.",
	"RNST":    "In the process of being reinstated due to late payment of renewal.",
	"SUSPEN":  "Suspended — privileges temporarily withdrawn.",
	"SUSPEND": "Suspended (glossary spelling).",
	"SUREND":  "Surrendered — voluntarily given up.",
	"S/REV":   "Revoked for SLMS (Social Services hold).",
	"SLMS":    "Social Services Hold — noncompliance with child-support/benefits rules.",
	"VOID":    "Voided — never took effect / cancelled.",
	"WDRL":    "Withdrawn — application withdrawn by applicant.",
	"PDEV":    "Observed in daily export; pending development/review status.",
}

var derivedStatuses = map[string]string{
	"OVERDUE": "Still ACTIVE in the export but past its expiration date — renewal failure / auto-revocation candidate (ABC glossary term REVP).",
}

func validStatus(s string) bool { _, ok := statusMeanings[s]; return ok }

func ParseABCDate(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	parts := strings.Split(raw, "-")
	if len(parts) != 3 {
		return time.Time{}, fmt.Errorf("invalid ABC date %q", raw)
	}
	day, err := strconv.Atoi(parts[0])
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid ABC date %q: %w", raw, err)
	}
	year, err := strconv.Atoi(parts[2])
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid ABC date %q: %w", raw, err)
	}
	months := map[string]time.Month{"JAN": time.January, "FEB": time.February, "MAR": time.March, "APR": time.April, "MAY": time.May, "JUN": time.June, "JUL": time.July, "AUG": time.August, "SEP": time.September, "OCT": time.October, "NOV": time.November, "DEC": time.December}
	month, ok := months[strings.ToUpper(parts[1])]
	if !ok {
		return time.Time{}, fmt.Errorf("invalid ABC date %q", raw)
	}
	result := time.Date(year, month, day, 0, 0, 0, 0, laLocation)
	if result.Day() != day || result.Month() != month || result.Year() != year {
		return time.Time{}, fmt.Errorf("invalid ABC date %q", raw)
	}
	return result, nil
}

var bannerDate = regexp.MustCompile(`(?i)(\d{1,2})(?:st|nd|rd|th)?\s+of\s+([a-z]+)\s+(\d{4})`)

func parseBannerDate(banner string) time.Time {
	m := bannerDate.FindStringSubmatch(banner)
	if len(m) != 4 {
		return time.Time{}
	}
	parsed, err := time.ParseInLocation("2 January 2006", m[1]+" "+m[2]+" "+m[3], laLocation)
	if err != nil {
		return time.Time{}
	}
	return parsed
}

func historyPathFor(dbPath string) string {
	return filepath.Join(filepath.Dir(dbPath), "history.sqlite")
}

func withExclusiveLock(path string, fn func() error) error {
	lockPath := path + ".lock"
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return fmt.Errorf("ABC refresh is already running for %s; retry after it finishes", path)
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	return fn()
}

func ImportCSV(ctx context.Context, csvPath, dbPath string) (result ImportResult, retErr error) {
	var out ImportResult
	err := withExclusiveLock(dbPath, func() error {
		var err error
		out, err = importCSVLocked(ctx, csvPath, dbPath)
		return err
	})
	return out, err
}

func importCSVLocked(ctx context.Context, csvPath, dbPath string) (result ImportResult, retErr error) {
	input, err := os.Open(csvPath)
	if err != nil {
		return result, err
	}
	defer input.Close()
	buffered := bufio.NewReader(input)
	if prefix, _ := buffered.Peek(3); string(prefix) == "\xef\xbb\xbf" {
		_, _ = buffered.Discard(3)
	}
	reader := csv.NewReader(buffered)
	reader.FieldsPerRecord = -1
	banner, err := reader.Read()
	if err != nil {
		return result, fmt.Errorf("reading export banner: %w", err)
	}
	header, err := reader.Read()
	if err != nil {
		return result, fmt.Errorf("reading export header: %w", err)
	}
	expectedHeader := []string{"License Type", "File Number", "Lic or App", "Type Status", "Type Orig Iss Date", "Expir Date", "Fee Codes", "Dup Counts", "Master Ind", "Term in # of Months", "Geo Code", "District", "Primary Name", "Prem Addr 1", "Prem Addr 2", "Prem City", "Prem State", "Prem Zip", "DBA Name", "Mail Addr 1", "Mail Addr 2", "Mail City", "Mail State", "Mail Zip", "Prem County", "Prem Census Tract #"}
	if len(header) != len(expectedHeader) {
		return result, fmt.Errorf("official export header has %d fields, want %d", len(header), len(expectedHeader))
	}
	indexes := map[string]int{}
	for i, name := range header {
		trimmed := strings.TrimSpace(name)
		if trimmed != expectedHeader[i] {
			return result, fmt.Errorf("official export header field %d is %q, want %q", i+1, trimmed, expectedHeader[i])
		}
		indexes[trimmed] = i
	}
	exportDate := parseBannerDate(strings.Join(banner, ""))
	if exportDate.IsZero() {
		return result, errors.New("official export banner did not contain an export date")
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		return result, err
	}
	tmp, err := os.CreateTemp(filepath.Dir(dbPath), filepath.Base(dbPath)+".tmp-*")
	if err != nil {
		return result, err
	}
	tempPath := tmp.Name()
	_ = tmp.Close()
	defer func() {
		if retErr != nil {
			_ = os.Remove(tempPath)
		}
	}()
	db, err := sql.Open("sqlite", tempPath)
	if err != nil {
		return result, err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	schema := `PRAGMA journal_mode=DELETE; PRAGMA synchronous=FULL;
CREATE TABLE licenses (license_type TEXT NOT NULL,file_number TEXT NOT NULL,lic_or_app TEXT NOT NULL,status TEXT NOT NULL,orig_issue_date TEXT,expire_date TEXT,district TEXT,primary_name TEXT,prem_addr1 TEXT,prem_addr2 TEXT,prem_city TEXT,prem_state TEXT,prem_zip TEXT,dba_name TEXT,mail_addr1 TEXT,mail_addr2 TEXT,mail_city TEXT,mail_state TEXT,mail_zip TEXT,prem_county TEXT);
CREATE TABLE metadata (key TEXT PRIMARY KEY, value TEXT NOT NULL);`
	if _, err := db.ExecContext(ctx, schema); err != nil {
		return result, fmt.Errorf("creating mirror schema: %w", err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	insert, err := tx.PrepareContext(ctx, `INSERT INTO licenses VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return result, err
	}
	defer insert.Close()
	value := func(row []string, name string) string { return strings.TrimSpace(row[indexes[name]]) }
	count := 0
	for {
		row, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		line := count + 3
		if err != nil {
			return result, fmt.Errorf("reading export row %d: %w", line, err)
		}
		if len(row) != len(header) {
			return result, fmt.Errorf("official export row %d has %d fields, want %d", line, len(row), len(header))
		}
		licType, fileNumber, licOrApp, status := value(row, "License Type"), value(row, "File Number"), strings.ToUpper(value(row, "Lic or App")), strings.ToUpper(value(row, "Type Status"))
		if !regexp.MustCompile(`^\d{2}$`).MatchString(licType) {
			return result, fmt.Errorf("official export row %d invalid license type %q", line, licType)
		}
		if !regexp.MustCompile(`^\d{8}$`).MatchString(fileNumber) {
			return result, fmt.Errorf("official export row %d invalid file number %q", line, fileNumber)
		}
		if licOrApp != "LIC" && licOrApp != "APP" {
			return result, fmt.Errorf("official export row %d invalid Lic or App %q", line, licOrApp)
		}
		if status == "" {
			return result, fmt.Errorf("official export row %d missing status", line)
		}
		if d := value(row, "Type Orig Iss Date"); strings.TrimSpace(d) != "" {
			if _, err := ParseABCDate(d); err != nil {
				return result, fmt.Errorf("official export row %d invalid orig issue date: %w", line, err)
			}
		}
		if d := value(row, "Expir Date"); strings.TrimSpace(d) != "" {
			if _, err := ParseABCDate(d); err != nil {
				return result, fmt.Errorf("official export row %d invalid expire date: %w", line, err)
			}
		}
		_, err = insert.ExecContext(ctx, licType, fileNumber, licOrApp, status, value(row, "Type Orig Iss Date"), value(row, "Expir Date"), value(row, "District"), value(row, "Primary Name"), value(row, "Prem Addr 1"), value(row, "Prem Addr 2"), value(row, "Prem City"), value(row, "Prem State"), value(row, "Prem Zip"), value(row, "DBA Name"), value(row, "Mail Addr 1"), value(row, "Mail Addr 2"), value(row, "Mail City"), value(row, "Mail State"), value(row, "Mail Zip"), value(row, "Prem County"))
		if err != nil {
			return result, fmt.Errorf("inserting export row %d: %w", line, err)
		}
		count++
	}
	if count == 0 {
		return result, errors.New("official export contained no data rows")
	}
	loaded := time.Now().UTC().Format(time.RFC3339)
	if _, err := tx.ExecContext(ctx, `INSERT INTO metadata(key,value) VALUES ('export_banner', ?), ('export_date', ?), ('loaded_at', ?), ('row_count', ?), ('source_url', ?)`, strings.Join(banner, ""), exportDate.Format("2006-01-02"), loaded, strconv.Itoa(count), ExportURL); err != nil {
		return result, err
	}
	if err := tx.Commit(); err != nil {
		return result, err
	}
	committed = true
	if _, err := db.ExecContext(ctx, `CREATE INDEX idx_licenses_file ON licenses(file_number); CREATE INDEX idx_licenses_status ON licenses(status); CREATE INDEX idx_licenses_zip ON licenses(prem_zip); CREATE INDEX idx_licenses_city ON licenses(prem_city); CREATE INDEX idx_licenses_county ON licenses(prem_county);`); err != nil {
		return result, err
	}
	if err := snapshotDB(ctx, db, historyPathFor(dbPath), exportDate.Format("2006-01-02")); err != nil {
		return result, err
	}
	if err := db.Close(); err != nil {
		return result, err
	}
	if err := os.Chmod(tempPath, 0o600); err != nil {
		return result, err
	}
	if err := os.Rename(tempPath, dbPath); err != nil {
		return result, fmt.Errorf("installing rebuilt mirror: %w", err)
	}
	return ImportResult{Records: count, ExportDate: exportDate, Database: dbPath}, nil
}

func snapshotDB(ctx context.Context, db *sql.DB, historyPath, exportDate string) error {
	hist, err := sql.Open("sqlite", historyPath)
	if err != nil {
		return err
	}
	defer hist.Close()
	hist.SetMaxOpenConns(1)
	if _, err := hist.ExecContext(ctx, `PRAGMA journal_mode=DELETE; PRAGMA synchronous=FULL; CREATE TABLE IF NOT EXISTS history_snapshots (export_date TEXT NOT NULL, seq INTEGER NOT NULL, license_type TEXT NOT NULL, file_number TEXT NOT NULL, lic_or_app TEXT NOT NULL, row_hash TEXT NOT NULL, payload TEXT NOT NULL, PRIMARY KEY(export_date, seq)); CREATE INDEX IF NOT EXISTS idx_history_date ON history_snapshots(export_date); CREATE INDEX IF NOT EXISTS idx_history_key ON history_snapshots(export_date, license_type, file_number, lic_or_app);`); err != nil {
		return err
	}
	var n int
	if err := hist.QueryRowContext(ctx, `SELECT COUNT(*) FROM history_snapshots WHERE export_date=?`, exportDate).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	rows, err := db.QueryContext(ctx, `SELECT license_type,file_number,lic_or_app,status,orig_issue_date,expire_date,district,primary_name,prem_addr1,prem_addr2,prem_city,prem_state,prem_zip,dba_name,mail_addr1,mail_addr2,mail_city,mail_state,mail_zip,prem_county FROM licenses ORDER BY rowid`)
	if err != nil {
		return err
	}
	defer rows.Close()
	tx, err := hist.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO history_snapshots(export_date,seq,license_type,file_number,lic_or_app,row_hash,payload) VALUES (?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	seq := 0
	for rows.Next() {
		vals := make([]string, 20)
		dest := make([]any, 20)
		for i := range vals {
			dest[i] = &vals[i]
		}
		if err := rows.Scan(dest...); err != nil {
			return err
		}
		payloadMap := map[string]string{"license_type": vals[0], "file_number": vals[1], "lic_or_app": vals[2], "status": vals[3], "orig_issue_date": vals[4], "expire_date": vals[5], "district": vals[6], "primary_name": vals[7], "prem_addr1": vals[8], "prem_addr2": vals[9], "prem_city": vals[10], "prem_state": vals[11], "prem_zip": vals[12], "dba_name": vals[13], "mail_addr1": vals[14], "mail_addr2": vals[15], "mail_city": vals[16], "mail_state": vals[17], "mail_zip": vals[18], "prem_county": vals[19]}
		payload, err := json.Marshal(payloadMap)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(payload)
		if _, err := stmt.ExecContext(ctx, exportDate, seq, vals[0], vals[1], vals[2], hex.EncodeToString(sum[:]), string(payload)); err != nil {
			return err
		}
		seq++
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}

func EnsureData(ctx context.Context, force bool) (*Store, SnapshotInfo, error) {
	dir, csvPath, dbPath, err := CachePaths()
	if err != nil {
		return nil, SnapshotInfo{}, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, SnapshotInfo{}, err
	}
	var store *Store
	var imported ImportResult
	refreshed := false
	err = withExclusiveLock(dbPath, func() error {
		csvInfo, csvErr := os.Stat(csvPath)
		shouldDownload := force || csvErr != nil || time.Since(csvInfo.ModTime()) > cacheTTL
		if shouldDownload {
			if err := DownloadExport(ctx, csvPath); err != nil {
				return err
			}
			refreshed = true
		}
		csvDate, err := csvExportDate(csvPath)
		if err != nil {
			return err
		}
		var dbDate time.Time
		if _, err := os.Stat(dbPath); err == nil {
			if existing, err := Open(dbPath); err == nil {
				dbDate, _ = existing.ExportDate(ctx)
				_ = existing.Close()
			}
		}
		shouldImport := force || dbDate.IsZero() || csvDate.After(dbDate)
		if shouldImport {
			imported, err = importCSVLocked(ctx, csvPath, dbPath)
			if err != nil {
				return err
			}
			refreshed = true
		}
		return nil
	})
	if err != nil {
		return nil, SnapshotInfo{}, err
	}
	store, err = Open(dbPath)
	if err != nil {
		return nil, SnapshotInfo{}, err
	}
	stats, err := store.Stats(ctx)
	if err != nil {
		_ = store.Close()
		return nil, SnapshotInfo{}, err
	}
	exportDate := imported.ExportDate
	if exportDate.IsZero() {
		exportDate, _ = store.ExportDate(ctx)
	}
	return store, SnapshotInfo{Records: stats.TotalRecords, ExportDate: exportDate, Database: dbPath, CSV: csvPath, Refreshed: refreshed}, nil
}

func csvExportDate(path string) (time.Time, error) {
	f, err := os.Open(path)
	if err != nil {
		return time.Time{}, err
	}
	defer f.Close()
	b := bufio.NewReader(f)
	if prefix, _ := b.Peek(3); string(prefix) == "\xef\xbb\xbf" {
		_, _ = b.Discard(3)
	}
	r := csv.NewReader(b)
	row, err := r.Read()
	if err != nil {
		return time.Time{}, fmt.Errorf("reading export banner: %w", err)
	}
	d := parseBannerDate(strings.Join(row, ""))
	if d.IsZero() {
		return time.Time{}, errors.New("official export banner did not contain an export date")
	}
	return d, nil
}

func DownloadExport(ctx context.Context, destination string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ExportURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "abc-agent/0.1 (read-only public-data-client)")
	official, _ := url.Parse(ExportURL)
	client := &http.Client{Timeout: 3 * time.Minute, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if req.URL.Scheme != "https" {
			return errors.New("refusing non-HTTPS redirect")
		}
		if req.URL.Host != official.Host {
			return errors.New("refusing redirect away from abc.ca.gov")
		}
		if len(via) >= 3 {
			return errors.New("too many redirects")
		}
		return nil
	}}
	response, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("downloading official ABC export: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("downloading official ABC export: HTTP %s", response.Status)
	}
	zipFile, err := os.CreateTemp(filepath.Dir(destination), filepath.Base(destination)+".download-*.zip")
	if err != nil {
		return err
	}
	zipPath := zipFile.Name()
	defer os.Remove(zipPath)
	_, copyErr := io.Copy(zipFile, io.LimitReader(response.Body, maxZip+1))
	closeErr := zipFile.Close()
	if copyErr != nil || closeErr != nil {
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	}
	if info, err := os.Stat(zipPath); err != nil {
		return err
	} else if info.Size() > maxZip {
		return errors.New("official ABC ZIP exceeded size limit")
	}
	archive, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("opening official ABC ZIP: %w", err)
	}
	defer archive.Close()
	var source *zip.File
	for _, entry := range archive.File {
		name := filepath.Clean(entry.Name)
		if strings.HasSuffix(strings.ToLower(name), ".csv") && !strings.HasPrefix(name, "..") {
			if source != nil {
				return errors.New("official ABC ZIP contained multiple CSV files")
			}
			source = entry
		}
	}
	if source == nil {
		return errors.New("official ABC ZIP contained no CSV file")
	}
	if source.UncompressedSize64 > maxCSV {
		return errors.New("official ABC CSV exceeded size limit")
	}
	input, err := source.Open()
	if err != nil {
		return err
	}
	defer input.Close()
	temp, err := os.CreateTemp(filepath.Dir(destination), filepath.Base(destination)+".tmp-*.csv")
	if err != nil {
		return err
	}
	tempCSV := temp.Name()
	_, copyErr = io.Copy(temp, io.LimitReader(input, maxCSV+1))
	closeErr = temp.Close()
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(tempCSV)
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	}
	if info, err := os.Stat(tempCSV); err != nil {
		_ = os.Remove(tempCSV)
		return err
	} else if info.Size() > maxCSV {
		_ = os.Remove(tempCSV)
		return errors.New("official ABC CSV exceeded size limit")
	}
	return os.Rename(tempCSV, destination)
}

func (s *Store) ExportDate(ctx context.Context) (time.Time, error) {
	var v string
	if err := s.db.QueryRowContext(ctx, `SELECT value FROM metadata WHERE key='export_date'`).Scan(&v); err == nil {
		return time.ParseInLocation("2006-01-02", v, laLocation)
	}
	var banner string
	if err := s.db.QueryRowContext(ctx, `SELECT value FROM metadata WHERE key='export_banner'`).Scan(&banner); err != nil {
		return time.Time{}, err
	}
	return parseBannerDate(banner), nil
}
func (s *Store) metadata(ctx context.Context) map[string]string {
	rows, err := s.db.QueryContext(ctx, `SELECT key,value FROM metadata`)
	if err != nil {
		return map[string]string{}
	}
	defer rows.Close()
	m := map[string]string{}
	for rows.Next() {
		var k, v string
		if rows.Scan(&k, &v) == nil {
			m[k] = v
		}
	}
	return m
}

const recordColumns = `license_type,file_number,lic_or_app,status,primary_name,dba_name,prem_addr1,prem_addr2,prem_city,prem_state,prem_zip,prem_county,district,expire_date,orig_issue_date`

func scanRecord(scanner interface{ Scan(...any) error }) (Record, error) {
	var r Record
	var a1, a2, city, state, zipCode string
	err := scanner.Scan(&r.Type, &r.FileNumber, &r.LicOrApp, &r.Status, &r.Name, &r.DBA, &a1, &a2, &city, &state, &zipCode, &r.County, &r.District, &r.Expire, &r.OrigIssue)
	if err != nil {
		return r, err
	}
	parts := []string{}
	for _, p := range []string{a1, a2, city, state, zipCode} {
		if p = strings.TrimSpace(p); p != "" {
			parts = append(parts, p)
		}
	}
	r.Address = strings.Join(parts, ", ")
	return r, nil
}
func limitOffset(limit, offset int) (int, int) {
	if limit < 0 {
		limit = 0
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}
func (s *Store) queryRecords(ctx context.Context, where string, args []any, limit, offset int) ([]Record, error) {
	limit, offset = limitOffset(limit, offset)
	query := `SELECT ` + recordColumns + ` FROM licenses ` + where + ` ORDER BY file_number, license_type, lic_or_app, rowid`
	if limit > 0 {
		query += ` LIMIT ? OFFSET ?`
		args = append(args, limit, offset)
	} else if offset > 0 {
		query += ` LIMIT -1 OFFSET ?`
		args = append(args, offset)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var records []Record
	for rows.Next() {
		r, err := scanRecord(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, r)
	}
	if records == nil {
		records = []Record{}
	}
	return records, rows.Err()
}
func likeLiteral(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return "%" + s + "%"
}
func prefixLiteral(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s + "%"
}
func addOptions(where string, args []any, o QueryOptions) (string, []any, error) {
	if err := ValidateOptions(o); err != nil {
		return where, args, err
	}
	if o.Status != "" {
		where += ` AND status = ?`
		args = append(args, strings.ToUpper(o.Status))
	}
	if o.County != "" {
		where += ` AND UPPER(prem_county) LIKE UPPER(?) ESCAPE '\'`
		args = append(args, likeLiteral(o.County))
	}
	if o.City != "" {
		where += ` AND UPPER(prem_city) LIKE UPPER(?) ESCAPE '\'`
		args = append(args, likeLiteral(o.City))
	}
	if o.Type != "" {
		where += ` AND license_type = ?`
		args = append(args, o.Type)
	}
	if o.LicOrApp != "" {
		where += ` AND lic_or_app = ?`
		args = append(args, strings.ToUpper(o.LicOrApp))
	}
	if o.ExpireYear != "" {
		where += ` AND expire_date LIKE ?`
		args = append(args, "%-"+o.ExpireYear)
	}
	return where, args, nil
}
func areaCondition(a AreaFilter) (string, any) {
	switch {
	case a.ZIP != "":
		return `UPPER(prem_zip) LIKE UPPER(?) ESCAPE '\'`, prefixLiteral(a.ZIP)
	case a.City != "":
		return `UPPER(prem_city) LIKE UPPER(?) ESCAPE '\'`, prefixLiteral(a.City)
	case a.County != "":
		return `UPPER(prem_county) LIKE UPPER(?) ESCAPE '\'`, prefixLiteral(a.County)
	case a.District != "":
		return `UPPER(district) LIKE UPPER(?) ESCAPE '\'`, prefixLiteral(a.District)
	default:
		return "", nil
	}
}

func (s *Store) Address(ctx context.Context, fragment string, o QueryOptions) ([]Record, error) {
	if err := ValidateOptions(o); err != nil {
		return nil, err
	}
	frag := likeLiteral(strings.TrimSpace(fragment))
	cols := []string{"prem_addr1", "prem_addr2", "prem_city", "prem_zip", "prem_state"}
	if o.MatchMail {
		cols = append(cols, "mail_addr1", "mail_addr2", "mail_city", "mail_zip", "mail_state")
	}
	cond := []string{}
	args := []any{}
	for _, c := range cols {
		cond = append(cond, "UPPER("+c+") LIKE UPPER(?) ESCAPE '\\'")
		args = append(args, frag)
	}
	where, args, err := addOptions("WHERE ("+strings.Join(cond, " OR ")+")", args, o)
	if err != nil {
		return nil, err
	}
	return s.queryRecords(ctx, where, args, o.Limit, o.Offset)
}
func (s *Store) Get(ctx context.Context, fileNumber string) ([]Record, error) {
	if !regexp.MustCompile(`^\d{8}$`).MatchString(fileNumber) {
		return nil, validationf("file number must be exactly eight digits")
	}
	return s.queryRecords(ctx, `WHERE file_number = ?`, []any{fileNumber}, 0, 0)
}
func (s *Store) Pending(ctx context.Context, a AreaFilter) ([]Record, error) {
	if err := ValidateArea(a); err != nil {
		return nil, err
	}
	where := `WHERE status = 'PEND'`
	args := []any{}
	if c, v := areaCondition(a); c != "" {
		where += " AND " + c
		args = append(args, v)
	}
	return s.queryRecords(ctx, where, args, a.Limit, a.Offset)
}
func (s *Store) Search(ctx context.Context, q string, o QueryOptions) ([]Record, error) {
	needle := likeLiteral(strings.TrimSpace(q))
	where := `WHERE (UPPER(primary_name) LIKE UPPER(?) ESCAPE '\' OR UPPER(dba_name) LIKE UPPER(?) ESCAPE '\' OR UPPER(file_number) LIKE UPPER(?) ESCAPE '\')`
	args := []any{needle, needle, needle}
	var err error
	where, args, err = addOptions(where, args, o)
	if err != nil {
		return nil, err
	}
	return s.queryRecords(ctx, where, args, o.Limit, o.Offset)
}
func (s *Store) ByArea(ctx context.Context, a AreaFilter, o QueryOptions) ([]Record, error) {
	if err := ValidateArea(a); err != nil {
		return nil, err
	}
	if err := ValidateOptions(o); err != nil {
		return nil, err
	}
	where := `WHERE 1=1`
	args := []any{}
	if c, v := areaCondition(a); c != "" {
		where += " AND " + c
		args = append(args, v)
	}
	var err error
	where, args, err = addOptions(where, args, o)
	if err != nil {
		return nil, err
	}
	limit, offset := o.Limit, o.Offset
	if limit == 0 {
		limit = a.Limit
	}
	if offset == 0 {
		offset = a.Offset
	}
	return s.queryRecords(ctx, where, args, limit, offset)
}
func (s *Store) Overdue(ctx context.Context, a AreaFilter, now time.Time) ([]Record, error) {
	if err := ValidateArea(a); err != nil {
		return nil, err
	}
	return s.filterDates(ctx, a, 0, now, func(exp, today time.Time) bool {
		if a.MinDays == 0 {
			return exp.Before(today)
		}
		return !exp.After(today.AddDate(0, 0, -a.MinDays))
	})
}
func (s *Store) Expiring(ctx context.Context, a AreaFilter, days int, now time.Time) ([]Record, error) {
	if days < 0 {
		return nil, validationf("days must be nonnegative")
	}
	if err := ValidateArea(a); err != nil {
		return nil, err
	}
	return s.filterDates(ctx, a, days, now, func(exp, today time.Time) bool { return !exp.Before(today) && !exp.After(today.AddDate(0, 0, days)) })
}
func (s *Store) filterDates(ctx context.Context, a AreaFilter, _ int, now time.Time, keep func(time.Time, time.Time) bool) ([]Record, error) {
	where := `WHERE status = 'ACTIVE' AND lic_or_app = 'LIC' AND TRIM(expire_date) <> ''`
	args := []any{}
	if c, v := areaCondition(a); c != "" {
		where += " AND " + c
		args = append(args, v)
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+recordColumns+` FROM licenses `+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	today := time.Date(now.In(laLocation).Year(), now.In(laLocation).Month(), now.In(laLocation).Day(), 0, 0, 0, 0, laLocation)
	records := []Record{}
	for rows.Next() {
		r, err := scanRecord(rows)
		if err != nil {
			return nil, err
		}
		exp, err := ParseABCDate(r.Expire)
		if err == nil {
			exp = time.Date(exp.In(laLocation).Year(), exp.In(laLocation).Month(), exp.In(laLocation).Day(), 0, 0, 0, 0, laLocation)
			if keep(exp, today) {
				records = append(records, r)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Slice(records, func(i, j int) bool {
		li, _ := ParseABCDate(records[i].Expire)
		lj, _ := ParseABCDate(records[j].Expire)
		if !li.Equal(lj) {
			return li.Before(lj)
		}
		if records[i].FileNumber != records[j].FileNumber {
			return records[i].FileNumber < records[j].FileNumber
		}
		if records[i].Type != records[j].Type {
			return records[i].Type < records[j].Type
		}
		if records[i].LicOrApp != records[j].LicOrApp {
			return records[i].LicOrApp < records[j].LicOrApp
		}
		bi, _ := json.Marshal(records[i])
		bj, _ := json.Marshal(records[j])
		return string(bi) < string(bj)
	})
	limit, offset := limitOffset(a.Limit, a.Offset)
	if offset > len(records) {
		return []Record{}, nil
	}
	records = records[offset:]
	if limit > 0 && len(records) > limit {
		records = records[:limit]
	}
	return records, nil
}

func (s *Store) Stats(ctx context.Context) (Stats, error) {
	r := Stats{ByStatus: map[string]int{}, ByLicenseType: map[string]int{}, ApplicationsVsLicenses: map[string]int{}}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM licenses`).Scan(&r.TotalRecords); err != nil {
		return r, err
	}
	fill := func(q string, m map[string]int) error {
		rows, err := s.db.QueryContext(ctx, q)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var k string
			var c int
			if err := rows.Scan(&k, &c); err != nil {
				return err
			}
			m[k] = c
		}
		return rows.Err()
	}
	if err := fill(`SELECT status, COUNT(*) FROM licenses GROUP BY status ORDER BY COUNT(*) DESC`, r.ByStatus); err != nil {
		return r, err
	}
	if err := fill(`SELECT license_type, COUNT(*) FROM licenses GROUP BY license_type ORDER BY COUNT(*) DESC`, r.ByLicenseType); err != nil {
		return r, err
	}
	if err := fill(`SELECT lic_or_app, COUNT(*) FROM licenses GROUP BY lic_or_app`, r.ApplicationsVsLicenses); err != nil {
		return r, err
	}
	r.ActiveLicenses = r.ByStatus["ACTIVE"]
	r.PendingApplications = r.ByStatus["PEND"]
	r.Surrendered = r.ByStatus["SUREND"]
	md := s.metadata(ctx)
	r.SourceURL = md["source_url"]
	r.ExportDate = md["export_date"]
	r.LoadedAt = md["loaded_at"]
	r.Stale = snapshotStale(md, time.Now())
	return r, nil
}
func (s *Store) Statuses(ctx context.Context, now time.Time) (any, error) {
	stats, err := s.Stats(ctx)
	if err != nil {
		return nil, err
	}
	md := s.metadata(ctx)
	stale := snapshotStale(md, now)
	overdue, err := s.Overdue(ctx, AreaFilter{}, now)
	if err != nil {
		return nil, err
	}
	return map[string]any{"source_url": ExportURL, "export_date": md["export_date"], "loaded_at": md["loaded_at"], "stale": stale, "by_status": stats.ByStatus, "observed_in_export": stats.ByStatus, "active_licenses": stats.ActiveLicenses, "pending_applications": stats.PendingApplications, "total_records": stats.TotalRecords, "overdue_active_licenses": len(overdue), "vocabulary": statusMeanings, "derived": derivedStatuses}, nil
}
func (s *Store) HistorySnapshot(ctx context.Context) (any, error) {
	md := s.metadata(ctx)
	exportDate := md["export_date"]
	if exportDate == "" {
		d, err := s.ExportDate(ctx)
		if err != nil {
			return nil, err
		}
		exportDate = d.Format("2006-01-02")
	}
	if err := snapshotDB(ctx, s.db, s.historyPath, exportDate); err != nil {
		return nil, err
	}
	hist, err := sql.Open("sqlite", s.historyPath)
	if err != nil {
		return nil, err
	}
	defer hist.Close()
	var records int
	if err := hist.QueryRowContext(ctx, `SELECT COUNT(*) FROM history_snapshots WHERE export_date=?`, exportDate).Scan(&records); err != nil {
		return nil, err
	}
	return map[string]any{"export_date": exportDate, "records": records, "source_url": ExportURL, "history_database": s.historyPath}, nil
}

type historyRow struct{ key, typ, file, licapp, hash, payload string }

func (s *Store) HistoryDiff(ctx context.Context) (any, error) {
	hist, err := sql.Open("sqlite", s.historyPath)
	if err != nil {
		return nil, err
	}
	defer hist.Close()
	if _, err := hist.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS history_snapshots (export_date TEXT NOT NULL, seq INTEGER NOT NULL, license_type TEXT NOT NULL, file_number TEXT NOT NULL, lic_or_app TEXT NOT NULL, row_hash TEXT NOT NULL, payload TEXT NOT NULL, PRIMARY KEY(export_date, seq))`); err != nil {
		return nil, err
	}
	rows, err := hist.QueryContext(ctx, `SELECT DISTINCT export_date FROM history_snapshots ORDER BY export_date DESC LIMIT 2`)
	if err != nil {
		return nil, err
	}
	dates := []string{}
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			rows.Close()
			return nil, err
		}
		dates = append(dates, d)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if len(dates) < 2 {
		return map[string]any{"from_export_date": "", "to_export_date": first(dates), "added": 0, "removed": 0, "changed": 0, "added_records": []map[string]any{}, "removed_records": []map[string]any{}, "changed_records": []map[string]any{}}, nil
	}
	to, from := dates[0], dates[1]
	load := func(day string) (map[string][]historyRow, error) {
		q, err := hist.QueryContext(ctx, `SELECT license_type,file_number,lic_or_app,row_hash,payload FROM history_snapshots WHERE export_date=? ORDER BY license_type,file_number,lic_or_app,payload,seq`, day)
		if err != nil {
			return nil, err
		}
		defer q.Close()
		m := map[string][]historyRow{}
		for q.Next() {
			var r historyRow
			if err := q.Scan(&r.typ, &r.file, &r.licapp, &r.hash, &r.payload); err != nil {
				return nil, err
			}
			r.key = r.typ + "\x00" + r.file + "\x00" + r.licapp
			m[r.key] = append(m[r.key], r)
		}
		return m, q.Err()
	}
	oldm, err := load(from)
	if err != nil {
		return nil, err
	}
	newm, err := load(to)
	if err != nil {
		return nil, err
	}
	keys := map[string]bool{}
	for k := range oldm {
		keys[k] = true
	}
	for k := range newm {
		keys[k] = true
	}
	ordered := make([]string, 0, len(keys))
	for k := range keys {
		ordered = append(ordered, k)
	}
	sort.Strings(ordered)
	addedRows, removedRows, changedRows := []map[string]any{}, []map[string]any{}, []map[string]any{}
	added, removed, changed := 0, 0, 0
	for _, k := range ordered {
		old := append([]historyRow(nil), oldm[k]...)
		neu := append([]historyRow(nil), newm[k]...)
		usedOld := make([]bool, len(old))
		usedNew := make([]bool, len(neu))
		for i := range neu {
			for j := range old {
				if !usedOld[j] && !usedNew[i] && neu[i].payload == old[j].payload {
					usedOld[j], usedNew[i] = true, true
					break
				}
			}
		}
		oldLeft, newLeft := []historyRow{}, []historyRow{}
		for i, r := range old {
			if !usedOld[i] {
				oldLeft = append(oldLeft, r)
			}
		}
		for i, r := range neu {
			if !usedNew[i] {
				newLeft = append(newLeft, r)
			}
		}
		pairs := len(oldLeft)
		if len(newLeft) < pairs {
			pairs = len(newLeft)
		}
		for i := 0; i < pairs; i++ {
			changed++
			changedRows = append(changedRows, bucketRow(newLeft[i], oldLeft[i]))
		}
		for _, r := range newLeft[pairs:] {
			added++
			addedRows = append(addedRows, bucketRow(r, historyRow{}))
		}
		for _, r := range oldLeft[pairs:] {
			removed++
			removedRows = append(removedRows, bucketRow(r, historyRow{}))
		}
	}
	return map[string]any{"from_export_date": from, "to_export_date": to, "added": added, "removed": removed, "changed": changed, "added_records": addedRows, "removed_records": removedRows, "changed_records": changedRows}, nil
}

func bucketRow(r, prev historyRow) map[string]any {
	m := map[string]any{"license_type": r.typ, "file_number": r.file, "lic_or_app": r.licapp, "row_hash": r.hash, "payload": r.payload}
	if prev.payload != "" {
		m["previous_row_hash"] = prev.hash
		m["previous_payload"] = prev.payload
	}
	return m
}
func first(s []string) string {
	if len(s) == 0 {
		return ""
	}
	return s[0]
}
