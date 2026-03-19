package irods

import (
	"context"
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
	UploadFileName = irods_common.IRODSAPIPrefix + "upload_file"
)

type UploadFileInputArgs struct {
	LocalPath string `json:"local_path"`
	IrodsPath string `json:"irods_path"`
	IsDir     bool   `json:"is_dir,omitempty"`
}

type UploadFile struct {
	mcpServer *IRODSMCPServer
	config    *common.Config
}

func NewUploadFile(svr *IRODSMCPServer) ToolAPI {
	return &UploadFile{
		mcpServer: svr,
		config:    svr.GetConfig(),
	}
}

func (t *UploadFile) GetName() string {
	return UploadFileName
}

func (t *UploadFile) GetDescription() string {
	return `Returns how to upload the full contgent of a file (data-object) to the specified path.
	The specified path must be an iRODS path.
	Returns how to upload the file using WebDAV, GoCommands (gocmd), and iCommands.`
}

func (t *UploadFile) GetTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        t.GetName(),
		Description: t.GetDescription(),
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"local_path": {
					Type:        "string",
					Description: "The local path to the file (data-object) to upload.",
				},
				"irods_path": {
					Type:        "string",
					Description: "The target iRODS path to upload the file (data-object) to.",
				},
				"is_dir": {
					Type:        "boolean",
					Description: "Set to true if uploading a directory (collection). Default is false.",
					Default:     json.RawMessage("false"),
				},
			},
			Required: []string{"local_path", "irods_path"},
		},
	}
}

func (t *UploadFile) GetHandler() mcp.ToolHandler {
	return t.Handler
}

func (t *UploadFile) GetAccessiblePaths(authValue *common.AuthValue) []string {
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

func (t *UploadFile) Handler(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// arguments
	args := UploadFileInputArgs{}
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

	irodsPath := irods_common.MakeIRODSPath(t.config, fs.GetAccount(), args.IrodsPath)

	// check permission
	if !irods_common.IsAccessAllowed(irodsPath, t.GetAccessiblePaths(&authValue)) {
		outputErr := errors.Newf("%q request is not permitted for path %q", t.GetName(), irodsPath)
		return irods_common.ToolErrorResult(outputErr), nil
	}

	content, err := t.uploadFile(fs, args.LocalPath, irodsPath, args.IsDir)
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to create an instruction to upload file (data-object) or directory (collection) for %q", irodsPath)
		return irods_common.ToolErrorResult(outputErr), nil
	}

	return irods_common.ToolTextResult(content), nil
}

func (t *UploadFile) uploadFile(fs *irodsclient_fs.FileSystem, localPath string, irodsPath string, isDir bool) (string, error) {
	webdavURI := irods_common.MakeWebdavURL(t.config, irodsPath, fs.GetAccount())

	recursive := isDir

	curlInst := t.getCurlInstruction(localPath, webdavURI, recursive)
	goCmdInst := t.getGoCommandsInstruction(localPath, irodsPath)
	iCmdInst := t.getICommandsInstruction(localPath, irodsPath, recursive)

	return fmt.Sprintf("%s\n%s\n%s\n", curlInst, goCmdInst, iCmdInst), nil
}

func (t *UploadFile) getCurlInstruction(localPath string, webdavURI string, recursive bool) string {
	inst := ""
	if recursive {
		inst = `You cannot upload the entire directory using curl. Please use other methods for uploading directories.
		`
	} else {
		inst = `To upload the file using curl, run the following command: 
curl -L -T %s %s
This is just an example command. You may need to adjust it based on your requirements.
`
	}

	return fmt.Sprintf(inst, localPath, webdavURI)
}

func (t *UploadFile) getGoCommandsInstruction(localPath string, irodsPath string) string {
	inst := `To upload the entire directory using gocommands, run the following command: 
	gocmd put -K --progress %s %s
	This is just an example command. You may need to adjust it based on your requirements.
	You will need to have gocommands installed and configured to use this command.
	Check out https://learning.cyverse.org/ds/gocommands/ for more details.
	`

	return fmt.Sprintf(inst, localPath, irodsPath)
}

func (t *UploadFile) getICommandsInstruction(localPath string, irodsPath string, recursive bool) string {
	inst := ""
	if recursive {
		inst = `To upload the entire directory using gocommands, run the following command: 
	iput -r -P -K %s %s
	This is just an example command. You may need to adjust it based on your requirements.
	You will need to have iCommands installed and configured to use this command.
	Check out https://learning.cyverse.org/ds/icommands/ for more details.
	`
	} else {
		inst = `To upload the file using gocommands, run the following command: 
	iput -K %s %s
	This is just an example command. You may need to adjust it based on your requirements.
	You will need to have iCommands installed and configured to use this command.
	Check out https://learning.cyverse.org/ds/icommands/ for more details.
	`
	}

	return fmt.Sprintf(inst, localPath, irodsPath)
}
