package config

import (
	"os"
	"strings"
)

// TODO ! Add more overrides here
func envOverride(envCfg *EnvConfig) *EnvConfig {
	if logLevel := strings.TrimSpace(os.Getenv("LOG_LEVEL")); len(logLevel) > 0 {
		envCfg.Log.Level = logLevel
	}
	if logFormat := strings.TrimSpace(os.Getenv("LOG_FORMAT")); len(logFormat) > 0 {
		envCfg.Log.Format = logFormat
	}
	// DB backend selection — env overrides let a k8s manifest flip a site
	// between bytdb and the Postgres fallback without editing options.yml.
	if dbType := strings.TrimSpace(os.Getenv("DB_TYPE")); len(dbType) > 0 {
		envCfg.DB.Type = dbType
	}
	if dbFile := strings.TrimSpace(os.Getenv("DB_FILE")); len(dbFile) > 0 {
		envCfg.DB.File = dbFile
	}
	if dbListen := strings.TrimSpace(os.Getenv("DB_LISTEN")); len(dbListen) > 0 {
		envCfg.DB.Listen = dbListen
	}
	if pgUser := strings.TrimSpace(os.Getenv("PG_USER")); len(pgUser) > 0 {
		envCfg.PG.User = pgUser
	}
	if pgWord := strings.TrimSpace(os.Getenv("PG_WORD")); len(pgWord) > 0 {
		envCfg.PG.Word = pgWord
	}
	// Bootstrap superadmin credentials — allows automated first-run setup
	if adminUser := strings.TrimSpace(os.Getenv("BOOTSTRAP_ADMIN_USER")); len(adminUser) > 0 {
		envCfg.Bootstrap.AdminUser = adminUser
	}
	if adminPass := strings.TrimSpace(os.Getenv("BOOTSTRAP_ADMIN_PASS")); len(adminPass) > 0 {
		envCfg.Bootstrap.AdminPass = adminPass
	}
	// Stripe API keys — env takes precedence over yaml so secrets can stay out
	// of config files entirely (yaml keys remain as a fallback for deployments
	// that prefer file-based config; both sites gitignore options.yml).
	if stripePub := strings.TrimSpace(os.Getenv("STRIPE_PUB_KEY")); len(stripePub) > 0 {
		envCfg.Stripe.PubKey = stripePub
	}
	if stripePriv := strings.TrimSpace(os.Getenv("STRIPE_PRIV_KEY")); len(stripePriv) > 0 {
		envCfg.Stripe.PrivKey = stripePriv
	}
	// Webhook signing secret (whsec_...). Env override matters for local dev in
	// particular: `stripe listen` mints a fresh secret per machine, which would
	// otherwise force editing options.yml just to smoke-test payments.
	if stripeWebhook := strings.TrimSpace(os.Getenv("STRIPE_WEBHOOK_SECRET")); len(stripeWebhook) > 0 {
		envCfg.Stripe.WebhookSecret = stripeWebhook
	}
	return envCfg
}
