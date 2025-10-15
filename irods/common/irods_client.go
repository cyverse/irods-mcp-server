package common

import (
	"fmt"
	"strings"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/irods-mcp-server/common"
)

// TODO: make these configurable via env vars or config files
func GetEmptyIRODSAccount(config *common.Config) *irodsclient_types.IRODSAccount {
	return config.IRODSConfig.ToIRODSAccount()
}

func GetHomePath(config *common.Config) string {
	account := config.IRODSConfig.ToIRODSAccount()
	if account.IsAnonymousUser() {
		return GetSharedPath(config)
	}

	return account.GetHomeDirPath()
}

func GetSharedPath(config *common.Config) string {
	account := config.IRODSConfig.ToIRODSAccount()
	return fmt.Sprintf("/%s/home/%s", account.ClientZone, config.IRODSSharedDirName)
}

func MakeWebdavURL(config *common.Config, irodsPath string) string {
	if config.IRODSWebDAVURL == "" {
		return ""
	}

	return config.IRODSWebDAVURL + "/" + strings.TrimLeft(irodsPath, "/")
}

// GetIRODSFSClient returns a file system client
func GetIRODSFSClient(account *irodsclient_types.IRODSAccount) (*irodsclient_fs.FileSystem, error) {
	fsConfig := irodsclient_fs.NewFileSystemConfig(ClientProgramName)

	// set operation time out
	fsConfig.MetadataConnection.OperationTimeout = FilesystemTimeout
	fsConfig.IOConnection.OperationTimeout = FilesystemTimeout

	// set tcp buffer size
	fsConfig.MetadataConnection.TcpBufferSize = GetDefaultTCPBufferSize()
	fsConfig.IOConnection.TcpBufferSize = GetDefaultTCPBufferSize()

	fsConfig.Cache.InvalidateParentEntryCacheImmediately = true
	fsConfig.Cache.StartNewTransaction = false

	return irodsclient_fs.NewFileSystem(account, fsConfig)
}
