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

type ResourceTemplateAPI interface {
	GetScheme() string
	GetURITemplate() string
	GetName() string
	GetDescription() string
	GetResourceTemplate() mcp.ResourceTemplate
	GetHandler() server.ResourceTemplateHandlerFunc
}
