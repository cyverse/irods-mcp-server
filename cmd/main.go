package main

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/cockroachdb/errors"
	"github.com/cyverse/irods-mcp-server/cmd/flag"
	"github.com/cyverse/irods-mcp-server/common"
	irods "github.com/cyverse/irods-mcp-server/irods"
	"github.com/mark3labs/mcp-go/server"
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

			// set emtpy stdio, stdout, stderr
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

func startHTTPServer(svr *server.MCPServer, serviceUrl string, oauth2 *common.OAuth2) error {
	logger := log.WithFields(log.Fields{})

	logger.Info("starting MCP server in HTTP mode...")

	// fix url
	if !strings.HasPrefix(serviceUrl, "http://") && !strings.HasPrefix(serviceUrl, "https://") {
		serviceUrl = "http://" + serviceUrl
	}

	u, err := url.Parse(serviceUrl)
	if err != nil {
		return errors.Wrapf(err, "failed to parse service URL %q", serviceUrl)
	}

	logger.Infof("address: %s", u.String())

	sseServer := server.NewSSEServer(svr,
		server.WithBaseURL(u.String()),
		server.WithSSEContextFunc(common.AuthForHTTP),
	)

	sseEndpoint := strings.TrimRight(u.Path, "/") + "/sse"
	sseMessageEndpoint := strings.TrimRight(u.Path, "/") + "/message"

	logger.Infof("SSE endpoint: %s", sseEndpoint)
	logger.Infof("SSE message endpoint: %s", sseMessageEndpoint)

	streamableHttpEndpoint := strings.TrimRight(u.Path, "/") + "/mcp"

	streamableHttpServer := server.NewStreamableHTTPServer(svr,
		server.WithEndpointPath(streamableHttpEndpoint),
		server.WithHTTPContextFunc(common.AuthForHTTP),
	)

	logger.Infof("Streamable-HTTP endpoint: %s", streamableHttpEndpoint)

	// do not print out logs to the terminal (stdout)
	common.SetTerminalOutput(os.Stderr)

	mux := http.NewServeMux()

	if oauth2 != nil {
		mux.HandleFunc(streamableHttpEndpoint, oauth2.RequireOAuth(streamableHttpServer))

		// OAuth2
		mux.HandleFunc(strings.TrimRight(u.Path, "/")+"/.well-known/oauth-protected-resource", oauth2.HandleResourceMetadataURI)
		mux.HandleFunc(strings.TrimRight(u.Path, "/")+"/.well-known/oauth-protected-resource/mcp", oauth2.HandleResourceMetadataURI)
		mux.HandleFunc(strings.TrimRight(u.Path, "/")+"/.well-known/oauth-authorization-server", oauth2.HandleAuthServerMetadataURI)
		mux.HandleFunc(strings.TrimRight(u.Path, "/")+"/.well-known/oauth-authorization-server/mcp", oauth2.HandleAuthServerMetadataURI)
		mux.HandleFunc(strings.TrimRight(u.Path, "/")+"/.well-known/openid-configuration", oauth2.HandleOIDCDiscoveryURI)
		mux.HandleFunc(strings.TrimRight(u.Path, "/")+"/.well-known/openid-configuration/mcp", oauth2.HandleOIDCDiscoveryURI)
	} else {
		mux.Handle(streamableHttpEndpoint, streamableHttpServer)
	}
	mux.Handle(sseEndpoint, sseServer)
	mux.Handle(sseMessageEndpoint, sseServer)

	httpServer := &http.Server{
		Addr:    u.Host,
		Handler: mux,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		<-sigChan
		httpServer.Shutdown(ctx)
	}()

	err = httpServer.ListenAndServe()
	if err != nil {
		if err == http.ErrServerClosed {
			logger.Info("HTTP server closed")
		} else {
			return errors.Wrapf(err, "failed to start HTTP server %q", serviceUrl)
		}
	}

	logger.Info("terminating MCP server in HTTP mode...")
	return nil

}

func startSTDIOServer(svr *server.MCPServer) error {
	logger := log.WithFields(log.Fields{})

	logger.Info("starting MCP server in STDIO mode...")

	// do not print out logs to the terminal (stdout)
	common.SetTerminalOutput(os.Stderr)

	// Start the stdio server
	if err := server.ServeStdio(svr, server.WithStdioContextFunc(common.AuthForStdio)); err != nil {
		if !strings.Contains(err.Error(), "context canceled") {
			return errors.Wrapf(err, "failed to start STDIO server")
		}
	}

	logger.Info("terminating MCP server in STDIO mode...")
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
	svr := server.NewMCPServer(
		"iRODS MCP Server",
		common.GetServerVersionWithoutV(),
		server.WithResourceCapabilities(true, true),
		server.WithToolCapabilities(true),
	)

	// Initialize the iRODS service
	_, err = irods.NewIRODSMCPServer(svr, config)
	if err != nil {
		return errors.Wrapf(err, "failed to initialize irods service")
	}
	var oauth2 *common.OAuth2
	if config.IsOAuth2Enabled() {
		oauth2, err = common.NewOAuth2(config.GetPublicServiceURL()+"/mcp", config.OIDCDiscoveryURL, config.OAuth2ClientID, config.OAuth2ClientSecret)
		if err != nil {
			return errors.Wrapf(err, "failed to initialize OAuth2")
		}
	}

	if config.Remote {
		err = startHTTPServer(svr, config.GetServiceURL(), oauth2)
		if err != nil {
			return errors.Wrapf(err, "failed to start HTTP server %q", config.GetServiceURL())
		}
	} else {
		err = startSTDIOServer(svr)
		if err != nil {
			return errors.Wrapf(err, "failed to start STDIO server")
		}
	}

	return nil
}
