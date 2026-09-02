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

	buffer := make([]byte, maxReadLen)
	total := 0
	for total < len(buffer) {
		n, err := handle.ReadAt(buffer[total:], offset+int64(total))
		total += n
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, errors.Wrapf(err, "failed to read file %q at offset %d length %d", sourcePath, offset, maxReadLen)
		}
	}

	return buffer[:total], nil
}

func WriteDataObject(filesystem *irodsclient_fs.FileSystem, destPath string, offset int64, content []byte) error {
	// "w" = FileOpenModeWriteOnly: write without truncation.
	// Use "w+" (FileOpenModeWriteTruncate) only when a full overwrite is intended.
	handle, err := filesystem.OpenFile(destPath, "", "w")
	if err != nil {
		return errors.Wrapf(err, "failed to open file %q", destPath)
	}
	defer handle.Close()

	written := 0
	for written < len(content) {
		n, err := handle.WriteAt(content[written:], offset+int64(written))
		written += n
		if err != nil {
			return errors.Wrapf(err, "failed to write file %q at offset %d length %d", destPath, offset, len(content))
		}
	}

	return nil
}
