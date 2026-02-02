package irods

import (
	"context"
	"encoding/json"

	"github.com/cockroachdb/errors"
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	"github.com/cyverse/irods-mcp-server/common"
	irods_common "github.com/cyverse/irods-mcp-server/irods/common"
	"github.com/cyverse/irods-mcp-server/irods/model"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	ModifyAccessInheritanceName = irods_common.IRODSAPIPrefix + "modify_access_inheritance"
)

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

func (t *ModifyAccessInheritance) GetTool() mcp.Tool {
	return mcp.NewTool(
		t.GetName(),
		mcp.WithDescription(t.GetDescription()),
		mcp.WithString(
			"path",
			mcp.Required(),
			mcp.Description("The path to the directory (collection) to modify access."),
		),
		mcp.WithBoolean(
			"inherit",
			mcp.Required(),
			mcp.Description("If set, access to the directory (collection) will be inherited by all child entries."),
		),
		mcp.WithBoolean(
			"recurse",
			mcp.DefaultBool(false),
			mcp.Description("If set, apply the inheritance flag to all entries within the given directory (collection) recursively."),
		),
	)
}

func (t *ModifyAccessInheritance) GetHandler() server.ToolHandlerFunc {
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

func (t *ModifyAccessInheritance) Handler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	arguments := request.GetArguments()

	path, ok := arguments["path"].(string)
	if !ok {
		outputErr := errors.New("failed to get path from arguments")
		return irods_common.OutputMCPError(outputErr)
	}

	inherit, ok := arguments["inherit"].(bool)
	if !ok {
		inherit = false
	}

	recurse, ok := arguments["recurse"].(bool)
	if !ok {
		recurse = false
	}

	// auth
	authValue, err := common.GetAuthValue(ctx)
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to get auth value")
		return irods_common.OutputMCPError(outputErr)
	}

	// make a irods filesystem client
	fs, err := t.mcpServer.GetIRODSFSClientFromAuthValue(&authValue)
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to create a irods fs client")
		return irods_common.OutputMCPError(outputErr)
	}

	irodsPath := irods_common.MakeIRODSPath(t.config, fs.GetAccount(), path)

	// check permission
	if !irods_common.IsAccessAllowed(irodsPath, t.GetAccessiblePaths(&authValue)) {
		outputErr := errors.Newf("%q request is not permitted for path %q", t.GetName(), irodsPath)
		return irods_common.OutputMCPError(outputErr)
	}

	if !fs.Exists(irodsPath) {
		outputErr := errors.Newf("path %q does not exist", irodsPath)
		return irods_common.OutputMCPError(outputErr)
	}

	// Modify Access Inheritance
	content, err := t.modifyAccessInheritance(fs, irodsPath, inherit, recurse)
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to modify access inheritance for %q", irodsPath)
		return irods_common.OutputMCPError(outputErr)
	}

	return mcp.NewToolResultText(content), nil
}

func (t *ModifyAccessInheritance) modifyAccessInheritance(fs *irodsclient_fs.FileSystem, path string, inherit bool, recurse bool) (string, error) {
	err := fs.ChangeDirACLInheritance(path, inherit, recurse, false)
	if err != nil {
		return "", errors.Wrapf(err, "failed to change access inheritance for %q", path)
	}

	modifyAccessInheritanceOutput := model.ModifyAccessInheritanceOutput{
		Path:    path,
		Inherit: inherit,
	}

	jsonBytes, err := json.Marshal(modifyAccessInheritanceOutput)
	if err != nil {
		return "", errors.Wrapf(err, "failed to marshal JSON")
	}

	return string(jsonBytes), nil
}
