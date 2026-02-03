package irods

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/cockroachdb/errors"
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	"github.com/cyverse/irods-mcp-server/common"
	iroirods_common "github.com/cyverse/irods-mcp-server/irods/common"
	"github.com/cyverse/irods-mcp-server/irods/model"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	AddAVUName = iroirods_common.IRODSAPIPrefix + "add_avu"
)

type AddAVU struct {
	mcpServer *IRODSMCPServer
	config    *common.Config
}

func NewAddAVU(svr *IRODSMCPServer) ToolAPI {
	return &AddAVU{
		mcpServer: svr,
		config:    svr.GetConfig(),
	}
}

func (t *AddAVU) GetName() string {
	return AddAVUName
}

func (t *AddAVU) GetDescription() string {
	return `Add a new AVU (attribute-value-unit) to a file (data-object), directory (collection), resource, or user.`
}

func (t *AddAVU) GetTool() mcp.Tool {
	return mcp.NewTool(
		t.GetName(),
		mcp.WithDescription(t.GetDescription()),
		mcp.WithString(
			"target_type",
			mcp.Enum("path", "resource", "user"),
			mcp.Required(),
			mcp.Description("The type of the target to add AVU. It can be 'path', 'resource', or 'user'."),
		),
		mcp.WithString(
			"target",
			mcp.Required(),
			mcp.Description("The target to add AVU. Path for 'path' target_type, resource name for 'resource' target_type, and user name for 'user' target_type."),
		),
		mcp.WithString(
			"attribute",
			mcp.Required(),
			mcp.Description("The attribute of the AVU to add."),
		),
		mcp.WithString(
			"value",
			mcp.Required(),
			mcp.Description("The value of the AVU to add."),
		),
		mcp.WithString(
			"unit",
			mcp.DefaultString(""),
			mcp.Description("The unit of the AVU to add. Default is an empty string."),
		),
	)
}

func (t *AddAVU) GetHandler() server.ToolHandlerFunc {
	return t.Handler
}

func (t *AddAVU) GetAccessiblePaths(authValue *common.AuthValue) []string {
	account, err := t.mcpServer.GetIRODSAccountFromAuthValue(authValue)
	if err != nil {
		return []string{}
	}

	homePath := iroirods_common.GetHomePath(t.config, account)
	sharedPath := iroirods_common.GetSharedPath(t.config, account)

	paths := []string{
		sharedPath + "/*",
	}

	if !account.IsAnonymousUser() {
		paths = append(paths, homePath)
		paths = append(paths, homePath+"/*")
	}

	return paths
}

func (t *AddAVU) Handler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	arguments := request.GetArguments()

	targetType, err := iroirods_common.GetInputStringArgument(arguments, "target_type", true)
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to get target_type from arguments")
		return iroirods_common.OutputMCPError(outputErr)
	}

	target, err := iroirods_common.GetInputStringArgument(arguments, "target", true)
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to get target from arguments")
		return iroirods_common.OutputMCPError(outputErr)
	}

	attribute, err := iroirods_common.GetInputStringArgument(arguments, "attribute", true)
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to get attribute from arguments")
		return iroirods_common.OutputMCPError(outputErr)
	}

	value, err := iroirods_common.GetInputStringArgument(arguments, "value", true)
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to get value from arguments")
		return iroirods_common.OutputMCPError(outputErr)
	}

	unit, err := iroirods_common.GetInputStringArgument(arguments, "unit", false)
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to get unit from arguments")
		return iroirods_common.OutputMCPError(outputErr)
	}

	// auth
	authValue, err := common.GetAuthValue(ctx)
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to get auth value")
		return iroirods_common.OutputMCPError(outputErr)
	}

	// make a irods filesystem client
	fs, err := t.mcpServer.GetIRODSFSClientFromAuthValue(&authValue)
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to create a irods fs client")
		return iroirods_common.OutputMCPError(outputErr)
	}

	if targetType == "path" {
		target = iroirods_common.MakeIRODSPath(t.config, fs.GetAccount(), target)

		// check permission
		if !iroirods_common.IsAccessAllowed(target, t.GetAccessiblePaths(&authValue)) {
			outputErr := errors.Newf("%q request is not permitted for path %q", t.GetName(), target)
			return iroirods_common.OutputMCPError(outputErr)
		}
	}

	// Add AVU
	content, err := t.addAVU(fs, targetType, target, attribute, value, unit)
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to add AVU to %q in %q type, attr %q", target, targetType, attribute)
		return iroirods_common.OutputMCPError(outputErr)
	}

	return mcp.NewToolResultText(content), nil
}

func (t *AddAVU) addAVU(fs *irodsclient_fs.FileSystem, targetType string, target string, attribute string, value string, unit string) (string, error) {
	switch targetType {
	case "path":
		return t.addAVUToPath(fs, target, attribute, value, unit)
	case "resource":
		return t.addAVUToResource(fs, target, attribute, value, unit)
	case "user":
		return t.addAVUToUser(fs, target, attribute, value, unit)
	default:
		return "", errors.Newf("invalid target_type %q", targetType)
	}
}

func (t *AddAVU) addAVUToPath(fs *irodsclient_fs.FileSystem, path string, attribute string, value string, unit string) (string, error) {
	if !fs.Exists(path) {
		return "", errors.Newf("path %q does not exist", path)
	}

	err := fs.AddMetadata(path, attribute, value, unit)
	if err != nil {
		return "", errors.Wrapf(err, "failed to add AVU to path %q", path)
	}

	addAVUOutput := model.AddAVUOutput{
		TargetType: "path",
		Target:     path,
		Attribute:  attribute,
	}

	jsonBytes, err := json.Marshal(addAVUOutput)
	if err != nil {
		return "", errors.Wrapf(err, "failed to marshal JSON")
	}

	return string(jsonBytes), nil
}

func (t *AddAVU) addAVUToResource(fs *irodsclient_fs.FileSystem, resourceName string, attribute string, value string, unit string) (string, error) {
	err := fs.AddResourceMetadata(resourceName, attribute, value, unit)
	if err != nil {
		return "", errors.Wrapf(err, "failed to add AVU to resource %q", resourceName)
	}

	addAVUOutput := model.AddAVUOutput{
		TargetType: "resource",
		Target:     resourceName,
		Attribute:  attribute,
	}

	jsonBytes, err := json.Marshal(addAVUOutput)
	if err != nil {
		return "", errors.Wrapf(err, "failed to marshal JSON")
	}

	return string(jsonBytes), nil
}

func (t *AddAVU) addAVUToUser(fs *irodsclient_fs.FileSystem, userName string, attribute string, value string, unit string) (string, error) {
	account := fs.GetAccount()

	user := ""
	zone := account.ClientZone

	parts := strings.Split(userName, "#")
	if len(parts) == 2 {
		user = parts[0]
		zone = parts[1]
	} else {
		user = userName
	}

	err := fs.AddUserMetadata(user, zone, attribute, value, unit)
	if err != nil {
		return "", errors.Wrapf(err, "failed to add AVU to user %q", userName)
	}

	addAVUOutput := model.AddAVUOutput{
		TargetType: "user",
		Target:     userName,
		Attribute:  attribute,
	}

	jsonBytes, err := json.Marshal(addAVUOutput)
	if err != nil {
		return "", errors.Wrapf(err, "failed to marshal JSON")
	}

	return string(jsonBytes), nil
}
