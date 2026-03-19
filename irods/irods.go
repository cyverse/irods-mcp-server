package irods

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/cockroachdb/errors"
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	"github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/irods-mcp-server/common"
	irods_common "github.com/cyverse/irods-mcp-server/irods/common"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	log "github.com/sirupsen/logrus"
)

type IRODSMCPServer struct {
	config            *common.Config
	mcpServer         *mcp.Server
	irodsfsClientPool *irods_common.IRODSFSClientPool
	resourceTemplates []ResourceTemplateAPI
	tools             []ToolAPI
}

func NewIRODSMCPServer(svr *mcp.Server, config *common.Config) (*IRODSMCPServer, error) {
	s := &IRODSMCPServer{
		config:            config,
		mcpServer:         svr,
		irodsfsClientPool: irods_common.NewIRODSFSClientPool(),
		resourceTemplates: []ResourceTemplateAPI{},
		tools:             []ToolAPI{},
	}

	err := s.registerResourceTemplates()
	if err != nil {
		return nil, err
	}

	err = s.registerTools()
	if err != nil {
		return nil, err
	}

	return s, nil
}

func (svr *IRODSMCPServer) GetConfig() *common.Config {
	return svr.config
}

func (svr *IRODSMCPServer) Start() error {
	if svr.config.Remote {
		err := svr.startHTTPServer()
		if err != nil {
			return errors.Wrapf(err, "failed to start HTTP server")
		}
	} else {
		err := svr.startSTDIOServer()
		if err != nil {
			return errors.Wrapf(err, "failed to start STDIO server")
		}
	}
	return nil
}

func (svr *IRODSMCPServer) startSTDIOServer() error {
	logger := log.WithFields(log.Fields{})

	logger.Info("starting MCP server in STDIO mode...")

	// do not print out logs to the terminal (stdout)
	common.SetTerminalOutput(os.Stderr)

	svr.mcpServer.AddReceivingMiddleware(svr.getAuthMiddleWare())

	// Start the stdio server
	if err := svr.mcpServer.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		return errors.Wrapf(err, "failed to start STDIO server")
	}

	logger.Info("terminating MCP server in STDIO mode...")
	return nil
}

func (svr *IRODSMCPServer) startHTTPServer() error {
	serviceURL := svr.config.GetServiceURL()

	logger := log.WithFields(log.Fields{
		"service_url": serviceURL,
	})

	logger.Info("starting MCP server in HTTP mode...")

	// fix url
	if !strings.HasPrefix(serviceURL, "http://") && !strings.HasPrefix(serviceURL, "https://") {
		serviceURL = "http://" + serviceURL
	}

	u, err := url.Parse(serviceURL)
	if err != nil {
		return errors.Wrapf(err, "failed to parse service URL %q", serviceURL)
	}

	serviceURL = u.String()
	logger.Infof("address: %s", serviceURL)

	sseEndpoint := strings.TrimRight(u.Path, "/") + "/sse"
	streamableHttpEndpoint := strings.TrimRight(u.Path, "/") + "/mcp"
	healthCheckEndpoint := strings.TrimRight(u.Path, "/") + "/health"

	logger.Infof("SSE endpoint: %s", sseEndpoint)
	logger.Infof("Streamable-HTTP endpoint: %s", streamableHttpEndpoint)
	logger.Infof("Health check endpoint: %s", healthCheckEndpoint)

	mcpFunc := func(request *http.Request) *mcp.Server {
		return svr.mcpServer
	}

	sseOptions := mcp.SSEOptions{}
	sseHandler := mcp.NewSSEHandler(mcpFunc, &sseOptions)

	shttpOptions := mcp.StreamableHTTPOptions{
		Stateless: false,
	}
	shttpHandler := mcp.NewStreamableHTTPHandler(mcpFunc, &shttpOptions)

	healthCheckHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status": "healthy",
			"time":   time.Now().Format(time.RFC3339),
		})
	}

	svr.mcpServer.AddReceivingMiddleware(svr.getAuthMiddleWare())

	// do not print out logs to the terminal (stdout)
	common.SetTerminalOutput(os.Stderr)

	mux := http.NewServeMux()

	// oauth2
	if svr.config.IsOAuth2Enabled() {
		oauth2, err := common.NewOAuth2(svr.config.GetPublicServiceURL()+"/mcp", svr.config.OIDCDiscoveryURL, svr.config.OAuth2ClientID, svr.config.OAuth2ClientSecret)
		if err != nil {
			return errors.Wrapf(err, "failed to initialize OAuth2")
		}

		wellknownEndpoint := strings.TrimRight(u.Path, "/") + "/.well-known"

		mux.HandleFunc(wellknownEndpoint+"/oauth-protected-resource", oauth2.HandleResourceMetadataURI)
		mux.HandleFunc(wellknownEndpoint+"/oauth-protected-resource/mcp", oauth2.HandleResourceMetadataURI)
		mux.HandleFunc(wellknownEndpoint+"/oauth-authorization-server", oauth2.HandleAuthServerMetadataURI)
		mux.HandleFunc(wellknownEndpoint+"/oauth-authorization-server/mcp", oauth2.HandleAuthServerMetadataURI)
		mux.HandleFunc(wellknownEndpoint+"/openid-configuration", oauth2.HandleOIDCDiscoveryURI)
		mux.HandleFunc(wellknownEndpoint+"/openid-configuration/mcp", oauth2.HandleOIDCDiscoveryURI)

		mux.HandleFunc(sseEndpoint, oauth2.CheckOAuth(sseHandler))
		mux.HandleFunc(streamableHttpEndpoint, oauth2.CheckOAuth(shttpHandler))
	} else {
		mux.HandleFunc(sseEndpoint, sseHandler.ServeHTTP)
		mux.HandleFunc(streamableHttpEndpoint, shttpHandler.ServeHTTP)
	}

	mux.HandleFunc(healthCheckEndpoint, healthCheckHandler)

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
			return errors.Wrapf(err, "failed to start HTTP server %q", serviceURL)
		}
	}

	logger.Info("terminating MCP server in HTTP mode...")
	return nil
}

