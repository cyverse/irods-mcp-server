package common

import (
	"fmt"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
)

// TODO: make these configurable via env vars or config files
func GetEmptyIRODSAccount() *irodsclient_types.IRODSAccount {
	return &irodsclient_types.IRODSAccount{
		Host:                 "data.cyverse.org",
		Port:                 1247,
		ClientZone:           "iplant",
		AuthenticationScheme: irodsclient_types.AuthSchemeNative,
	}
}

func GetHomePath() string {
	account := GetEmptyIRODSAccount()
	return fmt.Sprintf("/%s/home", account.ClientZone)
}

// TODO: make this configurable via env vars or config files
func GetSharedPath() string {
	account := GetEmptyIRODSAccount()
	return fmt.Sprintf("/%s/home/shared", account.ClientZone)
}

// TODO: make this configurable via env vars or config files
func MakeWebdavURL(irodsPath string) string {
	account := GetEmptyIRODSAccount()
	return fmt.Sprintf("https://%s/dav%s", account.Host, irodsPath)
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
