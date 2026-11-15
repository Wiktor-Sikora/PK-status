// Package storage persists check results in SQLite.
//
// Data model (schema version 2):
//
//   - targets       — dictionary of monitored URLs; other tables reference the
//     integer id instead of repeating the URL in every row.
//   - latest        — exactly one row per target with its most recent result,
//     updated on every insert, so /api/status reads one row per target instead
//     of searching the whole history.
//   - checks_hourly — per-target, per-hour aggregates (count, ok count, latency
//     sum/min/max) for recent hours.
//   - checks_daily  — the same aggregates per UTC day. Once a whole day is older
//     than the hourly retention, its hours are rolled up into one daily row and
//     deleted, so every check is counted in exactly one of the two tables.
//     Uptime is computed from hourly + daily aggregates, never from raw data.
//   - checks        — raw results in a compact WITHOUT ROWID table keyed by
//     (target_id, checked_at). Kept only for the raw retention period and used
//     by /api/history.
//
// Timestamps are INTEGER unix time, always UTC: milliseconds for check times,
// seconds for hour/day buckets and certificate expiry.
package storage

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/Wiktor-Sikora/pk-status/backend/internal/checker"

	_ "modernc.org/sqlite"
)

const schemaVersion = 2

const schema = `
CREATE TABLE IF NOT EXISTS targets (
	id  INTEGER PRIMARY KEY,
	url TEXT NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS checks (
	target_id      INTEGER NOT NULL REFERENCES targets(id),
	checked_at     INTEGER NOT NULL,          -- unix ms
	status_code    INTEGER NOT NULL DEFAULT 0,
	ok             INTEGER NOT NULL DEFAULT 0,
	latency_ms     INTEGER NOT NULL DEFAULT 0,
	error          TEXT,
	cert_not_after INTEGER,                   -- unix s
	PRIMARY KEY (target_id, checked_at)
) WITHOUT ROWID;

CREATE TABLE IF NOT EXISTS latest (
	target_id      INTEGER PRIMARY KEY REFERENCES targets(id),
	checked_at     INTEGER NOT NULL,          -- unix ms
	status_code    INTEGER NOT NULL DEFAULT 0,
	ok             INTEGER NOT NULL DEFAULT 0,
	latency_ms     INTEGER NOT NULL DEFAULT 0,
	error          TEXT,
	cert_issuer    TEXT,
	cert_not_after INTEGER,                   -- unix s
	cert_days_left INTEGER
);

CREATE TABLE IF NOT EXISTS checks_hourly (
	target_id   INTEGER NOT NULL REFERENCES targets(id),
	hour        INTEGER NOT NULL,             -- unix s, start of the UTC hour
	total       INTEGER NOT NULL,
	ok          INTEGER NOT NULL,
	latency_sum INTEGER NOT NULL,
	latency_min INTEGER NOT NULL,
	latency_max INTEGER NOT NULL,
	PRIMARY KEY (target_id, hour)
) WITHOUT ROWID;

CREATE TABLE IF NOT EXISTS checks_daily (
	target_id   INTEGER NOT NULL REFERENCES targets(id),
	day         INTEGER NOT NULL,             -- unix s, start of the UTC day
	total       INTEGER NOT NULL,
	ok          INTEGER NOT NULL,
	latency_sum INTEGER NOT NULL,
	latency_min INTEGER NOT NULL,
	latency_max INTEGER NOT NULL,
	PRIMARY KEY (target_id, day)
) WITHOUT ROWID;
`

// Retention says how long each level of detail is kept. A non-positive value
// keeps that level forever (for Hourly: hours are never rolled up into days).
type Retention struct {
	Raw    time.Duration // raw check results
	Hourly time.Duration // hourly aggregates, then rolled up into daily ones
	Daily  time.Duration // daily aggregates
}

// DefaultRetention keeps the database well under 1 GB / year (NFR-05).
var DefaultRetention = Retention{
	Raw:    30 * 24 * time.Hour,
	Hourly: 7 * 24 * time.Hour,
	Daily:  365 * 24 * time.Hour,
}

// PruneStats reports what one retention pass did.
type PruneStats struct {
	RawDeleted    int64 // raw rows deleted
	HoursRolledUp int64 // hourly rows merged into daily rows and deleted
	DailyDeleted  int64 // daily rows deleted
}

// Store wraps a SQLite database holding check results.
type Store struct {
	db *sql.DB

	mu      sync.Mutex
	targets map[string]int64 // url -> targets.id cache
}

