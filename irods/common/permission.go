package common

import (
	"path"
	"strings"
)

func IsAccessAllowed(irodsPath string, allowedPaths []string) bool {
	irodsPath = path.Clean(irodsPath)

	for _, allowedPath := range allowedPaths {
		//fmt.Printf("Checking access: irodsPath=%q, allowedPath=%q\n", irodsPath, allowedPath)

		if strings.HasSuffix(allowedPath, "/*") {
			baseDir := strings.TrimSuffix(allowedPath, "/*")

			if strings.HasPrefix(irodsPath, baseDir+"/") {
				//fmt.Printf("Access allowed (directory wildcard): irodsPath=%q, allowedPath=%q\n", irodsPath, allowedPath)
				return true
			}
		} else {
			matched, _ := path.Match(allowedPath, irodsPath)
			if matched {
				//fmt.Printf("Access allowed: irodsPath=%q, allowedPath=%q\n", irodsPath, allowedPath)
				return true
			}
		}
	}

	return false
}
