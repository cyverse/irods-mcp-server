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
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	WriteFileName = irods_common.IRODSAPIPrefix + "write_file"
)

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
	return `Write the patial content to a file (data-object) with the specified path and offset.
	The specified path must be an iRODS path.
	If the file is too large to be displayed inline, use the WebDAV URI to access it.`
}

func (t *WriteFile) GetTool() mcp.Tool {
	return mcp.NewTool(
		t.GetName(),
		mcp.WithDescription(t.GetDescription()),
		mcp.WithString(
			"path",
			mcp.Required(),
			mcp.Description("The path to the file (data-object) to write to"),
		),
		mcp.WithNumber(
			"offset",
			mcp.DefaultNumber(float64(0)),
			mcp.Description("The offset to start writing the file from. Default is 0."),
		),
		mcp.WithString(
			"content",
			mcp.Required(),
			mcp.Description(fmt.Sprintf("The Base64-encoded content to write to the file (data-object). Maximum size is %d bytes.", irods_common.MaxInlineSize)),
		),
	)
}

func (t *WriteFile) GetHandler() server.ToolHandlerFunc {
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

func (t *WriteFile) Handler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	arguments := request.GetArguments()

	inputPath, ok := arguments["path"].(string)
	if !ok {
		return nil, errors.Errorf("failed to get path from arguments")
	}

	inputOffsetFloat, ok := arguments["offset"].(float64)
	if !ok {
		// default value
		inputOffsetFloat = float64(0)
	}

	inputContent, ok := arguments["content"].(string)
	if !ok {
		return nil, errors.Errorf("failed to get content from arguments")
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
		outputErr := errors.Errorf("%q request is not permitted for path %q", t.GetName(), irodsPath)
		return irods_common.OutputMCPError(outputErr)
	}

	// Get file info
	fileSize := int64(0)
	entry, err := fs.Stat(irodsPath)
	if err != nil {
		if !irodsclient_types.IsFileNotFoundError(err) {
			outputErr := errors.Wrapf(err, "failed to stat file info for %q", irodsPath)
			return irods_common.OutputMCPError(outputErr)
		}
	} else {
		if entry.IsDir() {
			outputErr := errors.Errorf("path %q is a directory (collection)", irodsPath)
			return irods_common.OutputMCPError(outputErr)
		}

		fileSize = entry.Size
	}

	inputOffset := int64(inputOffsetFloat)
	if inputOffset < 0 {
		inputOffset = 0
	} else if inputOffset >= fileSize {
		inputOffset = fileSize
	}

	content, err := t.writeFile(fs, irodsPath, int64(inputOffset), inputContent)
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to write file (data-object) for %q", irodsPath)
		return irods_common.OutputMCPError(outputErr)
	}

	return mcp.NewToolResultText(content), nil
}

func (t *WriteFile) writeFile(fs *irodsclient_fs.FileSystem, path string, offset int64, inputContent string) (string, error) {
	byteContent, err := base64.StdEncoding.DecodeString(inputContent)
	if err != nil {
		return "", errors.Wrapf(err, "failed to decode base64 content for file (data-object) %q", path)
	}

	// write the file content
	err = irods_common.WriteDataObject(fs, path, offset, byteContent)
	if err != nil {
		return "", errors.Wrapf(err, "failed to write file (data-object) %q", path)
	}

	fileWriteOutput := model.WriteFileOutput{
		Path:         path,
		Offset:       offset,
		BytesWritten: len(byteContent),
	}

	jsonBytes, err := json.Marshal(fileWriteOutput)
	if err != nil {
		return "", errors.Wrapf(err, "failed to marshal JSON")
	}

	return string(jsonBytes), nil
}
