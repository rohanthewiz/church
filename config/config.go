// This pkg is imported by most all other packages in the project,
// Do not import any other project pkgs to avoid an import loop
// The Single Responsibility here is Configuration
package config

import (
	"log"
	"os"
	"strings"

	"gopkg.in/yaml.v2"
)

const (
	AdminPrefix            = "/admin"
	APP_NAME               = "Church"
	configFile             = "cfg/options.yml"
	IncomingDateTimeFormat = "2006-01-02 15:04 MST" // 2017-06-10 19:30 CST
	DisplayDateTimeFormat  = "2006-01-02 15:04"     // 3:04 PM"
	DisplayDateFormat      = "2006-01-02"

	PresenterDateFormat   = "2006-01-02"
	DisplayDateFormatLong = "01/02/2006"

	DisplayShortDateFormat = "1/2"
	DisplayTimeFormat      = "15:04"
)

var AppEnv string
var Options *EnvConfig // Current Environment Configuration

var Version = "Version not available"
var GitCommitHash = "Unknown"
var BuildTimestamp = "Unknown"

// Poor man's enum :-)
var Environments = environments{Development: "development", Test: "test", Production: "production"}

type environments struct{ Development, Test, Production string }

type ConfigAll struct {
	Development *EnvConfig `yaml:"development"`
	Test        *EnvConfig `yaml:"test"`
	Production  *EnvConfig `yaml:"production"`
}

// This maps an environment section of the yaml config
type EnvConfig struct {
	Theme           string `yaml:"theme"`
	BannerInnerHTML string `yaml:"banner_inner_html"`
	BannerExt       string `yaml:"banner_ext"`
	CopyrightOwner  string `yaml:"copyright_owner"`
	AppTimeout      int64  `yaml:"app_timeout"` // App max time in minutes
	Server          struct {
		Domain   string `yaml:"domain"`
		Port     string `yaml:"port"`
		UseTLS   bool   `yaml:"use_tls"`
		CertFile string `yaml:"cert_file"`
		KeyFile  string `yaml:"key_file"`
		// TLSPort is where the HTTPS listener binds when use_tls is true
		// (default "443"). `port` then carries the plain-HTTP listener that
		// answers ACME HTTP-01 challenges and redirects everything else to
		// HTTPS — so in production use port: "80" alongside use_tls.
		TLSPort string `yaml:"tls_port"`
		// AutoCert switches TLS to fully in-process Let's Encrypt: certs are
		// issued and renewed automatically via ACME (autocert), no certbot or
		// cron needed. When false but use_tls is true, cert_file/key_file are
		// served with hot reload, so an external renewer (e.g. certbot) only
		// has to replace the files — no restart.
		AutoCert bool `yaml:"auto_cert"`
		// Domains autocert may respond for (Let's Encrypt hard-requires a
		// public DNS name). Falls back to [domain] when empty.
		AutoCertDomains []string `yaml:"auto_cert_domains"`
		// Optional contact for Let's Encrypt expiry/problem notices
		AutoCertEmail string `yaml:"auto_cert_email"`
		// Where autocert persists issued certs across restarts
		// (default "certs/autocert"). Keep this out of version control.
		AutoCertCacheDir string `yaml:"auto_cert_cache_dir"`
	} `yaml:"server"`
	Log struct {
		Level     string `yaml:"level"`
		Format    string `yaml:"format"`
		InfoPath  string `yaml:"info_path"`
		ErrorPath string `yaml:"error_path"`
		// SlackAPICfg logger.SlackAPICfg `yaml:"slack_api_cfg"`
	} `yaml:"log"`
	// DB selects the storage backend. bytdb (embedded, in-process, served to the
	// app over a loopback Postgres wire connection) is the default so each site
	// runs as a single self-contained binary — the target deployment is one pod
	// per site with the data file on a block-storage volume. Postgres remains a
	// fallback for deployments that already run it; set type: postgres and fill
	// the pg: block below.
	DB struct {
		Type string `yaml:"type"` // "bytdb" (default when empty) or "postgres"
		// File is the bytdb data file (WAL-backed single file). Default "data/church.db".
		// Must live on a real filesystem (block storage in k8s) — never object storage,
		// which cannot honor the WAL's fsync-before-ack durability contract.
		File string `yaml:"file"`
		// Listen is the loopback address pgwire serves on. Default "127.0.0.1:0"
		// (ephemeral port) so multiple sites coexist on one host without clashing;
		// pin a port (e.g. "127.0.0.1:5433") to inspect the live DB with psql.
		Listen string `yaml:"listen"`
	} `yaml:"db"`
	PG struct {
		Host     string `yaml:"host"`
		Port     string `yaml:"port"`
		User     string `yaml:"user"`
		Word     string `yaml:"word"`
		Database string `yaml:"database"`
	} `yaml:"pg"`
	PG2 struct {
		Host     string `yaml:"host"`
		Port     string `yaml:"port"`
		User     string `yaml:"user"`
		Word     string `yaml:"word"`
		Database string `yaml:"database"`
	} `yaml:"pg2"`
	FTP struct {
		Main   FTPConfig `yaml:"main"`
		Backup FTPConfig `yaml:"backup"`
	} `yaml:"ftp"`
	// Redis was retired when sessions/tokens moved to the in-process core/kvstore.
	// Any `redis:` block in options.yml is simply ignored now. Kept for reference
	// until a durable external store (for mobile tokens / multi-instance) is chosen.
	// Redis struct {
	// 	Host string `yaml:"host"`
	// 	Port string `yaml:"port"`
	// } `yaml:"redis"`
	IDrive struct {
		Enabled         bool   `yaml:"enabled"`
		EndPoint        string `yaml:"end_point"`
		Region          string `yaml:"region"`
		Bucket          string `yaml:"bucket"`
		AccessKey       string `yaml:"access_key"`
		SecretKey       string `yaml:"secret_key"`
		LocalSermonsDir string `yaml:"local_sermons_dir"`
		// AutoCleanup gates the background LRU eviction loop. Off unless explicitly
		// set true. The admin Sermon Cleanup tool is unaffected by this flag.
		AutoCleanup bool `yaml:"auto_cleanup"`
		// Local sermon cache eviction tuning. Both are Go duration strings
		// (e.g. "1h", "4h", "30m"). Empty/invalid values fall back to defaults
		// in core/idrive (1h scan interval, 4h idle TTL).
		CacheCleanupInterval string `yaml:"cache_cleanup_interval"` // how often the eviction scan runs
		CacheIdleTTL         string `yaml:"cache_idle_ttl"`         // idle window before a cached copy is eligible for eviction
	} `yaml:"idrive"`
	Stripe struct {
		PubKey  string `yaml:"pub_key"`
		PrivKey string `yaml:"priv_key"`
		// TxDescription labels each charge in the Stripe dashboard and on receipts.
		// Configurable per site because this framework serves multiple churches --
		// a hardcoded description would brand every site's donations with the wrong name.
		// When empty, payment_controller falls back to CopyrightOwner + " Donation".
		TxDescription string `yaml:"tx_description"`
		// WebhookSecret is the signing secret (whsec_...) for this site's Stripe
		// webhook endpoint (dashboard -> Developers -> Webhooks). Used to verify
		// the Stripe-Signature header on /webhooks/stripe. Leave empty to
		// effectively disable webhook processing (the endpoint answers 503).
		WebhookSecret string `yaml:"webhook_secret"`
	} `yaml:"stripe"`
	Bootstrap struct {
		AdminUser string `yaml:"admin_user"` // Superadmin username for auto-bootstrap
		AdminPass string `yaml:"admin_pass"` // Superadmin password for auto-bootstrap
	} `yaml:"bootstrap"`
	GivingContacts []string `yaml:"giving_contacts"` // typically used on the Giving form
	Gmail          struct {
		Account  string   `yaml:"account"`
		FromName string   `yaml:"from"`
		Word     string   `yaml:"word"`
		BCCs     []string `yaml:"bcc"`
	} `yaml:"gmail"`
}

