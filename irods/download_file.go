package irods

import (
	"context"
	"fmt"

	"github.com/cockroachdb/errors"
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	"github.com/cyverse/irods-mcp-server/common"
	irods_common "github.com/cyverse/irods-mcp-server/irods/common"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	DownloadFileName = irods_common.IRODSAPIPrefix + "download_file"
)

type DownloadFile struct {
	mcpServer *IRODSMCPServer
	config    *common.Config
}

func NewDownloadFile(svr *IRODSMCPServer) ToolAPI {
	return &DownloadFile{
		mcpServer: svr,
		config:    svr.GetConfig(),
	}
}

func (t *DownloadFile) GetName() string {
	return DownloadFileName
}

func (t *DownloadFile) GetDescription() string {
	return `Returns how to download the full contgent of a file (data-object) with the specified path.
	The specified path must be an iRODS path.
	Returns how to download the file using WebDAV, GoCommands (gocmd), and iCommands.`
}

func (t *DownloadFile) GetTool() mcp.Tool {
	return mcp.NewTool(
		t.GetName(),
		mcp.WithDescription(t.GetDescription()),
		mcp.WithString(
			"irods_path",
			mcp.Required(),
			mcp.Description("The iRODS path to the file (data-object) to download"),
		),
		mcp.WithString(
			"local_path",
			mcp.Required(),
			mcp.Description("The local path to download the file (data-object) to. Must be a full path including the file name."),
		),
	)
}

func (t *DownloadFile) GetHandler() server.ToolHandlerFunc {
	return t.Handler
}

func (t *DownloadFile) GetAccessiblePaths(authValue *common.AuthValue) []string {
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

func (t *DownloadFile) Handler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	arguments := request.GetArguments()

	localPath, ok := arguments["local_path"].(string)
	if !ok {
		return nil, errors.New("failed to get local_path from arguments")
	}

	irodsPath, ok := arguments["irods_path"].(string)
	if !ok {
		return nil, errors.New("failed to get irods_path from arguments")
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

	irodsPath = irods_common.MakeIRODSPath(t.config, fs.GetAccount(), irodsPath)

	// check permission
	if !irods_common.IsAccessAllowed(irodsPath, t.GetAccessiblePaths(&authValue)) {
		outputErr := errors.Newf("%q request is not permitted for path %q", t.GetName(), irodsPath)
		return irods_common.OutputMCPError(outputErr)
	}

	// Get file info
	entry, err := fs.Stat(irodsPath)
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to stat file info for %q", irodsPath)
		return irods_common.OutputMCPError(outputErr)
	}

	content, err := t.downloadFile(fs, entry, localPath)
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to create an instruction to download file (data-object) or directory (collection) for %q", irodsPath)
		return irods_common.OutputMCPError(outputErr)
	}

	return &mcp.CallToolResult{
		Content: content,
	}, nil
}

func (t *DownloadFile) downloadFile(fs *irodsclient_fs.FileSystem, sourceEntry *irodsclient_fs.Entry, localPath string) ([]mcp.Content, error) {
	webdavURI := irods_common.MakeWebdavURL(t.config, sourceEntry.Path, fs.GetAccount())

	recursive := sourceEntry.IsDir()

	curlInst := t.getCurlInstruction(webdavURI, localPath, recursive)
	wgetInst := t.getWgetInstruction(webdavURI, localPath, recursive)
	goCmdInst := t.getGoCommandsInstruction(sourceEntry.Path, localPath)
	iCmdInst := t.getICommandsInstruction(sourceEntry.Path, localPath, recursive)

	return []mcp.Content{
		mcp.TextContent{
			Type: "text",
			Text: fmt.Sprintf("%s\n%s\n%s\n%s\n", curlInst, wgetInst, goCmdInst, iCmdInst),
		},
	}, nil
}

func (t *DownloadFile) getCurlInstruction(webdavURI string, localPath string, recursive bool) string {
	inst := ""
	if recursive {
		inst = `To download the entire directory using curl, run the following command: 
curl -r -L -o %s %s
This is just an example command. You may need to adjust it based on your requirements.
`
	} else {
		inst = `To download the file using curl, run the following command: 
curl -L -o %s %s
This is just an example command. You may need to adjust it based on your requirements.
`
	}

	return fmt.Sprintf(inst, localPath, webdavURI)
}

func (t *DownloadFile) getWgetInstruction(webdavURI string, localPath string, recursive bool) string {
	inst := ""
	if recursive {
		inst = `You cannot download the entire directory using wget. Please use other methods for downloading directories.
		`
	} else {
		inst = `To download the file using wget, run the following command: 
	wget -O %s %s
	This is just an example command. You may need to adjust it based on your requirements.
	`
	}

	return fmt.Sprintf(inst, localPath, webdavURI)
}

func (t *DownloadFile) getGoCommandsInstruction(irodsPath string, localPath string) string {
	inst := `To download the entire directory using gocommands, run the following command: 
	gocmd get -K --progress %s %s
	This is just an example command. You may need to adjust it based on your requirements.
	You will need to have gocommands installed and configured to use this command.
	Check out https://learning.cyverse.org/ds/gocommands/ for more details.
	`
	return fmt.Sprintf(inst, irodsPath, localPath)
}

func (t *DownloadFile) getICommandsInstruction(irodsPath string, localPath string, recursive bool) string {
	inst := ""
	if recursive {
		inst = `To download the entire directory using gocommands, run the following command: 
	iget -K -r -P %s %s
	This is just an example command. You may need to adjust it based on your requirements.
	You will need to have iCommands installed and configured to use this command.
	Check out https://learning.cyverse.org/ds/icommands/ for more details.
	`
	} else {
		inst = `To download the file using gocommands, run the following command: 
	iget -K -P %s %s
	This is just an example command. You may need to adjust it based on your requirements.
	You will need to have iCommands installed and configured to use this command.
	Check out https://learning.cyverse.org/ds/icommands/ for more details.
	`
	}

	return fmt.Sprintf(inst, irodsPath, localPath)
}
