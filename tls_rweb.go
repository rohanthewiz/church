package church

// TLS setup for ServeRWeb. Two modes, selected by server.auto_cert:
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
//
// Both modes hand rweb a *tls.Config through TLSCfg.Config (the dynamic-cert
// hook added to rweb for exactly this), and both start a plain-HTTP listener
// on server.port that redirects to HTTPS.
//
//	              :port (HTTP)                :tls_port (HTTPS)
//	   ┌─────────────────────────────┐   ┌───────────────────────────┐
//	   │ ACME HTTP-01 (autocert)     │   │ rweb server               │
//	   │ everything else → redirect ─┼──▶│ TLS via GetCertificate:   │
//	   └─────────────────────────────┘   │  autocert or certReloader │
//	                                     └───────────────────────────┘

import (
	"crypto/tls"
	"net"
	"net/http"
	"os"
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
		// HTTPHandler answers HTTP-01 challenges; non-challenge traffic gets
		// its default redirect-to-HTTPS behavior (nil fallback).
		startHTTPHelper(mgr.HTTPHandler(nil), tlsPort)
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
		startHTTPHelper(nil, tlsPort)
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

// startHTTPHelper runs the plain-HTTP listener on server.port. In autocert
// mode the handler serves ACME HTTP-01 challenges first; otherwise (or for
// non-challenge paths) traffic is redirected to the HTTPS listener.
func startHTTPHelper(handler http.Handler, tlsPort string) {
	port := config.Options.Server.Port
	if port == "" || port == tlsPort {
		// Nothing sane to bind — TLS still works (ALPN challenges ride the
		// HTTPS listener), but HTTP-01 and the convenience redirect are off.
		logger.Info("TLS: skipping HTTP challenge/redirect listener",
			"reason", "server.port empty or same as tls_port")
		return
	}

	if handler == nil {
		handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			host := r.Host
			// Strip any explicit port so we don't redirect to https://host:80
			if h, _, err := net.SplitHostPort(host); err == nil {
				host = h
			}
			target := "https://" + host
			if tlsPort != "443" {
				target += ":" + tlsPort
			}
			http.Redirect(w, r, target+r.RequestURI, http.StatusMovedPermanently)
		})
	}

	go func() {
		if err := http.ListenAndServe(":"+port, handler); err != nil {
			// Non-fatal: HTTPS keeps serving; only the redirect/HTTP-01 path is lost
			logger.LogErr(serr.Wrap(err, "HTTP challenge/redirect listener failed", "port", port))
		}
	}()
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
