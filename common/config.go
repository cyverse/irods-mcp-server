package common

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	yaml "gopkg.in/yaml.v2"

	"github.com/cockroachdb/errors"
	irods_config "github.com/cyverse/go-irodsclient/config"
	"github.com/kelseyhightower/envconfig"
	log "github.com/sirupsen/logrus"
)

const (
	DefaultServiceURL     string = "http://:8080"
	DefaultServiceLogPath string = "./irods-mcp-server.log"

	DefaultIRODSPort          int    = 1247
	DefaultIRODSSharedDirName string = "public"
)

// Config holds the parameters list which can be configured
type Config struct {
	Remote           bool   `yaml:"remote" json:"remote" envconfig:"IRODS_MCP_SVR_REMOTE"`
	ServiceURL       string `yaml:"service_url" json:"service_url" envconfig:"IRODS_MCP_SVR_SERVICE_URL"`
	PublicServiceURL string `yaml:"public_service_url,omitempty" json:"public_service_url,omitempty" envconfig:"IRODS_MCP_SVR_PUBLIC_SERVICE_URL"`
	Background       bool   `yaml:"background,omitempty" json:"background,omitempty" envconfig:"IRODS_MCP_SVR_BACKGROUND"`
	Debug            bool   `yaml:"debug" json:"debug" envconfig:"IRODS_MCP_SVR_DEBUG"`
	LogPath          string `yaml:"log_path,omitempty" json:"log_path,omitempty" envconfig:"IRODS_MCP_SVR_LOG_PATH"`

	// IRODS config
	irods_config.Config `yaml:",inline" json:",inline"`

	// Extra config
	IRODSProxyAuth     bool   `yaml:"irods_proxy_auth,omitempty" json:"irods_proxy_auth,omitempty" envconfig:"IRODS_MCP_SVR_IRODS_PROXY_AUTH"`
	IRODSSharedDirName string `yaml:"irods_shared_dir_name,omitempty" json:"irods_shared_dir_name,omitempty" envconfig:"IRODS_MCP_SVR_IRODS_SHARED_DIR_NAME"`
	IRODSWebDAVURL     string `yaml:"irods_webdav_url,omitempty" json:"irods_webdav_url,omitempty" envconfig:"IRODS_MCP_SVR_IRODS_WEBDAV_URL"`

	// OAuth2 / OIDC config
	OIDCDiscoveryURL   string `yaml:"oidc_discovery_url" json:"oidc_discovery_url" envconfig:"IRODS_MCP_SVR_OIDC_DISCOVERY_URL"`
	OAuth2ClientID     string `yaml:"oauth2_client_id" json:"oauth2_client_id" envconfig:"IRODS_MCP_SVR_OAUTH2_CLIENT_ID"`
	OAuth2ClientSecret string `yaml:"oauth2_client_secret" json:"oauth2_client_secret" envconfig:"IRODS_MCP_SVR_OAUTH2_CLIENT_SECRET"`
}

// NewDefaultConfig returns a default config
func NewDefaultConfig() *Config {
	config := &Config{
		Remote:           false,
		ServiceURL:       DefaultServiceURL, // use default
		PublicServiceURL: DefaultServiceURL, // use default
		Background:       false,
		Debug:            false,
		LogPath:          "", // use default

		Config: *irods_config.GetDefaultConfig(),

		IRODSProxyAuth:     false,                     // do not use proxy auth by default
		IRODSSharedDirName: DefaultIRODSSharedDirName, // use default
		IRODSWebDAVURL:     "",

		OIDCDiscoveryURL:   "",
		OAuth2ClientID:     "",
		OAuth2ClientSecret: "",
	}

	config.Config.Port = DefaultIRODSPort // use default
	config.Config.Username = "anonymous"  // use anonymous user by default

	return config
}

