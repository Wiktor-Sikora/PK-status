package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/Wiktor-Sikora/pk-status/backend/internal/checker"
)

// Store is the subset of storage the API needs.
type Store interface {
	LatestPerTarget() ([]checker.Result, error)
	History(target string, limit int) ([]checker.Result, error)
}

const (
	defaultHistoryLimit = 100
	maxHistoryLimit     = 1000
)

// Handler builds the HTTP handler exposing the status API.
func Handler(store Store) http.Handler {
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
		writeJSON(w, http.StatusOK, results)
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
		writeJSON(w, http.StatusOK, results)
	})

	return mux
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
