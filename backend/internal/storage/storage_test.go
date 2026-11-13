package storage

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/Wiktor-Sikora/pk-status/backend/internal/checker"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})
	return s
}

func mustInsert(t *testing.T, s *Store, r checker.Result) {
	t.Helper()
	if err := s.InsertResult(r); err != nil {
		t.Fatalf("InsertResult: %v", err)
	}
}

func TestLatestPerTarget(t *testing.T) {
	s := newTestStore(t)
	base := time.Date(2026, time.January, 2, 12, 0, 0, 0, time.UTC)

	mustInsert(t, s, checker.Result{
		Target: "https://a.example", CheckedAt: base, StatusCode: 200, OK: true, LatencyMS: 10,
	})
	mustInsert(t, s, checker.Result{
		Target: "https://b.example", CheckedAt: base, StatusCode: 201, OK: true, LatencyMS: 20,
	})
	mustInsert(t, s, checker.Result{
		Target: "https://a.example", CheckedAt: base.Add(time.Minute), StatusCode: 500,
		OK: false, LatencyMS: 30, Error: "boom",
	})

	results, err := s.LatestPerTarget()
	if err != nil {
		t.Fatalf("LatestPerTarget: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2: %#v", len(results), results)
	}

	a, b := results[0], results[1]
	if a.Target != "https://a.example" {
		t.Errorf("results[0].Target = %q, want a.example", a.Target)
	}
	if a.StatusCode != 500 || a.OK || a.Error != "boom" || a.LatencyMS != 30 {
		t.Errorf("latest result for a = %#v, want newest (500, not ok)", a)
	}
	if !a.CheckedAt.Equal(base.Add(time.Minute)) {
		t.Errorf("a.CheckedAt = %s, want %s", a.CheckedAt, base.Add(time.Minute))
	}
	if b.Target != "https://b.example" || b.StatusCode != 201 {
		t.Errorf("results[1] = %#v, want b.example 201", b)
	}
}

func TestHistoryOrderLimitAndIsolation(t *testing.T) {
	s := newTestStore(t)
	base := time.Date(2026, time.January, 2, 12, 0, 0, 0, time.UTC)

	for i := 0; i < 5; i++ {
		mustInsert(t, s, checker.Result{
			Target:     "https://a.example",
			CheckedAt:  base.Add(time.Duration(i) * time.Minute),
			StatusCode: 200,
			OK:         true,
			LatencyMS:  int64(i),
		})
	}
	mustInsert(t, s, checker.Result{
		Target: "https://b.example", CheckedAt: base, StatusCode: 200, OK: true, LatencyMS: 99,
	})

	history, err := s.History("https://a.example", 3)
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(history) != 3 {
		t.Fatalf("got %d results, want 3", len(history))
	}
	for i, r := range history {
		if r.Target != "https://a.example" {
			t.Errorf("history[%d].Target = %q, want a.example", i, r.Target)
		}
		if want := int64(i + 2); r.LatencyMS != want {
			t.Errorf("history[%d].LatencyMS = %d, want %d (oldest first, last 3)", i, r.LatencyMS, want)
		}
	}

	all, err := s.History("https://a.example", 10)
	if err != nil {
		t.Fatalf("History(all): %v", err)
	}
	if len(all) != 5 {
		t.Errorf("got %d results, want 5", len(all))
	}

	empty, err := s.History("https://missing.example", 3)
	if err != nil {
		t.Fatalf("History(missing): %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("got %d results for unknown target, want 0", len(empty))
	}
}

func TestNullableFieldsRoundTrip(t *testing.T) {
	s := newTestStore(t)
	base := time.Date(2026, time.January, 2, 12, 0, 0, 0, time.UTC)
	notAfter := base.Add(90 * 24 * time.Hour)
	daysLeft := 90

	mustInsert(t, s, checker.Result{
		Target:       "https://tls.example",
		CheckedAt:    base,
		StatusCode:   200,
		OK:           true,
		LatencyMS:    12,
		CertIssuer:   "Test CA",
		CertNotAfter: &notAfter,
		CertDaysLeft: &daysLeft,
	})
	mustInsert(t, s, checker.Result{
		Target:     "https://plain.example",
		CheckedAt:  base,
		StatusCode: 200,
		OK:         true,
	})

	results, err := s.LatestPerTarget()
	if err != nil {
		t.Fatalf("LatestPerTarget: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}

	plain := results[0]
	if plain.Target != "https://plain.example" {
		t.Fatalf("results[0].Target = %q", plain.Target)
	}
	if plain.Error != "" || plain.CertIssuer != "" {
		t.Errorf("plain result has unexpected string fields: %#v", plain)
	}
	if plain.CertNotAfter != nil || plain.CertDaysLeft != nil {
		t.Errorf("plain result has unexpected cert fields: %#v", plain)
	}

	tlsResult := results[1]
	if tlsResult.Target != "https://tls.example" {
		t.Fatalf("results[1].Target = %q", tlsResult.Target)
	}
	if tlsResult.CertIssuer != "Test CA" {
		t.Errorf("CertIssuer = %q, want Test CA", tlsResult.CertIssuer)
	}
	if tlsResult.CertNotAfter == nil || !tlsResult.CertNotAfter.Equal(notAfter) {
		t.Errorf("CertNotAfter = %v, want %v", tlsResult.CertNotAfter, notAfter)
	}
	if tlsResult.CertDaysLeft == nil || *tlsResult.CertDaysLeft != daysLeft {
		t.Errorf("CertDaysLeft = %v, want %d", tlsResult.CertDaysLeft, daysLeft)
	}
}
