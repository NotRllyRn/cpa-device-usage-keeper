// Package device owns aggregate device history, independently of CPA requests.
package device

import (
	"cpa-usage-keeper/internal/devicereport"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

const MaxTokens = devicereport.MaxTokens

var identifier = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._:+-]{0,159}$`)
var modelIdentifier = regexp.MustCompile(`^[a-zA-Z0-9\[][a-zA-Z0-9/._:+\[\] -]{0,159}$`)

type Tokens = devicereport.Tokens
type Model = devicereport.Model
type Snapshot = devicereport.Snapshot

type Store struct {
	db   *sql.DB
	lock *os.File
}

func OpenStore(dir string) (*Store, error) {
	lock, err := os.OpenFile(filepath.Join(dir, ".store.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		lock.Close()
		return nil, errors.New("data directory already in use")
	}
	success := false
	defer func() {
		if !success {
			lock.Close()
		}
	}()
	path := filepath.Join(dir, "devices.sqlite")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if err == nil {
		err = f.Close()
	} else if os.IsExist(err) {
		err = nil
	}
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return nil, errors.New("database must be a private regular file")
	}
	db, err := sql.Open("sqlite3", (&url.URL{Scheme: "file", Path: path}).String()+"?_busy_timeout=2000&_journal_mode=WAL&_foreign_keys=on&_synchronous=FULL")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`PRAGMA max_page_count=262144;
 CREATE TABLE IF NOT EXISTS devices(id TEXT PRIMARY KEY,name TEXT NOT NULL,token_hash TEXT NOT NULL UNIQUE,revoked INTEGER NOT NULL DEFAULT 0,created_at TEXT NOT NULL,last_seen TEXT);
 CREATE TABLE IF NOT EXISTS device_prices(model TEXT PRIMARY KEY,payload BLOB NOT NULL);
 CREATE TABLE IF NOT EXISTS snapshots(device_id TEXT NOT NULL REFERENCES devices(id),agent TEXT NOT NULL,day TEXT NOT NULL,revision INTEGER NOT NULL,digest TEXT NOT NULL,payload BLOB NOT NULL,PRIMARY KEY(device_id,agent,day));`)
	if err != nil {
		db.Close()
		return nil, err
	}
	success = true
	return &Store{db: db, lock: lock}, nil
}
func (s *Store) Close() error { err := s.db.Close(); _ = s.lock.Close(); return err }
func randomToken() (string, error) {
	var b [32]byte
	_, err := rand.Read(b[:])
	return hex.EncodeToString(b[:]), err
}
func tokenHash(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
func (s *Store) Enroll(name string) (id, token string, err error) {
	if !identifier.MatchString(name) {
		return "", "", errors.New("invalid device name")
	}
	token, err = randomToken()
	if err != nil {
		return
	}
	id, err = randomToken()
	if err != nil {
		return
	}
	id = id[:24]
	result, e := s.db.Exec(`INSERT INTO devices(id,name,token_hash,created_at) SELECT ?,?,?,? WHERE (SELECT COUNT(*) FROM devices)<1000`, id, name, tokenHash(token), time.Now().UTC().Format(time.RFC3339Nano))
	err = e
	if err == nil {
		n, e := result.RowsAffected()
		err = e
		if n == 0 {
			err = errors.New("device enrollment limit reached")
		}
	}
	return
}

var ErrUnauthorized = errors.New("unauthorized")
var ErrConflict = errors.New("revision conflict")

// Apply checks revocation and revision inside the same transaction. An identical
// retry is acknowledged; stale or conflicting revisions never change totals.
func (s *Store) Apply(token string, snapshot Snapshot) (bool, error) {
	if err := snapshot.Validate(time.Now()); err != nil {
		return false, err
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		return false, err
	}
	digest := tokenHash(string(raw))
	tx, err := s.db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	var id string
	if err = tx.QueryRow(`SELECT id FROM devices WHERE token_hash=? AND revoked=0`, tokenHash(token)).Scan(&id); errors.Is(err, sql.ErrNoRows) {
		return false, ErrUnauthorized
	} else if err != nil {
		return false, err
	}
	var rev int64
	var old string
	err = tx.QueryRow(`SELECT revision,digest FROM snapshots WHERE device_id=? AND agent=? AND day=?`, id, snapshot.Agent, snapshot.Day).Scan(&rev, &old)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, err
	}
	replay := err == nil && rev == snapshot.Revision && old == digest
	if err == nil && (snapshot.Revision < rev || snapshot.Revision == rev && !replay) {
		return false, ErrConflict
	}
	if !replay {
		_, err = tx.Exec(`INSERT INTO snapshots(device_id,agent,day,revision,digest,payload) VALUES(?,?,?,?,?,?) ON CONFLICT(device_id,agent,day) DO UPDATE SET revision=excluded.revision,digest=excluded.digest,payload=excluded.payload`, id, snapshot.Agent, snapshot.Day, snapshot.Revision, digest, raw)
		if err != nil {
			return false, err
		}
	}
	_, err = tx.Exec(`UPDATE devices SET last_seen=? WHERE id=?`, time.Now().UTC().Format(time.RFC3339Nano), id)
	if err != nil {
		return false, err
	}
	return replay, tx.Commit()
}
func (s *Store) Authorized(token string) bool {
	var id string
	return s.db.QueryRow(`SELECT id FROM devices WHERE token_hash=? AND revoked=0`, tokenHash(token)).Scan(&id) == nil
}
func (s *Store) Revoke(id string) error {
	_, err := s.db.Exec(`UPDATE devices SET revoked=1 WHERE id=?`, id)
	return err
}

type Device struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Revoked  bool    `json:"revoked"`
	LastSeen *string `json:"last_seen"`
}

func (s *Store) Devices() ([]Device, error) {
	rows, err := s.db.Query(`SELECT id,name,revoked,last_seen FROM devices ORDER BY id LIMIT 1000`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Device{}
	for rows.Next() {
		var d Device
		if err = rows.Scan(&d.ID, &d.Name, &d.Revoked, &d.LastSeen); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

type Record struct {
	DeviceID string `json:"device_id"`
	Cursor   int64  `json:"cursor"`
	Snapshot
}

func (s *Store) History(offset int) ([]Record, error) {
	rows, err := s.db.Query(`SELECT rowid,device_id,payload FROM snapshots WHERE rowid>? ORDER BY rowid LIMIT 200`, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Record{}
	for rows.Next() {
		var r Record
		var raw []byte
		if err = rows.Scan(&r.Cursor, &r.DeviceID, &raw); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(raw, &r.Snapshot); err != nil {
			return nil, fmt.Errorf("stored snapshot: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
