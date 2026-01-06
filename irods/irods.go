package irods

import (
	"github.com/cockroachdb/errors"
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	"github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/irods-mcp-server/common"
	irods_common "github.com/cyverse/irods-mcp-server/irods/common"
	"github.com/mark3labs/mcp-go/server"
)

type IRODSMCPServer struct {
	config            *common.Config
	mcpServer         *server.MCPServer
	irodsfsClientPool *irods_common.IRODSFSClientPool
	resources         []ResourceAPI
	tools             []ToolAPI
}

func NewIRODSMCPServer(svr *server.MCPServer, config *common.Config) (*IRODSMCPServer, error) {
	s := &IRODSMCPServer{
		config:            config,
		mcpServer:         svr,
		irodsfsClientPool: irods_common.NewIRODSFSClientPool(),
		resources:         []ResourceAPI{},
		tools:             []ToolAPI{},
	}

	err := s.registerResources()
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
			return nil, errors.Errorf("user and password must be set")
		}

		if authValue.IsBasicAuth() {
			return nil, errors.Errorf("proxy auth is not supported with basic auth")
		}

		// we only support bearer auth for proxy user access
		account.ClientUser = authValue.Username
	} else {
		return nil, errors.Errorf("invalid auth value with empty username and password")
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

func (svr *IRODSMCPServer) GetMCPServer() *server.MCPServer {
	return svr.mcpServer
}

func (svr *IRODSMCPServer) registerResources() error {
	// Register the resources with the server
	svr.addResource(NewFilesystem(svr))
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

func (svr *IRODSMCPServer) addResource(rs ResourceAPI) {
	svr.resources = append(svr.resources, rs)

	if svr.mcpServer != nil {
		svr.mcpServer.AddResource(rs.GetResource(), rs.GetHandler())
	}
}

func (svr *IRODSMCPServer) addTool(tool ToolAPI) {
	svr.tools = append(svr.tools, tool)

	if svr.mcpServer != nil {
		svr.mcpServer.AddTool(tool.GetTool(), tool.GetHandler())
	}
}

func (svr *IRODSMCPServer) GetResources() []ResourceAPI {
	return svr.resources
}

func (svr *IRODSMCPServer) GetResource(name string) ResourceAPI {
	for _, resource := range svr.resources {
		if resource.GetName() == name {
			return resource
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
