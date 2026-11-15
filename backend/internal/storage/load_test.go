package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Wiktor-Sikora/pk-status/backend/internal/checker"
)

// TestLoad reproduces the BUG-03/04/05 measurement: 60 targets checked every
// minute for 30 days (2 592 000 rows), written in the legacy layout and then
// migrated. It checks NFR-02 (< 100 ms) and NFR-05 (< 1 GB / year).
//
//	PKSTATUS_LOAD=1 go test ./internal/storage -run TestLoad -v -timeout 30m
func TestLoad(t *testing.T) {
	if os.Getenv("PKSTATUS_LOAD") == "" {
		t.Skip("set PKSTATUS_LOAD=1 to run the load test")
	}
	const (
		targets = 60
		days    = 30
		perDay  = 24 * 60
	)
	path := filepath.Join(t.TempDir(), "load.db")
	end := time.Now().UTC().Truncate(time.Minute)
	start := end.Add(-days * 24 * time.Hour)
	urls := make([]string, targets)
	for i := range urls {
		urls[i] = fmt.Sprintf("https://service-%02d.pk.edu.pl", i)
	}
	gen := func(minute int) []checker.Result {
		at := start.Add(time.Duration(minute) * time.Minute)
		out := make([]checker.Result, targets)
		for i, u := range urls {
			r := checker.Result{Target: u, CheckedAt: at.Add(time.Duration(i) * 100 * time.Millisecond),
				StatusCode: 200, OK: true, LatencyMS: int64(50 + (minute*7+i*13)%400)}
			if (minute+i)%97 == 0 {
				r = checker.Result{Target: u, CheckedAt: r.CheckedAt, LatencyMS: 10000,
					Error: `Get "` + u + `": context deadline exceeded (Client.Timeout exceeded while awaiting headers)`}
			} else if i%5 != 0 { // most targets serve TLS
				na := end.Add(time.Duration(30+i) * 24 * time.Hour).Truncate(time.Second)
				d := int(na.Sub(at).Hours() / 24)
				r.CertIssuer, r.CertNotAfter, r.CertDaysLeft = "R11", &na, &d
			}
			out[i] = r
		}
		return out
	}

	// 1. Legacy database, as produced by the old code.
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(legacySchema); err != nil {
		t.Fatal(err)
	}
	t0 := time.Now()
	for day := 0; day < days; day++ {
		var batch []checker.Result
		for m := day * perDay; m < (day+1)*perDay; m++ {
			batch = append(batch, gen(m)...)
		}
		insertLegacy(t, db, batch)
	}
	legacyQuery := `SELECT target, checked_at FROM checks c
		WHERE checked_at = (SELECT MAX(checked_at) FROM checks WHERE target = c.target)
		GROUP BY target ORDER BY target`
	legacyLatest := timeIt(t, 3, func() error { return drain(db.Query(legacyQuery)) })
	db.Close()
	legacySize := fileSize(t, path)
	t.Logf("legacy: %d rows written in %s, file %.1f MB, LatestPerTarget (old query) %s",
		targets*days*perDay, time.Since(t0).Round(time.Second), mb(legacySize), legacyLatest)

	// 2. Migration to the new model.
	t0 = time.Now()
	s, err := Open(path)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	defer s.Close()
	t.Logf("migration: %s", time.Since(t0).Round(time.Second))

	// 3. Queries on the migrated database.
	var nRaw int
	s.db.QueryRow(`SELECT count(*) FROM checks`).Scan(&nRaw)
	if nRaw != targets*days*perDay {
		t.Fatalf("raw rows after migration = %d, want %d", nRaw, targets*days*perDay)
	}
	for _, q := range []struct {
		name string
		sql  string
		args []any
	}{
		{"LatestPerTarget", `SELECT t.url FROM latest l JOIN targets t ON t.id = l.target_id ORDER BY t.url`, nil},
		{"Uptime(all)", `SELECT url, SUM(total) FROM (
			SELECT t.id, t.url, h.total FROM targets t CROSS JOIN checks_hourly h
			WHERE h.target_id = t.id AND h.hour >= ?1 AND (?3 = '' OR t.url = ?3)
			UNION ALL
			SELECT t.id, t.url, d.total FROM targets t CROSS JOIN checks_daily d
			WHERE d.target_id = t.id AND d.day >= ?2 AND (?3 = '' OR t.url = ?3)
		) GROUP BY id ORDER BY url`,
			[]any{hourOf(start.UnixMilli()), dayOf(start.UnixMilli()), ""}},
	} {
		t.Logf("plan %s:\n%s", q.name, plan(t, s.db, q.sql, q.args...))
	}

	since := start
	checks := []struct {
		name  string
		limit time.Duration
		fn    func() (int, error)
	}{
		{"LatestPerTarget", 100 * time.Millisecond, func() (int, error) { r, err := s.LatestPerTarget(); return len(r), err }},
		{"Uptime 30d, one target", 100 * time.Millisecond, func() (int, error) { r, err := s.Uptime(since, urls[17]); return len(r), err }},
		{"Uptime 30d, all targets", 100 * time.Millisecond, func() (int, error) { r, err := s.Uptime(since, ""); return len(r), err }},
		{"History(limit=1000)", 100 * time.Millisecond, func() (int, error) { r, err := s.History(urls[17], 1000); return len(r), err }},
		{"InsertResult (1 check)", 100 * time.Millisecond, func() (int, error) {
			end = end.Add(time.Minute)
			return 1, s.InsertResult(checker.Result{Target: urls[3], CheckedAt: end, OK: true, StatusCode: 200, LatencyMS: 42})
		}},
	}
	measure := func(label string) {
		t.Logf("-- %s", label)
		for _, c := range checks {
			var n int
			d := timeIt(t, 20, func() error { var err error; n, err = c.fn(); return err })
			t.Logf("%-26s %10s  (%d rows)", c.name, d, n)
			if d > c.limit {
				t.Errorf("%s took %s, limit %s", c.name, d, c.limit)
			}
		}
	}
	measure("hourly aggregates only (right after migration)")
	before, _ := s.Uptime(since, "")
	if len(before) != targets || before[0].Checks < days*perDay {
		t.Fatalf("uptime = %d targets, first %#v", len(before), before[0])
	}

	t0 = time.Now()
	st, err := s.Prune(end, DefaultRetention)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("retention pass (roll-up of ~23 days): %+v in %s", st, time.Since(t0).Round(time.Millisecond))
	t.Logf("rows: hourly=%d daily=%d", count(t, s, "checks_hourly"), count(t, s, "checks_daily"))
	measure("hourly (7 days) + daily aggregates")
	after, _ := s.Uptime(since, "")
	for i := range before {
		if after[i].Checks < before[i].Checks || after[i].OKChecks < before[i].OKChecks {
			t.Errorf("uptime of %s lost checks after roll-up: %+v -> %+v", before[i].Target, before[i], after[i])
		}
	}

	// 4. Size. 30 days raw + 7 days hourly is the steady state with the default
	// retention; add a full year of daily aggregates to get the yearly footprint.
	checkpoint(t, s.db)
	if _, err := s.db.Exec(`VACUUM`); err != nil { // drop pages freed by the roll-up
		t.Fatal(err)
	}
	size30 := fileSize(t, path)
	tx, _ := s.db.Begin()
	ins, _ := tx.Prepare(`INSERT OR IGNORE INTO checks_daily VALUES (?, ?, 1440, 1425, 288000, 50, 10000)`)
	var ids []int64
	rows, _ := tx.Query(`SELECT id FROM targets`)
	for rows.Next() {
		var id int64
		rows.Scan(&id)
		ids = append(ids, id)
	}
	rows.Close()
	for _, id := range ids {
		for d := int64(0); d < 365; d++ {
			ins.Exec(id, dayOf(end.UnixMilli())-d*86400)
		}
	}
	ins.Close()
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	checkpoint(t, s.db)
	sizeYear := fileSize(t, path)
	t.Logf("size: legacy %.1f MB / 30 days -> new %.1f MB / 30 days raw; steady state for a year (30 d raw + 7 d hourly + 365 d daily) %.1f MB",
		mb(legacySize), mb(size30), mb(sizeYear))
	t.Logf("tables:\n%s", tableSizes(t, s.db))
	if sizeYear > 1<<30 {
		t.Errorf("yearly size %.1f MB exceeds 1 GB", mb(sizeYear))
	}
}

