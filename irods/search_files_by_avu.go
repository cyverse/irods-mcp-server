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
	SearchFilesByAVUName = irods_common.IRODSAPIPrefix + "search_files_by_avu"
)

type SearchFilesByAVUInputArgs struct {
	Attribute string `json:"attribute"`
	Value     string `json:"value"`
}

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

func (t *SearchFilesByAVU) GetTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        t.GetName(),
		Description: t.GetDescription(),
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"attribute": {
					Type:        "string",
					Description: "The attribute to search for.",
				},
				"value": {
					Type:        "string",
					Description: "The value of the attribute to search for.",
				},
			},
			Required: []string{"attribute", "value"},
		},
	}
}

func (t *SearchFilesByAVU) GetHandler() mcp.ToolHandler {
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

func (t *SearchFilesByAVU) Handler(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// arguments
	args := SearchFilesByAVUInputArgs{}
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

	accessiblePaths := t.GetAccessiblePaths(&authValue)

	// search
	content, err := t.search(fs, accessiblePaths, args.Attribute, args.Value)
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to search files (data-objects) or directories (collections) matching attribute %q and value %q", args.Attribute, args.Value)
		return irods_common.ToolErrorResult(outputErr), nil
	}

	return irods_common.ToolJSONResult(*content)
}

func (t *SearchFilesByAVU) search(fs *irodsclient_fs.FileSystem, accessiblePaths []string, attribute string, value string) (*model.SearchFilesByAVUOutput, error) {
	outputEntries := []model.EntryWithAccess{}

	entries, err := fs.SearchByMeta(attribute, value)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to search by AVU %q=%q", attribute, value)
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

	searchFilesOutput := &model.SearchFilesByAVUOutput{
		SearchAttribute: attribute,
		SearchValue:     value,
		MatchingEntries: outputEntries,
	}

	return searchFilesOutput, nil
}
