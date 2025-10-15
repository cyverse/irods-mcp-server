package common

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/xerrors"
	yaml "gopkg.in/yaml.v2"

	"github.com/kelseyhightower/envconfig"
	log "github.com/sirupsen/logrus"
)

const (
	DefaultServiceURL     string = "http://:8080"
	DefaultServiceLogPath string = "./irods-mcp-server.log"
)

// Config holds the parameters list which can be configured
type Config struct {
	Remote     bool   `yaml:"remote" json:"remote" envconfig:"IRODS_MCP_SVR_REMOTE"`
	ServiceURL string `yaml:"service_url" json:"service_url" envconfig:"IRODS_MCP_SVR_SERVICE_URL"`
	Background bool   `yaml:"background,omitempty" json:"background,omitempty" envconfig:"IRODS_MCP_SVR_BACKGROUND"`
	Debug      bool   `yaml:"debug" json:"debug" envconfig:"IRODS_MCP_SVR_DEBUG"`
	LogPath    string `yaml:"log_path,omitempty" json:"log_path,omitempty" envconfig:"IRODS_MCP_SVR_LOG_PATH"`
}

// NewDefaultConfig returns a default config
func NewDefaultConfig() *Config {
	return &Config{
		Remote:     false,
		ServiceURL: DefaultServiceURL, // use default
		Background: false,
		Debug:      false,
		LogPath:    "", // use default
	}
}

// NewConfigFromFile creates Config from file
func NewConfigFromFile(existingConfig *Config, filePath string) (*Config, error) {
	st, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, xerrors.Errorf("file %q does not exist: %w", filePath, err)
		}

		return nil, xerrors.Errorf("failed to stat file %q: %w", filePath, err)
	}

	if st.IsDir() {
		return nil, xerrors.Errorf("path %q is a directory: %w", filePath, err)
	}

	ext := filepath.Ext(filePath)
	if ext == ".yaml" || ext == ".yml" {
		return NewConfigFromYAMLFile(existingConfig, filePath)
	}

	return NewConfigFromJSONFile(existingConfig, filePath)
}

// NewConfigFromYAMLFile creates Config from YAML
func NewConfigFromYAMLFile(existingConfig *Config, yamlPath string) (*Config, error) {
	cfg := Config{}
	if existingConfig != nil {
		cfg = *existingConfig
	}

	yamlBytes, err := os.ReadFile(yamlPath)
	if err != nil {
		return nil, xerrors.Errorf("failed to read YAML file %q: %w", yamlPath, err)
	}

	err = yaml.Unmarshal(yamlBytes, &cfg)
	if err != nil {
		return nil, xerrors.Errorf("failed to unmarshal YAML file %q to config: %w", yamlPath, err)
	}

	return &cfg, nil
}

// NewConfigFromYAML creates Config from YAML
func NewConfigFromYAML(existingConfig *Config, yamlBytes []byte) (*Config, error) {
	cfg := NewDefaultConfig()
	if existingConfig != nil {
		cfg = existingConfig
	}

	err := yaml.Unmarshal(yamlBytes, cfg)
	if err != nil {
		return nil, xerrors.Errorf("failed to unmarshal yaml into config: %w", err)
	}

	return cfg, nil
}

// NewConfigFromJSONFile creates Config from JSON
func NewConfigFromJSONFile(existingConfig *Config, jsonPath string) (*Config, error) {
	cfg := Config{}
	if existingConfig != nil {
		cfg = *existingConfig
	}

	jsonBytes, err := os.ReadFile(jsonPath)
	if err != nil {
		return nil, xerrors.Errorf("failed to read YAML file %q: %w", jsonPath, err)
	}

	err = json.Unmarshal(jsonBytes, &cfg)
	if err != nil {
		return nil, xerrors.Errorf("failed to unmarshal JSON file %q to config: %w", jsonPath, err)
	}

	return &cfg, nil
}

// NewConfigFromJSON creates Config from JSON
func NewConfigFromJSON(existingConfig *Config, jsonBytes []byte) (*Config, error) {
	cfg := NewDefaultConfig()
	if existingConfig != nil {
		cfg = existingConfig
	}

	err := json.Unmarshal(jsonBytes, cfg)
	if err != nil {
		return nil, xerrors.Errorf("failed to unmarshal json into config: %w", err)
	}

	return cfg, nil
}

// NewConfigFromEnv creates Config from Environmental variables
func NewConfigFromEnv(existingConfig *Config) (*Config, error) {
	cfg := NewDefaultConfig()
	if existingConfig != nil {
		cfg = existingConfig
	}

	err := envconfig.Process("", cfg)
	if err != nil {
		return nil, xerrors.Errorf("failed to read config from environmental variables: %w", err)
	}

	return cfg, nil
}

// GetLogFilePath returns log file path
func (config *Config) GetLogFilePath() string {
	if config.Background {
		if len(config.LogPath) == 0 {
			config.LogPath = DefaultServiceLogPath // default log path for background
		}
	}

	return config.LogPath
}

func (config *Config) GetServiceURL() string {
	if len(config.ServiceURL) > 0 {
		return config.ServiceURL
	}

	return DefaultServiceURL
}

// MakeLogDir makes a log dir required
func (config *Config) MakeLogDir() error {
	logger := log.WithFields(log.Fields{
		"package":  "common",
		"object":   "Config",
		"function": "MakeLogDir",
	})

	logFilePath := config.GetLogFilePath()
	logDirPath := filepath.Dir(logFilePath)

	logger.Debugf("making log dir %q", logDirPath)
	err := config.makeDir(logDirPath)
	if err != nil {
		return err
	}

	return nil
}

// makeDir makes a dir for use
func (config *Config) makeDir(path string) error {
	if len(path) == 0 {
		return xerrors.Errorf("failed to create a dir with empty path")
	}

	dirInfo, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			// make
			mkdirErr := os.MkdirAll(path, 0775)
			if mkdirErr != nil {
				return xerrors.Errorf("making a dir %q error: %w", path, mkdirErr)
			}

			return nil
		}

		return xerrors.Errorf("stating a dir %q error: %w", path, err)
	}

	if !dirInfo.IsDir() {
		return xerrors.Errorf("a file %q exist, not a directory", path)
	}

	dirPerm := dirInfo.Mode().Perm()
	if dirPerm&0200 != 0200 {
		return xerrors.Errorf("a dir %q exist, but does not have the write permission", path)
	}

	return nil
}

// Validate validates configuration
func (config *Config) Validate() error {
	if config.Remote {
		if !strings.HasPrefix(config.ServiceURL, "http://") && !strings.HasPrefix(config.ServiceURL, "https://") {
			return xerrors.Errorf("service URL must start with http:// or https://")
		}
	}

	return nil
}
