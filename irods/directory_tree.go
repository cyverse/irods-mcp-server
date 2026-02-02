package irods

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cockroachdb/errors"
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/irods-mcp-server/common"
	irods_common "github.com/cyverse/irods-mcp-server/irods/common"
	"github.com/cyverse/irods-mcp-server/irods/model"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	DirectoryTreeName = irods_common.IRODSAPIPrefix + "directory_tree"
)

type DirectoryTree struct {
	mcpServer *IRODSMCPServer
	config    *common.Config
}

func NewDirectoryTree(svr *IRODSMCPServer) ToolAPI {
	return &DirectoryTree{
		mcpServer: svr,
		config:    svr.GetConfig(),
	}
}

func (t *DirectoryTree) GetName() string {
	return DirectoryTreeName
}

func (t *DirectoryTree) GetDescription() string {
	return `Get a recursive tree view of files (data-objects) and directories (collections).
	The specified path must be an iRODS path. The output is in JSON format.
	The output contains all entries in the given directory (collection) path.`
}

func (t *DirectoryTree) GetTool() mcp.Tool {
	return mcp.NewTool(
		t.GetName(),
		mcp.WithDescription(t.GetDescription()),
		mcp.WithString(
			"path",
			mcp.Required(),
			mcp.Description("The path to the directory (collection) to list"),
		),
		mcp.WithNumber(
			"depth",
			mcp.DefaultNumber(float64(irods_common.DefaultTreeScanMaxDepth)),
			mcp.Description(fmt.Sprintf("The depth of the directory tree to list. Default value is %d. Depth must be greater than or equal to 1. Depth must not be too large, otherwise the output may be too large. Maximum value is %d.", irods_common.DefaultTreeScanMaxDepth, irods_common.MaxTreeScanDepth)),
		),
	)
}

func (t *DirectoryTree) GetHandler() server.ToolHandlerFunc {
	return t.Handler
}

func (t *DirectoryTree) GetAccessiblePaths(authValue *common.AuthValue) []string {
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

func (t *DirectoryTree) Handler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	arguments := request.GetArguments()

	inputPath, ok := arguments["path"].(string)
	if !ok {
		return nil, errors.New("failed to get path from arguments")
	}

	inputDepthFloat, ok := arguments["depth"].(float64)
	if !ok {
		// default value
		inputDepthFloat = float64(irods_common.DefaultTreeScanMaxDepth)
	}

	inputDepth := int(inputDepthFloat)
	if inputDepth < 0 {
		inputDepth = 1
	} else if inputDepth > irods_common.MaxTreeScanDepth {
		inputDepth = irods_common.MaxTreeScanDepth
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

	irodsPath := irods_common.MakeIRODSPath(t.config, fs.GetAccount(), inputPath)

	// check permission
	if !irods_common.IsAccessAllowed(irodsPath, t.GetAccessiblePaths(&authValue)) {
		outputErr := errors.Newf("%q request is not permitted for path %q", t.GetName(), irodsPath)
		return irods_common.OutputMCPError(outputErr)
	}

	// list
	sourceEntry, err := fs.Stat(irodsPath)
	if err != nil {
		if !irodsclient_types.IsFileNotFoundError(err) {
			outputErr := errors.Wrapf(err, "failed to find a directory (collection) %q", irodsPath)
			return irods_common.OutputMCPError(outputErr)
		}

		outputErr := errors.Wrapf(err, "failed to stat %q", irodsPath)
		return irods_common.OutputMCPError(outputErr)
	}

	if !sourceEntry.IsDir() {
		outputErr := errors.Newf("path %q is not a directory (collection)", irodsPath)
		return irods_common.OutputMCPError(outputErr)
	}

	// collection
	content, err := t.listCollectionRecursively(fs, sourceEntry, inputDepth)
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to list a directory (collection) %q", irodsPath)
		return irods_common.OutputMCPError(outputErr)
	}

	return mcp.NewToolResultText(content), nil
}

func (t *DirectoryTree) listCollectionRecursively(fs *irodsclient_fs.FileSystem, sourceEntry *irodsclient_fs.Entry, maxDepth int) (string, error) {
	outputEntries, err := t.listCollectionRecursivelyInternal(fs, sourceEntry, 1, maxDepth)
	if err != nil {
		return "", errors.Wrapf(err, "failed to list directory (collection) recursively %q", sourceEntry.Path)
	}

	listDirectoryOutput := model.ListDirectoryOutput{
		Directory:            sourceEntry,
		DirectoryResourceURI: irods_common.MakeResourceURI(sourceEntry.Path),
		DirectoryWebDAVURI:   irods_common.MakeWebdavURL(t.config, sourceEntry.Path, fs.GetAccount()),
		DirectoryEntries:     outputEntries,
	}

	jsonBytes, err := json.Marshal(listDirectoryOutput)
	if err != nil {
		return "", errors.Wrapf(err, "failed to marshal JSON")
	}

	return string(jsonBytes), nil
}

func (t *DirectoryTree) listCollectionRecursivelyInternal(fs *irodsclient_fs.FileSystem, sourceEntry *irodsclient_fs.Entry, curDepth int, maxDepth int) ([]model.EntryWithAccess, error) {
	outputEntries := []model.EntryWithAccess{}

	dirEntries, err := fs.List(sourceEntry.Path)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to list directory (collection) %q", sourceEntry.Path)
	}

	for _, dirEntry := range dirEntries {
		var subEntries []model.EntryWithAccess = nil
		if dirEntry.IsDir() && curDepth+1 <= maxDepth {
			subEntries, err = t.listCollectionRecursivelyInternal(fs, dirEntry, curDepth+1, maxDepth)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to list directory (collection) recursively %q", dirEntry.Path)
			}
		}

		entryStruct := model.EntryWithAccess{
			Entry:            dirEntry,
			ResourceURI:      irods_common.MakeResourceURI(dirEntry.Path),
			WebDAVURI:        irods_common.MakeWebdavURL(t.config, dirEntry.Path, fs.GetAccount()),
			DirectoryEntries: subEntries,
		}

		outputEntries = append(outputEntries, entryStruct)
	}

	return outputEntries, nil
}