func timeIt(t *testing.T, runs int, fn func() error) time.Duration {
	t.Helper()
	ds := make([]time.Duration, runs)
	for i := range ds {
		t0 := time.Now()
		if err := fn(); err != nil {
			t.Fatal(err)
		}
		ds[i] = time.Since(t0)
	}
	sort.Slice(ds, func(i, j int) bool { return ds[i] < ds[j] })
	return ds[len(ds)/2].Round(10 * time.Microsecond) // median
}

func drain(rows *sql.Rows, err error) error {
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
	}
	return rows.Err()
}

func plan(t *testing.T, db *sql.DB, q string, args ...any) string {
	rows, err := db.Query("EXPLAIN QUERY PLAN "+q, args...)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var b strings.Builder
	for rows.Next() {
		var id, parent, notused int
		var detail string
		rows.Scan(&id, &parent, &notused, &detail)
		fmt.Fprintf(&b, "  %s\n", detail)
	}
	return b.String()
}

func tableSizes(t *testing.T, db *sql.DB) string {
	rows, err := db.Query(`SELECT name, SUM(pgsize) FROM dbstat GROUP BY name ORDER BY 2 DESC`)
	if err != nil {
		return "  (dbstat unavailable: " + err.Error() + ")"
	}
	defer rows.Close()
	var b strings.Builder
	for rows.Next() {
		var name string
		var size int64
		rows.Scan(&name, &size)
		fmt.Fprintf(&b, "  %-30s %8.1f MB\n", name, mb(size))
	}
	return b.String()
}

func checkpoint(t *testing.T, db *sql.DB) {
	if _, err := db.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		t.Fatal(err)
	}
}

func fileSize(t *testing.T, path string) int64 {
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return fi.Size()
}

func mb(n int64) float64 { return float64(n) / (1 << 20) }
