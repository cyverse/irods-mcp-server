package irods

import (
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	"github.com/cyverse/irods-mcp-server/common"
	irods_common "github.com/cyverse/irods-mcp-server/irods/common"
	"github.com/cyverse/irods-mcp-server/irods/permission"
	"github.com/mark3labs/mcp-go/server"
)

type IRODSMCPServer struct {
	mcpServer         *server.MCPServer
	irodsfsClientPool *irods_common.IRODSFSClientPool
	permission        *permission.APIPermissionManager
	resources         []ResourceAPI
	tools             []ToolAPI
}

func NewIRODSMCPServer(svr *server.MCPServer) (*IRODSMCPServer, error) {
	s := &IRODSMCPServer{
		mcpServer:         svr,
		irodsfsClientPool: irods_common.NewIRODSFSClientPool(),
		permission:        permission.NewAPIPermissionManager(),
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

func (svr *IRODSMCPServer) GetIRODSFSClientPool() *irods_common.IRODSFSClientPool {
	return svr.irodsfsClientPool
}

func (svr *IRODSMCPServer) GetIRODSFSClientFromAuthValue(authValue *common.AuthValue) (*irodsclient_fs.FileSystem, error) {
	account := irods_common.GetEmptyIRODSAccount()
	account.ClientUser = authValue.Username
	account.Password = authValue.Password

	// get the IRODSFSClient from the pool
	return svr.irodsfsClientPool.GetIRODSFSClient(account)
}

func (svr *IRODSMCPServer) GetPermissionManager() *permission.APIPermissionManager {
	return svr.permission
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
	svr.addTool(NewGetFileInfo(svr))
	svr.addTool(NewReadFile(svr))
	return nil
}

func (svr *IRODSMCPServer) addResource(rs ResourceAPI) {
	svr.resources = append(svr.resources, rs)

	if svr.mcpServer != nil {
		svr.mcpServer.AddResource(rs.GetResource(), rs.GetHandler())
	}

	apiName := rs.GetName()
	accessiblePaths := rs.GetAccessiblePaths()

	for _, accessiblePath := range accessiblePaths {
		svr.permission.Add(accessiblePath, apiName)
	}
}

func (svr *IRODSMCPServer) addTool(tool ToolAPI) {
	svr.tools = append(svr.tools, tool)

	if svr.mcpServer != nil {
		svr.mcpServer.AddTool(tool.GetTool(), tool.GetHandler())
	}

	apiName := tool.GetName()
	accessiblePaths := tool.GetAccessiblePaths()

	for _, accessiblePath := range accessiblePaths {
		svr.permission.Add(accessiblePath, apiName)
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