// NewConfigFromFile creates Config from file
func NewConfigFromFile(existingConfig *Config, filePath string) (*Config, error) {
	st, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.Wrapf(err, "file %q does not exist", filePath)
		}

		return nil, errors.Wrapf(err, "failed to stat file %q", filePath)
	}

	if st.IsDir() {
		return nil, errors.Errorf("path %q is a directory", filePath)
	}

	ext := filepath.Ext(filePath)
	if ext == ".yaml" || ext == ".yml" {
		return NewConfigFromYAMLFile(existingConfig, filePath)
	}

	return NewConfigFromJSONFile(existingConfig, filePath)
}

// NewConfigFromYAMLFile creates Config from YAML
func NewConfigFromYAMLFile(existingConfig *Config, yamlPath string) (*Config, error) {
	cfg := NewDefaultConfig()
	if existingConfig != nil {
		cfg = existingConfig
	}

	yamlBytes, err := os.ReadFile(yamlPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read YAML file %q", yamlPath)
	}

	err = yaml.Unmarshal(yamlBytes, &cfg)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal YAML file %q to config", yamlPath)
	}

	return cfg, nil
}

// NewConfigFromYAML creates Config from YAML
func NewConfigFromYAML(existingConfig *Config, yamlBytes []byte) (*Config, error) {
	cfg := NewDefaultConfig()
	if existingConfig != nil {
		cfg = existingConfig
	}

	err := yaml.Unmarshal(yamlBytes, cfg)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal yaml into config")
	}

	return cfg, nil
}

// NewConfigFromJSONFile creates Config from JSON
func NewConfigFromJSONFile(existingConfig *Config, jsonPath string) (*Config, error) {
	cfg := NewDefaultConfig()
	if existingConfig != nil {
		cfg = existingConfig
	}

	jsonBytes, err := os.ReadFile(jsonPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read YAML file %q", jsonPath)
	}

	err = json.Unmarshal(jsonBytes, cfg)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal JSON file %q to config", jsonPath)
	}

	return cfg, nil
}

// NewConfigFromJSON creates Config from JSON
func NewConfigFromJSON(existingConfig *Config, jsonBytes []byte) (*Config, error) {
	cfg := NewDefaultConfig()
	if existingConfig != nil {
		cfg = existingConfig
	}

	err := json.Unmarshal(jsonBytes, cfg)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal json into config")
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
		return nil, errors.Wrapf(err, "failed to read config from environmental variables")
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

func (config *Config) GetPublicServiceURL() string {
	if len(config.PublicServiceURL) > 0 {
		return config.PublicServiceURL
	}

	return config.GetServiceURL()
}

func (config *Config) IsOAuth2Enabled() bool {
	return len(config.OIDCDiscoveryURL) > 0 && len(config.OAuth2ClientID) > 0 && len(config.OAuth2ClientSecret) > 0
}

// MakeLogDir makes a log dir required
func (config *Config) MakeLogDir() error {
	logger := log.WithFields(log.Fields{})

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
		return errors.Errorf("failed to create a dir with empty path")
	}

	dirInfo, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			// make
			mkdirErr := os.MkdirAll(path, 0775)
			if mkdirErr != nil {
				return errors.Wrapf(mkdirErr, "making a dir %q error", path)
			}

			return nil
		}

		return errors.Wrapf(err, "stating a dir %q error", path)
	}

	if !dirInfo.IsDir() {
		return errors.Errorf("a file %q exist, not a directory", path)
	}

	dirPerm := dirInfo.Mode().Perm()
	if dirPerm&0200 != 0200 {
		return errors.Errorf("a dir %q exist, but does not have the write permission", path)
	}

	return nil
}

// Validate validates configuration
func (config *Config) Validate() error {
	if config.Remote {
		if !strings.HasPrefix(config.ServiceURL, "http://") && !strings.HasPrefix(config.ServiceURL, "https://") {
			return errors.Errorf("service URL must start with http:// or https://")
		}
	}

	if config.IRODSProxyAuth {
		if len(config.Config.Username) == 0 || len(config.Config.Password) == 0 {
			return errors.Errorf("user and password must be set when proxy auth is enabled")
		}
	}

	account := config.Config.ToIRODSAccount()
	err := account.Validate()
	if err != nil {
		return errors.Wrapf(err, "invalid iRODS account configuration")
	}

	return nil
}
