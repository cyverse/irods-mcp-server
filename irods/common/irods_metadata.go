package common

var (
	systemAttributes   = []string{"ipc_UUID"}
	systemAttributeMap = map[string]bool{}
)

func init() {
	for _, a := range systemAttributes {
		systemAttributeMap[a] = true
	}
}

func GetSystemAttributes() []string {
	return systemAttributes
}

func IsSystemAttribute(attr string) bool {
	return systemAttributeMap[attr]
}
