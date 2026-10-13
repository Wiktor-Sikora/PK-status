package checker

import (
	"context"
	"log"
	"net/http"
	"time"
)

// Result is the outcome of a single check of one target.
type Result struct {
	Target       string     `json:"target"`
	CheckedAt    time.Time  `json:"checked_at"`
	StatusCode   int        `json:"status_code"`
	OK           bool       `json:"ok"`
	LatencyMS    int64      `json:"latency_ms"`
	Error        string     `json:"error,omitempty"`
	CertIssuer   string     `json:"cert_issuer,omitempty"`
	CertNotAfter *time.Time `json:"cert_not_after,omitempty"`
	CertDaysLeft *int       `json:"cert_days_left,omitempty"`
}

// Store is the subset of storage the scheduler needs.
type Store interface {
	InsertResult(r Result) error
}

// client is shared across checks; it follows redirects and times out.
var client = &http.Client{Timeout: 10 * time.Second}

// Check performs a single GET against url, measuring latency and inspecting the
// TLS certificate when present. It never returns an error: failures are encoded
// in the Result (OK=false, Error set).
func Check(ctx context.Context, url string) Result {
	res := Result{Target: url, CheckedAt: time.Now().UTC()}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		res.Error = err.Error()
		return res
	}

	start := time.Now()
	resp, err := client.Do(req)
	res.LatencyMS = time.Since(start).Milliseconds()
	if err != nil {
		res.Error = err.Error()
		return res
	}
	defer resp.Body.Close()

	res.StatusCode = resp.StatusCode
	res.OK = resp.StatusCode >= 200 && resp.StatusCode < 400

	if resp.TLS != nil && len(resp.TLS.PeerCertificates) > 0 {
		leaf := resp.TLS.PeerCertificates[0]
		res.CertIssuer = leaf.Issuer.CommonName
		notAfter := leaf.NotAfter
		res.CertNotAfter = &notAfter
		daysLeft := int(time.Until(notAfter).Hours() / 24)
		res.CertDaysLeft = &daysLeft
	}

	return res
}

// Scheduler periodically checks all targets and persists each result.
type Scheduler struct {
	targets  []string
	interval time.Duration
	store    Store
}

func NewScheduler(targets []string, interval time.Duration, store Store) *Scheduler {
	return &Scheduler{targets: targets, interval: interval, store: store}
}

// Run blocks until ctx is cancelled, checking all targets immediately and then
// on every tick.
func (s *Scheduler) Run(ctx context.Context) {
	s.checkAll(ctx)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.checkAll(ctx)
		}
	}
}

func (s *Scheduler) checkAll(ctx context.Context) {
	for _, target := range s.targets {
		r := Check(ctx, target)
		if err := s.store.InsertResult(r); err != nil {
			log.Printf("checker: failed to store result for %s: %v", target, err)
			continue
		}
		if r.OK {
			log.Printf("checker: %s ok status=%d latency=%dms", target, r.StatusCode, r.LatencyMS)
		} else {
			log.Printf("checker: %s DOWN status=%d err=%q", target, r.StatusCode, r.Error)
		}
	}
}
