package irods

import (
	"context"

	"github.com/cockroachdb/errors"
	"github.com/cyverse/irods-mcp-server/common"
	irods_common "github.com/cyverse/irods-mcp-server/irods/common"
	"github.com/cyverse/irods-mcp-server/irods/model"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
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

func (t *ListAllowedDirectories) GetTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        t.GetName(),
		Description: t.GetDescription(),
		InputSchema: &jsonschema.Schema{
			Type:       "object",
			Properties: map[string]*jsonschema.Schema{},
		},
	}
}

func (t *ListAllowedDirectories) GetHandler() mcp.ToolHandler {
	return t.Handler
}

func (t *ListAllowedDirectories) GetAccessiblePaths(authValue *common.AuthValue) []string {
	return []string{}
}

func (t *ListAllowedDirectories) Handler(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// auth
	authValue, err := common.GetAuthValue(ctx)
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to get auth value")
		return irods_common.ToolErrorResult(outputErr), nil
	}

	content, err := t.listAllowedDirectories(&authValue)
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to list allowed directories (collections) and APIs")
		return irods_common.ToolErrorResult(outputErr), nil
	}

	return irods_common.ToolJSONResult(*content)
}

func (t *ListAllowedDirectories) listAllowedDirectories(authValue *common.AuthValue) (*model.ListAllowedDirectories, error) {
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

	listAllowedDirectoriesOutput := &model.ListAllowedDirectories{
		Directories: allowedAPIList,
	}

	return listAllowedDirectoriesOutput, nil
}
