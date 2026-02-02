package irods

import (
	"context"
	"encoding/json"

	"github.com/cockroachdb/errors"
	"github.com/cyverse/irods-mcp-server/common"
	irods_common "github.com/cyverse/irods-mcp-server/irods/common"
	"github.com/cyverse/irods-mcp-server/irods/model"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	ListAllowedDirectoriesName = irods_common.IRODSAPIPrefix + "list_allowed_directories"
)

type ListAllowedDirectories struct {
	mcpServer *IRODSMCPServer
	config    *common.Config
}

func NewListAllowedDirectories(svr *IRODSMCPServer) ToolAPI {
	return &ListAllowedDirectories{
		mcpServer: svr,
		config:    svr.GetConfig(),
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
	// auth
	authValue, err := common.GetAuthValue(ctx)
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to get auth value")
		return irods_common.OutputMCPError(outputErr)
	}

	content, err := t.listAllowedDirectories(&authValue)
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to list allowed directories (collections) and APIs")
		return irods_common.OutputMCPError(outputErr)
	}

	return mcp.NewToolResultText(content), nil
}

func (t *ListAllowedDirectories) GetAccessiblePaths(authValue *common.AuthValue) []string {
	return []string{}
}

func (t *ListAllowedDirectories) listAllowedDirectories(authValue *common.AuthValue) (string, error) {
	// collect all allowed directories (collections) and APIs
	// key = path, value = list of API names
	allowedAPIs := map[string][]string{}

	for _, t := range t.mcpServer.tools {
		accessiblePaths := t.GetAccessiblePaths(authValue)
		for _, accessiblePath := range accessiblePaths {
			if allowedAPIsForPath, ok := allowedAPIs[accessiblePath]; ok {
				allowedAPIsForPath = append(allowedAPIsForPath, t.GetName())
				allowedAPIs[accessiblePath] = allowedAPIsForPath
			} else {
				allowedAPIs[accessiblePath] = []string{t.GetName()}
			}
		}
	}

	allowedAPIList := []model.AllowedAPIs{}

	for path, apiNames := range allowedAPIs {
		allowedAPIList = append(allowedAPIList, model.AllowedAPIs{
			Path:        path,
			ResourceURI: irods_common.MakeResourceURI(path),
			APIs:        apiNames,
			Allowed:     true,
		})
	}

	listAllowedDirectoriesOutput := model.ListAllowedDirectories{
		Directories: allowedAPIList,
	}

	jsonBytes, err := json.Marshal(listAllowedDirectoriesOutput)
	if err != nil {
		return "", errors.Wrapf(err, "failed to marshal JSON")
	}

	return string(jsonBytes), nil
}
