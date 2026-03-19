package main

import (
	"os"

	"github.com/cockroachdb/errors"
	"github.com/cyverse/irods-mcp-server/cmd/flag"
	"github.com/cyverse/irods-mcp-server/common"
	irods "github.com/cyverse/irods-mcp-server/irods"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	daemonizer "github.com/cyverse/go-daemonizer"
)

var rootCmd = &cobra.Command{
	Use:          "irods-mcp-server [flags]",
	Short:        "iRODS MCP Server: A MCP Server for iRODS",
	Long:         `iRODS MCP Server is an agent that connects AI clients with iRODS Data Storage.`,
	RunE:         processCommand,
	SilenceUsage: true,
	//SilenceErrors: true,
	CompletionOptions: cobra.CompletionOptions{
		DisableDefaultCmd:   true,
		DisableNoDescFlag:   true,
		DisableDescriptions: true,
		HiddenDefaultCmd:    true,
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func processCommand(command *cobra.Command, args []string) error {
	config, cont, err := flag.ProcessCommonFlags(command)
	if err != nil {
		return err
	}

	if !cont {
		return nil
	}

	if config.Background {
		// run as a daemon
		d, err := daemonizer.NewDaemonizer()
		if err != nil {
			return errors.Wrapf(err, "failed to create daemonizer")
		}

		if !d.IsDaemon() {
			// parent process
			// daemonize the process

			params := map[string]interface{}{}
			option := daemonizer.DaemonizeOption{}

			// set empty stdio, stdout, stderr
			//option.UseNullIO()

			// pass the echo server config to the daemon process
			err = d.Daemonize(params, option)
			if err != nil {
				return errors.Wrapf(err, "failed to daemonize the process")
			}

			// exit the parent process
			// daemon process will continue to run
			common.Println("Daemonized the process")
			return nil
		}
	}

	logWriter := common.InitLogWriter(config)
	defer logWriter.Close()

	err = run(config)
	if err != nil {
		return err
	}

	return nil
}

func main() {
	common.InitTerminalOutput()

	log.SetFormatter(&log.TextFormatter{
		TimestampFormat: "2006-01-02 15:04:05.000",
		FullTimestamp:   true,
	})

	log.SetReportCaller(true)

	log.SetLevel(log.InfoLevel)
	log.SetOutput(common.GetTerminalWriter())

	logger := log.WithFields(log.Fields{})

	// attach common flags
	flag.SetCommonFlags(rootCmd)

	err := Execute()
	if err != nil {
		logger.Errorf("%+v", err)
		os.Exit(1)
	}
}

// run runs service
func run(config *common.Config) error {
	logger := log.WithFields(log.Fields{})

	versionInfo := common.GetVersion()
	logger.Infof("iRODS MCP Server version - %q, commit - %q", versionInfo.ServerVersion, versionInfo.GitCommit)

	err := config.Validate()
	if err != nil {
		return errors.Wrapf(err, "invalid configuration")
	}

	// Initialize the MCP server
	mcpImpl := mcp.Implementation{
		Name:    "iRODS MCP Server",
		Version: common.GetServerVersion(),
	}
	mcpServerOptions := mcp.ServerOptions{}
	mcpServer := mcp.NewServer(&mcpImpl, &mcpServerOptions)

	// Initialize the iRODS service
	irodsMcpServer, err := irods.NewIRODSMCPServer(mcpServer, config)
	if err != nil {
		return errors.Wrapf(err, "failed to initialize irods mcp server")
	}

	err = irodsMcpServer.Start()
	if err != nil {
		return errors.Wrapf(err, "failed to start irods mcp server")
	}

	return nil
}
