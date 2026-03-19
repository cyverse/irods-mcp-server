package irods

import (
	"context"
	"strings"

	"github.com/cockroachdb/errors"
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	"github.com/cyverse/irods-mcp-server/common"
	irods_common "github.com/cyverse/irods-mcp-server/irods/common"
	"github.com/cyverse/irods-mcp-server/irods/model"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	ListAVUsName = irods_common.IRODSAPIPrefix + "list_avus"
)

type ListAVUsInputArgs struct {
	TargetType string `json:"target_type"`
	Target     string `json:"target"`
}

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

func (t *ListAVUs) GetTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        t.GetName(),
		Description: t.GetDescription(),
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"target_type": {
					Type:        "string",
					Enum:        []interface{}{"path", "resource", "user"},
					Description: "The type of the target to list AVU. It can be 'path', 'resource', or 'user'.",
				},
				"target": {
					Type:        "string",
					Description: "The target to list AVU. Path for 'path' target_type, resource name for 'resource' target_type, and user name for 'user' target_type.",
				},
			},
			Required: []string{"target_type", "target"},
		},
	}
}

func (t *ListAVUs) GetHandler() mcp.ToolHandler {
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

func (t *ListAVUs) Handler(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// arguments
	args := ListAVUsInputArgs{}
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

	if args.TargetType == "path" {
		args.Target = irods_common.MakeIRODSPath(t.config, fs.GetAccount(), args.Target)

		// check permission
		if !irods_common.IsAccessAllowed(args.Target, t.GetAccessiblePaths(&authValue)) {
			outputErr := errors.Newf("%q request is not permitted for path %q", t.GetName(), args.Target)
			return irods_common.ToolErrorResult(outputErr), nil
		}
	}

	// List AVUs
	content, err := t.listAVUs(fs, args.TargetType, args.Target)
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to list AVUs from %q in %q type", args.Target, args.TargetType)
		return irods_common.ToolErrorResult(outputErr), nil
	}

	return irods_common.ToolJSONResult(*content)
}

func (t *ListAVUs) listAVUs(fs *irodsclient_fs.FileSystem, targetType string, target string) (*model.ListAVUsOutput, error) {
	switch targetType {
	case "path":
		return t.listAVUsFromPath(fs, target)
	case "resource":
		return t.listAVUsFromResource(fs, target)
	case "user":
		return t.listAVUsFromUser(fs, target)
	default:
		return nil, errors.Newf("invalid target_type %q", targetType)
	}
}

func (t *ListAVUs) listAVUsFromPath(fs *irodsclient_fs.FileSystem, path string) (*model.ListAVUsOutput, error) {
	if !fs.Exists(path) {
		return nil, errors.Newf("path %q does not exist", path)
	}

	metadata, err := fs.ListMetadata(path)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to list AVUs from path %q", path)
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

	listAVUsOutput := &model.ListAVUsOutput{
		TargetType: "path",
		Target:     path,
		AVUs:       avus,
	}

	return listAVUsOutput, nil
}

func (t *ListAVUs) listAVUsFromResource(fs *irodsclient_fs.FileSystem, resourceName string) (*model.ListAVUsOutput, error) {
	metadata, err := fs.ListResourceMetadata(resourceName)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to list AVU from resource %q", resourceName)
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

	listAVUsOutput := &model.ListAVUsOutput{
		TargetType: "resource",
		Target:     resourceName,
		AVUs:       avus,
	}

	return listAVUsOutput, nil
}

func (t *ListAVUs) listAVUsFromUser(fs *irodsclient_fs.FileSystem, userName string) (*model.ListAVUsOutput, error) {
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
		return nil, errors.Wrapf(err, "failed to list AVU from user %q", userName)
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

	listAVUsOutput := &model.ListAVUsOutput{
		TargetType: "user",
		Target:     userName,
		AVUs:       avus,
	}

	return listAVUsOutput, nil
}