type FTPConfig struct {
	Enabled       bool   `yaml:"enabled"` // Legacy FTP upload (deprecated - use IDrive.Enabled instead)
	Host          string `yaml:"host"`
	Port          string `yaml:"port"`
	User          string `yaml:"user"`
	Word          string `yaml:"word"`
	WebAccessPath string `yaml:"web_access_path"`
}

// InitConfig is called externally
// Errors here are fatal bc we don't want to run on a bad configuration
func InitConfig(version, commitHash, buildStamp string) {
	var err error

	// Don't cache config here, since this function is normally only called on init()
	// Not caching allows us to be able to hot reload config in the future
	// if Options != nil { // return cached Options if already loaded
	//	return  // Options are already loaded
	// }

	AppEnv = "development"
	if env := strings.TrimSpace(os.Getenv("APP_ENV")); env != "" {
		AppEnv = env
	}
	log.Println("config.AppEnv is", AppEnv)

	// Load build variables
	Version = version
	GitCommitHash = commitHash
	BuildTimestamp = buildStamp

	configData, err := loadConfigFile()
	if err != nil {
		log.Fatal("Error: ", err.Error())
	}
	env_cfg := getOptionsForEnvironment(configData)
	env_cfg = envOverride(env_cfg) // Override some settings with environment variables
	Options = env_cfg
}

// cfg holds the unmarshalled data of our Options.yml file
func getOptionsForEnvironment(cfg *ConfigAll) *EnvConfig {
	switch strings.ToLower(AppEnv) {
	case "development":
		if cfg.Development == nil {
			log.Fatal(`"development" section not found in config file: ` + configFile)
		}
		return cfg.Development
	case "test":
		if cfg.Test == nil {
			log.Fatal(`"test" section not found in config file: ` + configFile)
		}
		return cfg.Test
	case "production":
		if cfg.Production == nil {
			log.Fatal(`"production" section not found in config file: ` + configFile)
		}
		return cfg.Production
	default:
		log.Fatal("Error - Unknown environment", "configFile:", configFile,
			"- tip:", "Environments must be one of 'development', 'test', 'production'")
	}
	return nil
}

func loadConfigFile() (*ConfigAll, error) {
	var cfgAll = new(ConfigAll)

	fileData, err := os.ReadFile(configFile)
	if err != nil {
		log.Fatal("error - Could not load configuration. ", err.Error(), " - config_file: ", configFile,
			" tip - Are you in the project root?",
			" tip2 - Did you remember to copy 'cfg/options-sample.yml' to 'cfg/options.yml' ?")
		return cfgAll, err
	}
	err = yaml.Unmarshal(fileData, cfgAll)
	if err != nil {
		log.Fatal("Error unmarshalling yaml configuration file", err.Error(), "config_file", configFile,
			"tip - Double check that the contents of the config file is proper yaml (http://www.yamllint.com/)")
		return cfgAll, err
	}
	return cfgAll, err
}
