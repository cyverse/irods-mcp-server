package irods

import (
	"context"
	"encoding/json"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/irods-mcp-server/common"
	irods_common "github.com/cyverse/irods-mcp-server/irods/common"
	"github.com/cyverse/irods-mcp-server/irods/model"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"golang.org/x/xerrors"
)

const (
	GetFileInfoName = irods_common.IRODSAPIPrefix + "get_file_info"
)

type GetFileInfo struct {
	mcpServer             *IRODSMCPServer
	config                *common.Config
	systemMetadataNameMap map[string]string
}

func NewGetFileInfo(svr *IRODSMCPServer) ToolAPI {
	systemMetadataNames := irods_common.GetSystemMetadataNames()
	systemMetadataNameMap := map[string]string{}
	for _, systemMetadataName := range systemMetadataNames {
		systemMetadataNameMap[systemMetadataName] = systemMetadataName
	}

	return &GetFileInfo{
		mcpServer:             svr,
		config:                svr.GetConfig(),
		systemMetadataNameMap: systemMetadataNameMap,
	}
}

func (t *GetFileInfo) GetName() string {
	return GetFileInfoName
}

func (t *GetFileInfo) GetDescription() string {
	return `Retrieve detailed metadata about a file or directory.`
}

func (t *GetFileInfo) GetTool() mcp.Tool {
	return mcp.NewTool(
		t.GetName(),
		mcp.WithDescription(t.GetDescription()),
		mcp.WithString(
			"path",
			mcp.Required(),
			mcp.Description("The path to the file (data-object) or directory (collection)"),
		),
	)
}

func (t *GetFileInfo) GetHandler() server.ToolHandlerFunc {
	return t.Handler
}

func (t *GetFileInfo) GetAccessiblePaths(authValue *common.AuthValue) []string {
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
		paths = append(paths, homePath+"/*")
	}

	return paths
}

func (t *GetFileInfo) Handler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	arguments := request.GetArguments()

	inputPath, ok := arguments["path"].(string)
	if !ok {
		return nil, xerrors.Errorf("failed to get path from arguments")
	}

	// auth
	authValue, err := common.GetAuthValue(ctx)
	if err != nil {
		return nil, xerrors.Errorf("failed to get auth value: %w", err)
	}

	// make a irods filesystem client
	fs, err := t.mcpServer.GetIRODSFSClientFromAuthValue(&authValue)
	if err != nil {
		return nil, xerrors.Errorf("failed to create a irods fs client: %w", err)
	}

	irodsPath := irods_common.MakeIRODSPath(t.config, fs.GetAccount(), inputPath)

	// check permission
	if !irods_common.IsAccessAllowed(irodsPath, t.GetAccessiblePaths(&authValue)) {
		outputErr := xerrors.Errorf("%q request is not permitted for path %q", t.GetName(), irodsPath)
		return irods_common.OutputMCPError(outputErr)
	}

	// Get file info
	sourceEntry, err := fs.Stat(irodsPath)
	if err != nil {
		outputErr := xerrors.Errorf("failed to stat file or directory info for %q: %w", irodsPath, err)
		return irods_common.OutputMCPError(outputErr)
	}

	content, err := t.getFileInfo(fs, sourceEntry)
	if err != nil {
		outputErr := xerrors.Errorf("failed to get file (data-object) info for %q: %w", irodsPath, err)
		return irods_common.OutputMCPError(outputErr)
	}

	return mcp.NewToolResultText(content), nil
}

func (t *GetFileInfo) getFileInfo(fs *irodsclient_fs.FileSystem, sourceEntry *irodsclient_fs.Entry) (string, error) {
	accesses, err := fs.ListACLs(sourceEntry.Path)
	if err != nil {
		return "", xerrors.Errorf("failed to list ACLs for %q: %w", sourceEntry.Path, err)
	}

	var accessInherit *irodsclient_types.IRODSAccessInheritance
	if sourceEntry.IsDir() {
		accessInherit, err = fs.GetDirACLInheritance(sourceEntry.Path)
		if err != nil {
			return "", xerrors.Errorf("failed to get access inheritance info for %q: %w", sourceEntry.Path, err)
		}
	}

	metadatas, err := fs.ListMetadata(sourceEntry.Path)
	if err != nil {
		return "", xerrors.Errorf("failed to list metadata for %q: %w", sourceEntry.Path, err)
	}

	filteredMetadatas := []*irodsclient_types.IRODSMeta{}
	for _, metadata := range metadatas {
		if t.shouldHideMetadata(fs, metadata.Name) {
			filteredMetadatas = append(filteredMetadatas, metadata)
		}
	}

	mimeType := "Directory"
	if !sourceEntry.IsDir() {
		// read the file content
		content, err := irods_common.ReadDataObject(fs, sourceEntry.Path, irods_common.MIME_TYPE_READ_SIZE)
		if err != nil {
			return "", xerrors.Errorf("failed to read file (data-object) %q: %w", sourceEntry.Path, err)
		}

		mimeType = irods_common.DetectMimeType(sourceEntry.Path, content)
	}

	getFileInfoOutput := model.GetFileInfoOutput{
		MIMEType:          mimeType,
		EntryInfo:         sourceEntry,
		ResourceURI:       irods_common.MakeResourceURI(sourceEntry.Path),
		WebDAVURI:         irods_common.MakeWebdavURL(t.config, sourceEntry.Path),
		Accesses:          accesses,
		AccessInheritance: accessInherit,
		Metadata:          filteredMetadatas,
	}

	jsonBytes, err := json.Marshal(getFileInfoOutput)
	if err != nil {
		return "", xerrors.Errorf("failed to marshal JSON: %w", err)
	}

	return string(jsonBytes), nil
}

func (t *GetFileInfo) shouldHideMetadata(fs *irodsclient_fs.FileSystem, attr string) bool {
	if !fs.GetAccount().IsAnonymousUser() {
		// if the user is not anonymous, do not hide any metadata
		return false
	}

	if _, ok := t.systemMetadataNameMap[attr]; ok {
		// has it
		return true
	}

	return false
}
