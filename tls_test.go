package church

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rohanthewiz/church/config"
)

// writeCertPair writes a self-signed cert/key PEM pair identified by
// commonName, so tests can tell which pair the reloader is serving.
func writeCertPair(t *testing.T, certFile, keyFile, commonName string) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: commonName},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDer, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}

	certPem := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPem := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDer})
	if err = os.WriteFile(certFile, certPem, 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(keyFile, keyPem, 0600); err != nil {
		t.Fatal(err)
	}
}

// servedCN extracts the CommonName of the cert the reloader currently serves
func servedCN(t *testing.T, r *certReloader) string {
	t.Helper()
	cert, err := r.GetCertificate(nil)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	return leaf.Subject.CommonName
}

// The whole point of the reloader: a certbot-style file replacement must be
// picked up by a later handshake with no restart.
func TestCertReloaderPicksUpRenewal(t *testing.T) {
	dir := t.TempDir()
	certFile := filepath.Join(dir, "site.crt")
	keyFile := filepath.Join(dir, "site.key")

	writeCertPair(t, certFile, keyFile, "original")
	r, err := newCertReloader(certFile, keyFile)
	if err != nil {
		t.Fatal(err)
	}
	if cn := servedCN(t, r); cn != "original" {
		t.Fatalf("expected original cert, got %q", cn)
	}

	// "Renew": replace both files, and force a clearly different mtime in
	// case the filesystem's timestamp granularity blurs a fast overwrite
	writeCertPair(t, certFile, keyFile, "renewed")
	future := time.Now().Add(2 * time.Second)
	if err = os.Chtimes(certFile, future, future); err != nil {
		t.Fatal(err)
	}

	if cn := servedCN(t, r); cn != "renewed" {
		t.Errorf("reloader still serving %q after renewal", cn)
	}
}

// A botched renewal (e.g. cert replaced, key write failed) must not take down
// TLS: the reloader should keep serving the previous, still-valid pair.
func TestCertReloaderKeepsOldCertOnBadRenewal(t *testing.T) {
	dir := t.TempDir()
	certFile := filepath.Join(dir, "site.crt")
	keyFile := filepath.Join(dir, "site.key")

	writeCertPair(t, certFile, keyFile, "original")
	r, err := newCertReloader(certFile, keyFile)
	if err != nil {
		t.Fatal(err)
	}

	// Corrupt the cert file (mismatched/garbage content) with a fresh mtime
	if err = os.WriteFile(certFile, []byte("not a pem"), 0600); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(2 * time.Second)
	if err = os.Chtimes(certFile, future, future); err != nil {
		t.Fatal(err)
	}

	if cn := servedCN(t, r); cn != "original" {
		t.Errorf("expected previous cert to survive bad renewal, got %q", cn)
	}
}

// Startup contract: a missing/bad pair should fail fast, matching the old
// load-once behavior, so misconfiguration surfaces at boot rather than at
// first handshake.
func TestCertReloaderFailsFastOnMissingFiles(t *testing.T) {
	if _, err := newCertReloader("", ""); err == nil {
		t.Error("expected error for empty cert/key paths")
	}
	if _, err := newCertReloader("/nonexistent.crt", "/nonexistent.key"); err == nil {
		t.Error("expected error for missing cert files")
	}
}

// writeChallenge drops a token file where certbot --webroot would.
func writeChallenge(t *testing.T, webroot, token, body string) {
	t.Helper()
	dir := filepath.Join(webroot, ".well-known", "acme-challenge")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, token), []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
}

// The port-80 handler must serve certbot's challenge files (so renewals work
// with the site up), refuse to be a general file server, and redirect the rest.
func TestACMEWebrootHandler(t *testing.T) {
	webroot := t.TempDir()
	const token = "Xk3_Zl-9aBcDeFgHiJkLmNoPqRsTuVwXyZ0123456789"
	writeChallenge(t, webroot, token, token+".keyauth")
	// A secret beside (and above) the challenge dir that must stay unreachable
	if err := os.WriteFile(filepath.Join(webroot, "secret.txt"), []byte("nope"), 0644); err != nil {
		t.Fatal(err)
	}

	h := acmeWebrootHandler(webroot, httpsRedirect("443"))

	cases := []struct {
		name, target string
		wantCode     int
		wantBody     string // checked when non-empty
		wantLocation string // checked when non-empty
	}{
		{"valid token is served", acmeChallengePath + token, http.StatusOK, token + ".keyauth", ""},
		{"unknown token is 404", acmeChallengePath + "missingToken123", http.StatusNotFound, "", ""},
		{"empty token is 404", acmeChallengePath, http.StatusNotFound, "", ""},
		{"traversal is 404", acmeChallengePath + "..%2Fsecret.txt", http.StatusNotFound, "", ""},
		{"dotted name is 404", acmeChallengePath + "secret.txt", http.StatusNotFound, "", ""},
		{"other paths redirect", "/sermons?page=2", http.StatusMovedPermanently, "", "https://example.org/sermons?page=2"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "http://example.org"+tc.target, nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != tc.wantCode {
				t.Fatalf("code = %d, want %d", rec.Code, tc.wantCode)
			}
			if tc.wantBody != "" && rec.Body.String() != tc.wantBody {
				t.Fatalf("body = %q, want %q", rec.Body.String(), tc.wantBody)
			}
			if tc.wantLocation != "" && rec.Header().Get("Location") != tc.wantLocation {
				t.Fatalf("Location = %q, want %q", rec.Header().Get("Location"), tc.wantLocation)
			}
		})
	}
}

