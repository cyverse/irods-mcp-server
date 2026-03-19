package irods

import (
	"context"

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
	ListDirectoryName = irods_common.IRODSAPIPrefix + "list_directory"
)

type ListDirectoryInputArgs struct {
	Path string `json:"path"`
}

type ListDirectory struct {
	mcpServer *IRODSMCPServer
	config    *common.Config
}

func NewListDirectory(svr *IRODSMCPServer) ToolAPI {
	return &ListDirectory{
		mcpServer: svr,
		config:    svr.GetConfig(),
	}
}

func (t *ListDirectory) GetName() string {
	return ListDirectoryName
}

func (t *ListDirectory) GetDescription() string {
	return `Get a list of files (data-objects) and directories (collections) in a specified path.
	The specified path must be an iRODS path. The output is in JSON format.
	The output contains entries in the given directory (collection) path.`
}

func (t *ListDirectory) GetTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        t.GetName(),
		Description: t.GetDescription(),
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"path": {
					Type:        "string",
					Description: "The path to the directory (collection) to list.",
				},
			},
			Required: []string{"path"},
		},
	}
}

func (t *ListDirectory) GetHandler() mcp.ToolHandler {
	return t.Handler
}

func (t *ListDirectory) GetAccessiblePaths(authValue *common.AuthValue) []string {
	account, err := t.mcpServer.GetIRODSAccountFromAuthValue(authValue)
	if err != nil {
		return []string{}
	}

	homePath := irods_common.GetHomePath(t.config, account)
	sharedPath := irods_common.GetSharedPath(t.config, account)

	paths := []string{
		sharedPath,
		sharedPath + "/*",
	}

	if !account.IsAnonymousUser() {
		paths = append(paths, homePath)
		paths = append(paths, homePath+"/*")
	}

	return paths
}

func (t *ListDirectory) Handler(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// arguments
	args := ListDirectoryInputArgs{}
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

	// list
	sourceEntry, err := fs.Stat(irodsPath)
	if err != nil {
		if !irodsclient_types.IsFileNotFoundError(err) {
			outputErr := errors.Wrapf(err, "failed to find a directory (collection) %q", irodsPath)
			return irods_common.ToolErrorResult(outputErr), nil
		}

		outputErr := errors.Wrapf(err, "failed to stat %q", irodsPath)
		return irods_common.ToolErrorResult(outputErr), nil
	}

	if !sourceEntry.IsDir() {
		outputErr := errors.Newf("path %q is not a directory (collection)", irodsPath)
		return irods_common.ToolErrorResult(outputErr), nil
	}

	// collection
	content, err := t.listCollection(fs, sourceEntry)
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to list a directory (collection) %q", irodsPath)
		return irods_common.ToolErrorResult(outputErr), nil
	}

	return irods_common.ToolJSONResult(*content)
}

func (t *ListDirectory) listCollection(fs *irodsclient_fs.FileSystem, sourceEntry *irodsclient_fs.Entry) (*model.ListDirectoryOutput, error) {
	outputEntries := []model.EntryWithAccess{}

	dirEntries, err := fs.List(sourceEntry.Path)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to list directory (collection) %q", sourceEntry.Path)
	}

	for _, dirEntry := range dirEntries {
		entryStruct := model.EntryWithAccess{
			Entry:       dirEntry,
			ResourceURI: irods_common.MakeResourceURI(dirEntry.Path),
			WebDAVURI:   irods_common.MakeWebdavURL(t.config, dirEntry.Path, fs.GetAccount()),
		}

		outputEntries = append(outputEntries, entryStruct)
	}

	listDirectoryOutput := &model.ListDirectoryOutput{
		Directory:            sourceEntry,
		DirectoryResourceURI: irods_common.MakeResourceURI(sourceEntry.Path),
		DirectoryWebDAVURI:   irods_common.MakeWebdavURL(t.config, sourceEntry.Path, fs.GetAccount()),
		DirectoryEntries:     outputEntries,
	}

	return listDirectoryOutput, nil
}
