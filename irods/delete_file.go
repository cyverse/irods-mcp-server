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
	DeleteFileName = irods_common.IRODSAPIPrefix + "delete_file"
)

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

func (t *DeleteFile) GetTool() mcp.Tool {
	return mcp.NewTool(
		t.GetName(),
		mcp.WithDescription(t.GetDescription()),
		mcp.WithString(
			"path",
			mcp.Required(),
			mcp.Description("The path to the file (data-object) or directory (collection) to delete."),
		),
	)
}

func (t *DeleteFile) GetHandler() server.ToolHandlerFunc {
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

func (t *DeleteFile) Handler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	arguments := request.GetArguments()

	path, ok := arguments["path"].(string)
	if !ok {
		return nil, errors.New("failed to get path from arguments")
	}

	// auth
	authValue, err := common.GetAuthValue(ctx)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get auth value")
	}

	// make a irods filesystem client
	fs, err := t.mcpServer.GetIRODSFSClientFromAuthValue(&authValue)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create a irods fs client")
	}

	irodsPath := irods_common.MakeIRODSPath(t.config, fs.GetAccount(), path)

	// check permission
	if !irods_common.IsAccessAllowed(irodsPath, t.GetAccessiblePaths(&authValue)) {
		outputErr := errors.Newf("%q request is not permitted for path %q", t.GetName(), irodsPath)
		return irods_common.OutputMCPError(outputErr)
	}

	// Delete file
	targetEntry, err := fs.Stat(irodsPath)
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to stat file or directory info for %q", irodsPath)
		return irods_common.OutputMCPError(outputErr)
	}

	content, err := t.deleteFile(fs, targetEntry)
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to delete file (data-object) or directory (collection) %q", irodsPath)
		return irods_common.OutputMCPError(outputErr)
	}

	return mcp.NewToolResultText(content), nil
}

func (t *DeleteFile) deleteFile(fs *irodsclient_fs.FileSystem, targetEntry *irodsclient_fs.Entry) (string, error) {
	if targetEntry.IsDir() {
		// dir
		err := fs.RemoveDir(targetEntry.Path, true, true)
		if err != nil {
			return "", errors.Wrapf(err, "failed to delete directory (collection) %q", targetEntry.Path)
		}
	} else {
		// file
		err := fs.RemoveFile(targetEntry.Path, true)
		if err != nil {
			return "", errors.Wrapf(err, "failed to delete file (data-object) %q", targetEntry.Path)
		}
	}

	fileRemoveOutput := model.RemoveFileOutput{
		Path:      targetEntry.Path,
		EntryInfo: targetEntry,
	}

	jsonBytes, err := json.Marshal(fileRemoveOutput)
	if err != nil {
		return "", errors.Wrapf(err, "failed to marshal JSON")
	}

	return string(jsonBytes), nil
}
