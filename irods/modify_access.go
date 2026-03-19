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
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	ModifyAccessName = irods_common.IRODSAPIPrefix + "modify_access"
)

type ModifyAccessInputArgs struct {
	AccessLevel string `json:"access_level"`
	UserOrGroup string `json:"user_or_group"`
	Path        string `json:"path"`
	Recurse     bool   `json:"recurse,omitempty"`
}

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

func (t *ModifyAccess) GetTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        t.GetName(),
		Description: t.GetDescription(),
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"access_level": {
					Type:        "string",
					Enum:        []interface{}{"own", "delete_object", "modify_object", "create_object", "delete_metadata", "modify_metadata", "create_metadata", "read_object", "read_metadata", "null", "read", "write"},
					Description: "The access level to set to the user. It can be 'own', 'delete_object', 'modify_object', 'create_object', 'delete_metadata', 'modify_metadata', 'create_metadata', 'read_object', 'read_metadata', or 'null'. For iRODS version prior to 4.3.0, only 'own', 'write', 'read', and 'null' are allowed.",
				},
				"user_or_group": {
					Type:        "string",
					Description: "The user or group to set access. You can specify a user by 'username#zone' or a group by 'groupname#zone' to set zone. if zone is not specified, the client's zone will be used.",
				},
				"path": {
					Type:        "string",
					Description: "The path to the file (data-object) or directory (collection) to modify access.",
				},
				"recurse": {
					Type:        "boolean",
					Description: "If set, apply the given access to all entries within the given directory (collection) recursively.",
					Default:     json.RawMessage("false"),
				},
			},
			Required: []string{"access_level", "user_or_group", "path"},
		},
	}
}

func (t *ModifyAccess) GetHandler() mcp.ToolHandler {
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

func (t *ModifyAccess) Handler(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// arguments
	args := ModifyAccessInputArgs{}
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

	// Modify Access
	content, err := t.modifyAccess(fs, args.UserOrGroup, irodsPath, args.AccessLevel, args.Recurse)
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to modify access for %q to %q with access level %q", args.UserOrGroup, irodsPath, args.AccessLevel)
		return irods_common.ToolErrorResult(outputErr), nil
	}

	return irods_common.ToolJSONResult(*content)
}

func (t *ModifyAccess) modifyAccess(fs *irodsclient_fs.FileSystem, userOrGroup string, path string, accessLevel string, recurse bool) (*model.ModifyAccessOutput, error) {
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
		return nil, errors.Wrapf(err, "failed to change ACLs for %q to %q with access level %q", userOrGroup, path, accessLevel)
	}

	modifyAccessOutput := &model.ModifyAccessOutput{
		Path:        path,
		UserName:    user,
		UserZone:    zone,
		AccessLevel: types.IRODSAccessLevelType(accessLevel),
	}

	return modifyAccessOutput, nil
}
