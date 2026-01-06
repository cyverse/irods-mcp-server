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
	MakeDirectoryName = irods_common.IRODSAPIPrefix + "make_directory"
)

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

func (t *MakeDirectory) GetTool() mcp.Tool {
	return mcp.NewTool(
		t.GetName(),
		mcp.WithDescription(t.GetDescription()),
		mcp.WithString(
			"path",
			mcp.Required(),
			mcp.Description("The path to the new directory to create."),
		),
	)
}

func (t *MakeDirectory) GetHandler() server.ToolHandlerFunc {
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

func (t *MakeDirectory) Handler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	arguments := request.GetArguments()

	path, ok := arguments["path"].(string)
	if !ok {
		return nil, errors.Errorf("failed to get path from arguments")
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
		outputErr := errors.Errorf("%q request is not permitted for path %q", t.GetName(), irodsPath)
		return irods_common.OutputMCPError(outputErr)
	}

	// Make directory
	content, err := t.makeDirectory(fs, irodsPath)
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to make directory (collection) for %q", irodsPath)
		return irods_common.OutputMCPError(outputErr)
	}

	return mcp.NewToolResultText(content), nil
}

func (t *MakeDirectory) makeDirectory(fs *irodsclient_fs.FileSystem, path string) (string, error) {
	err := fs.MakeDir(path, true)
	if err != nil {
		return "", errors.Wrapf(err, "failed to make directory (collection) for %q", path)
	}

	destEntry, err := fs.Stat(path)
	if err != nil {
		return "", errors.Wrapf(err, "failed to stat file or directory info for %q", path)
	}

	makeDirectoryOutput := model.MakeDirectoryOutput{
		Path:      path,
		EntryInfo: destEntry,
	}

	jsonBytes, err := json.Marshal(makeDirectoryOutput)
	if err != nil {
		return "", errors.Wrapf(err, "failed to marshal JSON")
	}

	return string(jsonBytes), nil
}