// Uptime summarises availability of one target over a period.
type Uptime struct {
	Target   string  `json:"target"`
	Name     string  `json:"name,omitempty"`
	Checks   int64   `json:"checks"`
	OKChecks int64   `json:"ok_checks"`
	Percent  float64 `json:"uptime_pct"`
	AvgMS    float64 `json:"avg_latency_ms"`
}

// Open opens (creating if needed) the SQLite database at path and migrates it
// to the current schema.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// SQLite handles one writer at a time; keep a single connection to avoid
	// "database is locked" under the scheduler + API.
	db.SetMaxOpenConns(1)

	for _, p := range []string{
		"PRAGMA journal_mode = WAL",
		"PRAGMA synchronous = NORMAL",
		"PRAGMA foreign_keys = ON",
	} {
		if _, err := db.Exec(p); err != nil {
			db.Close()
			return nil, fmt.Errorf("%s: %w", p, err)
		}
	}

	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return &Store{db: db, targets: map[string]int64{}}, nil
}

func (s *Store) Close() error { return s.db.Close() }

// InsertResult persists a single check result.
func (s *Store) InsertResult(r checker.Result) error {
	return s.InsertResults([]checker.Result{r})
}

// InsertResults persists results in one transaction: for each result the raw
// row, the hourly aggregate and the per-target latest row are updated together.
func (s *Store) InsertResults(rs []checker.Result) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("insert result: %w", err)
	}
	defer tx.Rollback()

	w, err := newWriter(tx)
	if err != nil {
		return fmt.Errorf("insert result: %w", err)
	}
	defer w.close()

	created := map[string]bool{}
	for _, r := range rs {
		id, ok := s.targets[r.Target]
		if !ok {
			if id, err = upsertTarget(tx, r.Target); err != nil {
				s.forget(created)
				return fmt.Errorf("insert result: %w", err)
			}
			s.targets[r.Target] = id
			created[r.Target] = true
		}
		if err := w.insert(id, r); err != nil {
			s.forget(created)
			return fmt.Errorf("insert result: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		s.forget(created)
		return fmt.Errorf("insert result: %w", err)
	}
	return nil
}

// forget drops cached target ids created in a transaction that did not commit.
func (s *Store) forget(urls map[string]bool) {
	for u := range urls {
		delete(s.targets, u)
	}
}

func upsertTarget(tx *sql.Tx, url string) (int64, error) {
	var id int64
	err := tx.QueryRow(
		`INSERT INTO targets (url) VALUES (?)
		 ON CONFLICT (url) DO UPDATE SET url = excluded.url
		 RETURNING id`, url).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("target id: %w", err)
	}
	return id, nil
}

// writer holds the prepared statements used to store results.
type writer struct {
	raw, hourly, latest *sql.Stmt
}

func newWriter(tx *sql.Tx) (*writer, error) {
	w := &writer{}
	var err error
	if w.raw, err = tx.Prepare(`
		INSERT INTO checks (target_id, checked_at, status_code, ok, latency_ms, error, cert_not_after)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT DO NOTHING`); err != nil {
		w.close()
		return nil, err
	}
	if w.hourly, err = tx.Prepare(`
		INSERT INTO checks_hourly (target_id, hour, total, ok, latency_sum, latency_min, latency_max)
		VALUES (?, ?, 1, ?, ?, ?, ?)
		ON CONFLICT (target_id, hour) DO UPDATE SET
			total       = total + 1,
			ok          = ok + excluded.ok,
			latency_sum = latency_sum + excluded.latency_sum,
			latency_min = min(latency_min, excluded.latency_min),
			latency_max = max(latency_max, excluded.latency_max)`); err != nil {
		w.close()
		return nil, err
	}
	if w.latest, err = tx.Prepare(`
		INSERT INTO latest (target_id, checked_at, status_code, ok, latency_ms, error, cert_issuer, cert_not_after, cert_days_left)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (target_id) DO UPDATE SET
			checked_at     = excluded.checked_at,
			status_code    = excluded.status_code,
			ok             = excluded.ok,
			latency_ms     = excluded.latency_ms,
			error          = excluded.error,
			cert_issuer    = excluded.cert_issuer,
			cert_not_after = excluded.cert_not_after,
			cert_days_left = excluded.cert_days_left
		WHERE excluded.checked_at >= latest.checked_at`); err != nil {
		w.close()
		return nil, err
	}
	return w, nil
}

func (w *writer) close() {
	for _, st := range []*sql.Stmt{w.raw, w.hourly, w.latest} {
		if st != nil {
			st.Close()
		}
	}
}

