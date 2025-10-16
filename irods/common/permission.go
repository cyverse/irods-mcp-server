package common

import (
	"strings"
)

func IsAccessAllowed(irodsPath string, allowedPaths []string) bool {
	for _, allowedPath := range allowedPaths {
		// exact match
		if irodsPath == allowedPath {
			return true
		}

		// wildcard match (*)
		if strings.HasSuffix(allowedPath, "/*") {
			if irodsPath == strings.TrimSuffix(allowedPath, "/*") {
				return true
			} else if strings.HasPrefix(irodsPath, strings.TrimSuffix(allowedPath, "*")) {
				return true
			}
		}
	}

	return false
}
