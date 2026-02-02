package flag

import (
	"fmt"

	daemonizer "github.com/cyverse/go-daemonizer"
	"github.com/cyverse/irods-mcp-server/common"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type CommonFlagValues struct {
	ShowVersion bool
	ShowHelp    bool

	ConfigPath string
	Remote     bool
	Background bool
	Debug      bool
	LogPath    string
}

var (
	commonFlagValues CommonFlagValues
)

func SetCommonFlags(command *cobra.Command) {
	command.Flags().BoolVarP(&commonFlagValues.ShowVersion, "version", "v", false, "Display version information")
	command.Flags().BoolVarP(&commonFlagValues.ShowHelp, "help", "h", false, "Display help information about available commands and options")

	command.Flags().StringVarP(&commonFlagValues.ConfigPath, "config", "c", "", "Set config file (yaml)")
	command.Flags().BoolVarP(&commonFlagValues.Remote, "remote", "r", false, "Run MCP Server as a remote server with HTTP/SSE and Streamable-HTTP")
	command.Flags().BoolVarP(&commonFlagValues.Background, "background", "b", false, "Run in background mode")
	command.Flags().BoolVarP(&commonFlagValues.Debug, "debug", "d", false, "Enable debug mode")
	command.Flags().StringVar(&commonFlagValues.LogPath, "log_path", "", "Set log path")

	// daemonizer
	command.Flags().Bool(daemonizer.DaemonProcessArgumentName, false, "")
	command.Flags().MarkHidden(daemonizer.DaemonProcessArgumentName)
}

func ProcessCommonFlags(command *cobra.Command) (*common.Config, bool, error) {
	logger := log.WithFields(log.Fields{})

	if commonFlagValues.ShowHelp {
		PrintHelp(command)
		return nil, false, nil // stop here
	}

	if commonFlagValues.ShowVersion {
		PrintVersion(command)
		return nil, false, nil // stop here
	}

	// read from env by default
	config, err := common.NewConfigFromEnv(nil)
	if err != nil {
		logger.Errorf("%+v", err)
		return nil, false, err // stop here
	}

	if len(commonFlagValues.ConfigPath) > 0 {
		serviceConfig, err := common.NewConfigFromFile(config, commonFlagValues.ConfigPath)
		if err != nil {
			logger.Errorf("%+v", err)
			return nil, false, err // stop here
		}

		// overwrite config
		config = serviceConfig
	}

	if commonFlagValues.Remote {
		config.Remote = true
	}

	if commonFlagValues.Background {
		config.Background = true
	}

	if commonFlagValues.Debug {
		config.Debug = true
	}

	if len(commonFlagValues.LogPath) > 0 {
		config.LogPath = commonFlagValues.LogPath
	}

	err = config.Validate()
	if err != nil {
		return nil, false, err // stop here
	}

	if config.Debug {
		log.SetLevel(log.DebugLevel)
	}

	if len(config.GetLogFilePath()) > 0 {
		err := config.MakeLogDir()
		if err != nil {
			return nil, false, err // stop here
		}
	}

	return config, true, nil // continue
}

func PrintVersion(command *cobra.Command) error {
	info, err := common.GetVersionJSON()
	if err != nil {
		return err
	}

	fmt.Println(info)
	return nil
}

func PrintHelp(command *cobra.Command) error {
	return command.Usage()
}
