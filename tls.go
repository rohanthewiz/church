package church

// TLS setup for Serve. Two modes, selected by server.auto_cert:
//
//   auto_cert: true  — fully in-process Let's Encrypt. autocert issues and
//     renews certificates via ACME; nothing external to schedule. Challenges
//     are satisfied two ways: TLS-ALPN on the HTTPS listener itself (via the
//     manager's GetCertificate) and HTTP-01 on the plain-HTTP helper listener.
//
//   auto_cert: false — cert_file/key_file are served through a hot reloader:
//     each handshake cheaply stats the cert file and reloads the pair when it
//     changes, so an external renewer (certbot cron) only replaces files and
//     the running server picks them up. No restart, no cert pinned at boot.
//     With server.acme_webroot set, the HTTP listener also serves certbot's
//     --webroot challenge files, so `certbot renew` runs unattended from
//     certbot's own timer while the site stays up. A daily expiry check
//     logs a warning if renewal has silently stopped working.
//
// Both modes hand rweb a *tls.Config through TLSCfg.Config (the dynamic-cert
// hook added to rweb for exactly this), and both start a plain-HTTP listener
// on server.port that redirects to HTTPS. That listener replaces what used
// to be two standalone helpers on bare-metal hosts: an HTTP→HTTPS forwarder
// holding port 80, and a challenge server swapped in by hand for renewals.
//
//	              :port (HTTP)                       :tls_port (HTTPS)
//	   ┌────────────────────────────────────┐   ┌───────────────────────────┐
//	   │ /.well-known/acme-challenge/*      │   │ rweb server               │
//	   │   autocert: in-memory token        │   │ TLS via GetCertificate:   │
//	   │   cert files: <acme_webroot> file  │   │  autocert or certReloader │
//	   │ everything else → 301 redirect ────┼──▶│                           │
//	   └────────────────────────────────────┘   └───────────────────────────┘
//
//	Unattended certbot renewal (cert-file mode, acme_webroot set):
//	  certbot.timer → certbot renew → writes token under acme_webroot
//	    → Let's Encrypt fetches it via :80 (served above) → new files in live/
//	    → next handshake: certReloader sees the new mtime and swaps the pair

import (
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rohanthewiz/church/config"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/rweb"
	"github.com/rohanthewiz/serr"
	"golang.org/x/crypto/acme/autocert"
)

// buildTLSCfg assembles rweb's TLS configuration from config.Options.Server
// and, when TLS is on, starts the HTTP challenge/redirect listener.
// A disabled (zero) TLSCfg is returned outside of TLS mode; development
// always runs plain HTTP so local work never needs certs.
func buildTLSCfg() (tlsCfg rweb.TLSCfg, err error) {
	srv := config.Options.Server
	if !srv.UseTLS || config.AppEnv == "development" {
		return tlsCfg, nil
	}

	tlsPort := srv.TLSPort
	if tlsPort == "" {
		tlsPort = "443"
	}
	tlsCfg = rweb.TLSCfg{UseTLS: true, TLSAddr: ":" + tlsPort}

	if srv.AutoCert {
		mgr, err := newAutocertManager()
		if err != nil {
			return tlsCfg, serr.Wrap(err, "autocert setup failed")
		}
		// The manager's config carries GetCertificate (issue/renew on demand)
		// and the acme-tls/1 ALPN proto needed for TLS-ALPN-01 challenges.
		tlsCfg.Config = mgr.TLSConfig()
		// HTTPHandler answers HTTP-01 challenges; non-challenge traffic goes
		// to our redirect. (Its nil-fallback redirect would drop a non-443
		// tls_port, sending browsers to a port nothing listens on.)
		startHTTPHelper(mgr.HTTPHandler(httpsRedirect(tlsPort)))
		logger.Info("TLS: Let's Encrypt autocert enabled", "tls_port", tlsPort)
	} else {
		rld, err := newCertReloader(srv.CertFile, srv.KeyFile)
		if err != nil {
			return tlsCfg, serr.Wrap(err, "could not load TLS certificate",
				"cert_file", srv.CertFile, "key_file", srv.KeyFile)
		}
		tlsCfg.Config = &tls.Config{
			GetCertificate: rld.GetCertificate,
			MinVersion:     tls.VersionTLS12,
		}
		// Challenge files first (when certbot uses a webroot), redirect for
		// the rest. With no webroot configured this is just the redirect.
		var handler http.Handler = httpsRedirect(tlsPort)
		if srv.ACMEWebroot != "" {
			handler = acmeWebrootHandler(srv.ACMEWebroot, handler)
			logger.Info("TLS: serving ACME HTTP-01 challenges from webroot", "acme_webroot", srv.ACMEWebroot)
		}
		startHTTPHelper(handler)
		rld.watchExpiry(certExpiryWarnWithin, 24*time.Hour)
		logger.Info("TLS: serving cert files with hot reload", "cert_file", srv.CertFile)
	}
	return tlsCfg, nil
}