func (svr *IRODSMCPServer) getAuthMiddleWare() mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			if svr.config.Remote {
				// http
				authVal := common.NewAuthValueForHTTP(req.GetExtra().Header)
				ctxWithVal := context.WithValue(ctx, common.AuthKey{}, authVal)
				return next(ctxWithVal, method, req)
			} else {
				// stdio
				authVal := common.NewAuthValueForSTDIO(svr.config)
				ctxWithVal := context.WithValue(ctx, common.AuthKey{}, authVal)
				return next(ctxWithVal, method, req)
			}
		}
	}
}

func (svr *IRODSMCPServer) GetIRODSFSClientPool() *irods_common.IRODSFSClientPool {
	return svr.irodsfsClientPool
}

func (svr *IRODSMCPServer) GetIRODSAccountFromAuthValue(authValue *common.AuthValue) (*types.IRODSAccount, error) {
	account := irods_common.GetEmptyIRODSAccount(svr.config)

	if !authValue.IsHTTP() {
		return account, nil
	}

	// only handle HTTP auth values below
	if authValue.IsAnonymous() {
		// anonymous access
		account.ProxyUser = ""
		account.ClientUser = authValue.Username
		account.Password = ""
	} else if len(authValue.Username) > 0 && len(authValue.Password) > 0 {
		// use the provided username and password
		account.ProxyUser = ""
		account.ClientUser = authValue.Username
		account.Password = authValue.Password
	} else if len(authValue.Username) > 0 && len(authValue.Password) == 0 {
		// empty password
		// proxy access with the provided username
		if !svr.config.IRODSProxyAuth {
			return nil, errors.New("user and password must be set")
		}

		if authValue.IsBasicAuth() {
			return nil, errors.New("proxy auth is not supported with basic auth")
		}

		// we only support bearer auth for proxy user access
		account.ClientUser = authValue.Username
	} else {
		return nil, errors.New("invalid auth value with empty username and password")
	}

	account.FixAuthConfiguration()
	return account, nil
}

func (svr *IRODSMCPServer) GetIRODSFSClientFromAuthValue(authValue *common.AuthValue) (*irodsclient_fs.FileSystem, error) {
	account, err := svr.GetIRODSAccountFromAuthValue(authValue)
	if err != nil {
		return nil, err
	}

	// get the IRODSFSClient from the pool
	return svr.irodsfsClientPool.GetIRODSFSClient(account)
}

func (svr *IRODSMCPServer) GetMCPServer() *mcp.Server {
	return svr.mcpServer
}

func (svr *IRODSMCPServer) registerResourceTemplates() error {
	// Register the resource templates with the server
	svr.addResourceTemplate(NewIRODSResourceTemplate(svr))
	return nil
}

func (svr *IRODSMCPServer) registerTools() error {
	// Register the tools with the server
	svr.addTool(NewListAllowedDirectories(svr))
	svr.addTool(NewListDirectory(svr))
	svr.addTool(NewListDirectoryDetails(svr))
	svr.addTool(NewDirectoryTree(svr))
	svr.addTool(NewSearchFiles(svr))
	svr.addTool(NewSearchFilesByAVU(svr))
	svr.addTool(NewGetFileInfo(svr))
	svr.addTool(NewReadFile(svr))
	svr.addTool(NewWriteFile(svr))
	svr.addTool(NewListTickets(svr))
	svr.addTool(NewGetTicketInfo(svr))
	svr.addTool(NewMoveFile(svr))
	svr.addTool(NewCopyFile(svr))
	svr.addTool(NewMakeDirectory(svr))
	svr.addTool(NewDeleteFile(svr))
	svr.addTool(NewUploadFile(svr))
	svr.addTool(NewDownloadFile(svr))
	svr.addTool(NewListAVUs(svr))
	svr.addTool(NewAddAVU(svr))
	svr.addTool(NewDeleteAVU(svr))
	svr.addTool(NewModifyAccess(svr))
	svr.addTool(NewModifyAccessInheritance(svr))
	return nil
}

func (svr *IRODSMCPServer) addResourceTemplate(tpl ResourceTemplateAPI) {
	svr.resourceTemplates = append(svr.resourceTemplates, tpl)

	if svr.mcpServer != nil {
		svr.mcpServer.AddResourceTemplate(tpl.GetResourceTemplate(), tpl.GetHandler())
	}
}

func (svr *IRODSMCPServer) addTool(tool ToolAPI) {
	svr.tools = append(svr.tools, tool)

	if svr.mcpServer != nil {
		svr.mcpServer.AddTool(tool.GetTool(), tool.GetHandler())
	}
}

func (svr *IRODSMCPServer) GetResourceTemplates() []ResourceTemplateAPI {
	return svr.resourceTemplates
}

func (svr *IRODSMCPServer) GetResourceTemplate(name string) ResourceTemplateAPI {
	for _, resourceTemplate := range svr.resourceTemplates {
		if resourceTemplate.GetName() == name {
			return resourceTemplate
		}
	}

	return nil
}

func (svr *IRODSMCPServer) GetTools() []ToolAPI {
	return svr.tools
}

func (svr *IRODSMCPServer) GetTool(name string) ToolAPI {
	for _, tool := range svr.tools {
		if tool.GetName() == name {
			return tool
		}
	}

	return nil
}
