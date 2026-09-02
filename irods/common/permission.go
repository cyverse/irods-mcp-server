package common

import (
	"path"
	"strings"
)

// IsAccessAllowed reports whether irodsPath is covered by any entry in allowedPaths.
//
// Each entry in allowedPaths is interpreted as either:
//   - a "/*"-suffixed pattern  → prefix match: irodsPath must be a direct or
//     nested child of the base directory (e.g. "/zone/home/user/*" covers
//     "/zone/home/user/foo" and deeper paths).
//   - a bare path              → exact match after path.Clean.
//
// No glob semantics are applied; special characters in iRODS paths (e.g. "[",
// "]") are treated literally.
func IsAccessAllowed(irodsPath string, allowedPaths []string) bool {
	irodsPath = path.Clean(irodsPath)

	for _, allowedPath := range allowedPaths {
		if strings.HasSuffix(allowedPath, "/*") {
			baseDir := strings.TrimSuffix(allowedPath, "/*")
			if strings.HasPrefix(irodsPath, baseDir+"/") {
				return true
			}
		} else {
			if path.Clean(allowedPath) == irodsPath {
				return true
			}
		}
	}

	return false
}