// newAutocertManager builds the ACME manager. HostPolicy is a strict
// whitelist — without one, anyone pointing DNS at the server could mint
// certs against our Let's Encrypt rate limits.
func newAutocertManager() (*autocert.Manager, error) {
	srv := config.Options.Server

	domains := srv.AutoCertDomains
	if len(domains) == 0 && srv.Domain != "" {
		domains = []string{srv.Domain}
	}
	if len(domains) == 0 {
		return nil, serr.New("auto_cert requires server.auto_cert_domains (or server.domain) to be set")
	}

	cacheDir := srv.AutoCertCacheDir
	if cacheDir == "" {
		cacheDir = "certs/autocert"
	}
	// Persist issued certs so restarts don't re-hit the CA (and stay under
	// its duplicate-certificate rate limit). 0700: the dir holds private keys.
	if err := os.MkdirAll(cacheDir, 0700); err != nil {
		return nil, serr.Wrap(err, "could not create autocert cache dir", "dir", cacheDir)
	}

	return &autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(domains...),
		Cache:      autocert.DirCache(cacheDir),
		Email:      srv.AutoCertEmail,
	}, nil
}

// httpHelperBindRetry is how long the HTTP helper waits between attempts to
// bind server.port. Var, not const, so tests can shorten it.
var httpHelperBindRetry = 30 * time.Second

// startHTTPHelper runs the plain-HTTP listener on server.port with the given
// handler (challenge answering + redirect, assembled by buildTLSCfg).
//
// A failed bind is retried forever rather than given up on. On a bare-metal
// cutover the old port-80 forwarder may still hold the port when the site
// starts; retrying means the operator can stop it in either order and the
// site takes the port over within one retry interval — no second restart.
// Losing this listener never takes HTTPS down, but it does silently break
// HTTP-01 renewals, so giving up quietly after one attempt was the worst
// option.
func startHTTPHelper(handler http.Handler) {
	srv := config.Options.Server
	tlsPort := srv.TLSPort
	if tlsPort == "" {
		tlsPort = "443"
	}
	port := srv.Port
	if port == "" || port == tlsPort {
		// Nothing sane to bind — TLS still works (ALPN challenges ride the
		// HTTPS listener), but HTTP-01 and the convenience redirect are off.
		logger.Info("TLS: skipping HTTP challenge/redirect listener",
			"reason", "server.port empty or same as tls_port")
		return
	}
	addr := ":" + port

	// Explicit timeouts: this port faces the open internet and only ever
	// serves tiny responses, so there is no reason to let a slow client hold
	// a connection (net/http's zero values mean "no timeout").
	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		// Log the first failure and the eventual recovery, not every retry,
		// so a long-held port doesn't flood the log every 30s.
		failing := false
		for {
			ln, err := net.Listen("tcp", addr)
			if err != nil {
				if !failing {
					logger.LogErr(serr.Wrap(err, "HTTP challenge/redirect listener could not bind; retrying",
						"port", port, "retry_every", httpHelperBindRetry.String()))
					failing = true
				}
				time.Sleep(httpHelperBindRetry)
				continue
			}
			if failing {
				logger.Info("HTTP challenge/redirect listener bound after retry", "port", port)
				failing = false
			}
			// Serve only returns on a listener failure; loop to rebind
			err = httpSrv.Serve(ln)
			logger.LogErr(serr.Wrap(err, "HTTP challenge/redirect listener stopped; rebinding", "port", port))
			time.Sleep(httpHelperBindRetry)
		}
	}()
}

// httpsRedirect 301s every request to the same host and URI on the HTTPS
// listener. This is the job the standalone HTTP→HTTPS forwarder used to do.
func httpsRedirect(tlsPort string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		// Strip any explicit port so we don't redirect to https://host:80
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
		target := "https://" + host
		if tlsPort != "443" {
			target += ":" + tlsPort
		}
		// r.URL.RequestURI(), not r.RequestURI: the raw request-target may be
		// in absolute form ("GET http://host/path", legal and sent by some
		// proxies), which would otherwise produce https://hosthttp://host/path
		http.Redirect(w, r, target+r.URL.RequestURI(), http.StatusMovedPermanently)
	})
}

const acmeChallengePath = "/.well-known/acme-challenge/"

// acmeWebrootHandler answers ACME HTTP-01 challenges from a certbot
// --webroot directory and hands every other request to next.
//
// certbot writes the key authorization to
// <webroot>/.well-known/acme-challenge/<token> just before asking Let's
// Encrypt to validate, and deletes it afterwards; we only read that one file.
// Deliberately not a general static file server: the token is checked
// against the ACME alphabet (base64url, RFC 8555 §8.3), which rules out
// "..", slashes and dotfiles, so nothing else under the webroot — or outside
// it — can be read through port 80. Anything that isn't a valid, present
// token is a plain 404, which is also what Let's Encrypt expects for a
// stale token.
func acmeWebrootHandler(webroot string, next http.Handler) http.Handler {
	challengeDir := filepath.Join(webroot, ".well-known", "acme-challenge")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, isChallenge := strings.CutPrefix(r.URL.Path, acmeChallengePath)
		if !isChallenge {
			next.ServeHTTP(w, r)
			return
		}
		if !validACMEToken(token) {
			http.NotFound(w, r)
			return
		}
		body, err := os.ReadFile(filepath.Join(challengeDir, token))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write(body)
	})
}

