package irods

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

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
	WriteFileName = irods_common.IRODSAPIPrefix + "write_file"
)

type WriteFileInputArgs struct {
	Path    string `json:"path"`
	Offset  int64  `json:"offset,omitempty"`
	Content string `json:"content"`
}

type WriteFile struct {
	mcpServer *IRODSMCPServer
	config    *common.Config
}

func NewWriteFile(svr *IRODSMCPServer) ToolAPI {
	return &WriteFile{
		mcpServer: svr,
		config:    svr.GetConfig(),
	}
}

func (t *WriteFile) GetName() string {
	return WriteFileName
}

func (t *WriteFile) GetDescription() string {
	return `Write the partial content to a file (data-object) with the specified path and offset.
	The specified path must be an iRODS path.
	If the file is too large to be displayed inline, use the WebDAV URI to access it.`
}

func (t *WriteFile) GetTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        t.GetName(),
		Description: t.GetDescription(),
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"path": {
					Type:        "string",
					Description: "The path to the file (data-object) to write to.",
				},
				"offset": {
					Type:        "string",
					Description: "The offset to start writing the file from. Default is 0.",
					Default:     json.RawMessage("0"),
				},
				"content": {
					Type:        "string",
					Description: fmt.Sprintf("The Base64-encoded content to write to the file (data-object). Maximum size is %d bytes.", irods_common.MaxInlineSize),
				},
			},
			Required: []string{"path", "content"},
		},
	}
}

func (t *WriteFile) GetHandler() mcp.ToolHandler {
	return t.Handler
}

func (t *WriteFile) GetAccessiblePaths(authValue *common.AuthValue) []string {
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

func (t *WriteFile) Handler(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// arguments
	args := WriteFileInputArgs{}
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
	fileSize := int64(0)
	entry, err := fs.Stat(irodsPath)
	if err != nil {
		if !irodsclient_types.IsFileNotFoundError(err) {
			outputErr := errors.Wrapf(err, "failed to stat file info for %q", irodsPath)
			return irods_common.ToolErrorResult(outputErr), nil
		}
	} else {
		if entry.IsDir() {
			outputErr := errors.Newf("path %q is a directory (collection)", irodsPath)
			return irods_common.ToolErrorResult(outputErr), nil
		}

		fileSize = entry.Size
	}

	inputOffset := args.Offset
	if inputOffset < 0 {
		inputOffset = 0
	} else if inputOffset >= fileSize {
		inputOffset = fileSize
	}

	content, err := t.writeFile(fs, irodsPath, inputOffset, args.Content)
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to write file (data-object) for %q", irodsPath)
		return irods_common.ToolErrorResult(outputErr), nil
	}

	return irods_common.ToolJSONResult(*content)
}

func (t *WriteFile) writeFile(fs *irodsclient_fs.FileSystem, path string, offset int64, inputContent string) (*model.WriteFileOutput, error) {
	byteContent, err := base64.StdEncoding.DecodeString(inputContent)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to decode base64 content for file (data-object) %q", path)
	}

	// write the file content
	err = irods_common.WriteDataObject(fs, path, offset, byteContent)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to write file (data-object) %q", path)
	}

	fileWriteOutput := &model.WriteFileOutput{
		Path:         path,
		Offset:       offset,
		BytesWritten: len(byteContent),
	}

	return fileWriteOutput, nil
}
