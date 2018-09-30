// This pkg is imported by most all other packages in the project,
// Do not import any other project pkgs to avoid an import loop
// The Single Responsibility here is Configuration
package config

import (
	"os"
	"log"
	"strings"
	"io/ioutil"
	"gopkg.in/yaml.v2"
)

const (
	AdminPrefix = "/admin"
	APP_NAME    = "Church"
	configFile = "cfg/options.yml"
	IncomingDateTimeFormat = "2006-01-02 15:04 MST" // 2017-06-10 19:30 CST
	DisplayDateTimeFormat = "2006-01-02 15:04" // 3:04 PM"
	DisplayDateFormat = "2006-01-02"
	DisplayShortDateFormat = "1/2"
	DisplayTimeFormat = "15:04"
)

var AppEnv string
var Options *EnvConfig // Current Environment Configuration

var Version = "Version not available"
var GitCommitHash = "Unknown"
var BuildTimestamp = "Unknown"

// Poor man's enum :-)
var Environments = environments{ Development: "development", Test: "test", Production: "production" }
type environments struct{ Development, Test, Production string }

type ConfigAll struct {
	Development *EnvConfig `yaml:"development"`
	Test *EnvConfig `yaml:"test"`
	Production *EnvConfig `yaml:"production"`
}

// This maps an environment section of the yaml config
type EnvConfig struct {
	Theme string `yaml:"theme"`
	AppTimeout int64 `yaml:"app_timeout"`  // App max time in minutes
	Server struct {
		Port string `yaml:"port"`
	} `yaml:"server"`
	Log struct {
		Level string `yaml:"level"`
		Format string `yaml:"format"`
		InfoPath string `yaml:"info_path"`
		ErrorPath string `yaml:"error_path"`
	} `yaml:"log"`
	PG struct {
		Host string `yaml:"host"`
		Port string `yaml:"port"`
		User string `yaml:"user"`
		Word string `yaml:"word"`
		Database string `yaml:"database"`
	} `yaml:"pg"`
	PG2 struct {
		Host string `yaml:"host"`
		Port string `yaml:"port"`
		User string `yaml:"user"`
		Word string `yaml:"word"`
		Database string `yaml:"database"`
	} `yaml:"pg2"`
	FTP struct {
		Main   FTPConfig `yaml:"main"`
		Backup FTPConfig `yaml:"backup"`
	} `yaml:"ftp"`
	Redis struct {
		Host string `yaml:"host"`
		Port string `yaml:"port"`
	} `yaml:"redis"`
}

type FTPConfig struct {
	Enabled bool `yaml:"enabled"`
	Host string `yaml:"host"`
	Port string `yaml:"port"`
	User string `yaml:"user"`
	Word string `yaml:"word"`
	WebAccessPath string `yaml:"web_access_path"`
}

// Errors here are fatal bc we don't want to run on a bad configuration
func InitConfig(version, commitHash, buildStamp string) {
	var err error

	// Don't cache config here, since this function is normally only called on init()
	// Not caching allows us to be able to hot reload config in the future
	//if Options != nil { // return cached Options if already loaded
	//	return  // Options are already loaded
	//}

	AppEnv = "development"
	if env := strings.TrimSpace(os.Getenv("APP_ENV")); env != "" {
		AppEnv = env
	}
	log.Println("config.AppEnv is", AppEnv)

	// Load build variables
	Version = version
	GitCommitHash = commitHash
	BuildTimestamp = buildStamp

	config_data, err := loadConfigFile()
	if err != nil {
		log.Fatal("Error: ", err.Error())
	}
	env_cfg := getOptionsForEnvironment(config_data)
	env_cfg = envOverride(env_cfg)  // Override some settings with environment variables
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
	var config_all  = new(ConfigAll)

	file_data, err := ioutil.ReadFile(configFile)
	if err != nil {
		log.Fatal("error - Could not load configuration. ", err.Error(),	" - config_file: ", configFile,
			" tip - Are you in the project root?",
			" tip2 - Did you remember to copy 'cfg/options.yml.sample' to 'cfg/options.yml' ?")
		return config_all, err
	}
	err = yaml.Unmarshal(file_data, config_all)
	if err != nil {
		log.Fatal("Error unmarshalling yaml configuration file", err.Error(), "config_file", configFile,
			"tip - Double check that the contents of the config file is proper yaml (http://www.yamllint.com/)")
		return config_all, err
	}
	return config_all, err
}

