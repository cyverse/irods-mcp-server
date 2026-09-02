package irods

import (
	"context"
	"strings"

	"github.com/cockroachdb/errors"
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	"github.com/cyverse/irods-mcp-server/common"
	irods_common "github.com/cyverse/irods-mcp-server/irods/common"
	"github.com/cyverse/irods-mcp-server/irods/model"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	SearchFilesName = irods_common.IRODSAPIPrefix + "search_files"
)

type SearchFilesInputArgs struct {
	Path  string `json:"path"`
	Limit int    `json:"limit,omitempty"`
}

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

func (t *SearchFiles) GetTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        t.GetName(),
		Description: t.GetDescription(),
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"path": {
					Type:        "string",
					Description: "The search path, which may include wildcard patterns such as '?' and '*'. Exact paths without wildcards are also supported.",
				},
				"limit": {
					Type:        "number",
					Description: "Maximum number of entries to return. Default: 100, max: 500.",
				},
			},
			Required: []string{"path"},
		},
	}
}

func (t *SearchFiles) GetHandler() mcp.ToolHandler {
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
		sharedPath,
		sharedPath + "/*",
	}

	if !account.IsAnonymousUser() {
		paths = append(paths, homePath)
		paths = append(paths, homePath+"/*")
	}

	return paths
}

func (t *SearchFiles) Handler(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// arguments
	args := SearchFilesInputArgs{}
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

	// check permission using root path (portion before first wildcard, or full path for exact search)
	wildIdx := strings.IndexAny(irodsPath, "?*")
	var irodsRootPath string
	if wildIdx >= 0 {
		irodsRootPath = irods_common.GetDir(irodsPath[:wildIdx])
	} else {
		irodsRootPath = irods_common.GetDir(irodsPath)
	}
	if !irods_common.IsAccessAllowed(irodsRootPath, t.GetAccessiblePaths(&authValue)) {
		outputErr := errors.Newf("%q request is not permitted for path %q", t.GetName(), irodsRootPath)
		return irods_common.ToolErrorResult(outputErr), nil
	}

	limit := args.Limit
	if limit <= 0 {
		limit = 100
	} else if limit > 500 {
		limit = 500
	}

	// search
	content, err := t.search(fs, irodsPath, limit)
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to search files (data-objects) or directories (collections) matching %q", irodsPath)
		return irods_common.ToolErrorResult(outputErr), nil
	}

	return irods_common.ToolJSONResult(*content)
}

func (t *SearchFiles) search(fs *irodsclient_fs.FileSystem, searchPath string, limit int) (*model.SearchFilesOutput, error) {
	outputEntries := []model.EntryWithAccess{}

	dirEntries, err := fs.SearchDirUnixWildcard(searchPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to search directories (collections) %q", searchPath)
	}

	fileEntries, err := fs.SearchFileUnixWildcard(searchPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to search files (data-objects) %q", searchPath)
	}

	for _, dirEntry := range dirEntries {
		if len(outputEntries) >= limit {
			break
		}
		outputEntries = append(outputEntries, model.EntryWithAccess{
			Entry:       dirEntry,
			ResourceURI: irods_common.MakeResourceURI(dirEntry.Path),
			WebDAVURI:   irods_common.MakeWebdavURL(t.config, dirEntry.Path, fs.GetAccount()),
		})
	}

	for _, fileEntry := range fileEntries {
		if len(outputEntries) >= limit {
			break
		}
		outputEntries = append(outputEntries, model.EntryWithAccess{
			Entry:       fileEntry,
			ResourceURI: irods_common.MakeResourceURI(fileEntry.Path),
			WebDAVURI:   irods_common.MakeWebdavURL(t.config, fileEntry.Path, fs.GetAccount()),
		})
	}

	searchFilesOutput := &model.SearchFilesOutput{
		SearchPath:      searchPath,
		MatchingEntries: outputEntries,
	}

	return searchFilesOutput, nil
}
