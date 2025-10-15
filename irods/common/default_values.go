package common

import (
	"time"

	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
)

const (
	ClientProgramName          string                     = "irods-mcp-server"
	FilesystemTimeout          irodsclient_types.Duration = irodsclient_types.Duration(1 * time.Minute)
	LongFilesystemTimeout      irodsclient_types.Duration = irodsclient_types.Duration(10 * time.Minute) // exceptionally long timeout for listing dirs or users
	transferThreadNumDefault   int                        = 5
	tcpBufferSizeStringDefault string                     = "1MB"
	fsPoolTimeout              time.Duration              = 10 * time.Minute
)

const (
	IRODSAPIPrefix          string = "irods_"
	DefaultTreeScanMaxDepth int    = 3
	MaxTreeScanDepth        int    = 10
	IRODSScheme             string = "irods"
)

func GetDefaultTCPBufferSize() int {
	size, _ := ParseSize(GetDefaultTCPBufferSizeString())
	return int(size)
}

func GetDefaultTCPBufferSizeString() string {
	return tcpBufferSizeStringDefault
}

func GetDefaultTransferThreadNum() int {
	return transferThreadNumDefault
}

func GetDefaultVerifyChecksum() bool {
	return false
}
