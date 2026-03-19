package irods

import (
	"context"

	"github.com/cockroachdb/errors"
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/irods-mcp-server/common"
	irods_common "github.com/cyverse/irods-mcp-server/irods/common"
	"github.com/cyverse/irods-mcp-server/irods/model"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	GetFileInfoName = irods_common.IRODSAPIPrefix + "get_file_info"
)

type GetFileInfoInputArgs struct {
	Path string `json:"path"`
}

type GetFileInfo struct {
	mcpServer          *IRODSMCPServer
	config             *common.Config
	systemAttributeMap map[string]string
}

func NewGetFileInfo(svr *IRODSMCPServer) ToolAPI {
	systemAttributes := irods_common.GetSystemAttributes()
	systemAttributeMap := map[string]string{}
	for _, systemAttribute := range systemAttributes {
		systemAttributeMap[systemAttribute] = systemAttribute
	}

	return &GetFileInfo{
		mcpServer:          svr,
		config:             svr.GetConfig(),
		systemAttributeMap: systemAttributeMap,
	}
}

func (t *GetFileInfo) GetName() string {
	return GetFileInfoName
}

func (t *GetFileInfo) GetDescription() string {
	return `Retrieve detailed metadata about a file or directory.`
}

func (t *GetFileInfo) GetTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        t.GetName(),
		Description: t.GetDescription(),
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"path": {
					Type:        "string",
					Description: "The path to the file (data-object) or directory (collection).",
				},
			},
			Required: []string{"path"},
		},
	}
}

func (t *GetFileInfo) GetHandler() mcp.ToolHandler {
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
		paths = append(paths, homePath)
		paths = append(paths, homePath+"/*")
	}

	return paths
}

func (t *GetFileInfo) Handler(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// arguments
	args := GetFileInfoInputArgs{}
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

	// check permission
	if !irods_common.IsAccessAllowed(irodsPath, t.GetAccessiblePaths(&authValue)) {
		outputErr := errors.Newf("%q request is not permitted for path %q", t.GetName(), irodsPath)
		return irods_common.ToolErrorResult(outputErr), nil
	}

	// Get file info
	sourceEntry, err := fs.Stat(irodsPath)
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to stat file or directory info for %q", irodsPath)
		return irods_common.ToolErrorResult(outputErr), nil
	}

	content, err := t.getFileInfo(fs, sourceEntry)
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to get file (data-object) info for %q", irodsPath)
		return irods_common.ToolErrorResult(outputErr), nil
	}

	return irods_common.ToolJSONResult(*content)
}

func (t *GetFileInfo) getFileInfo(fs *irodsclient_fs.FileSystem, sourceEntry *irodsclient_fs.Entry) (*model.GetFileInfoOutput, error) {
	accesses, err := fs.ListACLs(sourceEntry.Path)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to list ACLs for %q", sourceEntry.Path)
	}

	var accessInherit *irodsclient_types.IRODSAccessInheritance
	if sourceEntry.IsDir() {
		accessInherit, err = fs.GetDirACLInheritance(sourceEntry.Path)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get access inheritance info for %q", sourceEntry.Path)
		}
	}

	avus, err := fs.ListMetadata(sourceEntry.Path)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to list metadata for %q", sourceEntry.Path)
	}

	filteredAVUs := []*irodsclient_types.IRODSMeta{}
	for _, avu := range avus {
		if t.shouldHideMetadata(fs, avu.Name) {
			filteredAVUs = append(filteredAVUs, avu)
		}
	}

	mimeType := "Directory"
	if !sourceEntry.IsDir() {
		// read the file content
		content, err := irods_common.ReadDataObject(fs, sourceEntry.Path, 0, irods_common.MIME_TYPE_READ_SIZE)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to read file (data-object) %q", sourceEntry.Path)
		}

		mimeType = irods_common.DetectMimeTypeWithContent(sourceEntry.Path, 0, content)
	}

	getFileInfoOutput := &model.GetFileInfoOutput{
		MIMEType:          mimeType,
		EntryInfo:         sourceEntry,
		ResourceURI:       irods_common.MakeResourceURI(sourceEntry.Path),
		WebDAVURI:         irods_common.MakeWebdavURLWithAccesses(t.config, sourceEntry.Path, fs.GetAccount(), accesses),
		Accesses:          accesses,
		AccessInheritance: accessInherit,
		AVUs:              filteredAVUs,
	}

	return getFileInfoOutput, nil
}

func (t *GetFileInfo) shouldHideMetadata(fs *irodsclient_fs.FileSystem, attr string) bool {
	if !fs.GetAccount().IsAnonymousUser() {
		// if the user is not anonymous, do not hide any metadata
		return false
	}

	return irods_common.IsSystemAttribute(attr)
}
