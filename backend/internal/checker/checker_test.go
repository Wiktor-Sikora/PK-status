package checker

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

type fakeStore struct {
	mu      sync.Mutex
	results []Result
}

func (f *fakeStore) InsertResult(r Result) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.results = append(f.results, r)
	return nil
}

func (f *fakeStore) snapshot() []Result {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]Result, len(f.results))
	copy(out, f.results)
	return out
}

func TestCheckOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	res := Check(context.Background(), srv.URL)

	if !res.OK {
		t.Errorf("OK = false, error = %q", res.Error)
	}
	if res.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want 200", res.StatusCode)
	}
	if res.Target != srv.URL {
		t.Errorf("Target = %q, want %q", res.Target, srv.URL)
	}
	if res.Error != "" {
		t.Errorf("Error = %q, want empty", res.Error)
	}
	if res.CheckedAt.IsZero() {
		t.Error("CheckedAt is zero")
	}
	if res.LatencyMS < 0 {
		t.Errorf("LatencyMS = %d, want >= 0", res.LatencyMS)
	}
	if res.CertNotAfter != nil || res.CertDaysLeft != nil {
		t.Errorf("unexpected cert info: %#v", res)
	}
}

func TestCheckRedirectAnd4xx(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/redirect", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/ok", http.StatusFound)
	})
	mux.HandleFunc("/ok", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/missing", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	redirected := Check(context.Background(), srv.URL+"/redirect")
	if !redirected.OK || redirected.StatusCode != http.StatusOK {
		t.Errorf("redirected: OK = %v, status = %d, want OK 200", redirected.OK, redirected.StatusCode)
	}

	missing := Check(context.Background(), srv.URL+"/missing")
	if missing.OK {
		t.Error("404 should not be OK")
	}
	if missing.StatusCode != http.StatusNotFound {
		t.Errorf("StatusCode = %d, want 404", missing.StatusCode)
	}
	if missing.Error != "" {
		t.Errorf("Error = %q, want empty for an HTTP response", missing.Error)
	}
}

func TestCheckServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	res := Check(context.Background(), srv.URL)

	if res.OK {
		t.Error("500 should not be OK")
	}
	if res.StatusCode != http.StatusInternalServerError {
		t.Errorf("StatusCode = %d, want 500", res.StatusCode)
	}
}

func TestCheckInvalidURL(t *testing.T) {
	res := Check(context.Background(), "://not-a-url")

	if res.OK {
		t.Error("invalid URL should not be OK")
	}
	if res.Error == "" {
		t.Error("Error is empty, want request error")
	}
}

func TestCheckConnectionError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close()

	res := Check(context.Background(), url)

	if res.OK {
		t.Error("connection error should not be OK")
	}
	if res.Error == "" {
		t.Error("Error is empty, want connection error")
	}
}

func TestCheckTLSDetails(t *testing.T) {
	url, tlsClient := newTLSServer(t)

	// The shared client does not trust the test's self-signed certificate, so
	// swap in a client that skips verification for this test.
	oldClient := client
	client = tlsClient
	t.Cleanup(func() { client = oldClient })

	res := Check(context.Background(), url)

	if !res.OK {
		t.Fatalf("OK = false, error = %q", res.Error)
	}
	if res.CertIssuer != "Test Issuer" {
		t.Errorf("CertIssuer = %q, want Test Issuer", res.CertIssuer)
	}
	if res.CertNotAfter == nil {
		t.Fatal("CertNotAfter is nil")
	}
	if res.CertDaysLeft == nil {
		t.Fatal("CertDaysLeft is nil")
	}
	if *res.CertDaysLeft < 1 {
		t.Errorf("CertDaysLeft = %d, want >= 1", *res.CertDaysLeft)
	}
}

// newTLSServer starts an HTTPS server using a generated certificate with a
// known issuer common name, returning its URL and a client that accepts it.
func newTLSServer(t *testing.T) (string, *http.Client) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "Test Issuer"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(48 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}

	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
		Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}},
	})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})}
	go srv.Serve(ln)
	t.Cleanup(func() { srv.Close() })

	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	return "https://" + ln.Addr().String(), client
}

func TestSchedulerStoresAllTargets(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	srv1 := httptest.NewServer(handler)
	defer srv1.Close()
	srv2 := httptest.NewServer(handler)
	defer srv2.Close()

	store := &fakeStore{}
	s := NewScheduler([]string{srv1.URL, srv2.URL}, time.Hour, store)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		s.Run(ctx)
		close(done)
	}()

	deadline := time.After(5 * time.Second)
	for len(store.snapshot()) < 2 {
		select {
		case <-deadline:
			cancel()
			t.Fatalf("only %d results stored, want 2", len(store.snapshot()))
		case <-time.After(10 * time.Millisecond):
		}
	}

	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("scheduler did not stop after context cancellation")
	}

	results := store.snapshot()
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
	seen := map[string]bool{}
	for _, r := range results {
		if !r.OK {
			t.Errorf("result for %s not OK: %q", r.Target, r.Error)
		}
		seen[r.Target] = true
	}
	if !seen[srv1.URL] || !seen[srv2.URL] {
		t.Errorf("scheduler stored targets %v, want %v and %v", seen, srv1.URL, srv2.URL)
	}
}

func TestSchedulerContinuesAfterStoreError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	srv1 := httptest.NewServer(handler)
	defer srv1.Close()
	srv2 := httptest.NewServer(handler)
	defer srv2.Close()

	store := &failOnceStore{}
	s := NewScheduler([]string{srv1.URL, srv2.URL}, time.Hour, store)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		s.Run(ctx)
		close(done)
	}()

	deadline := time.After(5 * time.Second)
	for len(store.snapshot()) < 1 {
		select {
		case <-deadline:
			cancel()
			t.Fatal("scheduler stored no results after a single insert error")
		case <-time.After(10 * time.Millisecond):
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("scheduler did not stop after context cancellation")
	}

	results := store.snapshot()
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1 (first insert failed)", len(results))
	}
	if results[0].Target != srv2.URL {
		t.Errorf("stored target = %q, want %q", results[0].Target, srv2.URL)
	}
}

type failOnceStore struct {
	mu       sync.Mutex
	attempts int
	results  []Result
}

func (f *failOnceStore) InsertResult(r Result) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.attempts++
	if f.attempts == 1 {
		return errors.New("insert failed")
	}
	f.results = append(f.results, r)
	return nil
}

func (f *failOnceStore) snapshot() []Result {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]Result, len(f.results))
	copy(out, f.results)
	return out
}
