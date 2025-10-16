package irods

import (
	"github.com/cyverse/irods-mcp-server/common"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type ToolAPI interface {
	GetName() string
	GetDescription() string
	GetTool() mcp.Tool
	GetHandler() server.ToolHandlerFunc
	GetAccessiblePaths(authValue *common.AuthValue) []string
}

type ResourceAPI interface {
	GetScheme() string
	GetURI() string
	GetName() string
	GetDescription() string
	GetResource() mcp.Resource
	GetHandler() server.ResourceHandlerFunc
}
