package irods

import (
	"context"

	"github.com/cockroachdb/errors"
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	"github.com/cyverse/irods-mcp-server/common"
	irods_common "github.com/cyverse/irods-mcp-server/irods/common"
	"github.com/cyverse/irods-mcp-server/irods/model"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	MakeDirectoryName = irods_common.IRODSAPIPrefix + "make_directory"
)

type MakeDirectoryInputArgs struct {
	Path string `json:"path"`
}

type MakeDirectory struct {
	mcpServer *IRODSMCPServer
	config    *common.Config
}

func NewMakeDirectory(svr *IRODSMCPServer) ToolAPI {
	return &MakeDirectory{
		mcpServer: svr,
		config:    svr.GetConfig(),
	}
}

func (t *MakeDirectory) GetName() string {
	return MakeDirectoryName
}

func (t *MakeDirectory) GetDescription() string {
	return `Make a new directory (collection).`
}

func (t *MakeDirectory) GetTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        t.GetName(),
		Description: t.GetDescription(),
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"path": {
					Type:        "string",
					Description: "The path to the new directory to create.",
				},
			},
			Required: []string{"path"},
		},
	}
}

func (t *MakeDirectory) GetHandler() mcp.ToolHandler {
	return t.Handler
}

func (t *MakeDirectory) GetAccessiblePaths(authValue *common.AuthValue) []string {
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

func (t *MakeDirectory) Handler(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// arguments
	args := MakeDirectoryInputArgs{}
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

	// Make directory
	content, err := t.makeDirectory(fs, irodsPath)
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to make directory (collection) for %q", irodsPath)
		return irods_common.ToolErrorResult(outputErr), nil
	}

	return irods_common.ToolJSONResult(*content)
}

func (t *MakeDirectory) makeDirectory(fs *irodsclient_fs.FileSystem, path string) (*model.MakeDirectoryOutput, error) {
	err := fs.MakeDir(path, true)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to make directory (collection) for %q", path)
	}

	destEntry, err := fs.Stat(path)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to stat file or directory info for %q", path)
	}

	makeDirectoryOutput := &model.MakeDirectoryOutput{
		Path:      path,
		EntryInfo: destEntry,
	}

	return makeDirectoryOutput, nil
}
