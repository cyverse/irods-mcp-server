package irods

import (
	"context"
	"encoding/json"

	"github.com/cockroachdb/errors"
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/irods-mcp-server/common"
	irods_common "github.com/cyverse/irods-mcp-server/irods/common"
	"github.com/cyverse/irods-mcp-server/irods/model"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	ApplyAVUOperationsName = irods_common.IRODSAPIPrefix + "apply_avu_operations"
)

type ApplyAVUOperationsInputArgs struct {
	Path       string                             `json:"path"`
	Operations []ApplyAVUOperationsInputOperation `json:"operations"`
}

type ApplyAVUOperationsInputOperation struct {
	Operation string `json:"operation"`
	Attribute string `json:"attribute"`
	Value     string `json:"value"`
	Units     string `json:"units,omitempty"`
}

type ApplyAVUOperations struct {
	mcpServer *IRODSMCPServer
	config    *common.Config
}

func NewApplyAVUOperations(svr *IRODSMCPServer) ToolAPI {
	return &ApplyAVUOperations{
		mcpServer: svr,
		config:    svr.GetConfig(),
	}
}

func (t *ApplyAVUOperations) GetName() string {
	return ApplyAVUOperationsName
}

func (t *ApplyAVUOperations) GetDescription() string {
	return `Add and/or remove multiple AVUs (attribute-value-unit) on a file (data-object) or directory (collection) atomically in a single request. Operations are applied in the given order, and either all of them are applied or none of them are applied.`
}

func (t *ApplyAVUOperations) GetTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        t.GetName(),
		Description: t.GetDescription(),
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"path": {
					Type:        "string",
					Description: "The path of the file or directory to apply AVU operations.",
				},
				"operations": {
					Type:        "array",
					Description: "The list of AVU operations to apply in order.",
					MinItems:    jsonschema.Ptr(1),
					Items: &jsonschema.Schema{
						Type: "object",
						Properties: map[string]*jsonschema.Schema{
							"operation": {
								Type:        "string",
								Enum:        []interface{}{"add", "remove"},
								Description: "The operation to apply. It can be 'add' or 'remove'.",
							},
							"attribute": {
								Type:        "string",
								Description: "The attribute of the AVU.",
							},
							"value": {
								Type:        "string",
								Description: "The value of the AVU.",
							},
							"units": {
								Type:        "string",
								Description: "The units of the AVU. Default is an empty string.",
								Default:     json.RawMessage(`""`),
							},
						},
						Required: []string{"operation", "attribute", "value"},
					},
				},
			},
			Required: []string{"path", "operations"},
		},
	}
}

func (t *ApplyAVUOperations) GetHandler() mcp.ToolHandler {
	return t.Handler
}

func (t *ApplyAVUOperations) GetAccessiblePaths(authValue *common.AuthValue) []string {
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

func (t *ApplyAVUOperations) Handler(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// arguments
	args := ApplyAVUOperationsInputArgs{}
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

	args.Path = irods_common.MakeIRODSPath(t.config, fs.GetAccount(), args.Path)

	// check permission
	if !irods_common.IsAccessAllowed(args.Path, t.GetAccessiblePaths(&authValue)) {
		outputErr := errors.Newf("%q request is not permitted for path %q", t.GetName(), args.Path)
		return irods_common.ToolErrorResult(outputErr), nil
	}

	// Apply AVU operations
	content, err := t.applyAVUOperations(fs, args.Path, args.Operations)
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to apply AVU operations to %q", args.Path)
		return irods_common.ToolErrorResult(outputErr), nil
	}

	return irods_common.ToolJSONResult(*content)
}

func (t *ApplyAVUOperations) applyAVUOperations(fs *irodsclient_fs.FileSystem, path string, operations []ApplyAVUOperationsInputOperation) (*model.ApplyAVUOperationsOutput, error) {
	if len(operations) == 0 {
		return nil, errors.New("operations must contain one or more AVU operations")
	}

	if !fs.Exists(path) {
		return nil, errors.Newf("path %q does not exist", path)
	}

	metaOperations := make([]irodsclient_types.IRODSMetaOperation, 0, len(operations))
	outputMetaOperations := make([]model.AVUOperationOutput, 0, len(operations))

	for _, operation := range operations {
		metaOperation := irodsclient_types.IRODSMetaOperation{
			Operation: irodsclient_types.IRODSMetaOperationType(operation.Operation),
			Attribute: operation.Attribute,
			Value:     operation.Value,
			Units:     operation.Units,
		}

		outputMetaOperation := model.AVUOperationOutput{
			Operation: operation.Operation,
			Attribute: operation.Attribute,
		}

		metaOperations = append(metaOperations, metaOperation)

		outputMetaOperations = append(outputMetaOperations, outputMetaOperation)
	}

	err := fs.ApplyMetadataOperations(path, metaOperations)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to apply AVU operations to path %q", path)
	}

	applyAVUOperationsOutput := &model.ApplyAVUOperationsOutput{
		Path:       path,
		Operations: outputMetaOperations,
	}

	return applyAVUOperationsOutput, nil
}