// validACMEToken reports whether token is a non-empty base64url string of
// sane length. Real tokens are ~43 chars; 256 is generous headroom.
func validACMEToken(token string) bool {
	if token == "" || len(token) > 256 {
		return false
	}
	for _, c := range token {
		switch {
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9', c == '-', c == '_':
		default:
			return false
		}
	}
	return true
}

// certReloader serves a cert/key pair from disk and transparently reloads it
// when the cert file changes, so certbot-style renewals take effect on the
// next handshake instead of the next deploy.
type certReloader struct {
	certFile, keyFile string

	mu      sync.RWMutex
	cert    *tls.Certificate
	modTime time.Time // cert file mtime at last successful load
}

func newCertReloader(certFile, keyFile string) (*certReloader, error) {
	if certFile == "" || keyFile == "" {
		return nil, serr.New("use_tls requires server.cert_file and server.key_file (or set auto_cert: true)")
	}
	r := &certReloader{certFile: certFile, keyFile: keyFile}
	// Fail startup on a bad pair — same contract as the old load-once path
	if err := r.reload(); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *certReloader) reload() error {
	// Load before locking so a slow disk never stalls in-flight handshakes
	cert, err := tls.LoadX509KeyPair(r.certFile, r.keyFile)
	if err != nil {
		return serr.Wrap(err, "loading TLS key pair")
	}
	fi, err := os.Stat(r.certFile)
	if err != nil {
		return serr.Wrap(err, "stat of cert file")
	}
	r.mu.Lock()
	r.cert = &cert
	r.modTime = fi.ModTime()
	r.mu.Unlock()
	return nil
}

// GetCertificate satisfies tls.Config.GetCertificate. The per-handshake cost
// is a single stat; handshakes are rare next to requests (keep-alive, session
// resumption), so polling here beats a background watcher goroutine for
// simplicity. On reload failure we keep serving the previous (still valid)
// cert rather than dropping handshakes.
func (r *certReloader) GetCertificate(_ *tls.ClientHelloInfo) (*tls.Certificate, error) {
	if fi, err := os.Stat(r.certFile); err == nil {
		r.mu.RLock()
		stale := !fi.ModTime().Equal(r.modTime)
		r.mu.RUnlock()
		if stale {
			if err := r.reload(); err != nil {
				logger.LogErr(err, "TLS cert reload failed; continuing with previous cert")
			} else {
				logger.Info("TLS certificate reloaded", "cert_file", r.certFile)
			}
		}
	}

	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.cert, nil
}

// certExpiryWarnWithin is when the daily check starts warning. certbot
// renews at 30 days left, so hitting 21 means renewals have been failing
// for over a week — early enough to fix by hand, late enough not to cry wolf.
const certExpiryWarnWithin = 21 * 24 * time.Hour

// watchExpiry checks the served cert immediately and then every interval,
// logging a warning when it expires within warnWithin.
//
// Unattended renewal is only "better" if its failure is loud: without this,
// a broken certbot timer or webroot shows up as browsers refusing the site
// on expiry day. Going through GetCertificate (not r.cert directly) also
// forces a reload check, so a renewal is picked up — and logged — even on
// a quiet site that saw no handshakes since certbot ran.
func (r *certReloader) watchExpiry(warnWithin, interval time.Duration) {
	go func() {
		for {
			r.checkExpiry(time.Now(), warnWithin)
			time.Sleep(interval)
		}
	}()
}

// checkExpiry logs the served cert's remaining validity and returns it.
// Split out from watchExpiry so tests can drive it with a fixed clock.
func (r *certReloader) checkExpiry(now time.Time, warnWithin time.Duration) (remaining time.Duration, err error) {
	cert, err := r.GetCertificate(nil)
	if err != nil || cert == nil || len(cert.Certificate) == 0 {
		return 0, serr.New("no TLS certificate loaded")
	}
	// LoadX509KeyPair fills Leaf (Go 1.23+), but parse as a fallback in case
	// that's disabled via GODEBUG x509keypairleaf=0.
	leaf := cert.Leaf
	if leaf == nil {
		if leaf, err = x509.ParseCertificate(cert.Certificate[0]); err != nil {
			return 0, serr.Wrap(err, "parsing served TLS certificate")
		}
	}

	remaining = leaf.NotAfter.Sub(now)
	days := strconv.Itoa(int(remaining.Hours() / 24))
	if remaining < warnWithin {
		logger.Warn("TLS certificate expires soon - automatic renewal may be failing (check `certbot renew --dry-run`)",
			"cert_file", r.certFile, "expires", leaf.NotAfter.Format(time.RFC3339), "days_left", days)
	} else {
		logger.Info("TLS certificate validity", "cert_file", r.certFile,
			"expires", leaf.NotAfter.Format(time.RFC3339), "days_left", days)
	}
	return remaining, nil
}