// The redirect drops an explicit :80 and carries a non-default HTTPS port.
func TestHTTPSRedirectPorts(t *testing.T) {
	cases := []struct{ tlsPort, host, want string }{
		{"443", "example.org:80", "https://example.org/a?b=1"},
		{"8443", "example.org", "https://example.org:8443/a?b=1"},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, "http://"+tc.host+"/a?b=1", nil)
		rec := httptest.NewRecorder()
		httpsRedirect(tc.tlsPort).ServeHTTP(rec, req)
		if got := rec.Header().Get("Location"); got != tc.want {
			t.Errorf("tls_port %s, host %s: Location = %q, want %q", tc.tlsPort, tc.host, got, tc.want)
		}
	}
}

// The expiry check reports remaining validity off the served cert, and goes
// through the reload path so a renewal on disk is reflected.
func TestCertReloaderCheckExpiry(t *testing.T) {
	dir := t.TempDir()
	certFile := filepath.Join(dir, "site.crt")
	keyFile := filepath.Join(dir, "site.key")
	writeCertPair(t, certFile, keyFile, "original") // valid for one more hour

	r, err := newCertReloader(certFile, keyFile)
	if err != nil {
		t.Fatal(err)
	}
	remaining, err := r.checkExpiry(time.Now(), certExpiryWarnWithin)
	if err != nil {
		t.Fatal(err)
	}
	if remaining <= 0 || remaining > time.Hour {
		t.Fatalf("remaining = %v, want within (0, 1h]", remaining)
	}

	// Evaluated two hours on, the same cert is already expired
	remaining, err = r.checkExpiry(time.Now().Add(2*time.Hour), certExpiryWarnWithin)
	if err != nil {
		t.Fatal(err)
	}
	if remaining >= 0 {
		t.Fatalf("remaining = %v, want negative for an expired cert", remaining)
	}
}

// validACMEToken accepts the base64url alphabet only.
func TestValidACMEToken(t *testing.T) {
	for _, ok := range []string{"abc", "A-Z_0-9", strings.Repeat("a", 256)} {
		if !validACMEToken(ok) {
			t.Errorf("validACMEToken(%q) = false, want true", ok)
		}
	}
	for _, bad := range []string{"", ".", "..", "a/b", "a.b", "a b", "a%2F", strings.Repeat("a", 257)} {
		if validACMEToken(bad) {
			t.Errorf("validACMEToken(%q) = true, want false", bad)
		}
	}
}

// The cutover case: the old port-80 forwarder still holds the port when the
// site starts. Once it lets go, the helper must take the port over by itself.
func TestHTTPHelperTakesPortWhenFreed(t *testing.T) {
	// Squat on a free port, as the old forwarder would
	squatter, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	_, port, _ := net.SplitHostPort(squatter.Addr().String())

	// config.Options is nil outside a real site main; give the helper a config
	oldRetry, oldOptions := httpHelperBindRetry, config.Options
	httpHelperBindRetry = 20 * time.Millisecond
	config.Options = &config.EnvConfig{}
	config.Options.Server.Port, config.Options.Server.TLSPort = port, "443"
	t.Cleanup(func() { httpHelperBindRetry, config.Options = oldRetry, oldOptions })

	startHTTPHelper(httpsRedirect("443"))
	time.Sleep(60 * time.Millisecond) // a few failed bind attempts
	_ = squatter.Close()

	client := &http.Client{
		Timeout:       time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		resp, err := client.Get("http://127.0.0.1:" + port + "/x")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode != http.StatusMovedPermanently {
				t.Fatalf("status = %d, want 301 from the helper", resp.StatusCode)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("helper never took the port over: %v", err)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
