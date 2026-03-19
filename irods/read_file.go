package irods

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/cockroachdb/errors"
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	"github.com/cyverse/irods-mcp-server/common"
	irods_common "github.com/cyverse/irods-mcp-server/irods/common"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	ReadFileName = irods_common.IRODSAPIPrefix + "read_file"
)

type ReadFileInputArgs struct {
	Path   string `json:"path"`
	Offset int64  `json:"offset,omitempty"`
	Length int64  `json:"length,omitempty"`
}

type ReadFile struct {
	mcpServer *IRODSMCPServer
	config    *common.Config
}

func NewReadFile(svr *IRODSMCPServer) ToolAPI {
	return &ReadFile{
		mcpServer: svr,
		config:    svr.GetConfig(),
	}
}

func (t *ReadFile) GetName() string {
	return ReadFileName
}

func (t *ReadFile) GetDescription() string {
	return `Read the partial content of a file (data-object) with the specified path and offset.
	The specified path must be an iRODS path.
	If the file is too large to be displayed inline, use the WebDAV URI to access it.`
}

func (t *ReadFile) GetTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        t.GetName(),
		Description: t.GetDescription(),
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"path": {
					Type:        "string",
					Description: "The path to the file (data-object) to read.",
				},
				"offset": {
					Type:        "number",
					Description: "The offset to start reading the file from. Default is 0.",
					Default:     json.RawMessage("0"),
				},
				"length": {
					Type:        "number",
					Description: fmt.Sprintf("The maximum length of the file to read. Default value is %d. Length must be greater than or equal to %d. Length must not be too large, otherwise the output may be too large. Maximum value is %d.", irods_common.MaxInlineSize, irods_common.MinReadLength, irods_common.MaxInlineSize),
					Default:     json.RawMessage(fmt.Sprintf("%d", irods_common.MinReadLength)),
				},
			},
			Required: []string{"path"},
		},
	}
}

func (t *ReadFile) GetHandler() mcp.ToolHandler {
	return t.Handler
}

func (t *ReadFile) GetAccessiblePaths(authValue *common.AuthValue) []string {
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

func (t *ReadFile) Handler(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// arguments
	args := ReadFileInputArgs{}
	err := irods_common.MarshalInputArguments(t.GetTool(), request, &args)
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to marshal input arguments")
		return irods_common.ToolErrorResult(outputErr), nil
	}

	if args.Length <= 0 {
		// default value
		args.Length = irods_common.MinReadLength
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

	inputLength := args.Length
	if inputLength < irods_common.MinReadLength {
		inputLength = irods_common.MinReadLength
	} else if inputLength > irods_common.MaxInlineSize {
		inputLength = irods_common.MaxInlineSize
	}

	// check permission
	if !irods_common.IsAccessAllowed(irodsPath, t.GetAccessiblePaths(&authValue)) {
		outputErr := errors.Newf("%q request is not permitted for path %q", t.GetName(), irodsPath)
		return irods_common.ToolErrorResult(outputErr), nil
	}

	// Get file info
	entry, err := fs.Stat(irodsPath)
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to stat file info for %q", irodsPath)
		return irods_common.ToolErrorResult(outputErr), nil
	}

	inputOffset := args.Offset
	if inputOffset < 0 {
		inputOffset = 0
	} else if inputOffset >= entry.Size {
		inputOffset = entry.Size
	}

	content, err := t.readFile(fs, entry, int64(inputOffset), int64(inputLength))
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to read file (data-object) for %q", irodsPath)
		return irods_common.ToolErrorResult(outputErr), nil
	}

	return content, err
}

func (t *ReadFile) readFile(fs *irodsclient_fs.FileSystem, sourceEntry *irodsclient_fs.Entry, offset int64, readLength int64) (*mcp.CallToolResult, error) {
	resourceURI := irods_common.MakeResourceURI(sourceEntry.Path)
	webdavURI := irods_common.MakeWebdavURL(t.config, sourceEntry.Path, fs.GetAccount())

	if sourceEntry.IsDir() {
		// For directories, return a resource reference instead
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("This is a directory (collection). Use the resource URI to browse its contents: %q", resourceURI),
				},
				&mcp.EmbeddedResource{
					Resource: &mcp.ResourceContents{
						URI:      resourceURI,
						MIMEType: "text/plain",
						Text:     fmt.Sprintf("Directory (collection): %q", sourceEntry.Path),
					},
				},
			},
			IsError: false,
		}, nil
	}

	// read the file content
	content, err := irods_common.ReadDataObject(fs, sourceEntry.Path, offset, readLength)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read file (data-object) %q", sourceEntry.Path)
	}

	mimeType := irods_common.DetectMimeTypeWithContent(sourceEntry.Path, offset, content)
	if irods_common.IsTextFile(mimeType) {
		// text file
		return irods_common.ToolTextResult(string(content)), nil
	} else if irods_common.IsImageFile(mimeType) {
		if sourceEntry.Size <= irods_common.MaxBase64Size {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{
						Text: fmt.Sprintf("Image file (data-object): %q (%q type, %d bytes)", sourceEntry.Path, mimeType, sourceEntry.Size),
					},
					&mcp.ImageContent{
						Data:     []byte(base64.StdEncoding.EncodeToString(content)),
						MIMEType: mimeType,
					},
				},
				IsError: false,
			}, nil
		} else {
			// Too large for base64, return a reference
			return irods_common.ToolTextResult(fmt.Sprintf("Image file (%q, %d bytes) is too large to encode to base64 format. Access it via WebDAV URI: %q", mimeType, sourceEntry.Size, webdavURI)), nil
		}
	} else {
		// binary file
		if sourceEntry.Size <= irods_common.MaxBase64Size {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{
						Text: fmt.Sprintf("Binary file (data-object): %q (%q type, %d bytes)", sourceEntry.Path, mimeType, sourceEntry.Size),
					},
					&mcp.EmbeddedResource{
						Resource: &mcp.ResourceContents{
							URI:      resourceURI,
							MIMEType: mimeType,
							Blob:     []byte(base64.StdEncoding.EncodeToString(content)),
						},
					},
				},
				IsError: false,
			}, nil
		} else {
			return irods_common.ToolTextResult(fmt.Sprintf("Binary file (%q, %d bytes) is too large to encode to base64 format. Access it via WebDAV URI: %q", mimeType, sourceEntry.Size, webdavURI)), nil
		}
	}
}
