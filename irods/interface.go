package irods

import (
	"github.com/cyverse/irods-mcp-server/common"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ToolAPI interface {
	GetName() string
	GetDescription() string
	GetTool() *mcp.Tool
	GetHandler() mcp.ToolHandler
	GetAccessiblePaths(authValue *common.AuthValue) []string
}

type ResourceTemplateAPI interface {
	GetScheme() string
	GetURITemplate() string
	GetName() string
	GetDescription() string
	GetResourceTemplate() *mcp.ResourceTemplate
	GetHandler() mcp.ResourceHandler
}
