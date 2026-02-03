package irods

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/cockroachdb/errors"
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	"github.com/cyverse/irods-mcp-server/common"
	irods_common "github.com/cyverse/irods-mcp-server/irods/common"
	"github.com/cyverse/irods-mcp-server/irods/model"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	DeleteAVUName = irods_common.IRODSAPIPrefix + "delete_avu"
)

type DeleteAVU struct {
	mcpServer *IRODSMCPServer
	config    *common.Config
}

func NewDeleteAVU(svr *IRODSMCPServer) ToolAPI {
	return &DeleteAVU{
		mcpServer: svr,
		config:    svr.GetConfig(),
	}
}

func (t *DeleteAVU) GetName() string {
	return DeleteAVUName
}

func (t *DeleteAVU) GetDescription() string {
	return `Delete an AVU (attribute-value-unit) from a file, directory, resource, or user.`
}

func (t *DeleteAVU) GetTool() mcp.Tool {
	return mcp.NewTool(
		t.GetName(),
		mcp.WithDescription(t.GetDescription()),
		mcp.WithString(
			"target_type",
			mcp.Enum("path", "resource", "user"),
			mcp.Required(),
			mcp.Description("The type of the target to delete AVU. It can be 'path', 'resource', or 'user'."),
		),
		mcp.WithString(
			"target",
			mcp.Required(),
			mcp.Description("The target to delete AVU. Path for 'path' target_type, resource name for 'resource' target_type, and user name for 'user' target_type."),
		),
		mcp.WithNumber(
			"id",
			mcp.DefaultNumber(0),
			mcp.Description("The ID of the AVU to delete. This field can be ignored if attribute is provided."),
		),
		mcp.WithString(
			"attribute",
			mcp.DefaultString(""),
			mcp.Description("The attribute of the AVU to delete. This field can be ignored if ID is provided."),
		),
		mcp.WithString(
			"value",
			mcp.DefaultString(""),
			mcp.Description("The value of the AVU to delete. Default is an empty string."),
		),
		mcp.WithString(
			"unit",
			mcp.DefaultString(""),
			mcp.Description("The unit of the AVU to delete. Default is an empty string."),
		),
	)
}

func (t *DeleteAVU) GetHandler() server.ToolHandlerFunc {
	return t.Handler
}

func (t *DeleteAVU) GetAccessiblePaths(authValue *common.AuthValue) []string {
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

func (t *DeleteAVU) Handler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	arguments := request.GetArguments()

	targetType, err := irods_common.GetInputStringArgument(arguments, "target_type", true)
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to get target_type from arguments")
		return irods_common.OutputMCPError(outputErr)
	}

	target, err := irods_common.GetInputStringArgument(arguments, "target", true)
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to get target from arguments")
		return irods_common.OutputMCPError(outputErr)
	}

	idFloat, err := irods_common.GetInputNumberArgument(arguments, "id")
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to get id from arguments")
		return irods_common.OutputMCPError(outputErr)
	}

	id := int64(idFloat)

	attribute, err := irods_common.GetInputStringArgument(arguments, "attribute", false)
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to get attribute from arguments")
		return irods_common.OutputMCPError(outputErr)
	}

	if id == 0 && attribute == "" {
		outputErr := errors.New("either id or attribute must be provided")
		return irods_common.OutputMCPError(outputErr)
	}

	value, err := irods_common.GetInputStringArgument(arguments, "value", false)
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to get value from arguments")
		return irods_common.OutputMCPError(outputErr)
	}

	unit, err := irods_common.GetInputStringArgument(arguments, "unit", false)
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to get unit from arguments")
		return irods_common.OutputMCPError(outputErr)
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

	if targetType == "path" {
		target = irods_common.MakeIRODSPath(t.config, fs.GetAccount(), target)

		// check permission
		if !irods_common.IsAccessAllowed(target, t.GetAccessiblePaths(&authValue)) {
			outputErr := errors.Newf("%q request is not permitted for path %q", t.GetName(), target)
			return irods_common.OutputMCPError(outputErr)
		}
	}

	// Delete AVU
	content, err := t.deleteAVU(fs, targetType, target, id, attribute, value, unit)
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to delete AVU from %q in %q type, attr %q", target, targetType, attribute)
		return irods_common.OutputMCPError(outputErr)
	}

	return mcp.NewToolResultText(content), nil
}

