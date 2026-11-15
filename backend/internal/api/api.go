package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/Wiktor-Sikora/pk-status/backend/internal/checker"
	"github.com/Wiktor-Sikora/pk-status/backend/internal/storage"
)

// Store is the subset of storage the API needs.
type Store interface {
	LatestPerTarget() ([]checker.Result, error)
	History(target string, limit int) ([]checker.Result, error)
	Uptime(since time.Time, target string) ([]storage.Uptime, error)
}

const (
	defaultHistoryLimit = 100
	maxHistoryLimit     = 1000

	defaultUptimeDays = 30
	maxUptimeDays     = 365
)

// Handler builds the HTTP handler exposing the status API. names maps target
// URLs to display names; /api/status only reports currently configured targets.
func Handler(store Store, names map[string]string) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("GET /api/status", func(w http.ResponseWriter, r *http.Request) {
		results, err := store.LatestPerTarget()
		if err != nil {
			log.Printf("api: status: %v", err)
			writeError(w, http.StatusInternalServerError, "failed to load status")
			return
		}
		writeJSON(w, http.StatusOK, withNames(results, names, true))
	})

	mux.HandleFunc("GET /api/history", func(w http.ResponseWriter, r *http.Request) {
		target := r.URL.Query().Get("target")
		if target == "" {
			writeError(w, http.StatusBadRequest, "missing 'target' query parameter")
			return
		}
		limit := defaultHistoryLimit
		if raw := r.URL.Query().Get("limit"); raw != "" {
			if n, err := strconv.Atoi(raw); err == nil && n > 0 {
				limit = n
			}
		}
		if limit > maxHistoryLimit {
			limit = maxHistoryLimit
		}

		results, err := store.History(target, limit)
		if err != nil {
			log.Printf("api: history: %v", err)
			writeError(w, http.StatusInternalServerError, "failed to load history")
			return
		}
		writeJSON(w, http.StatusOK, withNames(results, names, false))
	})

	mux.HandleFunc("GET /api/uptime", func(w http.ResponseWriter, r *http.Request) {
		days := defaultUptimeDays
		if raw := r.URL.Query().Get("days"); raw != "" {
			if n, err := strconv.Atoi(raw); err == nil && n > 0 {
				days = n
			}
		}
		if days > maxUptimeDays {
			days = maxUptimeDays
		}

		since := time.Now().Add(-time.Duration(days) * 24 * time.Hour)
		uptimes, err := store.Uptime(since, r.URL.Query().Get("target"))
		if err != nil {
			log.Printf("api: uptime: %v", err)
			writeError(w, http.StatusInternalServerError, "failed to load uptime")
			return
		}
		out := uptimes[:0]
		for _, u := range uptimes {
			name, known := names[u.Target]
			if !known {
				continue
			}
			u.Name = name
			out = append(out, u)
		}
		writeJSON(w, http.StatusOK, out)
	})

	return mux
}

// withNames fills in display names. When onlyKnown is set, results for targets
// that are no longer configured are dropped.
func withNames(results []checker.Result, names map[string]string, onlyKnown bool) []checker.Result {
	out := results[:0]
	for _, r := range results {
		name, known := names[r.Target]
		if onlyKnown && !known {
			continue
		}
		r.Name = name
		out = append(out, r)
	}
	return out
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("api: encode: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
