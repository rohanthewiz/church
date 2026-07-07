// Note: running tests in this root package requires cfg/random_seeds.txt
// (a dummy fixture is committed) because the transitively imported
// resource/auth package fatals in init() without it.
package church

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
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
