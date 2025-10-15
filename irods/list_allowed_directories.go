package irods

import (
	"context"
	"encoding/json"

	"github.com/cyverse/irods-mcp-server/irods/common"
	"github.com/cyverse/irods-mcp-server/irods/model"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"golang.org/x/xerrors"
)

const (
	ListAllowedDirectoriesName = common.IRODSAPIPrefix + "list_allowed_directories"
)

type ListAllowedDirectories struct {
	mcpServer *IRODSMCPServer
}

func NewListAllowedDirectories(svr *IRODSMCPServer) ToolAPI {
	return &ListAllowedDirectories{
		mcpServer: svr,
	}
}

func (t *ListAllowedDirectories) GetName() string {
	return ListAllowedDirectoriesName
}

func (t *ListAllowedDirectories) GetDescription() string {
	return `Get a list of directories (collections) that this server is allowed to access.
	The output also contains API names that can be requested to each directory (collection).`
}

func (t *ListAllowedDirectories) GetTool() mcp.Tool {
	return mcp.NewTool(
		t.GetName(),
		mcp.WithDescription(t.GetDescription()),
	)
}

func (t *ListAllowedDirectories) GetHandler() server.ToolHandlerFunc {
	return t.Handler
}

func (t *ListAllowedDirectories) Handler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	content, err := t.listAllowedDirectories()
	if err != nil {
		outputErr := xerrors.Errorf("failed to list allowed directories (collections) and APIs: %w", err)
		return common.OutputMCPError(outputErr)
	}

	return mcp.NewToolResultText(content), nil
}

func (t *ListAllowedDirectories) GetAccessiblePaths() []string {
	return []string{}
}

func (t *ListAllowedDirectories) listAllowedDirectories() (string, error) {
	permissionMgr := t.mcpServer.GetPermissionManager()
	apiPermissions := permissionMgr.GetAll()

	allowedAPIs := []model.AllowedAPIs{}

	for _, apiPermission := range apiPermissions {
		if len(apiPermission.APIs) == 0 {
			allowedAPIs = append(allowedAPIs, model.AllowedAPIs{
				Path:        apiPermission.Path,
				ResourceURI: common.MakeResourceURI(apiPermission.Path),
				Allowed:     false,
			})
		} else {
			allowedAPIs = append(allowedAPIs, model.AllowedAPIs{
				Path:        apiPermission.Path,
				ResourceURI: common.MakeResourceURI(apiPermission.Path),
				APIs:        apiPermission.APIs,
				Allowed:     true,
			})
		}
	}

	listAllowedDirectoriesOutput := model.ListAllowedDirectories{
		Directories: allowedAPIs,
	}

	jsonBytes, err := json.Marshal(listAllowedDirectoriesOutput)
	if err != nil {
		return "", xerrors.Errorf("failed to marshal JSON: %w", err)
	}

	return string(jsonBytes), nil
}
