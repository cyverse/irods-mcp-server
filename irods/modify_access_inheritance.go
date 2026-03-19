package irods

import (
	"context"
	"encoding/json"

	"github.com/cockroachdb/errors"
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	"github.com/cyverse/irods-mcp-server/common"
	irods_common "github.com/cyverse/irods-mcp-server/irods/common"
	"github.com/cyverse/irods-mcp-server/irods/model"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	ModifyAccessInheritanceName = irods_common.IRODSAPIPrefix + "modify_access_inheritance"
)

type ModifyAccessInheritanceInputArgs struct {
	Path    string `json:"path"`
	Inherit bool   `json:"inherit"`
	Recurse bool   `json:"recurse,omitempty"`
}

type ModifyAccessInheritance struct {
	mcpServer *IRODSMCPServer
	config    *common.Config
}

func NewModifyAccessInheritance(svr *IRODSMCPServer) ToolAPI {
	return &ModifyAccessInheritance{
		mcpServer: svr,
		config:    svr.GetConfig(),
	}
}

func (t *ModifyAccessInheritance) GetName() string {
	return ModifyAccessInheritanceName
}

func (t *ModifyAccessInheritance) GetDescription() string {
	return `Modify data access inheritance flag of a file or directory.`
}

func (t *ModifyAccessInheritance) GetTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        t.GetName(),
		Description: t.GetDescription(),
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"path": {
					Type:        "string",
					Description: "The path to the directory (collection) to modify access.",
				},
				"inherit": {
					Type:        "boolean",
					Description: "If set, access to the directory (collection) will be inherited by all child entries.",
				},
				"recurse": {
					Type:        "boolean",
					Description: "If set, apply the inheritance flag to all entries within the given directory (collection) recursively.",
					Default:     json.RawMessage("false"),
				},
			},
			Required: []string{"path", "inherit"},
		},
	}
}

func (t *ModifyAccessInheritance) GetHandler() mcp.ToolHandler {
	return t.Handler
}

func (t *ModifyAccessInheritance) GetAccessiblePaths(authValue *common.AuthValue) []string {
	account, err := t.mcpServer.GetIRODSAccountFromAuthValue(authValue)
	if err != nil {
		return []string{}
	}

	homePath := irods_common.GetHomePath(t.config, account)
	sharedPath := irods_common.GetSharedPath(t.config, account)

	paths := []string{
		sharedPath + "/*",
	}

	if !account.IsAnonymousUser() {
		paths = append(paths, homePath)
		paths = append(paths, homePath+"/*")
	}

	return paths
}

func (t *ModifyAccessInheritance) Handler(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// arguments
	args := ModifyAccessInheritanceInputArgs{}
	err := irods_common.MarshalInputArguments(t.GetTool(), request, &args)
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to marshal input arguments")
		return irods_common.ToolErrorResult(outputErr), nil
	}

	// auth
	authValue, err := common.GetAuthValue(ctx)
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to get auth value")
		return irods_common.ToolErrorResult(outputErr), nil
	}

	// make a irods filesystem client
	fs, err := t.mcpServer.GetIRODSFSClientFromAuthValue(&authValue)
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to create a irods fs client")
		return irods_common.ToolErrorResult(outputErr), nil
	}

	irodsPath := irods_common.MakeIRODSPath(t.config, fs.GetAccount(), args.Path)

	// check permission
	if !irods_common.IsAccessAllowed(irodsPath, t.GetAccessiblePaths(&authValue)) {
		outputErr := errors.Newf("%q request is not permitted for path %q", t.GetName(), irodsPath)
		return irods_common.ToolErrorResult(outputErr), nil
	}

	if !fs.Exists(irodsPath) {
		outputErr := errors.Newf("path %q does not exist", irodsPath)
		return irods_common.ToolErrorResult(outputErr), nil
	}

	// Modify Access Inheritance
	content, err := t.modifyAccessInheritance(fs, irodsPath, args.Inherit, args.Recurse)
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to modify access inheritance for %q", irodsPath)
		return irods_common.ToolErrorResult(outputErr), nil
	}

	return irods_common.ToolJSONResult(*content)
}

func (t *ModifyAccessInheritance) modifyAccessInheritance(fs *irodsclient_fs.FileSystem, path string, inherit bool, recurse bool) (*model.ModifyAccessInheritanceOutput, error) {
	err := fs.ChangeDirACLInheritance(path, inherit, recurse, false)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to change access inheritance for %q", path)
	}

	modifyAccessInheritanceOutput := &model.ModifyAccessInheritanceOutput{
		Path:    path,
		Inherit: inherit,
	}

	return modifyAccessInheritanceOutput, nil
}
