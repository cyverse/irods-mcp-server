package common

import "fmt"

func MakeResourceURI(irodsPath string) string {
	return fmt.Sprintf("%s://%s", IRODSScheme, irodsPath)
}
