package irods

import (
	"context"
	"encoding/base64"
	"fmt"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	"github.com/cyverse/irods-mcp-server/common"
	irods_common "github.com/cyverse/irods-mcp-server/irods/common"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"golang.org/x/xerrors"
)

const (
	ReadFileName = irods_common.IRODSAPIPrefix + "read_file"
)

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
	return `Read the complete content of a file (data-object) with the specified path.
	The specified path must be an iRODS path.
	If the file is too large to be displayed inline, use the WebDAV URI to access it.`
}

func (t *ReadFile) GetTool() mcp.Tool {
	return mcp.NewTool(
		t.GetName(),
		mcp.WithDescription(t.GetDescription()),
		mcp.WithString(
			"path",
			mcp.Required(),
			mcp.Description("The path to the directory (collection) to list"),
		),
		mcp.WithNumber(
			"length",
			mcp.DefaultNumber(float64(irods_common.MinReadLength)),
			mcp.Description(fmt.Sprintf("The maximum length of the file to read. Default value is %d. Length must be greater than or equal to %d. Length must not be too large, otherwise the output may be too large. Maximum value is %d.", irods_common.MaxInlineSize, irods_common.MinReadLength, irods_common.MaxInlineSize)),
		),
	)
}

func (t *ReadFile) GetHandler() server.ToolHandlerFunc {
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

func (t *ReadFile) Handler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	arguments := request.GetArguments()

	inputPath, ok := arguments["path"].(string)
	if !ok {
		return nil, xerrors.Errorf("failed to get path from arguments")
	}

	inputLengthFloat, ok := arguments["length"].(float64)
	if !ok {
		// default value
		inputLengthFloat = float64(irods_common.MinReadLength)
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

	inputLength := int(inputLengthFloat)
	if inputLength < int(irods_common.MinReadLength) {
		inputLength = int(irods_common.MinReadLength)
	} else if inputLength > int(irods_common.MaxInlineSize) {
		inputLength = int(irods_common.MaxInlineSize)
	}

	// check permission
	if !irods_common.IsAccessAllowed(irodsPath, t.GetAccessiblePaths(&authValue)) {
		outputErr := xerrors.Errorf("%q request is not permitted for path %q", t.GetName(), irodsPath)
		return irods_common.OutputMCPError(outputErr)
	}

	// Get file info
	entry, err := fs.Stat(irodsPath)
	if err != nil {
		outputErr := xerrors.Errorf("failed to stat file info for %q: %w", irodsPath, err)
		return irods_common.OutputMCPError(outputErr)
	}

	content, err := t.readFile(fs, entry, int64(inputLength))
	if err != nil {
		outputErr := xerrors.Errorf("failed to read file (data-object) for %q: %w", irodsPath, err)
		return irods_common.OutputMCPError(outputErr)
	}

	return &mcp.CallToolResult{
		Content: content,
	}, nil
}

func (t *ReadFile) readFile(fs *irodsclient_fs.FileSystem, sourceEntry *irodsclient_fs.Entry, readLength int64) ([]mcp.Content, error) {
	resourceURI := irods_common.MakeResourceURI(sourceEntry.Path)
	webdavURI := irods_common.MakeWebdavURL(t.config, sourceEntry.Path)

	if sourceEntry.IsDir() {
		// For directories, return a resource reference instead
		return []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("This is a directory (collection). Use the resource URI to browse its contents: %q", resourceURI),
			},
			mcp.EmbeddedResource{
				Type: "resource",
				Resource: mcp.TextResourceContents{
					URI:      resourceURI,
					MIMEType: "text/plain",
					Text:     fmt.Sprintf("Directory (collection): %q", sourceEntry.Path),
				},
			},
		}, nil
	}

	// read the file content
	content, err := irods_common.ReadDataObject(fs, sourceEntry.Path, readLength)
	if err != nil {
		return nil, xerrors.Errorf("failed to read file (data-object) %q: %w", sourceEntry.Path, err)
	}

	mimeType := irods_common.DetectMimeType(sourceEntry.Path, content)
	if irods_common.IsTextFile(mimeType) {
		// text file
		return []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: string(content),
			},
		}, nil
	} else if irods_common.IsImageFile(mimeType) {
		if sourceEntry.Size <= irods_common.MaxBase64Size {
			return []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Image file (data-object): %q (%q type, %d bytes)", sourceEntry.Path, mimeType, sourceEntry.Size),
				},
				mcp.ImageContent{
					Type:     "image",
					Data:     base64.StdEncoding.EncodeToString(content),
					MIMEType: mimeType,
				},
			}, nil
		} else {
			// Too large for base64, return a reference
			return []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Image file (%q, %d bytes) is too large to encode to base64 format. Access it via WebDAV URI: %q", mimeType, sourceEntry.Size, webdavURI),
				},
			}, nil
		}
	} else {
		// binary file
		if sourceEntry.Size <= irods_common.MaxBase64Size {
			return []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Binary file (data-object): %q (%q type, %d bytes)", sourceEntry.Path, mimeType, sourceEntry.Size),
				},
				mcp.EmbeddedResource{
					Type: "resource",
					Resource: mcp.BlobResourceContents{
						URI:      resourceURI,
						MIMEType: mimeType,
						Blob:     base64.StdEncoding.EncodeToString(content),
					},
				},
			}, nil
		} else {
			return []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Binary file (%q, %d bytes) is too large to encode to base64 format. Access it via WebDAV URI: %q", mimeType, sourceEntry.Size, webdavURI),
				},
			}, nil
		}
	}
}
