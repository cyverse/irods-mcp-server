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
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	AddAVUName = irods_common.IRODSAPIPrefix + "add_avu"
)

type AddAVUInputArgs struct {
	TargetType string `json:"target_type"`
	Target     string `json:"target"`
	Attribute  string `json:"attribute"`
	Value      string `json:"value"`
	Unit       string `json:"unit,omitempty"`
}

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

func (t *AddAVU) GetTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        t.GetName(),
		Description: t.GetDescription(),
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"target_type": {
					Type:        "string",
					Enum:        []interface{}{"path", "resource", "user"},
					Description: "The type of the target to add AVU. It can be 'path', 'resource', or 'user'.",
				},
				"target": {
					Type:        "string",
					Description: "The target to add AVU. Path for 'path' target_type, resource name for 'resource' target_type, and user name for 'user' target_type.",
				},
				"attribute": {
					Type:        "string",
					Description: "The attribute of the AVU to add.",
				},
				"value": {
					Type:        "string",
					Description: "The value of the AVU to add.",
				},
				"unit": {
					Type:        "string",
					Description: "The unit of the AVU to add. Default is an empty string.",
					Default:     json.RawMessage(`""`),
				},
			},
			Required: []string{"target_type", "target", "attribute", "value"},
		},
	}
}

func (t *AddAVU) GetHandler() mcp.ToolHandler {
	return t.Handler
}

func (t *AddAVU) GetAccessiblePaths(authValue *common.AuthValue) []string {
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

func (t *AddAVU) Handler(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// arguments
	args := AddAVUInputArgs{}
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

	// check permission if the target type is path
	if args.TargetType == "path" {
		args.Target = irods_common.MakeIRODSPath(t.config, fs.GetAccount(), args.Target)

		// check permission
		if !irods_common.IsAccessAllowed(args.Target, t.GetAccessiblePaths(&authValue)) {
			outputErr := errors.Newf("%q request is not permitted for path %q", t.GetName(), args.Target)
			return irods_common.ToolErrorResult(outputErr), nil
		}
	}

	// Add AVU
	content, err := t.addAVU(fs, args.TargetType, args.Target, args.Attribute, args.Value, args.Unit)
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to add AVU to %q in %q type, attr %q", args.Target, args.TargetType, args.Attribute)
		return irods_common.ToolErrorResult(outputErr), nil
	}

	return irods_common.ToolJSONResult(*content)
}

func (t *AddAVU) addAVU(fs *irodsclient_fs.FileSystem, targetType string, target string, attribute string, value string, unit string) (*model.AddAVUOutput, error) {
	switch targetType {
	case "path":
		return t.addAVUToPath(fs, target, attribute, value, unit)
	case "resource":
		return t.addAVUToResource(fs, target, attribute, value, unit)
	case "user":
		return t.addAVUToUser(fs, target, attribute, value, unit)
	default:
		return nil, errors.Newf("invalid target_type %q", targetType)
	}
}

func (t *AddAVU) addAVUToPath(fs *irodsclient_fs.FileSystem, path string, attribute string, value string, unit string) (*model.AddAVUOutput, error) {
	if !fs.Exists(path) {
		return nil, errors.Newf("path %q does not exist", path)
	}

	err := fs.AddMetadata(path, attribute, value, unit)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to add AVU to path %q", path)
	}

	addAVUOutput := &model.AddAVUOutput{
		TargetType: "path",
		Target:     path,
		Attribute:  attribute,
	}

	return addAVUOutput, nil
}

func (t *AddAVU) addAVUToResource(fs *irodsclient_fs.FileSystem, resourceName string, attribute string, value string, unit string) (*model.AddAVUOutput, error) {
	err := fs.AddResourceMetadata(resourceName, attribute, value, unit)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to add AVU to resource %q", resourceName)
	}

	addAVUOutput := &model.AddAVUOutput{
		TargetType: "resource",
		Target:     resourceName,
		Attribute:  attribute,
	}

	return addAVUOutput, nil
}

func (t *AddAVU) addAVUToUser(fs *irodsclient_fs.FileSystem, userName string, attribute string, value string, unit string) (*model.AddAVUOutput, error) {
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
		return nil, errors.Wrapf(err, "failed to add AVU to user %q", userName)
	}

	addAVUOutput := &model.AddAVUOutput{
		TargetType: "user",
		Target:     userName,
		Attribute:  attribute,
	}

	return addAVUOutput, nil
}
