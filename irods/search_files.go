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
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	SearchFilesName = irods_common.IRODSAPIPrefix + "search_files"
)

type SearchFiles struct {
	mcpServer *IRODSMCPServer
	config    *common.Config
}

func NewSearchFiles(svr *IRODSMCPServer) ToolAPI {
	return &SearchFiles{
		mcpServer: svr,
		config:    svr.GetConfig(),
	}
}

func (t *SearchFiles) GetName() string {
	return SearchFilesName
}

func (t *SearchFiles) GetDescription() string {
	return `Recursively search for files (data-objects) and directories (collections) matching a pattern.
	The specified search root path must be an iRODS path. Use unix wildcards, such as '?' and '*', for the search pattern. 
	The matching entries are returned in JSON format.`
}

func (t *SearchFiles) GetTool() mcp.Tool {
	return mcp.NewTool(
		t.GetName(),
		mcp.WithDescription(t.GetDescription()),
		mcp.WithString(
			"path",
			mcp.Required(),
			mcp.Description("The search path, which may include wildcard patterns such as '?' and '*'."),
		),
	)
}

func (t *SearchFiles) GetHandler() server.ToolHandlerFunc {
	return t.Handler
}

func (t *SearchFiles) GetAccessiblePaths(authValue *common.AuthValue) []string {
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

func (t *SearchFiles) Handler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	arguments := request.GetArguments()

	inputPath, err := irods_common.GetInputStringArgument(arguments, "path", true)
	if err != nil {
		outputErr := errors.New("failed to get path from arguments")
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

	irodsPath := irods_common.MakeIRODSPath(t.config, fs.GetAccount(), inputPath)

	// check permission
	// check first wildcard location
	wildIdx := strings.IndexAny(irodsPath, "?*")
	if wildIdx >= 0 {
		irodsRootPath := irodsPath[:wildIdx]
		irodsRootPath = irods_common.GetDir(irodsRootPath)

		if !irods_common.IsAccessAllowed(irodsRootPath, t.GetAccessiblePaths(&authValue)) {
			outputErr := errors.Newf("%q request is not permitted for path %q", t.GetName(), irodsRootPath)
			return irods_common.OutputMCPError(outputErr)
		}
	} else {
		// no wildcard return error
		outputErr := errors.Newf("no wildcard is in the path %q", irodsPath)
		return irods_common.OutputMCPError(outputErr)
	}

	// search
	content, err := t.search(fs, irodsPath)
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to search files (data-objects) or directories (collections) matching %q", irodsPath)
		return irods_common.OutputMCPError(outputErr)
	}

	return mcp.NewToolResultText(content), nil
}

func (t *SearchFiles) search(fs *irodsclient_fs.FileSystem, searchPath string) (string, error) {
	outputEntries := []model.EntryWithAccess{}

	dirEntries, err := fs.SearchDirUnixWildcard(searchPath)
	if err != nil {
		return "", errors.Wrapf(err, "failed to search directories (collections) %q", searchPath)
	}

	fileEntries, err := fs.SearchFileUnixWildcard(searchPath)
	if err != nil {
		return "", errors.Wrapf(err, "failed to search files (data-objects) %q", searchPath)
	}

	for _, dirEntry := range dirEntries {
		entryStruct := model.EntryWithAccess{
			Entry:       dirEntry,
			ResourceURI: irods_common.MakeResourceURI(dirEntry.Path),
			WebDAVURI:   irods_common.MakeWebdavURL(t.config, dirEntry.Path, fs.GetAccount()),
		}

		outputEntries = append(outputEntries, entryStruct)
	}

	for _, fileEntry := range fileEntries {
		entryStruct := model.EntryWithAccess{
			Entry:       fileEntry,
			ResourceURI: irods_common.MakeResourceURI(fileEntry.Path),
			WebDAVURI:   irods_common.MakeWebdavURL(t.config, fileEntry.Path, fs.GetAccount()),
		}

		outputEntries = append(outputEntries, entryStruct)
	}

	searchFilesOutput := model.SearchFilesOutput{
		SearchPath:      searchPath,
		MatchingEntries: outputEntries,
	}

	jsonBytes, err := json.Marshal(searchFilesOutput)
	if err != nil {
		return "", errors.Wrapf(err, "failed to marshal JSON")
	}

	return string(jsonBytes), nil
}
