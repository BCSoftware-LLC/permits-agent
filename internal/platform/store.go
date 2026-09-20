package platform

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

var ErrNotFound = errors.New("case not found")
var ErrConflict = errors.New("case changed; reload before updating")
var ErrQuota = errors.New("request allowance exceeded")

type Store struct{ db *sql.DB }
type Note struct {
	Kind       string `json:"kind"`
	Text       string `json:"text"`
	SourceURL  string `json:"source_url,omitempty"`
	RecordedAt string `json:"recorded_at"`
}
type Case struct {
	ID        string `json:"id"`
	Version   int    `json:"version"`
	Intake    Intake `json:"intake"`
	Notes     []Note `json:"notes"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func Open(path string) (*Store, error) {
	if path == "" {
		return nil, fmt.Errorf("case database path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	f.Close()
	if err = os.Chmod(path, 0600); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000;
 CREATE TABLE IF NOT EXISTS cases(tenant TEXT NOT NULL,id TEXT NOT NULL,version INTEGER NOT NULL,body TEXT NOT NULL,request_key TEXT NOT NULL,request_hash TEXT NOT NULL,PRIMARY KEY(tenant,id),UNIQUE(tenant,request_key));
 CREATE TABLE IF NOT EXISTS usage(id TEXT PRIMARY KEY,tenant TEXT NOT NULL,operation TEXT NOT NULL,created_at TEXT NOT NULL,outcome TEXT NOT NULL);
 CREATE INDEX IF NOT EXISTS usage_tenant_date ON usage(tenant,created_at);
 CREATE TABLE IF NOT EXISTS source_cache(id TEXT PRIMARY KEY,body TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS model_usage(id TEXT PRIMARY KEY,tenant TEXT NOT NULL,model TEXT NOT NULL,created_at TEXT NOT NULL,response_id TEXT,input_tokens INTEGER,output_tokens INTEGER);
 CREATE INDEX IF NOT EXISTS model_usage_tenant_date ON model_usage(tenant,created_at);
 CREATE TABLE IF NOT EXISTS audit(id TEXT PRIMARY KEY,tenant TEXT NOT NULL,case_id TEXT NOT NULL,operation TEXT NOT NULL,version INTEGER NOT NULL,created_at TEXT NOT NULL);`)
	if err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db}, nil
}
func (s *Store) Close() error { return s.db.Close() }
func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}
func digest(v []byte) string { h := sha256.Sum256(v); return hex.EncodeToString(h[:]) }
func now() string            { return time.Now().UTC().Format("2006-01-02T15:04:05.000000000Z") }
func (s *Store) Create(ctx context.Context, tenant, key string, i Intake) (Case, error) {
	if err := i.Validate(); err != nil {
		return Case{}, err
	}
	if len(key) < 8 || len(key) > 100 || strings.ContainsAny(key, "\r\n") {
		return Case{}, fmt.Errorf("request_key must be 8 to 100 characters")
	}
	raw, _ := json.Marshal(i)
	hash := digest(raw)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Case{}, err
	}
	defer tx.Rollback()
	var old, oldHash string
	err = tx.QueryRowContext(ctx, "SELECT body,request_hash FROM cases WHERE tenant=? AND request_key=?", tenant, key).Scan(&old, &oldHash)
	if err == nil {
		if hash != oldHash {
			return Case{}, fmt.Errorf("request_key already used with a different intake")
		}
		var c Case
		err = json.Unmarshal([]byte(old), &c)
		return c, err
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Case{}, err
	}
	c := Case{ID: newID(), Version: 1, Intake: i, Notes: []Note{}, CreatedAt: now(), UpdatedAt: now()}
	body, _ := json.Marshal(c)
	_, err = tx.ExecContext(ctx, "INSERT INTO cases VALUES(?,?,?,?,?,?)", tenant, c.ID, c.Version, string(body), key, hash)
	if err != nil {
		return Case{}, err
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO audit VALUES(?,?,?,?,?,?)", newID(), tenant, c.ID, "create", c.Version, now())
	if err != nil {
		return Case{}, err
	}
	return c, tx.Commit()
}
func (s *Store) Get(ctx context.Context, tenant, id string) (Case, error) {
	var body string
	err := s.db.QueryRowContext(ctx, "SELECT body FROM cases WHERE tenant=? AND id=?", tenant, id).Scan(&body)
	if errors.Is(err, sql.ErrNoRows) {
		return Case{}, ErrNotFound
	}
	if err != nil {
		return Case{}, err
	}
	var c Case
	err = json.Unmarshal([]byte(body), &c)
	return c, err
}
func (s *Store) List(ctx context.Context, tenant string, limit, offset int) ([]Case, error) {
	if limit < 1 || limit > 100 || offset < 0 || offset > 1000000 {
		return nil, fmt.Errorf("limit must be 1-100 and offset 0-1000000")
	}

	rows, err := s.db.QueryContext(ctx, "SELECT json_remove(body, '$.notes') FROM cases WHERE tenant=? ORDER BY rowid DESC LIMIT ? OFFSET ?", tenant, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Case{}
	for rows.Next() {
		var b string
		var c Case
		if err = rows.Scan(&b); err != nil {
			return nil, err
		}
		if err = json.Unmarshal([]byte(b), &c); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
func (s *Store) AddNote(ctx context.Context, tenant, id string, version int, n Note) (Case, error) {
	if n.Kind != "research" && n.Kind != "owner_answer" && n.Kind != "agency_receipt" {
		return Case{}, fmt.Errorf("kind must be research, owner_answer, or agency_receipt")
	}
	if strings.TrimSpace(n.Text) == "" || len(n.Text) > 8000 {
		return Case{}, fmt.Errorf("note text must be 1 to 8000 characters")
	}
	if n.SourceURL != "" {
		if err := validateEvidenceURL(n.SourceURL); err != nil {
			return Case{}, err
		}
	}
	c, err := s.Get(ctx, tenant, id)
	if err != nil {
		return Case{}, err
	}
	if c.Version != version {
		return Case{}, ErrConflict
	}
	if len(c.Notes) >= 200 {
		return Case{}, fmt.Errorf("case note limit reached")
	}
	n.RecordedAt = now()
	c.Notes = append(c.Notes, n)
	return s.saveRevision(ctx, tenant, c, version, "add_note")
}
func (s *Store) UpdateIntake(ctx context.Context, tenant, id string, version int, intake Intake) (Case, error) {
	if err := intake.Validate(); err != nil {
		return Case{}, err
	}
	c, err := s.Get(ctx, tenant, id)
	if err != nil {
		return Case{}, err
	}
	if c.Version != version {
		return Case{}, ErrConflict
	}
	c.Intake = intake
	return s.saveRevision(ctx, tenant, c, version, "update_intake")
}
func (s *Store) saveRevision(ctx context.Context, tenant string, c Case, version int, operation string) (Case, error) {
	c.Version++
	c.UpdatedAt = now()
	body, _ := json.Marshal(c)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Case{}, err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, "UPDATE cases SET body=?,version=? WHERE tenant=? AND id=? AND version=?", string(body), c.Version, tenant, c.ID, version)
	if err != nil {
		return Case{}, err
	}
	count, err := res.RowsAffected()
	if err != nil {
		return Case{}, err
	}
	if count != 1 {
		return Case{}, ErrConflict
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO audit VALUES(?,?,?,?,?,?)", newID(), tenant, c.ID, operation, c.Version, now())
	if err != nil {
		return Case{}, err
	}
	return c, tx.Commit()
}

// Reserve durably accounts for every authenticated API request, including
// failures. These events enforce allowance; they are not Stripe invoices.
func (s *Store) Reserve(ctx context.Context, tenant, operation string, monthly int) (string, error) {
	t := time.Now().UTC()
	month := t.Format("2006-01") + "-01T00:00:00.000000000Z"
	minute := t.Add(-time.Minute).Format("2006-01-02T15:04:05.000000000Z")
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	var total, recent int
	if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM usage WHERE tenant=? AND created_at>=?", tenant, month).Scan(&total); err != nil {
		return "", err
	}
	if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM usage WHERE tenant=? AND created_at>=?", tenant, minute).Scan(&recent); err != nil {
		return "", err
	}
	if total >= monthly || recent >= 60 {
		return "", ErrQuota
	}
	id := newID()
	_, err = tx.ExecContext(ctx, "INSERT INTO usage VALUES(?,?,?,?,?)", id, tenant, operation, now(), "started")
	if err != nil {
		return "", err
	}
	return id, tx.Commit()
}
func (s *Store) Finish(ctx context.Context, id, outcome string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE usage SET outcome=? WHERE id=?", outcome, id)
	return err
}
func (s *Store) Usage(ctx context.Context, tenant string) (map[string]any, error) {
	month := time.Now().UTC().Format("2006-01") + "-01T00:00:00.000000000Z"
	var count, failed int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*),COALESCE(SUM(CASE WHEN outcome != 'ok' THEN 1 ELSE 0 END),0) FROM usage WHERE tenant=? AND created_at>=?", tenant, month).Scan(&count, &failed)
	return map[string]any{"month_utc": month[:7], "requests": count, "failed_or_incomplete": failed, "billing_active": false}, err
}
