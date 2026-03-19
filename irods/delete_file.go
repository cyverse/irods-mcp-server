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
	DeleteFileName = irods_common.IRODSAPIPrefix + "delete_file"
)

type DeleteFileInputArgs struct {
	Path string `json:"path"`
}

type DeleteFile struct {
	mcpServer *IRODSMCPServer
	config    *common.Config
}

func NewDeleteFile(svr *IRODSMCPServer) ToolAPI {
	return &DeleteFile{
		mcpServer: svr,
		config:    svr.GetConfig(),
	}
}

func (t *DeleteFile) GetName() string {
	return DeleteFileName
}

func (t *DeleteFile) GetDescription() string {
	return `Delete a file (data-object) or directory (collection).`
}

func (t *DeleteFile) GetTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        t.GetName(),
		Description: t.GetDescription(),
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"path": {
					Type:        "string",
					Description: "The path to the file (data-object) or directory (collection) to delete.",
				},
			},
			Required: []string{"path"},
		},
	}
}

func (t *DeleteFile) GetHandler() mcp.ToolHandler {
	return t.Handler
}

func (t *DeleteFile) GetAccessiblePaths(authValue *common.AuthValue) []string {
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

func (t *DeleteFile) Handler(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// arguments
	args := DeleteFileInputArgs{}
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

	// Delete file
	targetEntry, err := fs.Stat(irodsPath)
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to stat file or directory info for %q", irodsPath)
		return irods_common.ToolErrorResult(outputErr), nil
	}

	content, err := t.deleteFile(fs, targetEntry)
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to delete file (data-object) or directory (collection) %q", irodsPath)
		return irods_common.ToolErrorResult(outputErr), nil
	}

	return irods_common.ToolJSONResult(*content)
}

func (t *DeleteFile) deleteFile(fs *irodsclient_fs.FileSystem, targetEntry *irodsclient_fs.Entry) (*model.RemoveFileOutput, error) {
	if targetEntry.IsDir() {
		// dir
		err := fs.RemoveDir(targetEntry.Path, true, true)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to delete directory (collection) %q", targetEntry.Path)
		}
	} else {
		// file
		err := fs.RemoveFile(targetEntry.Path, true)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to delete file (data-object) %q", targetEntry.Path)
		}
	}

	fileRemoveOutput := &model.RemoveFileOutput{
		Path:      targetEntry.Path,
		EntryInfo: targetEntry,
	}

	return fileRemoveOutput, nil
}
