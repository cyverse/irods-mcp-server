package common

import (
	"mime"
	"net/http"
	"path"
	"strings"
)

const (
	MIME_TYPE_READ_SIZE int64 = 512 // 512 bytes
)

// DetectMimeTypeWithExtension detects the mime type of a file based on its extension
func DetectMimeTypeWithExtension(sourcePath string) string {
	ext := path.Ext(sourcePath)
	if len(ext) > 0 {
		mimeType := mime.TypeByExtension(ext)
		if len(mimeType) > 0 {
			return mimeType
		}
	}

	return "application/octet-stream"
}

// DetectMimeTypeWithContent detects the mime type of a file based on its extension and content
func DetectMimeTypeWithContent(sourcePath string, offset int64, content []byte) string {
	ext := path.Ext(sourcePath)
	if len(ext) > 0 {
		mimeType := mime.TypeByExtension(ext)
		if len(mimeType) > 0 {
			return mimeType
		}
	}

	if offset == 0 {
		return http.DetectContentType(content)
	}

	return "application/octet-stream"
}

// IsTextFile checks if the mimetype is for test files
func IsTextFile(mimeType string) bool {
	return strings.HasPrefix(mimeType, "text/") ||
		mimeType == "application/json" ||
		mimeType == "application/xml" ||
		mimeType == "application/javascript" ||
		mimeType == "application/x-javascript" ||
		strings.Contains(mimeType, "+xml") ||
		strings.Contains(mimeType, "+json")
}

// IsImageFile checks if the mimetype is for image files
func IsImageFile(mimeType string) bool {
	return strings.HasPrefix(mimeType, "image/")
}
