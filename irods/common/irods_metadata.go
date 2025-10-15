package common

var (
	systemMetadataNames []string = []string{"ipc_UUID"}
)

func GetSystemMetadataNames() []string {
	return systemMetadataNames
}
