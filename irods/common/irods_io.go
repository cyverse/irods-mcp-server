package common

import (
	"io"

	"github.com/cockroachdb/errors"
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
)

const (
	MinReadLength int64 = 64 * 1024 // 64KB

	MaxInlineSize int64 = 1 * 1024 * 1024 // 1MB
	MaxBase64Size int64 = 1 * 1024 * 1024 // 1MB
)

func ReadDataObject(filesystem *irodsclient_fs.FileSystem, sourcePath string, offset int64, maxReadLen int64) ([]byte, error) {
	handle, err := filesystem.OpenFile(sourcePath, "", "r")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open file %q", sourcePath)
	}
	defer handle.Close()

	// read the file content
	buffer := make([]byte, maxReadLen)

	n, err := handle.ReadAt(buffer, offset)
	if err != nil {
		if err == io.EOF {
			// EOF is not an error
			return buffer[:n], nil
		}

		return nil, errors.Wrapf(err, "failed to read file %q at offset %d length %d", sourcePath, offset, maxReadLen)
	}

	return buffer[:n], nil
}

func WriteDataObject(filesystem *irodsclient_fs.FileSystem, destPath string, offset int64, content []byte) error {
	handle, err := filesystem.OpenFile(destPath, "", "w")
	if err != nil {
		return errors.Wrapf(err, "failed to open file %q", destPath)
	}
	defer handle.Close()

	// write the file content
	_, err = handle.WriteAt(content, offset)
	if err != nil {
		return errors.Wrapf(err, "failed to write file %q at offset %d length %d", destPath, offset, len(content))
	}

	return nil
}
