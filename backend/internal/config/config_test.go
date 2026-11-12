package config

import (
	"reflect"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	for _, key := range []string{"TARGETS", "POLL_INTERVAL", "DB_PATH", "PORT"} {
		t.Setenv(key, "")
	}

	cfg := Load()

	wantTargets := []Target{{Name: "pk.edu.pl", URL: "https://pk.edu.pl"}}
	if !reflect.DeepEqual(cfg.Targets, wantTargets) {
		t.Errorf("Targets = %#v, want %#v", cfg.Targets, wantTargets)
	}
	if cfg.PollInterval != 60*time.Second {
		t.Errorf("PollInterval = %s, want 60s", cfg.PollInterval)
	}
	if cfg.DBPath != "/data/pk-status.db" {
		t.Errorf("DBPath = %q, want /data/pk-status.db", cfg.DBPath)
	}
	if cfg.Port != "8080" {
		t.Errorf("Port = %q, want 8080", cfg.Port)
	}
}

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("TARGETS", "pk=https://pk.edu.pl, https://www.biblos.pk.edu.pl")
	t.Setenv("POLL_INTERVAL", "15s")
	t.Setenv("DB_PATH", "/tmp/custom.db")
	t.Setenv("PORT", "9090")

	cfg := Load()

	if len(cfg.Targets) != 2 {
		t.Fatalf("got %d targets, want 2: %#v", len(cfg.Targets), cfg.Targets)
	}
	if cfg.Targets[0] != (Target{Name: "pk", URL: "https://pk.edu.pl"}) {
		t.Errorf("Targets[0] = %#v", cfg.Targets[0])
	}
	if cfg.Targets[1] != (Target{Name: "www.biblos.pk.edu.pl", URL: "https://www.biblos.pk.edu.pl"}) {
		t.Errorf("Targets[1] = %#v", cfg.Targets[1])
	}
	if cfg.PollInterval != 15*time.Second {
		t.Errorf("PollInterval = %s, want 15s", cfg.PollInterval)
	}
	if cfg.DBPath != "/tmp/custom.db" {
		t.Errorf("DBPath = %q", cfg.DBPath)
	}
	if cfg.Port != "9090" {
		t.Errorf("Port = %q", cfg.Port)
	}
}

func TestLoadInvalidDurationFallsBack(t *testing.T) {
	t.Setenv("POLL_INTERVAL", "not-a-duration")

	if got := Load().PollInterval; got != 60*time.Second {
		t.Errorf("PollInterval = %s, want default 60s", got)
	}
}

func TestSplitTargets(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []Target
	}{
		{
			name: "bare url uses host as name",
			raw:  "https://pk.edu.pl",
			want: []Target{{Name: "pk.edu.pl", URL: "https://pk.edu.pl"}},
		},
		{
			name: "name and url",
			raw:  "dziekanat=https://ehms.pk.edu.pl",
			want: []Target{{Name: "dziekanat", URL: "https://ehms.pk.edu.pl"}},
		},
		{
			name: "multiple entries with whitespace",
			raw:  " a=https://a.example , https://b.example ,, c=https://c.example ",
			want: []Target{
				{Name: "a", URL: "https://a.example"},
				{Name: "b.example", URL: "https://b.example"},
				{Name: "c", URL: "https://c.example"},
			},
		},
		{
			name: "query string is not treated as a name",
			raw:  "https://pk.edu.pl/?page=1",
			want: []Target{{Name: "pk.edu.pl", URL: "https://pk.edu.pl/?page=1"}},
		},
		{
			name: "empty input",
			raw:  "",
			want: []Target{},
		},
		{
			name: "entry without url is skipped",
			raw:  "name=",
			want: []Target{},
		},
		{
			name: "unparseable url keeps raw value as name",
			raw:  "not a url",
			want: []Target{{Name: "not a url", URL: "not a url"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitTargets(tt.raw)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("splitTargets(%q) = %#v, want %#v", tt.raw, got, tt.want)
			}
		})
	}
}

func TestURLsAndNames(t *testing.T) {
	cfg := Config{Targets: []Target{
		{Name: "pk", URL: "https://pk.edu.pl"},
		{Name: "biblos", URL: "https://www.biblos.pk.edu.pl"},
	}}

	wantURLs := []string{"https://pk.edu.pl", "https://www.biblos.pk.edu.pl"}
	if got := cfg.URLs(); !reflect.DeepEqual(got, wantURLs) {
		t.Errorf("URLs() = %#v, want %#v", got, wantURLs)
	}

	wantNames := map[string]string{
		"https://pk.edu.pl":            "pk",
		"https://www.biblos.pk.edu.pl": "biblos",
	}
	if got := cfg.Names(); !reflect.DeepEqual(got, wantNames) {
		t.Errorf("Names() = %#v, want %#v", got, wantNames)
	}
}
