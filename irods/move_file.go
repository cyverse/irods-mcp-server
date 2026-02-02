package irods

import (
	"context"
	"encoding/json"

	"github.com/cockroachdb/errors"
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	"github.com/cyverse/irods-mcp-server/common"
	irods_common "github.com/cyverse/irods-mcp-server/irods/common"
	"github.com/cyverse/irods-mcp-server/irods/model"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	MoveFileName = irods_common.IRODSAPIPrefix + "move_file"
)

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

func (t *MoveFile) GetTool() mcp.Tool {
	return mcp.NewTool(
		t.GetName(),
		mcp.WithDescription(t.GetDescription()),
		mcp.WithString(
			"old_path",
			mcp.Required(),
			mcp.Description("The old path to the file (data-object) or directory (collection)"),
		),
		mcp.WithString(
			"new_path",
			mcp.Required(),
			mcp.Description("The new, complete path to move the file (data-object) or directory (collection) to, including its new name. The path must not already exist."),
		),
	)
}

func (t *MoveFile) GetHandler() server.ToolHandlerFunc {
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

func (t *MoveFile) Handler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	arguments := request.GetArguments()

	oldPath, ok := arguments["old_path"].(string)
	if !ok {
		outputErr := errors.New("failed to get old_path from arguments")
		return irods_common.OutputMCPError(outputErr)
	}
	newPath, ok := arguments["new_path"].(string)
	if !ok {
		outputErr := errors.New("failed to get new_path from arguments")
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

	irodsOldPath := irods_common.MakeIRODSPath(t.config, fs.GetAccount(), oldPath)
	irodsNewPath := irods_common.MakeIRODSPath(t.config, fs.GetAccount(), newPath)

	// check permission
	if !irods_common.IsAccessAllowed(irodsOldPath, t.GetAccessiblePaths(&authValue)) {
		outputErr := errors.Newf("%q request is not permitted for path %q", t.GetName(), irodsOldPath)
		return irods_common.OutputMCPError(outputErr)
	}
	if !irods_common.IsAccessAllowed(irodsNewPath, t.GetAccessiblePaths(&authValue)) {
		outputErr := errors.Newf("%q request is not permitted for path %q", t.GetName(), irodsNewPath)
		return irods_common.OutputMCPError(outputErr)
	}

	// Move file
	sourceEntry, err := fs.Stat(irodsOldPath)
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to stat file or directory info for %q", irodsOldPath)
		return irods_common.OutputMCPError(outputErr)
	}

	content, err := t.moveFile(fs, sourceEntry, irodsNewPath)
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to move file (data-object) or directory (collection) from %q to %q", irodsOldPath, irodsNewPath)
		return irods_common.OutputMCPError(outputErr)
	}

	return mcp.NewToolResultText(content), nil
}

func (t *MoveFile) moveFile(fs *irodsclient_fs.FileSystem, sourceEntry *irodsclient_fs.Entry, newPath string) (string, error) {
	if sourceEntry.IsDir() {
		// dir
		err := fs.RenameDirToDir(sourceEntry.Path, newPath)
		if err != nil {
			return "", errors.Wrapf(err, "failed to move directory (collection) from %q to %q", sourceEntry.Path, newPath)
		}
	} else {
		// file
		err := fs.RenameFileToFile(sourceEntry.Path, newPath)
		if err != nil {
			return "", errors.Wrapf(err, "failed to move file (data-object) from %q to %q", sourceEntry.Path, newPath)
		}
	}

	destEntry, err := fs.Stat(newPath)
	if err != nil {
		return "", errors.Wrapf(err, "failed to stat file or directory info for %q", newPath)
	}

	fileMoveOutput := model.MoveFileOutput{
		OldPath:      sourceEntry.Path,
		OldEntryInfo: sourceEntry,
		NewPath:      newPath,
		NewEntryInfo: destEntry,
	}

	jsonBytes, err := json.Marshal(fileMoveOutput)
	if err != nil {
		return "", errors.Wrapf(err, "failed to marshal JSON")
	}

	return string(jsonBytes), nil
}