func (t *DeleteAVU) deleteAVU(fs *irodsclient_fs.FileSystem, targetType string, target string, id int64, attribute string, value string, unit string) (string, error) {
	switch targetType {
	case "path":
		return t.deleteAVUFromPath(fs, target, id, attribute, value, unit)
	case "resource":
		return t.deleteAVUFromResource(fs, target, id, attribute, value, unit)
	case "user":
		return t.deleteAVUFromUser(fs, target, id, attribute, value, unit)
	default:
		return "", errors.Newf("invalid target_type %q", targetType)
	}
}

func (t *DeleteAVU) deleteAVUFromPath(fs *irodsclient_fs.FileSystem, path string, id int64, attribute string, value string, unit string) (string, error) {
	if !fs.Exists(path) {
		return "", errors.Newf("path %q does not exist", path)
	}

	var err error
	if id > 0 {
		err = fs.DeleteMetadata(path, id)
	} else {
		err = fs.DeleteMetadataByAVU(path, attribute, value, unit)
	}

	if err != nil {
		return "", errors.Wrapf(err, "failed to delete AVU from path %q", path)
	}

	deleteAVUOutput := model.DeleteAVUOutput{
		TargetType: "path",
		Target:     path,
		ID:         id,
		Attribute:  attribute,
		Value:      value,
		Unit:       unit,
	}

	jsonBytes, err := json.Marshal(deleteAVUOutput)
	if err != nil {
		return "", errors.Wrapf(err, "failed to marshal JSON")
	}

	return string(jsonBytes), nil
}

func (t *DeleteAVU) deleteAVUFromResource(fs *irodsclient_fs.FileSystem, resourceName string, id int64, attribute string, value string, unit string) (string, error) {
	var err error
	if id > 0 {
		err = fs.DeleteResourceMetadata(resourceName, id)
	} else {
		err = fs.DeleteResourceMetadataByAVU(resourceName, attribute, value, unit)
	}

	if err != nil {
		return "", errors.Wrapf(err, "failed to delete AVU from resource %q", resourceName)
	}

	deleteAVUOutput := model.DeleteAVUOutput{
		TargetType: "resource",
		Target:     resourceName,
		ID:         id,
		Attribute:  attribute,
		Value:      value,
		Unit:       unit,
	}

	jsonBytes, err := json.Marshal(deleteAVUOutput)
	if err != nil {
		return "", errors.Wrapf(err, "failed to marshal JSON")
	}

	return string(jsonBytes), nil
}

func (t *DeleteAVU) deleteAVUFromUser(fs *irodsclient_fs.FileSystem, userName string, id int64, attribute string, value string, unit string) (string, error) {
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

	var err error
	if id > 0 {
		err = fs.DeleteUserMetadata(user, zone, id)
	} else {
		err = fs.DeleteUserMetadataByAVU(user, zone, attribute, value, unit)
	}

	if err != nil {
		return "", errors.Wrapf(err, "failed to delete AVU from user %q", userName)
	}

	deleteAVUOutput := model.DeleteAVUOutput{
		TargetType: "user",
		Target:     userName,
		ID:         id,
		Attribute:  attribute,
		Value:      value,
		Unit:       unit,
	}

	jsonBytes, err := json.Marshal(deleteAVUOutput)
	if err != nil {
		return "", errors.Wrapf(err, "failed to marshal JSON")
	}

	return string(jsonBytes), nil
}
