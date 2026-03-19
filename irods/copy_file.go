package irods

import (
	"context"
	"path"

	"github.com/cockroachdb/errors"
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	"github.com/cyverse/irods-mcp-server/common"
	irods_common "github.com/cyverse/irods-mcp-server/irods/common"
	"github.com/cyverse/irods-mcp-server/irods/model"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	CopyFileName = irods_common.IRODSAPIPrefix + "copy_file"
)

type CopyFileInputArgs struct {
	SourcePath      string `json:"source_path"`
	DestinationPath string `json:"destination_path"`
}

type CopyFile struct {
	mcpServer *IRODSMCPServer
	config    *common.Config
}

func NewCopyFile(svr *IRODSMCPServer) ToolAPI {
	return &CopyFile{
		mcpServer: svr,
		config:    svr.GetConfig(),
	}
}

func (t *CopyFile) GetName() string {
	return CopyFileName
}

func (t *CopyFile) GetDescription() string {
	return `Copy a file (data-object) or directory (collection) to a new location.`
}

func (t *CopyFile) GetTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        t.GetName(),
		Description: t.GetDescription(),
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"source_path": {
					Type:        "string",
					Description: "The path to the source file (data-object) or directory (collection). If directory path is given, the entire directory and its contents will be copied.",
				},
				"destination_path": {
					Type:        "string",
					Description: "The new, complete path to copy the file (data-object) or directory (collection) to, including its new name. The path must not already exist.",
				},
			},
			Required: []string{"source_path", "destination_path"},
		},
	}
}

func (t *CopyFile) GetHandler() mcp.ToolHandler {
	return t.Handler
}

func (t *CopyFile) GetAccessiblePaths(authValue *common.AuthValue) []string {
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

func (t *CopyFile) Handler(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// arguments
	args := CopyFileInputArgs{}
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

	irodsSourcePath := irods_common.MakeIRODSPath(t.config, fs.GetAccount(), args.SourcePath)
	irodsDestinationPath := irods_common.MakeIRODSPath(t.config, fs.GetAccount(), args.DestinationPath)

	// check permission
	if !irods_common.IsAccessAllowed(irodsSourcePath, t.GetAccessiblePaths(&authValue)) {
		outputErr := errors.Newf("%q request is not permitted for path %q", t.GetName(), irodsSourcePath)
		return irods_common.ToolErrorResult(outputErr), nil
	}
	if !irods_common.IsAccessAllowed(irodsDestinationPath, t.GetAccessiblePaths(&authValue)) {
		outputErr := errors.Newf("%q request is not permitted for path %q", t.GetName(), irodsDestinationPath)
		return irods_common.ToolErrorResult(outputErr), nil
	}

	// Copy file
	sourceEntry, err := fs.Stat(irodsSourcePath)
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to stat file or directory info for %q", irodsSourcePath)
		return irods_common.ToolErrorResult(outputErr), nil
	}

	content, err := t.copyFile(fs, sourceEntry, irodsDestinationPath)
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to copy file (data-object) or directory (collection) from %q to %q", irodsSourcePath, irodsDestinationPath)
		return irods_common.ToolErrorResult(outputErr), nil
	}

	return irods_common.ToolJSONResult(*content)
}

func (t *CopyFile) copyFile(fs *irodsclient_fs.FileSystem, sourceEntry *irodsclient_fs.Entry, destPath string) (*model.CopyFileOutput, error) {
	sourceEntries, copiedEntries, err := t.copyFileInternal(fs, sourceEntry, destPath)
	if err != nil {
		return nil, err
	}

	fileCopyOutput := &model.CopyFileOutput{
		SourcePath:          sourceEntry.Path,
		DestinationPath:     destPath,
		SourceEntryInfoList: sourceEntries,
		CopiedEntryInfoList: copiedEntries,
	}

	return fileCopyOutput, nil
}

func (t *CopyFile) copyFileInternal(fs *irodsclient_fs.FileSystem, sourceEntry *irodsclient_fs.Entry, destPath string) ([]*irodsclient_fs.Entry, []*irodsclient_fs.Entry, error) {
	sourceEntries := []*irodsclient_fs.Entry{sourceEntry}

	if !sourceEntry.IsDir() {
		// file
		err := fs.CopyFileToFile(sourceEntry.Path, destPath, true)
		if err != nil {
			return sourceEntries, nil, errors.Wrapf(err, "failed to copy file (data-object) from %q to %q", sourceEntry.Path, destPath)
		}

		destEntry, err := fs.Stat(destPath)
		if err != nil {
			return sourceEntries, nil, errors.Wrapf(err, "failed to stat file or directory info for %q", destPath)
		}

		return sourceEntries, []*irodsclient_fs.Entry{destEntry}, nil
	}

	// dir
	err := fs.MakeDir(destPath, true)
	if err != nil {
		return sourceEntries, nil, errors.Wrapf(err, "failed to copy directory (collection) from %q to %q", sourceEntry.Path, destPath)
	}

	destEntry, err := fs.Stat(destPath)
	if err != nil {
		return sourceEntries, nil, errors.Wrapf(err, "failed to stat file or directory info for %q", destPath)
	}

	copiedEntries := []*irodsclient_fs.Entry{destEntry}

	entries, err := fs.List(sourceEntry.Path)
	if err != nil {
		return sourceEntries, copiedEntries, errors.Wrapf(err, "failed to list directory (collection) entries for %q", sourceEntry.Path)
	}

	for _, entry := range entries {
		// copy recursively
		destEntryPath := path.Join(destPath, entry.Name)
		rsourceEntries, rcopiedEntries, err := t.copyFileInternal(fs, entry, destEntryPath)
		if err != nil {
			return sourceEntries, copiedEntries, err
		}

		sourceEntries = append(sourceEntries, rsourceEntries...)
		copiedEntries = append(copiedEntries, rcopiedEntries...)
	}

	return sourceEntries, copiedEntries, nil
}