func (w *writer) insert(targetID int64, r checker.Result) error {
	at := r.CheckedAt.UnixMilli()
	ok := boolInt(r.OK)
	notAfter := nullUnix(r.CertNotAfter)

	res, err := w.raw.Exec(targetID, at, r.StatusCode, ok, r.LatencyMS, nullString(r.Error), notAfter)
	if err != nil {
		return fmt.Errorf("raw: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil // duplicate (same target and millisecond): already counted
	}
	if _, err := w.hourly.Exec(targetID, hourOf(at), ok, r.LatencyMS, r.LatencyMS, r.LatencyMS); err != nil {
		return fmt.Errorf("hourly: %w", err)
	}
	if _, err := w.latest.Exec(targetID, at, r.StatusCode, ok, r.LatencyMS,
		nullString(r.Error), nullString(r.CertIssuer), notAfter, nullInt(r.CertDaysLeft)); err != nil {
		return fmt.Errorf("latest: %w", err)
	}
	return nil
}

// LatestPerTarget returns the most recent result for each target, ordered by
// target URL.
func (s *Store) LatestPerTarget() ([]checker.Result, error) {
	rows, err := s.db.Query(`
		SELECT t.url, l.checked_at, l.status_code, l.ok, l.latency_ms, l.error,
		       l.cert_issuer, l.cert_not_after, l.cert_days_left
		FROM latest l JOIN targets t ON t.id = l.target_id
		ORDER BY t.url`)
	if err != nil {
		return nil, fmt.Errorf("query latest: %w", err)
	}
	defer rows.Close()

	results := []checker.Result{}
	for rows.Next() {
		var (
			r          checker.Result
			at         int64
			errStr     sql.NullString
			certIssuer sql.NullString
			notAfter   sql.NullInt64
			daysLeft   sql.NullInt64
		)
		if err := rows.Scan(&r.Target, &at, &r.StatusCode, &r.OK, &r.LatencyMS,
			&errStr, &certIssuer, &notAfter, &daysLeft); err != nil {
			return nil, fmt.Errorf("scan latest: %w", err)
		}
		r.CheckedAt = time.UnixMilli(at).UTC()
		r.Error = errStr.String
		r.CertIssuer = certIssuer.String
		r.CertNotAfter = unixPtr(notAfter)
		if daysLeft.Valid {
			d := int(daysLeft.Int64)
			r.CertDaysLeft = &d
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// History returns up to limit recent raw results for a target, oldest first.
// Raw results exist only for the raw retention period. The certificate issuer
// is not stored per check; cert_days_left is derived from the expiry time.
func (s *Store) History(target string, limit int) ([]checker.Result, error) {
	rows, err := s.db.Query(`
		SELECT checked_at, status_code, ok, latency_ms, error, cert_not_after
		FROM (
			SELECT c.* FROM checks c JOIN targets t ON t.id = c.target_id
			WHERE t.url = ?
			ORDER BY c.checked_at DESC LIMIT ?
		) ORDER BY checked_at ASC`, target, limit)
	if err != nil {
		return nil, fmt.Errorf("query history: %w", err)
	}
	defer rows.Close()

	results := []checker.Result{}
	for rows.Next() {
		var (
			r        = checker.Result{Target: target}
			at       int64
			errStr   sql.NullString
			notAfter sql.NullInt64
		)
		if err := rows.Scan(&at, &r.StatusCode, &r.OK, &r.LatencyMS, &errStr, &notAfter); err != nil {
			return nil, fmt.Errorf("scan history: %w", err)
		}
		r.CheckedAt = time.UnixMilli(at).UTC()
		r.Error = errStr.String
		if r.CertNotAfter = unixPtr(notAfter); r.CertNotAfter != nil {
			d := int(r.CertNotAfter.Sub(r.CheckedAt).Hours() / 24)
			r.CertDaysLeft = &d
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// Uptime returns availability per target since the given time, computed from
// hourly and daily aggregates. The bucket containing since is counted in full:
// the hour for recent data, the whole UTC day for data already rolled up into
// days. When target is non-empty only that target is returned.
func (s *Store) Uptime(since time.Time, target string) ([]Uptime, error) {
	// An hour lives either in checks_hourly or (rolled up) in checks_daily,
	// never in both, so the two can simply be added. CROSS JOIN fixes the loop
	// order: for each target, a range search on the primary key of each table.
	ms := since.UnixMilli()
	rows, err := s.db.Query(`
		SELECT url, SUM(total), SUM(ok), SUM(latency_sum)
		FROM (
			SELECT t.id, t.url, h.total, h.ok, h.latency_sum
			FROM targets t CROSS JOIN checks_hourly h
			WHERE h.target_id = t.id AND h.hour >= ?1 AND (?3 = '' OR t.url = ?3)
			UNION ALL
			SELECT t.id, t.url, d.total, d.ok, d.latency_sum
			FROM targets t CROSS JOIN checks_daily d
			WHERE d.target_id = t.id AND d.day >= ?2 AND (?3 = '' OR t.url = ?3)
		)
		GROUP BY id
		ORDER BY url`, hourOf(ms), dayOf(ms), target)
	if err != nil {
		return nil, fmt.Errorf("query uptime: %w", err)
	}
	defer rows.Close()

	out := []Uptime{}
	for rows.Next() {
		var u Uptime
		var latencySum int64
		if err := rows.Scan(&u.Target, &u.Checks, &u.OKChecks, &latencySum); err != nil {
			return nil, fmt.Errorf("scan uptime: %w", err)
		}
		if u.Checks > 0 {
			u.Percent = 100 * float64(u.OKChecks) / float64(u.Checks)
			u.AvgMS = float64(latencySum) / float64(u.Checks)
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// Prune applies the retention policy as of now:
//   - deletes raw results older than r.Raw,
//   - rolls hourly aggregates of whole UTC days older than r.Hourly up into
//     checks_daily and deletes them from checks_hourly,
//   - deletes daily aggregates older than r.Daily.
//
// The roll-up runs in one transaction, so uptime never sees an hour twice or
// not at all. Rows arriving late for an already rolled-up day are merged into
// it on the next pass.
func (s *Store) Prune(now time.Time, r Retention) (PruneStats, error) {
	var st PruneStats
	tx, err := s.db.Begin()
	if err != nil {
		return st, fmt.Errorf("prune: %w", err)
	}
	defer tx.Rollback()

	// "target_id IN (...)" lets SQLite range-delete per target on the primary
	// key instead of scanning the whole table.
	exec := func(n *int64, q string, arg int64) error {
		res, err := tx.Exec(q, arg)
		if err != nil {
			return err
		}
		*n, _ = res.RowsAffected()
		return nil
	}
	if r.Raw > 0 {
		if err := exec(&st.RawDeleted,
			`DELETE FROM checks WHERE target_id IN (SELECT id FROM targets) AND checked_at < ?`,
			now.Add(-r.Raw).UnixMilli()); err != nil {
			return st, fmt.Errorf("prune raw: %w", err)
		}
	}
	if r.Hourly > 0 {
		// Only whole days: the cut-off is the start of the day containing now-Hourly.
		cutoff := dayOf(now.Add(-r.Hourly).UnixMilli())
		if _, err := tx.Exec(`
			INSERT INTO checks_daily (target_id, day, total, ok, latency_sum, latency_min, latency_max)
			SELECT target_id, hour - hour % 86400, SUM(total), SUM(ok), SUM(latency_sum),
			       MIN(latency_min), MAX(latency_max)
			FROM checks_hourly
			WHERE target_id IN (SELECT id FROM targets) AND hour < ?
			GROUP BY 1, 2
			ON CONFLICT (target_id, day) DO UPDATE SET
				total       = total + excluded.total,
				ok          = ok + excluded.ok,
				latency_sum = latency_sum + excluded.latency_sum,
				latency_min = min(latency_min, excluded.latency_min),
				latency_max = max(latency_max, excluded.latency_max)`, cutoff); err != nil {
			return st, fmt.Errorf("roll up hours: %w", err)
		}
		if err := exec(&st.HoursRolledUp,
			`DELETE FROM checks_hourly WHERE target_id IN (SELECT id FROM targets) AND hour < ?`,
			cutoff); err != nil {
			return st, fmt.Errorf("roll up hours: %w", err)
		}
	}
	if r.Daily > 0 {
		if err := exec(&st.DailyDeleted,
			`DELETE FROM checks_daily WHERE target_id IN (SELECT id FROM targets) AND day < ?`,
			now.Add(-r.Daily).Unix()); err != nil {
			return st, fmt.Errorf("prune daily: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return st, fmt.Errorf("prune: %w", err)
	}
	return st, nil
}

// RunRetention applies the retention policy immediately and then every
// interval until ctx is cancelled. SQLite reuses freed pages, so the file size
// stays bounded.
func (s *Store) RunRetention(ctx context.Context, interval time.Duration, r Retention) {
	prune := func() {
		st, err := s.Prune(time.Now(), r)
		if err != nil {
			log.Printf("storage: retention: %v", err)
			return
		}
		if st != (PruneStats{}) {
			log.Printf("storage: retention deleted %d raw rows, rolled up %d hourly rows into days, deleted %d daily rows",
				st.RawDeleted, st.HoursRolledUp, st.DailyDeleted)
		}
	}
	prune()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			prune()
		}
	}
}

// migrate brings db to schemaVersion. Version 0 is either an empty database
// or the legacy single-table layout, which is converted in place.
func migrate(db *sql.DB) error {
	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	if version == schemaVersion {
		return nil
	}
	if version > schemaVersion {
		return fmt.Errorf("database schema version %d is newer than supported %d", version, schemaVersion)
	}

	legacy, err := hasLegacyChecks(db)
	if err != nil {
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if legacy {
		if _, err := tx.Exec(`
			DROP INDEX IF EXISTS idx_checks_target_time;
			ALTER TABLE checks RENAME TO checks_legacy;`); err != nil {
			return fmt.Errorf("rename legacy table: %w", err)
		}
	}
	if _, err := tx.Exec(schema); err != nil {
		return fmt.Errorf("create schema: %w", err)
	}
	if legacy {
		start := time.Now()
		n, err := migrateLegacy(tx)
		if err != nil {
			return fmt.Errorf("migrate legacy data: %w", err)
		}
		log.Printf("storage: migrated %d legacy rows in %s", n, time.Since(start).Round(time.Millisecond))
	}
	if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", schemaVersion)); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if legacy {
		// Give the space of the dropped legacy table back to the filesystem.
		if _, err := db.Exec("VACUUM"); err != nil {
			return fmt.Errorf("vacuum: %w", err)
		}
	}
	return nil
}

func hasLegacyChecks(db *sql.DB) (bool, error) {
	var n int
	err := db.QueryRow(
		`SELECT count(*) FROM pragma_table_info('checks') WHERE name = 'target'`).Scan(&n)
	return n > 0, err
}

// migrateLegacy copies checks_legacy into the new tables and drops it. Rows
// are read in id order in batches; the legacy table stores timestamps as Go
// time strings, which the driver parses back into time.Time on scan.
func migrateLegacy(tx *sql.Tx) (int64, error) {
	w, err := newWriter(tx)
	if err != nil {
		return 0, err
	}
	defer w.close()

	ids := map[string]int64{}
	var lastID, total int64
	const batch = 10000
	for {
		buf, err := readLegacyBatch(tx, &lastID, batch)
		if err != nil {
			return total, err
		}
		if len(buf) == 0 {
			break
		}
		for _, r := range buf {
			id, ok := ids[r.Target]
			if !ok {
				if id, err = upsertTarget(tx, r.Target); err != nil {
					return total, err
				}
				ids[r.Target] = id
			}
			if err := w.insert(id, r); err != nil {
				return total, err
			}
		}
		total += int64(len(buf))
	}
	if _, err := tx.Exec("DROP TABLE checks_legacy"); err != nil {
		return total, err
	}
	return total, nil
}

func readLegacyBatch(tx *sql.Tx, lastID *int64, limit int) ([]checker.Result, error) {
	rows, err := tx.Query(`
		SELECT id, target, checked_at, status_code, ok, latency_ms, error,
		       cert_issuer, cert_not_after, cert_days_left
		FROM checks_legacy WHERE id > ? ORDER BY id LIMIT ?`, *lastID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []checker.Result
	for rows.Next() {
		var (
			r              checker.Result
			errStr, issuer sql.NullString
			notAfter       sql.NullTime
			daysLeft       sql.NullInt64
		)
		if err := rows.Scan(lastID, &r.Target, &r.CheckedAt, &r.StatusCode, &r.OK, &r.LatencyMS,
			&errStr, &issuer, &notAfter, &daysLeft); err != nil {
			return nil, err
		}
		r.Error, r.CertIssuer = errStr.String, issuer.String
		if notAfter.Valid {
			t := notAfter.Time
			r.CertNotAfter = &t
		}
		if daysLeft.Valid {
			d := int(daysLeft.Int64)
			r.CertDaysLeft = &d
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// hourOf returns the start of the UTC hour (unix seconds) containing ms.
func hourOf(ms int64) int64 {
	const hourMS = 3600 * 1000
	h := ms / hourMS
	if ms < 0 && ms%hourMS != 0 {
		h--
	}
	return h * 3600
}

// dayOf returns the start of the UTC day (unix seconds) containing ms.
func dayOf(ms int64) int64 {
	h := hourOf(ms)
	return h - ((h%86400)+86400)%86400
}

func unixPtr(v sql.NullInt64) *time.Time {
	if !v.Valid {
		return nil
	}
	t := time.Unix(v.Int64, 0).UTC()
	return &t
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullUnix(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.Unix()
}

func nullInt(i *int) any {
	if i == nil {
		return nil
	}
	return *i
}
