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
	DeleteAVUName = irods_common.IRODSAPIPrefix + "delete_avu"
)

type DeleteAVUInputArgs struct {
	TargetType string `json:"target_type"`
	Target     string `json:"target"`
	ID         int64  `json:"id,omitempty"`
	Attribute  string `json:"attribute,omitempty"`
	Value      string `json:"value,omitempty"`
	Unit       string `json:"unit,omitempty"`
}

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

func (t *DeleteAVU) GetTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        t.GetName(),
		Description: t.GetDescription(),
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"target_type": {
					Type:        "string",
					Enum:        []interface{}{"path", "resource", "user"},
					Description: "The type of the target to delete AVU. It can be 'path', 'resource', or 'user'.",
				},
				"target": {
					Type:        "string",
					Description: "The target to delete AVU. Path for 'path' target_type, resource name for 'resource' target_type, and user name for 'user' target_type.",
				},
				"id": {
					Type:        "number",
					Description: "The ID of the AVU to delete.",
					Default:     json.RawMessage("0"),
				},
				"attribute": {
					Type:        "string",
					Description: "The attribute of the AVU to delete. This field can be ignored if ID is provided.",
					Default:     json.RawMessage(`""`),
				},
				"value": {
					Type:        "string",
					Description: "The value of the AVU to delete. Default is an empty string.",
					Default:     json.RawMessage(`""`),
				},
				"unit": {
					Type:        "string",
					Description: "The unit of the AVU to delete. Default is an empty string.",
					Default:     json.RawMessage(`""`),
				},
			},
			Required: []string{"target_type", "target"},
			/*
				OneOf: []*jsonschema.Schema{
					{
						Required: []string{"target_type", "target", "id"},
					},
					{
						Required: []string{"target_type", "target", "attribute"},
					},
				},
			*/
		},
	}
}

func (t *DeleteAVU) GetHandler() mcp.ToolHandler {
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

func (t *DeleteAVU) Handler(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// arguments
	args := DeleteAVUInputArgs{}
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

	// Delete AVU
	content, err := t.deleteAVU(fs, args.TargetType, args.Target, args.ID, args.Attribute, args.Value, args.Unit)
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to delete AVU from %q in %q type, attr %q", args.Target, args.TargetType, args.Attribute)
		return irods_common.ToolErrorResult(outputErr), nil
	}

	return irods_common.ToolJSONResult(*content)
}

func (t *DeleteAVU) deleteAVU(fs *irodsclient_fs.FileSystem, targetType string, target string, id int64, attribute string, value string, unit string) (*model.DeleteAVUOutput, error) {
	switch targetType {
	case "path":
		return t.deleteAVUFromPath(fs, target, id, attribute, value, unit)
	case "resource":
		return t.deleteAVUFromResource(fs, target, id, attribute, value, unit)
	case "user":
		return t.deleteAVUFromUser(fs, target, id, attribute, value, unit)
	default:
		return nil, errors.Newf("invalid target_type %q", targetType)
	}
}

func (t *DeleteAVU) deleteAVUFromPath(fs *irodsclient_fs.FileSystem, path string, id int64, attribute string, value string, unit string) (*model.DeleteAVUOutput, error) {
	if !fs.Exists(path) {
		return nil, errors.Newf("path %q does not exist", path)
	}

	var err error
	if id > 0 {
		err = fs.DeleteMetadata(path, id)
	} else {
		err = fs.DeleteMetadataByAVU(path, attribute, value, unit)
	}

	if err != nil {
		return nil, errors.Wrapf(err, "failed to delete AVU from path %q", path)
	}

	deleteAVUOutput := &model.DeleteAVUOutput{
		TargetType: "path",
		Target:     path,
		ID:         id,
		Attribute:  attribute,
		Value:      value,
		Unit:       unit,
	}

	return deleteAVUOutput, nil
}

func (t *DeleteAVU) deleteAVUFromResource(fs *irodsclient_fs.FileSystem, resourceName string, id int64, attribute string, value string, unit string) (*model.DeleteAVUOutput, error) {
	var err error
	if id > 0 {
		err = fs.DeleteResourceMetadata(resourceName, id)
	} else {
		err = fs.DeleteResourceMetadataByAVU(resourceName, attribute, value, unit)
	}

	if err != nil {
		return nil, errors.Wrapf(err, "failed to delete AVU from resource %q", resourceName)
	}

	deleteAVUOutput := &model.DeleteAVUOutput{
		TargetType: "resource",
		Target:     resourceName,
		ID:         id,
		Attribute:  attribute,
		Value:      value,
		Unit:       unit,
	}

	return deleteAVUOutput, nil
}

func (t *DeleteAVU) deleteAVUFromUser(fs *irodsclient_fs.FileSystem, userName string, id int64, attribute string, value string, unit string) (*model.DeleteAVUOutput, error) {
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
		return nil, errors.Wrapf(err, "failed to delete AVU from user %q", userName)
	}

	deleteAVUOutput := &model.DeleteAVUOutput{
		TargetType: "user",
		Target:     userName,
		ID:         id,
		Attribute:  attribute,
		Value:      value,
		Unit:       unit,
	}

	return deleteAVUOutput, nil
}
