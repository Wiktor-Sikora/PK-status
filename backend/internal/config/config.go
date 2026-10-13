package config

import (
	"os"
	"strings"
	"time"
)

// Config holds the runtime configuration, all sourced from environment variables.
type Config struct {
	Targets      []string      // URLs to check
	PollInterval time.Duration // how often to check each target
	DBPath       string        // path to the SQLite file
	Port         string        // HTTP listen port
}

// Load reads configuration from the environment, applying sensible defaults.
func Load() Config {
	return Config{
		Targets:      splitTargets(getenv("TARGETS", "https://pk.edu.pl")),
		PollInterval: getDuration("POLL_INTERVAL", 60*time.Second),
		DBPath:       getenv("DB_PATH", "/data/pk-status.db"),
		Port:         getenv("PORT", "8080"),
	}
}

func getenv(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func getDuration(key string, def time.Duration) time.Duration {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

func splitTargets(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
