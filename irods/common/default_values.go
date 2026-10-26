package common

import (
	"time"

	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
)

const (
	ClientProgramName     string                     = "irods-mcp-server"
	FilesystemTimeout     irodsclient_types.Duration = irodsclient_types.Duration(5 * time.Minute)
	LongFilesystemTimeout irodsclient_types.Duration = irodsclient_types.Duration(10 * time.Minute) // exceptionally long timeout for listing dirs or users
	fsPoolTimeout         time.Duration              = 10 * time.Minute

	IRODSAPIPrefix          string = "irods__"
	DefaultTreeScanMaxDepth int    = 3
	MaxTreeScanDepth        int    = 10
	IRODSScheme             string = "irods"
)
