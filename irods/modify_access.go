package irods

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/cockroachdb/errors"
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	"github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/irods-mcp-server/common"
	irods_common "github.com/cyverse/irods-mcp-server/irods/common"
	"github.com/cyverse/irods-mcp-server/irods/model"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	ModifyAccessName = irods_common.IRODSAPIPrefix + "modify_access"
)

type ModifyAccess struct {
	mcpServer *IRODSMCPServer
	config    *common.Config
}

func NewModifyAccess(svr *IRODSMCPServer) ToolAPI {
	return &ModifyAccess{
		mcpServer: svr,
		config:    svr.GetConfig(),
	}
}

func (t *ModifyAccess) GetName() string {
	return ModifyAccessName
}

func (t *ModifyAccess) GetDescription() string {
	return `Modify data access of a user or group to a file (data-object) or directory (collection).`
}

func (t *ModifyAccess) GetTool() mcp.Tool {
	return mcp.NewTool(
		t.GetName(),
		mcp.WithDescription(t.GetDescription()),
		mcp.WithString(
			"access_level",
			mcp.Enum("own",
				"delete_object",
				"modify_object",
				"create_object",
				"delete_metadata",
				"modify_metadata",
				"create_metadata",
				"read_object",
				"read_metadata",
				"null",
				"read",
				"write"),
			mcp.Required(),
			mcp.Description("The access level to set to the user. It can be 'own', 'delete_object', 'modify_object', 'create_object', 'delete_metadata', 'modify_metadata', 'create_metadata', 'read_object', 'read_metadata', or 'null'. For iRODS version prior to 4.3.0, only 'own', 'write', 'read', and 'null' are allowed."),
		),
		mcp.WithString(
			"user_or_group",
			mcp.Required(),
			mcp.Description("The user or group to set access. You can specify a user by 'username#zone' or a group by 'groupname#zone' to set zone. if zone is not specified, the client's zone will be used."),
		),
		mcp.WithString(
			"path",
			mcp.Required(),
			mcp.Description("The path to the file (data-object) or directory (collection) to modify access."),
		),
		mcp.WithBoolean(
			"recurse",
			mcp.DefaultBool(false),
			mcp.Description("If set, apply the given access to all entries within the given directory (collection) recursively."),
		),
	)
}

func (t *ModifyAccess) GetHandler() server.ToolHandlerFunc {
	return t.Handler
}

func (t *ModifyAccess) GetAccessiblePaths(authValue *common.AuthValue) []string {
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

func (t *ModifyAccess) Handler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	arguments := request.GetArguments()

	accessLevel, ok := arguments["access_level"].(string)
	if !ok {
		outputErr := errors.New("failed to get access_level from arguments")
		return irods_common.OutputMCPError(outputErr)
	}

	userOrGroup, ok := arguments["user_or_group"].(string)
	if !ok {
		outputErr := errors.New("failed to get user_or_group from arguments")
		return irods_common.OutputMCPError(outputErr)
	}

	path, ok := arguments["path"].(string)
	if !ok {
		outputErr := errors.New("failed to get path from arguments")
		return irods_common.OutputMCPError(outputErr)
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

	// Modify Access
	content, err := t.modifyAccess(fs, userOrGroup, irodsPath, accessLevel, recurse)
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to modify access for %q to %q with access level %q", userOrGroup, irodsPath, accessLevel)
		return irods_common.OutputMCPError(outputErr)
	}

	return mcp.NewToolResultText(content), nil
}

func (t *ModifyAccess) modifyAccess(fs *irodsclient_fs.FileSystem, userOrGroup string, path string, accessLevel string, recurse bool) (string, error) {
	account := fs.GetAccount()

	user := ""
	zone := account.ClientZone

	parts := strings.Split(userOrGroup, "#")
	if len(parts) == 2 {
		user = parts[0]
		zone = parts[1]
	} else {
		user = userOrGroup
	}

	err := fs.ChangeACLs(path, types.IRODSAccessLevelType(accessLevel), user, zone, recurse, false)
	if err != nil {
		return "", errors.Wrapf(err, "failed to change ACLs for %q to %q with access level %q", userOrGroup, path, accessLevel)
	}

	modifyAccessOutput := model.ModifyAccessOutput{
		Path:        path,
		UserName:    user,
		UserZone:    zone,
		AccessLevel: types.IRODSAccessLevelType(accessLevel),
	}

	jsonBytes, err := json.Marshal(modifyAccessOutput)
	if err != nil {
		return "", errors.Wrapf(err, "failed to marshal JSON")
	}

	return string(jsonBytes), nil
}
