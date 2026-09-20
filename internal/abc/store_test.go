package abc

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const testCSV = "\ufeff" + `"Updated Tuesday 25th of August 2026 03:50:22 AM"
"License Type","File Number","Lic or App","Type Status","Type Orig Iss Date","Expir Date","Fee Codes","Dup Counts","Master Ind","Term in # of Months","Geo Code",District,"Primary Name","Prem Addr 1"," Prem Addr 2","Prem City"," Prem State","Prem Zip","DBA Name","Mail Addr 1","Mail Addr 2","Mail City","Mail State","Mail Zip","Prem County","Prem Census Tract #"
02,00677768,LIC,ACTIVE,01-JAN-2020,01-JAN-2020,,,,12,1900,04,"EXAMPLE OWNER","6417 SELMA_% AVE"," ",HOLLYWOOD,CA,90028,"EXAMPLE DBA",,,,,,LOS ANGELES,
47,00677768,LIC,ACTIVE,01-JAN-2020,31-DEC-2099,,,,12,1900,04,"EXAMPLE OWNER","6417 SELMA AVE"," ",HOLLYWOOD,CA,90028,"EXAMPLE DBA",,,,,,LOS ANGELES,
21,00700000,APP,PEND," "," ",,,,12,1900,04,"PENDING OWNER","100 TEST ST"," ",HOLLYWOOD,CA,90028,"PENDING DBA",,,,,,LOS ANGELES,
41,00800000,LIC,ACTIVE,01-JAN-2020,25-AUG-2026,,,,12,1900,05,"TODAY OWNER","200 TODAY ST"," ",BURBANK,CA,91502,"TODAY DBA",,,,,,LOS ANGELES,
`

func testStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "export.csv")
	dbPath := filepath.Join(dir, "abc.sqlite")
	if err := os.WriteFile(csvPath, []byte(testCSV), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := ImportCSV(context.Background(), csvPath, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if result.Records != 4 {
		t.Fatalf("records = %d, want 4", result.Records)
	}
	store, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestStoreCoreQueries(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	stats, err := store.Stats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if stats.TotalRecords != 4 || stats.ActiveLicenses != 3 || stats.PendingApplications != 1 {
		t.Fatalf("unexpected stats: %+v", stats)
	}

	address, err := store.Address(ctx, "6417 SELMA AVE", QueryOptions{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if len(address) != 1 || address[0].Type != "47" {
		t.Fatalf("literal address rows = %+v, want only type 47", address)
	}

	records, err := store.Get(ctx, "00677768")
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 || records[0].FileNumber != "00677768" {
		t.Fatalf("unexpected file rows: %+v", records)
	}

	pending, err := store.Pending(ctx, AreaFilter{ZIP: "90028", Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 || pending[0].Status != "PEND" {
		t.Fatalf("unexpected pending rows: %+v", pending)
	}

	overdue, err := store.Overdue(ctx, AreaFilter{County: "LOS ANGELES", Limit: 20}, time.Date(2026, 8, 25, 23, 30, 0, 0, time.FixedZone("UTC+14", 14*3600)))
	if err != nil {
		t.Fatal(err)
	}
	if len(overdue) != 1 || overdue[0].Expire != "01-JAN-2020" {
		t.Fatalf("unexpected overdue rows: %+v", overdue)
	}
}

func TestPublicBoundaryValidation(t *testing.T) {
	cases := []error{
		ValidateOptions(QueryOptions{Status: "BOGUS"}), ValidateOptions(QueryOptions{Type: "2"}),
		ValidateOptions(QueryOptions{LicOrApp: "BOTH"}), ValidateOptions(QueryOptions{ExpireYear: "26"}),
		ValidateOptions(QueryOptions{Limit: -1}), ValidateOptions(QueryOptions{Offset: -1}),
		ValidateArea(AreaFilter{ZIP: "9", City: "LA"}), ValidateArea(AreaFilter{Limit: -1}), ValidateArea(AreaFilter{MinDays: -1}),
	}
	for _, err := range cases {
		if !IsValidationError(err) {
			t.Fatalf("got %T %[1]v, want ValidationError", err)
		}
	}
	if err := ValidateOptions(QueryOptions{Status: "ACTIVE", Type: "02", LicOrApp: "LIC", ExpireYear: "2026", Limit: 0, Offset: 0}); err != nil {
		t.Fatal(err)
	}
}

func TestLimitZeroOffsetAndByArea(t *testing.T) {
	store := testStore(t)
	rows, err := store.ByArea(context.Background(), AreaFilter{County: "LOS ANGELES", Limit: 0, Offset: 1}, QueryOptions{Status: "ACTIVE"})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].FileNumber != "00677768" || rows[0].Type != "47" {
		t.Fatalf("unexpected rows: %+v", rows)
	}
}

func TestExpiringIncludesTodayThroughDays(t *testing.T) {
	store := testStore(t)
	rows, err := store.Expiring(context.Background(), AreaFilter{County: "LOS ANGELES"}, 0, time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].FileNumber != "00800000" {
		t.Fatalf("unexpected expiring rows: %+v", rows)
	}
}

func TestStatusesAndStatsExposeProvenance(t *testing.T) {
	store := testStore(t)
	status, err := store.Statuses(context.Background(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	m := status.(map[string]any)
	if m["source_url"] != ExportURL || m["export_date"] == "" || m["loaded_at"] == "" {
		t.Fatalf("missing provenance: %#v", m)
	}
	if !m["stale"].(bool) {
		t.Fatalf("old export must remain stale even when freshly loaded")
	}
}

func TestImportPreservesLastGoodOnCorruptOrEmpty(t *testing.T) {
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "export.csv")
	dbPath := filepath.Join(dir, "abc.sqlite")
	if err := os.WriteFile(csvPath, []byte(testCSV), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ImportCSV(context.Background(), csvPath, dbPath); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(csvPath, []byte("bad\nheader\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ImportCSV(context.Background(), csvPath, dbPath); err == nil {
		t.Fatal("corrupt import succeeded")
	}
	store, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	stats, err := store.Stats(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if stats.TotalRecords != 4 {
		t.Fatalf("last good lost: %+v", stats)
	}
}

func TestHistorySnapshotAndDiffSurviveRefresh(t *testing.T) {
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "export.csv")
	dbPath := filepath.Join(dir, "abc.sqlite")
	if err := os.WriteFile(csvPath, []byte(testCSV), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ImportCSV(context.Background(), csvPath, dbPath); err != nil {
		t.Fatal(err)
	}
	store, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := store.HistorySnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snap.(map[string]any)["records"].(int) != 4 {
		t.Fatalf("bad snapshot: %#v", snap)
	}
	_ = store.Close()

	changed := strings.Replace(testCSV, "31-DEC-2099", "31-DEC-2030", 1)
	changed = strings.Replace(changed, "Updated Tuesday 25th of August 2026", "Updated Wednesday 26th of August 2026", 1)
	changed = strings.Replace(changed, "21,00700000", "99,00999999", 1)
	if err := os.WriteFile(csvPath, []byte(changed), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ImportCSV(context.Background(), csvPath, dbPath); err != nil {
		t.Fatal(err)
	}
	store, err = Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	diff, err := store.HistoryDiff(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	d := diff.(map[string]any)
	if d["added"].(int) != 1 || d["removed"].(int) != 1 || d["changed"].(int) != 1 {
		t.Fatalf("bad diff: %#v", d)
	}
}

func TestParseABCDate(t *testing.T) {
	got, err := ParseABCDate("31-DEC-2026")
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 12, 31, 0, 0, 0, 0, laLocation)
	if !got.Equal(want) {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestOpenDefaultRequiresExplicitRefreshAndCacheEnv(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ABC_AGENT_CACHE", dir)
	t.Setenv("ABC_PP_CACHE", filepath.Join(t.TempDir(), "legacy"))
	_, _, dbPath, err := CachePaths()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(dbPath, dir) {
		t.Fatalf("ABC_AGENT_CACHE not preferred: %s", dbPath)
	}
	_, err = OpenDefault()
	var verr ValidationError
	if err == nil || errors.As(err, &verr) {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), "run abc-agent refresh first") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOfficialStatusVocabulary(t *testing.T) {
	for _, status := range []string{"ACTIVE", "PEND", "SUREND", "REVPEN", "SUSPEN", "R64B", "REV", "REVP", "PDEV"} {
		if err := ValidateOptions(QueryOptions{Status: status}); err != nil {
			t.Fatalf("status %s rejected: %v", status, err)
		}
	}
	for _, status := range []string{"CANCELED", "REVOKED", "INACTIVE", "SUSP", "EXPIRED", "TRANS", "PROTEST"} {
		if err := ValidateOptions(QueryOptions{Status: status}); err == nil {
			t.Fatalf("fabricated status %s accepted", status)
		}
	}
}

func TestStatusesExposeVocabularyAndDerivedOverdue(t *testing.T) {
	store := testStore(t)
	got, err := store.Statuses(context.Background(), time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	m := got.(map[string]any)
	vocab := m["vocabulary"].(map[string]string)
	if vocab["REVPEN"] == "" || vocab["SUSPEN"] == "" || vocab["R64B"] == "" || vocab["REVP"] == "" {
		t.Fatalf("missing real vocabulary: %#v", vocab)
	}
	if _, ok := vocab["CANCELED"]; ok {
		t.Fatalf("fabricated vocabulary present: %#v", vocab)
	}
	if m["overdue_active_licenses"].(int) != 1 {
		t.Fatalf("bad overdue count: %#v", m)
	}
	if m["derived"].(map[string]string)["OVERDUE"] == "" {
		t.Fatalf("missing derived statuses: %#v", m)
	}
}

func TestImportRejectsShortRowsAndInvalidRequiredFieldsWithoutCreatingDB(t *testing.T) {
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "bad.csv")
	dbPath := filepath.Join(dir, "abc.sqlite")
	bad := strings.Replace(testCSV, "02,00677768,LIC,ACTIVE,01-JAN-2020,01-JAN-2020,,,,12,1900,04,\"EXAMPLE OWNER\",\"6417 SELMA_% AVE\",\" \",HOLLYWOOD,CA,90028,\"EXAMPLE DBA\",,,,,,LOS ANGELES,", "02,00677768,LIC,ACTIVE", 1)
	if err := os.WriteFile(csvPath, []byte(bad), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ImportCSV(context.Background(), csvPath, dbPath); err == nil {
		t.Fatal("short row import succeeded")
	}
	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		t.Fatalf("failed import created db: %v", err)
	}

	bad = strings.Replace(testCSV, "02,00677768,LIC", "02,,LIC", 1)
	if err := os.WriteFile(csvPath, []byte(bad), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ImportCSV(context.Background(), csvPath, dbPath); err == nil {
		t.Fatal("missing file number import succeeded")
	}
}

func TestHistorySeparatePreservesDuplicatesAndPayloadDiff(t *testing.T) {
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "export.csv")
	dbPath := filepath.Join(dir, "abc.sqlite")
	histPath := filepath.Join(dir, "history.sqlite")
	dup := testCSV + "47,00677768,LIC,ACTIVE,01-JAN-2020,31-DEC-2099,,,,12,1900,04,\"EXAMPLE OWNER\",\"6417 SELMA AVE\",\" \",HOLLYWOOD,CA,90028,\"EXAMPLE DBA\",,,,,,LOS ANGELES,\n"
	if err := os.WriteFile(csvPath, []byte(dup), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ImportCSV(context.Background(), csvPath, dbPath); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(histPath); err != nil {
		t.Fatalf("separate history missing: %v", err)
	}
	store, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := store.HistorySnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snap.(map[string]any)["records"].(int) != 5 {
		t.Fatalf("duplicates lost in snapshot: %#v", snap)
	}
	_ = store.Close()

	changed := strings.Replace(dup, "Updated Tuesday 25th of August 2026", "Updated Wednesday 26th of August 2026", 1)
	changed = strings.Replace(changed, "\"EXAMPLE DBA\",,,,,,LOS ANGELES,\n21,00700000", "\"EXAMPLE DBA CHANGED\",,,,,,LOS ANGELES,\n21,00700000", 1)
	if err := os.WriteFile(csvPath, []byte(changed), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ImportCSV(context.Background(), csvPath, dbPath); err != nil {
		t.Fatal(err)
	}
	store, err = Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	diff, err := store.HistoryDiff(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	d := diff.(map[string]any)
	if d["changed"].(int) != 1 || d["added"].(int) != 0 || d["removed"].(int) != 0 {
		t.Fatalf("bad duplicate diff: %#v", d)
	}
	changedRows := d["changed_records"].([]map[string]any)
	if len(changedRows) != 1 || changedRows[0]["file_number"] != "00677768" {
		t.Fatalf("bad changed bucket: %#v", changedRows)
	}
}

func TestImportCSVFailedMirrorInstallDoesNotAppendHistorySnapshot(t *testing.T) {
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "export.csv")
	dbPath := filepath.Join(dir, "abc.sqlite")
	if err := os.WriteFile(csvPath, []byte(testCSV), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(dbPath, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dbPath, "block-rename"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := ImportCSV(context.Background(), csvPath, dbPath)
	if err == nil || !strings.Contains(err.Error(), "installing rebuilt mirror") {
		t.Fatalf("ImportCSV error = %v, want mirror install failure", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "history.sqlite")); !os.IsNotExist(err) {
		t.Fatalf("failed mirror install appended history snapshot: %v", err)
	}
}

func TestImportCSVRejectsMalformedHistoryBeforeMirrorInstall(t *testing.T) {
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "export.csv")
	dbPath := filepath.Join(dir, "abc.sqlite")
	histPath := filepath.Join(dir, "history.sqlite")
	if err := os.WriteFile(csvPath, []byte(testCSV), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ImportCSV(context.Background(), csvPath, dbPath); err != nil {
		t.Fatal(err)
	}
	badHistory := []byte("not sqlite history")
	if err := os.WriteFile(histPath, badHistory, 0o600); err != nil {
		t.Fatal(err)
	}
	newer := strings.Replace(testCSV, "Updated Tuesday 25th of August 2026", "Updated Wednesday 26th of August 2026", 1)
	if err := os.WriteFile(csvPath, []byte(newer), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := ImportCSV(context.Background(), csvPath, dbPath)
	if err == nil || !strings.Contains(err.Error(), "validating history database before mirror install") {
		t.Fatalf("ImportCSV error = %v, want malformed history validation failure", err)
	}
	gotHistory, err := os.ReadFile(histPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotHistory) != string(badHistory) {
		t.Fatalf("malformed history bytes changed: %q", gotHistory)
	}
	store, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	exportDate, err := store.ExportDate(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if exportDate.Format("2006-01-02") != "2026-08-25" {
		t.Fatalf("malformed history failure installed new mirror date %s", exportDate.Format("2006-01-02"))
	}
	if _, err := store.HistoryDiff(context.Background()); err == nil {
		t.Fatal("HistoryDiff succeeded against malformed history")
	}
}

func TestHistorySnapshotFailureAfterInstallIsExplicitPartialSuccess(t *testing.T) {
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "export.csv")
	dbPath := filepath.Join(dir, "abc.sqlite")
	if err := os.WriteFile(csvPath, []byte(testCSV), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ImportCSV(context.Background(), csvPath, dbPath); err != nil {
		t.Fatal(err)
	}
	histPath := filepath.Join(dir, "history.sqlite")
	hist, err := sql.Open("sqlite", histPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := hist.Exec(`DROP TABLE history_snapshots; CREATE TABLE history_snapshots (export_date TEXT NOT NULL, seq INTEGER NOT NULL, PRIMARY KEY(export_date, seq)); INSERT INTO history_snapshots(export_date, seq) VALUES ('2026-08-25', 0);`); err != nil {
		t.Fatal(err)
	}
	if err := hist.Close(); err != nil {
		t.Fatal(err)
	}
	newer := strings.Replace(testCSV, "Updated Tuesday 25th of August 2026", "Updated Wednesday 26th of August 2026", 1)
	if err := os.WriteFile(csvPath, []byte(newer), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := ImportCSV(context.Background(), csvPath, dbPath)
	if err == nil || !strings.Contains(err.Error(), "mirror installed, history snapshot failed; run history-snapshot") {
		t.Fatalf("ImportCSV error = %v, want explicit partial success", err)
	}
	if result.Database != dbPath || result.ExportDate.Format("2006-01-02") != "2026-08-26" || result.Records == 0 {
		t.Fatalf("partial success result does not describe installed mirror: %+v", result)
	}
	store, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	exportDate, err := store.ExportDate(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if exportDate.Format("2006-01-02") != "2026-08-26" {
		t.Fatalf("partial success did not install mirror date %s", exportDate.Format("2006-01-02"))
	}
}

func TestEnsureDataUsesExportDateForStaleSource(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ABC_AGENT_CACHE", dir)
	csvPath := filepath.Join(dir, "ABC-DailyDataExport.csv")
	dbPath := filepath.Join(dir, "abc.sqlite")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(csvPath, []byte(testCSV), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ImportCSV(context.Background(), csvPath, dbPath); err != nil {
		t.Fatal(err)
	}
	stale := strings.Replace(testCSV, "Updated Tuesday 25th of August 2026", "Updated Monday 24th of August 2026", 1)
	if err := os.WriteFile(csvPath, []byte(stale), 0o600); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(time.Hour)
	if err := os.Chtimes(csvPath, future, future); err != nil {
		t.Fatal(err)
	}
	store, info, err := EnsureData(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if info.Refreshed {
		t.Fatalf("stale source re-imported: %+v", info)
	}
	exportDate, err := store.ExportDate(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if exportDate.Format("2006-01-02") != "2026-08-25" {
		t.Fatalf("db regressed to stale source: %s", exportDate.Format("2006-01-02"))
	}
}

func TestRejectedDownloadPreservesCSVAndAllowsNonForceRetry(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ABC_AGENT_CACHE", dir)
	csv := filepath.Join(dir, "ABC-DailyDataExport.csv")
	db := filepath.Join(dir, "abc.sqlite")
	if err := os.WriteFile(csv, []byte(testCSV), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ImportCSV(context.Background(), csv, db); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-24 * time.Hour)
	if err := os.Chtimes(csv, old, old); err != nil {
		t.Fatal(err)
	}
	bad := strings.Replace(testCSV, "License Type", "INVALID", 1)
	_, _, err := ensureData(context.Background(), true, func(_ context.Context, dst string) error { return os.WriteFile(dst, []byte(bad), 0600) })
	if err == nil {
		t.Fatal("malformed download accepted")
	}
	preserved, err := os.ReadFile(csv)
	if err != nil || string(preserved) != testCSV {
		t.Fatal("last good CSV replaced")
	}
	downloaded := false
	s, _, err := ensureData(context.Background(), false, func(_ context.Context, dst string) error {
		downloaded = true
		return os.WriteFile(dst, []byte(testCSV), 0600)
	})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if !downloaded {
		t.Fatal("non-force retry was poisoned by rejected download")
	}
	stats, err := s.Stats(context.Background())
	if err != nil || stats.TotalRecords != 4 {
		t.Fatalf("mirror: %+v, %v", stats, err)
	}
}
