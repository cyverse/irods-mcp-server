package common

import (
	"io"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	"golang.org/x/xerrors"
)

const (
	MinReadLength int64 = 64 * 1024 // 64KB

	MaxInlineSize int64 = 1 * 1024 * 1024 // 1MB
	MaxBase64Size int64 = 1 * 1024 * 1024 // 1MB
)

func ReadDataObject(filesystem *irodsclient_fs.FileSystem, sourcePath string, maxReadLen int64) ([]byte, error) {
	handle, err := filesystem.OpenFile(sourcePath, "", "r")
	if err != nil {
		return nil, xerrors.Errorf("failed to open file %q: %w", sourcePath, err)
	}
	defer handle.Close()

	// read the file content
	buffer := make([]byte, maxReadLen)
	n, err := handle.Read(buffer)
	if err != nil {
		if err == io.EOF {
			// EOF is not an error
			return buffer[:n], nil
		}

		return nil, xerrors.Errorf("failed to read file %q: %w", sourcePath, err)
	}

	return buffer[:n], nil
}
