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
	if pgUser := strings.TrimSpace(os.Getenv("PG_USER")); len(pgUser) > 0 {
		envCfg.PG.User = pgUser
	}
	if pgWord := strings.TrimSpace(os.Getenv("PG_WORD")); len(pgWord) > 0 {
		envCfg.PG.Word = pgWord
	}
	return envCfg
}
