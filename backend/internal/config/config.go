package config

import (
	"net/url"
	"os"
	"strings"
	"time"
)

// Target is a single endpoint to check, with a human-friendly display name.
type Target struct {
	Name string
	URL  string
}

// Config holds the runtime configuration, all sourced from environment variables.
type Config struct {
	Targets      []Target      // endpoints to check
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

// splitTargets parses a comma-separated list of targets. Each entry is either
// "name=url" or a bare "url" (in which case the URL's host is used as the name).
func splitTargets(raw string) []Target {
	parts := strings.Split(raw, ",")
	out := make([]Target, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		var t Target
		// Only treat "=" as a name separator when it appears before the scheme,
		// so URLs with query strings (?a=b) still work as bare entries.
		if name, rest, ok := strings.Cut(p, "="); ok && !strings.Contains(name, "://") {
			t = Target{Name: strings.TrimSpace(name), URL: strings.TrimSpace(rest)}
		} else {
			t = Target{URL: p}
		}
		if t.URL == "" {
			continue
		}
		if t.Name == "" {
			t.Name = hostOf(t.URL)
		}
		out = append(out, t)
	}
	return out
}

func hostOf(raw string) string {
	if u, err := url.Parse(raw); err == nil && u.Host != "" {
		return u.Host
	}
	return raw
}

// URLs returns just the target URLs.
func (c Config) URLs() []string {
	out := make([]string, len(c.Targets))
	for i, t := range c.Targets {
		out[i] = t.URL
	}
	return out
}

// Names maps each target URL to its display name.
func (c Config) Names() map[string]string {
	out := make(map[string]string, len(c.Targets))
	for _, t := range c.Targets {
		out[t.URL] = t.Name
	}
	return out
}
