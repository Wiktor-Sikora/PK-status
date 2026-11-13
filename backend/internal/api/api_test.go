package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wiktor-Sikora/pk-status/backend/internal/checker"
)

type fakeStore struct {
	latest      []checker.Result
	latestErr   error
	history     []checker.Result
	historyErr  error
	lastTarget  string
	lastLimit   int
	historyCall int
}

func (f *fakeStore) LatestPerTarget() ([]checker.Result, error) {
	return f.latest, f.latestErr
}

func (f *fakeStore) History(target string, limit int) ([]checker.Result, error) {
	f.historyCall++
	f.lastTarget = target
	f.lastLimit = limit
	return f.history, f.historyErr
}

func result(target string) checker.Result {
	return checker.Result{
		Target:     target,
		CheckedAt:  time.Date(2026, time.January, 2, 12, 0, 0, 0, time.UTC),
		StatusCode: http.StatusOK,
		OK:         true,
		LatencyMS:  42,
	}
}

func serve(t *testing.T, store Store, names map[string]string, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, nil)
	Handler(store, names).ServeHTTP(rec, req)
	return rec
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode response %q: %v", rec.Body.String(), err)
	}
	return v
}

func TestHealth(t *testing.T) {
	rec := serve(t, &fakeStore{}, nil, http.MethodGet, "/api/health")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
	body := decode[map[string]string](t, rec)
	if body["status"] != "ok" {
		t.Errorf("body = %#v, want status ok", body)
	}
}

func TestHealthRejectsOtherMethods(t *testing.T) {
	rec := serve(t, &fakeStore{}, nil, http.MethodPost, "/api/health")

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", rec.Code)
	}
}

func TestStatusAddsNamesAndFiltersUnknownTargets(t *testing.T) {
	store := &fakeStore{latest: []checker.Result{
		result("https://a.example"),
		result("https://b.example"),
		result("https://retired.example"),
	}}
	names := map[string]string{
		"https://a.example": "A",
		"https://b.example": "B",
	}

	rec := serve(t, store, names, http.MethodGet, "/api/status")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	body := decode[[]checker.Result](t, rec)
	if len(body) != 2 {
		t.Fatalf("got %d results, want 2: %#v", len(body), body)
	}
	if body[0].Target != "https://a.example" || body[0].Name != "A" {
		t.Errorf("body[0] = %#v, want a.example named A", body[0])
	}
	if body[1].Target != "https://b.example" || body[1].Name != "B" {
		t.Errorf("body[1] = %#v, want b.example named B", body[1])
	}
}

func TestStatusStoreError(t *testing.T) {
	store := &fakeStore{latestErr: errors.New("db down")}

	rec := serve(t, store, nil, http.MethodGet, "/api/status")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	body := decode[map[string]string](t, rec)
	if body["error"] == "" {
		t.Error("error message is empty")
	}
}

func TestHistoryRequiresTarget(t *testing.T) {
	store := &fakeStore{}

	rec := serve(t, store, nil, http.MethodGet, "/api/history")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if store.historyCall != 0 {
		t.Error("store was queried despite missing target")
	}
	body := decode[map[string]string](t, rec)
	if body["error"] == "" {
		t.Error("error message is empty")
	}
}

func TestHistoryLimits(t *testing.T) {
	tests := []struct {
		name      string
		query     string
		wantLimit int
	}{
		{name: "default", query: "", wantLimit: defaultHistoryLimit},
		{name: "explicit", query: "&limit=25", wantLimit: 25},
		{name: "zero falls back", query: "&limit=0", wantLimit: defaultHistoryLimit},
		{name: "negative falls back", query: "&limit=-5", wantLimit: defaultHistoryLimit},
		{name: "invalid falls back", query: "&limit=abc", wantLimit: defaultHistoryLimit},
		{name: "clamped to max", query: "&limit=5000", wantLimit: maxHistoryLimit},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeStore{}
			rec := serve(t, store, nil, http.MethodGet, "/api/history?target=https://a.example"+tt.query)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
			}
			if store.lastTarget != "https://a.example" {
				t.Errorf("store target = %q, want https://a.example", store.lastTarget)
			}
			if store.lastLimit != tt.wantLimit {
				t.Errorf("store limit = %d, want %d", store.lastLimit, tt.wantLimit)
			}
		})
	}
}

func TestHistoryAddsNamesWithoutFiltering(t *testing.T) {
	store := &fakeStore{history: []checker.Result{
		result("https://a.example"),
		result("https://unknown.example"),
	}}
	names := map[string]string{"https://a.example": "A"}

	rec := serve(t, store, names, http.MethodGet, "/api/history?target=https://a.example")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := decode[[]checker.Result](t, rec)
	if len(body) != 2 {
		t.Fatalf("got %d results, want 2", len(body))
	}
	if body[0].Name != "A" {
		t.Errorf("body[0].Name = %q, want A", body[0].Name)
	}
	if body[1].Target != "https://unknown.example" || body[1].Name != "" {
		t.Errorf("body[1] = %#v, want unknown target with empty name", body[1])
	}
}

func TestHistoryStoreError(t *testing.T) {
	store := &fakeStore{historyErr: errors.New("db down")}

	rec := serve(t, store, nil, http.MethodGet, "/api/history?target=https://a.example")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	body := decode[map[string]string](t, rec)
	if body["error"] == "" {
		t.Error("error message is empty")
	}
}
