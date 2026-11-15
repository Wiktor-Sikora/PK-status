package storage

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/Wiktor-Sikora/pk-status/backend/internal/checker"
)

func TestLatestIgnoresOlderResults(t *testing.T) {
	s := newTestStore(t)
	base := time.Date(2026, time.January, 2, 12, 0, 0, 0, time.UTC)

	mustInsert(t, s, checker.Result{Target: "https://a.example", CheckedAt: base.Add(time.Minute), StatusCode: 200, OK: true})
	mustInsert(t, s, checker.Result{Target: "https://a.example", CheckedAt: base, StatusCode: 500})

	latest, err := s.LatestPerTarget()
	if err != nil {
		t.Fatalf("LatestPerTarget: %v", err)
	}
	if len(latest) != 1 || latest[0].StatusCode != 200 {
		t.Fatalf("latest = %#v, want the newer 200 result", latest)
	}
}

func TestUptimeFromHourlyAggregates(t *testing.T) {
	s := newTestStore(t)
	base := time.Date(2026, time.January, 2, 10, 0, 0, 0, time.UTC)

	var batch []checker.Result
	// a: 3 hours x 60 checks, every 10th one failing -> 90 %.
	for i := 0; i < 180; i++ {
		batch = append(batch, checker.Result{
			Target: "https://a.example", CheckedAt: base.Add(time.Duration(i) * time.Minute),
			OK: i%10 != 0, LatencyMS: int64(i % 3 * 10),
		})
	}
	// b: always up, only in the last hour.
	for i := 120; i < 180; i++ {
		batch = append(batch, checker.Result{
			Target: "https://b.example", CheckedAt: base.Add(time.Duration(i) * time.Minute), OK: true, LatencyMS: 5,
		})
	}
	if err := s.InsertResults(batch); err != nil {
		t.Fatalf("InsertResults: %v", err)
	}

	var hourly int
	if err := s.db.QueryRow(`SELECT count(*) FROM checks_hourly`).Scan(&hourly); err != nil {
		t.Fatal(err)
	}
	if hourly != 4 {
		t.Errorf("hourly rows = %d, want 4 (3 for a, 1 for b)", hourly)
	}

	all, err := s.Uptime(base, "")
	if err != nil {
		t.Fatalf("Uptime: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("got %d uptimes, want 2: %#v", len(all), all)
	}
	if a := all[0]; a.Target != "https://a.example" || a.Checks != 180 || a.OKChecks != 162 || a.Percent != 90 || a.AvgMS != 10 {
		t.Errorf("a = %#v, want 180 checks, 162 ok, 90%%, avg 10ms", a)
	}
	if b := all[1]; b.Target != "https://b.example" || b.Checks != 60 || b.Percent != 100 {
		t.Errorf("b = %#v, want 60 checks at 100%%", b)
	}

	// Since in the middle of the last hour: that hour is counted in full.
	last, err := s.Uptime(base.Add(150*time.Minute), "https://a.example")
	if err != nil {
		t.Fatalf("Uptime(a): %v", err)
	}
	if len(last) != 1 || last[0].Checks != 60 || last[0].OKChecks != 54 {
		t.Errorf("last hour of a = %#v, want 60 checks, 54 ok", last)
	}
}

func TestDuplicateResultIsNotCountedTwice(t *testing.T) {
	s := newTestStore(t)
	r := checker.Result{Target: "https://a.example", CheckedAt: time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC), OK: true}
	mustInsert(t, s, r)
	mustInsert(t, s, r)

	u, err := s.Uptime(r.CheckedAt, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(u) != 1 || u[0].Checks != 1 {
		t.Errorf("uptime = %#v, want a single check", u)
	}
}

func TestHistoryDerivesCertDaysLeft(t *testing.T) {
	s := newTestStore(t)
	base := time.Date(2026, time.January, 2, 12, 0, 0, 0, time.UTC)
	notAfter := base.Add(10*24*time.Hour + time.Hour)
	days := 10
	mustInsert(t, s, checker.Result{
		Target: "https://a.example", CheckedAt: base, OK: true, CertIssuer: "CA",
		CertNotAfter: &notAfter, CertDaysLeft: &days,
	})

	h, err := s.History("https://a.example", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(h) != 1 || h[0].CertNotAfter == nil || !h[0].CertNotAfter.Equal(notAfter) ||
		h[0].CertDaysLeft == nil || *h[0].CertDaysLeft != 10 {
		t.Errorf("history = %#v, want cert expiry and 10 days left", h)
	}
}

func count(t *testing.T, s *Store, table string) int {
	t.Helper()
	var n int
	if err := s.db.QueryRow(`SELECT count(*) FROM ` + table).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestPrune(t *testing.T) {
	s := newTestStore(t)
	now := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)

	var batch []checker.Result
	for d := 0; d < 60; d++ {
		for _, target := range []string{"https://a.example", "https://b.example"} {
			batch = append(batch, checker.Result{Target: target, CheckedAt: now.Add(-time.Duration(d) * 24 * time.Hour), OK: true})
		}
	}
	if err := s.InsertResults(batch); err != nil {
		t.Fatal(err)
	}

	st, err := s.Prune(now, Retention{Raw: 30 * 24 * time.Hour, Hourly: 7 * 24 * time.Hour, Daily: 45 * 24 * time.Hour})
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	// Per target: raw days 31..59 are older than 30 days (29); hours of days
	// 8..59 are rolled up (52); of those, days 46..59 are older than 45 (14).
	want := PruneStats{RawDeleted: 58, HoursRolledUp: 104, DailyDeleted: 28}
	if st != want {
		t.Errorf("stats = %+v, want %+v", st, want)
	}
	if raw, hourly, daily := count(t, s, "checks"), count(t, s, "checks_hourly"), count(t, s, "checks_daily"); raw != 62 || hourly != 16 || daily != 76 {
		t.Errorf("left raw=%d hourly=%d daily=%d, want 62, 16, 76", raw, hourly, daily)
	}
	if latest, _ := s.LatestPerTarget(); len(latest) != 2 {
		t.Errorf("latest rows = %d, want 2 (never pruned)", len(latest))
	}

	if st, _ := s.Prune(now, Retention{}); st != (PruneStats{}) {
		t.Errorf("zero retention did %+v, want nothing", st)
	}
}

func TestRollupKeepsUptimeExact(t *testing.T) {
	s := newTestStore(t)
	now := time.Date(2026, time.March, 1, 15, 30, 0, 0, time.UTC)
	start := now.Add(-10 * 24 * time.Hour)

	// One check every 10 minutes for 10 days, every 7th failing.
	var batch []checker.Result
	var total, ok int64
	for at := start; at.Before(now); at = at.Add(10 * time.Minute) {
		r := checker.Result{Target: "https://a.example", CheckedAt: at, OK: total%7 != 0, LatencyMS: total % 100}
		if r.OK {
			ok++
		}
		total++
		batch = append(batch, r)
	}
	if err := s.InsertResults(batch); err != nil {
		t.Fatal(err)
	}
	since := time.Date(2026, time.February, 19, 0, 0, 0, 0, time.UTC) // day boundary before start
	before, err := s.Uptime(since, "")
	if err != nil {
		t.Fatal(err)
	}

	st, err := s.Prune(now, Retention{Hourly: 7 * 24 * time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	// now-7d = Feb 22 15:30, so Feb 19, 20 and 21 (whole days) are rolled up;
	// Feb 19 only has data from 15:00 (start = Feb 19 15:30).
	if st.HoursRolledUp != 9+24+24 {
		t.Errorf("rolled up %d hours, want 57", st.HoursRolledUp)
	}
	if n := count(t, s, "checks_daily"); n != 3 {
		t.Errorf("daily rows = %d, want 3", n)
	}
	var minHour int64
	s.db.QueryRow(`SELECT min(hour) FROM checks_hourly`).Scan(&minHour)
	if want := time.Date(2026, time.February, 22, 0, 0, 0, 0, time.UTC).Unix(); minHour != want {
		t.Errorf("oldest hourly row = %s, want Feb 22 00:00", time.Unix(minHour, 0).UTC())
	}

	after, err := s.Uptime(since, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != 1 || after[0] != before[0] || after[0].Checks != total || after[0].OKChecks != ok {
		t.Errorf("uptime after roll-up = %+v, before = %+v, want %d/%d", after, before, ok, total)
	}

	// A late result for an already rolled-up day lands in checks_hourly and is
	// merged into the existing daily row on the next pass.
	mustInsert(t, s, checker.Result{Target: "https://a.example", CheckedAt: start.Add(time.Second), OK: true})
	if _, err := s.Prune(now, Retention{Hourly: 7 * 24 * time.Hour}); err != nil {
		t.Fatal(err)
	}
	if n := count(t, s, "checks_daily"); n != 3 {
		t.Errorf("daily rows after late result = %d, want 3", n)
	}
	if u, _ := s.Uptime(since, ""); u[0].Checks != total+1 || u[0].OKChecks != ok+1 {
		t.Errorf("uptime after late result = %+v, want %d/%d", u[0], ok+1, total+1)
	}
}

const legacySchema = `
CREATE TABLE checks (
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
CREATE INDEX idx_checks_target_time ON checks (target, checked_at);
`

// insertLegacy writes rows exactly like the pre-v2 InsertResult did.
func insertLegacy(t testing.TB, db *sql.DB, rs []checker.Result) {
	t.Helper()
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	st, err := tx.Prepare(`INSERT INTO checks
		(target, checked_at, status_code, ok, latency_ms, error, cert_issuer, cert_not_after, cert_days_left)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rs {
		var notAfter any
		if r.CertNotAfter != nil {
			notAfter = *r.CertNotAfter
		}
		if _, err := st.Exec(r.Target, r.CheckedAt, r.StatusCode, r.OK, r.LatencyMS,
			nullString(r.Error), nullString(r.CertIssuer), notAfter, nullInt(r.CertDaysLeft)); err != nil {
			t.Fatal(err)
		}
	}
	st.Close()
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
}

func TestMigrateLegacyDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(legacySchema); err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, time.January, 2, 12, 0, 0, 123_000_000, time.UTC)
	notAfter := base.Add(90 * 24 * time.Hour).Truncate(time.Second)
	days := 90
	insertLegacy(t, db, []checker.Result{
		{Target: "https://a.example", CheckedAt: base, StatusCode: 200, OK: true, LatencyMS: 10},
		{Target: "https://b.example", CheckedAt: base, StatusCode: 503, LatencyMS: 20, Error: "down"},
		{Target: "https://a.example", CheckedAt: base.Add(time.Minute), StatusCode: 200, OK: true, LatencyMS: 30,
			CertIssuer: "CA", CertNotAfter: &notAfter, CertDaysLeft: &days},
	})
	db.Close()

	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open legacy: %v", err)
	}
	defer s.Close()

	latest, err := s.LatestPerTarget()
	if err != nil {
		t.Fatal(err)
	}
	if len(latest) != 2 {
		t.Fatalf("latest = %#v, want 2 targets", latest)
	}
	a := latest[0]
	if a.Target != "https://a.example" || a.LatencyMS != 30 || a.CertIssuer != "CA" ||
		!a.CheckedAt.Equal(base.Add(time.Minute)) || a.CertNotAfter == nil || !a.CertNotAfter.Equal(notAfter) ||
		a.CertDaysLeft == nil || *a.CertDaysLeft != 90 {
		t.Errorf("latest a = %#v", a)
	}
	if b := latest[1]; b.Error != "down" || b.OK {
		t.Errorf("latest b = %#v", b)
	}

	h, err := s.History("https://a.example", 10)
	if err != nil || len(h) != 2 || h[0].LatencyMS != 10 {
		t.Errorf("history a = %#v, %v", h, err)
	}
	u, err := s.Uptime(base, "")
	if err != nil || len(u) != 2 || u[0].Checks != 2 || u[1].Percent != 0 {
		t.Errorf("uptime = %#v, %v", u, err)
	}

	var legacy int
	s.db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE name LIKE 'checks_legacy%' OR name = 'idx_checks_target_time'`).Scan(&legacy)
	if legacy != 0 {
		t.Errorf("legacy objects left behind: %d", legacy)
	}

	// New results keep working after migration and reopening is a no-op.
	mustInsert(t, s, checker.Result{Target: "https://c.example", CheckedAt: base, OK: true})
	s.Close()
	s2, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer s2.Close()
	if l, _ := s2.LatestPerTarget(); len(l) != 3 {
		t.Errorf("after reopen latest = %d rows, want 3", len(l))
	}
}

func TestDayOf(t *testing.T) {
	day := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)
	for _, at := range []time.Time{day, day.Add(23*time.Hour + 59*time.Minute)} {
		if got := dayOf(at.UnixMilli()); got != day.Unix() {
			t.Errorf("dayOf(%s) = %d, want %d", at, got, day.Unix())
		}
	}
	if got := dayOf(-1); got != -86400 {
		t.Errorf("dayOf(-1) = %d, want -86400", got)
	}
}

func TestHourOf(t *testing.T) {
	for _, tc := range []struct{ ms, want int64 }{
		{0, 0}, {3_599_999, 0}, {3_600_000, 3600}, {-1, -3600}, {-3_600_000, -3600},
	} {
		if got := hourOf(tc.ms); got != tc.want {
			t.Errorf("hourOf(%d) = %d, want %d", tc.ms, got, tc.want)
		}
	}
}
