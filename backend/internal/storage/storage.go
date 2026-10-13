package storage

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/Wiktor-Sikora/pk-status/backend/internal/checker"

	_ "modernc.org/sqlite"
)

// Store wraps a SQLite database holding check results.
type Store struct {
	db *sql.DB
}

const schema = `
CREATE TABLE IF NOT EXISTS checks (
	id             INTEGER PRIMARY KEY AUTOINCREMENT,
	target         TEXT    NOT NULL,
	checked_at     TIMESTAMP NOT NULL,
	status_code    INTEGER NOT NULL DEFAULT 0,
	ok             BOOLEAN NOT NULL DEFAULT 0,
	latency_ms     INTEGER NOT NULL DEFAULT 0,
	error          TEXT,
	cert_issuer    TEXT,
	cert_not_after TIMESTAMP,
	cert_days_left INTEGER
);
CREATE INDEX IF NOT EXISTS idx_checks_target_time ON checks (target, checked_at);
`

// Open opens (creating if needed) the SQLite database at path and runs migrations.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// SQLite handles one writer at a time; keep a single connection to avoid
	// "database is locked" under the scheduler + API.
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

// InsertResult persists a single check result.
func (s *Store) InsertResult(r checker.Result) error {
	_, err := s.db.Exec(
		`INSERT INTO checks
			(target, checked_at, status_code, ok, latency_ms, error, cert_issuer, cert_not_after, cert_days_left)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.Target, r.CheckedAt, r.StatusCode, r.OK, r.LatencyMS,
		nullString(r.Error), nullString(r.CertIssuer), nullTime(r.CertNotAfter), nullInt(r.CertDaysLeft),
	)
	if err != nil {
		return fmt.Errorf("insert result: %w", err)
	}
	return nil
}

// LatestPerTarget returns the most recent result for each distinct target.
func (s *Store) LatestPerTarget() ([]checker.Result, error) {
	rows, err := s.db.Query(`
		SELECT target, checked_at, status_code, ok, latency_ms, error, cert_issuer, cert_not_after, cert_days_left
		FROM checks c
		WHERE checked_at = (SELECT MAX(checked_at) FROM checks WHERE target = c.target)
		GROUP BY target
		ORDER BY target`)
	if err != nil {
		return nil, fmt.Errorf("query latest: %w", err)
	}
	defer rows.Close()
	return scanResults(rows)
}

// History returns up to limit recent results for a target, oldest first.
func (s *Store) History(target string, limit int) ([]checker.Result, error) {
	rows, err := s.db.Query(`
		SELECT target, checked_at, status_code, ok, latency_ms, error, cert_issuer, cert_not_after, cert_days_left
		FROM (
			SELECT * FROM checks WHERE target = ? ORDER BY checked_at DESC LIMIT ?
		) ORDER BY checked_at ASC`, target, limit)
	if err != nil {
		return nil, fmt.Errorf("query history: %w", err)
	}
	defer rows.Close()
	return scanResults(rows)
}

func scanResults(rows *sql.Rows) ([]checker.Result, error) {
	results := []checker.Result{}
	for rows.Next() {
		var (
			r            checker.Result
			errStr       sql.NullString
			certIssuer   sql.NullString
			certNotAfter sql.NullTime
			certDaysLeft sql.NullInt64
		)
		if err := rows.Scan(
			&r.Target, &r.CheckedAt, &r.StatusCode, &r.OK, &r.LatencyMS,
			&errStr, &certIssuer, &certNotAfter, &certDaysLeft,
		); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		if errStr.Valid {
			r.Error = errStr.String
		}
		if certIssuer.Valid {
			r.CertIssuer = certIssuer.String
		}
		if certNotAfter.Valid {
			t := certNotAfter.Time
			r.CertNotAfter = &t
		}
		if certDaysLeft.Valid {
			d := int(certDaysLeft.Int64)
			r.CertDaysLeft = &d
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return *t
}

func nullInt(i *int) any {
	if i == nil {
		return nil
	}
	return *i
}
