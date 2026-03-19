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
	MoveFileName = irods_common.IRODSAPIPrefix + "move_file"
)

type MoveFileInputArgs struct {
	OldPath string `json:"old_path"`
	NewPath string `json:"new_path"`
}

type MoveFile struct {
	mcpServer *IRODSMCPServer
	config    *common.Config
}

func NewMoveFile(svr *IRODSMCPServer) ToolAPI {
	return &MoveFile{
		mcpServer: svr,
		config:    svr.GetConfig(),
	}
}

func (t *MoveFile) GetName() string {
	return MoveFileName
}

func (t *MoveFile) GetDescription() string {
	return `Move a file (data-object) or directory (collection) to a new location.`
}

func (t *MoveFile) GetTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        t.GetName(),
		Description: t.GetDescription(),
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"old_path": {
					Type:        "string",
					Description: "The old path to the file (data-object) or directory (collection).",
				},
				"new_path": {
					Type:        "string",
					Description: "The new, complete path to move the file (data-object) or directory (collection) to, including its new name. The path must not already exist.",
				},
			},
			Required: []string{"old_path", "new_path"},
		},
	}
}

func (t *MoveFile) GetHandler() mcp.ToolHandler {
	return t.Handler
}

func (t *MoveFile) GetAccessiblePaths(authValue *common.AuthValue) []string {
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

func (t *MoveFile) Handler(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// arguments
	args := MoveFileInputArgs{}
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

	irodsOldPath := irods_common.MakeIRODSPath(t.config, fs.GetAccount(), args.OldPath)
	irodsNewPath := irods_common.MakeIRODSPath(t.config, fs.GetAccount(), args.NewPath)

	// check permission
	if !irods_common.IsAccessAllowed(irodsOldPath, t.GetAccessiblePaths(&authValue)) {
		outputErr := errors.Newf("%q request is not permitted for path %q", t.GetName(), irodsOldPath)
		return irods_common.ToolErrorResult(outputErr), nil
	}
	if !irods_common.IsAccessAllowed(irodsNewPath, t.GetAccessiblePaths(&authValue)) {
		outputErr := errors.Newf("%q request is not permitted for path %q", t.GetName(), irodsNewPath)
		return irods_common.ToolErrorResult(outputErr), nil
	}

	// Move file
	sourceEntry, err := fs.Stat(irodsOldPath)
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to stat file or directory info for %q", irodsOldPath)
		return irods_common.ToolErrorResult(outputErr), nil
	}

	content, err := t.moveFile(fs, sourceEntry, irodsNewPath)
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to move file (data-object) or directory (collection) from %q to %q", irodsOldPath, irodsNewPath)
		return irods_common.ToolErrorResult(outputErr), nil
	}

	return irods_common.ToolJSONResult(*content)
}

func (t *MoveFile) moveFile(fs *irodsclient_fs.FileSystem, sourceEntry *irodsclient_fs.Entry, newPath string) (*model.MoveFileOutput, error) {
	if sourceEntry.IsDir() {
		// dir
		err := fs.RenameDirToDir(sourceEntry.Path, newPath)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to move directory (collection) from %q to %q", sourceEntry.Path, newPath)
		}
	} else {
		// file
		err := fs.RenameFileToFile(sourceEntry.Path, newPath)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to move file (data-object) from %q to %q", sourceEntry.Path, newPath)
		}
	}

	destEntry, err := fs.Stat(newPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to stat file or directory info for %q", newPath)
	}

	fileMoveOutput := &model.MoveFileOutput{
		OldPath:      sourceEntry.Path,
		OldEntryInfo: sourceEntry,
		NewPath:      newPath,
		NewEntryInfo: destEntry,
	}

	return fileMoveOutput, nil
}
