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
	ListAVUsName = irods_common.IRODSAPIPrefix + "list_avus"
)

type ListAVUs struct {
	mcpServer *IRODSMCPServer
	config    *common.Config
}

func NewListAVUs(svr *IRODSMCPServer) ToolAPI {
	return &ListAVUs{
		mcpServer: svr,
		config:    svr.GetConfig(),
	}
}

func (t *ListAVUs) GetName() string {
	return ListAVUsName
}

func (t *ListAVUs) GetDescription() string {
	return `List AVUs (attribute-value-unit) from a file (data-object), directory (collection), resource, or user.`
}

func (t *ListAVUs) GetTool() mcp.Tool {
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
	)
}

func (t *ListAVUs) GetHandler() server.ToolHandlerFunc {
	return t.Handler
}

func (t *ListAVUs) GetAccessiblePaths(authValue *common.AuthValue) []string {
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

func (t *ListAVUs) Handler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	arguments := request.GetArguments()

	targetType, ok := arguments["target_type"].(string)
	if !ok {
		outputErr := errors.New("failed to get target_type from arguments")
		return irods_common.OutputMCPError(outputErr)
	}

	target, ok := arguments["target"].(string)
	if !ok {
		outputErr := errors.New("failed to get target from arguments")
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

	// List AVUs
	content, err := t.listAVUs(fs, targetType, target)
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to list AVUs from %q in %q type", target, targetType)
		return irods_common.OutputMCPError(outputErr)
	}

	return mcp.NewToolResultText(content), nil
}

func (t *ListAVUs) listAVUs(fs *irodsclient_fs.FileSystem, targetType string, target string) (string, error) {
	switch targetType {
	case "path":
		return t.listAVUsFromPath(fs, target)
	case "resource":
		return t.listAVUsFromResource(fs, target)
	case "user":
		return t.listAVUsFromUser(fs, target)
	default:
		return "", errors.Newf("invalid target_type %q", targetType)
	}
}

func (t *ListAVUs) listAVUsFromPath(fs *irodsclient_fs.FileSystem, path string) (string, error) {
	if !fs.Exists(path) {
		return "", errors.Newf("path %q does not exist", path)
	}

	metadata, err := fs.ListMetadata(path)
	if err != nil {
		return "", errors.Wrapf(err, "failed to list AVUs from path %q", path)
	}

	avus := []model.AVU{}
	for _, md := range metadata {
		avu := model.AVU{
			ID:        md.AVUID,
			Attribute: md.Name,
			Value:     md.Value,
			Unit:      md.Units,
		}
		avus = append(avus, avu)
	}

	listAVUsOutput := model.ListAVUsOutput{
		TargetType: "path",
		Target:     path,
		AVUs:       avus,
	}

	jsonBytes, err := json.Marshal(listAVUsOutput)
	if err != nil {
		return "", errors.Wrapf(err, "failed to marshal JSON")
	}

	return string(jsonBytes), nil
}

func (t *ListAVUs) listAVUsFromResource(fs *irodsclient_fs.FileSystem, resourceName string) (string, error) {
	metadata, err := fs.ListResourceMetadata(resourceName)
	if err != nil {
		return "", errors.Wrapf(err, "failed to list AVU from resource %q", resourceName)
	}

	avus := []model.AVU{}
	for _, md := range metadata {
		avu := model.AVU{
			ID:        md.AVUID,
			Attribute: md.Name,
			Value:     md.Value,
			Unit:      md.Units,
		}
		avus = append(avus, avu)
	}

	listAVUsOutput := model.ListAVUsOutput{
		TargetType: "resource",
		Target:     resourceName,
		AVUs:       avus,
	}

	jsonBytes, err := json.Marshal(listAVUsOutput)
	if err != nil {
		return "", errors.Wrapf(err, "failed to marshal JSON")
	}

	return string(jsonBytes), nil
}

func (t *ListAVUs) listAVUsFromUser(fs *irodsclient_fs.FileSystem, userName string) (string, error) {
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

	metadata, err := fs.ListUserMetadata(user, zone)
	if err != nil {
		return "", errors.Wrapf(err, "failed to list AVU from user %q", userName)
	}

	avus := []model.AVU{}
	for _, md := range metadata {
		avu := model.AVU{
			ID:        md.AVUID,
			Attribute: md.Name,
			Value:     md.Value,
			Unit:      md.Units,
		}
		avus = append(avus, avu)
	}

	listAVUsOutput := model.ListAVUsOutput{
		TargetType: "user",
		Target:     userName,
		AVUs:       avus,
	}

	jsonBytes, err := json.Marshal(listAVUsOutput)
	if err != nil {
		return "", errors.Wrapf(err, "failed to marshal JSON")
	}

	return string(jsonBytes), nil
}
