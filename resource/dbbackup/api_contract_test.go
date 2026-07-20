package dbbackup

// Contract tests for the backup trigger endpoint. These freeze the gate
// ordering the k8s CronJob and the manifests depend on: 503 when the
// destination isn't configured (misconfigured deploys fail loudly), 401 on
// missing/bad bearer token, and 503 when the active backend isn't bytdb.
// The happy path (snapshot + upload) needs a live engine and an object
// store, so it is exercised by the deploy runbook, not unit tests — but
// every response here must still be JSON (the CronJob's curl logs land in
// `kubectl logs`, and the uniform {"error": ...} shape keeps them greppable).

import (
	"net/http"
	"testing"

	"github.com/rohanthewiz/church/config"
	"github.com/rohanthewiz/church/resource/apiv1/apitest"
	"github.com/rohanthewiz/rweb"
)

const backupPath = "/api/admin/db/backup"

// Route registered exactly as in router_rweb.go so the path is part of the test.
func newBackupServer() *rweb.Server {
	s := apitest.NewServer()
	s.Post(backupPath, APIBackupRWeb)
	return s
}

// setBackupConfig installs a fully-specified backup config (with the given
// trigger token) into the process-global config.Options, restoring the prior
// value on cleanup. Global state — tests in this package must not t.Parallel().
func setBackupConfig(t *testing.T, token string) {
	t.Helper()
	prev := config.Options
	cfg := &config.EnvConfig{}
	cfg.Backup.Endpoint = "us-east-1.example.com"
	cfg.Backup.Bucket = "test-backups"
	cfg.Backup.AccessKey = "test-access"
	cfg.Backup.SecretKey = "test-secret"
	cfg.Backup.Prefix = "testsite"
	cfg.Backup.Token = token
	config.Options = cfg
	t.Cleanup(func() { config.Options = prev })
}

func bearer(token string) []rweb.Header {
	return []rweb.Header{{Key: "Authorization", Value: "Bearer " + token}}
}

func TestBackupUnconfigured(t *testing.T) {
	s := newBackupServer()

	// Both unconfigured shapes must answer 503, not 401: a CronJob pointed at
	// a site that never got its backup secret should scream "not configured".
	t.Run("nil options", func(t *testing.T) {
		prev := config.Options
		config.Options = nil
		t.Cleanup(func() { config.Options = prev })
		status, doc := apitest.RequestJSON(t, s, "POST", backupPath, bearer("whatever"), "")
		if status != http.StatusServiceUnavailable {
			t.Fatalf("want 503 with nil config, got %d (%v)", status, doc)
		}
	})

	t.Run("empty token", func(t *testing.T) {
		setBackupConfig(t, "") // destination present, trigger token absent
		status, doc := apitest.RequestJSON(t, s, "POST", backupPath, bearer("whatever"), "")
		if status != http.StatusServiceUnavailable {
			t.Fatalf("want 503 with empty token, got %d (%v)", status, doc)
		}
		if doc["error"] == "" {
			t.Fatal("error responses must carry the uniform {\"error\": ...} shape")
		}
	})
}

func TestBackupAuth(t *testing.T) {
	s := newBackupServer()
	setBackupConfig(t, "correct-horse-battery-staple")

	cases := []struct {
		name    string
		headers []rweb.Header
	}{
		{"no authorization header", nil},
		{"wrong scheme", []rweb.Header{{Key: "Authorization", Value: "Basic dXNlcjpwYXNz"}}},
		{"wrong token", bearer("wrong-token")},
		{"empty bearer", []rweb.Header{{Key: "Authorization", Value: "Bearer "}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, doc := apitest.RequestJSON(t, s, "POST", backupPath, tc.headers, "")
			if status != http.StatusUnauthorized {
				t.Fatalf("want 401, got %d (%v)", status, doc)
			}
			if doc["error"] == "" {
				t.Fatal("401 must carry the uniform {\"error\": ...} shape")
			}
		})
	}
}

func TestBackupRequiresBytdbBackend(t *testing.T) {
	s := newBackupServer()
	setBackupConfig(t, "correct-horse-battery-staple")

	// No embedded engine runs in the test process (db.BytDBWireAddr() is
	// empty), which is exactly the Postgres-fallback shape in production —
	// a correctly authenticated trigger must answer 503, never 500.
	status, doc := apitest.RequestJSON(t, s, "POST", backupPath,
		bearer("correct-horse-battery-staple"), "")
	if status != http.StatusServiceUnavailable {
		t.Fatalf("want 503 on non-bytdb backend, got %d (%v)", status, doc)
	}
}
