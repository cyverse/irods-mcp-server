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
	SearchFilesByAVUName = irods_common.IRODSAPIPrefix + "search_files_by_avu"
)

type SearchFilesByAVU struct {
	mcpServer *IRODSMCPServer
	config    *common.Config
}

func NewSearchFilesByAVU(svr *IRODSMCPServer) ToolAPI {
	return &SearchFilesByAVU{
		mcpServer: svr,
		config:    svr.GetConfig(),
	}
}

func (t *SearchFilesByAVU) GetName() string {
	return SearchFilesByAVUName
}

func (t *SearchFilesByAVU) GetDescription() string {
	return `Search for files (data-objects) and directories (collections) matching iRODS AVU (attribute-value-units) using specified attribute and value.
	The matching entries are returned in JSON format.`
}

func (t *SearchFilesByAVU) GetTool() mcp.Tool {
	return mcp.NewTool(
		t.GetName(),
		mcp.WithDescription(t.GetDescription()),
		mcp.WithString(
			"attribute",
			mcp.Required(),
			mcp.Description("The attribute to search for."),
		),
		mcp.WithString(
			"value",
			mcp.Required(),
			mcp.Description("The value of the attribute to search for."),
		),
	)
}

func (t *SearchFilesByAVU) GetHandler() server.ToolHandlerFunc {
	return t.Handler
}

func (t *SearchFilesByAVU) GetAccessiblePaths(authValue *common.AuthValue) []string {
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

func (t *SearchFilesByAVU) Handler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	arguments := request.GetArguments()

	attribute, ok := arguments["attribute"].(string)
	if !ok {
		return nil, errors.New("failed to get attribute from arguments")
	}

	value, ok := arguments["value"].(string)
	if !ok {
		return nil, errors.New("failed to get value from arguments")
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

	accessiblePaths := t.GetAccessiblePaths(&authValue)

	// search
	content, err := t.search(fs, accessiblePaths, attribute, value)
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to search files (data-objects) or directories (collections) matching attribute %q and value %q", attribute, value)
		return irods_common.OutputMCPError(outputErr)
	}

	return mcp.NewToolResultText(content), nil
}

func (t *SearchFilesByAVU) search(fs *irodsclient_fs.FileSystem, accessiblePaths []string, attribute string, value string) (string, error) {
	outputEntries := []model.EntryWithAccess{}

	entries, err := fs.SearchByMeta(attribute, value)
	if err != nil {
		return "", errors.Wrapf(err, "failed to search by AVU %q=%q", attribute, value)
	}

	for _, entry := range entries {
		// check permission
		// filter out entries not in accessible paths
		if irods_common.IsAccessAllowed(entry.Path, accessiblePaths) {
			entryStruct := model.EntryWithAccess{
				Entry:       entry,
				ResourceURI: irods_common.MakeResourceURI(entry.Path),
				WebDAVURI:   irods_common.MakeWebdavURL(t.config, entry.Path, fs.GetAccount()),
			}

			outputEntries = append(outputEntries, entryStruct)
		}
	}

	searchFilesOutput := model.SearchFilesByAVUOutput{
		SearchAttribute: attribute,
		SearchValue:     value,
		MatchingEntries: outputEntries,
	}

	jsonBytes, err := json.Marshal(searchFilesOutput)
	if err != nil {
		return "", errors.Wrapf(err, "failed to marshal JSON")
	}

	return string(jsonBytes), nil
}
