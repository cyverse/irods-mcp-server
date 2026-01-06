package common

import (
	"fmt"
	"net/url"
	"path"
	"strings"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/irods-mcp-server/common"
)

func GetEmptyIRODSAccount(config *common.Config) *irodsclient_types.IRODSAccount {
	return config.Config.ToIRODSAccount()
}

func GetHomePath(config *common.Config, account *irodsclient_types.IRODSAccount) string {
	return account.GetHomeDirPath()
}

func GetSharedPath(config *common.Config, account *irodsclient_types.IRODSAccount) string {
	if account == nil {
		account = GetEmptyIRODSAccount(config)
	}
	return fmt.Sprintf("/%s/home/%s", account.ClientZone, config.IRODSSharedDirName)
}

func MakeWebdavURL(config *common.Config, irodsPath string, account *irodsclient_types.IRODSAccount) string {
	return MakeWebdavURLWithAccesses(config, irodsPath, account, nil)
}

func MakeWebdavURLWithAccesses(config *common.Config, irodsPath string, account *irodsclient_types.IRODSAccount, accesses []*irodsclient_types.IRODSAccess) string {
	if account == nil || account.IsAnonymousUser() {
		return MakeWebdavURLForUser(config, irodsPath, "anonymous")
	}

	for _, access := range accesses {
		// if anonymous can read the file, return the anonymous webdav URL
		if access.UserName == "anonymous" {
			return MakeWebdavURLForUser(config, irodsPath, "anonymous")
		}
	}

	return MakeWebdavURLForUser(config, irodsPath, account.ClientUser)
}

func MakeWebdavURLForUser(config *common.Config, irodsPath string, user string) string {
	if config.IRODSWebDAVURL == "" {
		return ""
	}

	webdavURL, err := url.Parse(config.IRODSWebDAVURL)
	if err != nil {
		return ""
	}

	if user != "" {
		if user == "anonymous" {
			webdavURL.User = url.UserPassword("anonymous", "")
		} else {
			webdavURL.User = url.User(user)
		}
	}

	escapedIRODSPath := encodeIRODSPathSegments(irodsPath)

	// add irodsPath
	webdavURL.Path = path.Join(webdavURL.Path, escapedIRODSPath)
	return webdavURL.String()
}

func encodeIRODSPathSegments(irodsPath string) string {
	trimmedPath := strings.Trim(irodsPath, "/")

	if trimmedPath == "" {
		return ""
	}

	segments := strings.Split(trimmedPath, "/")

	var encodedSegments []string
	for _, segment := range segments {
		encodedSegments = append(encodedSegments, url.PathEscape(segment))
	}

	return strings.Join(encodedSegments, "/")
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
