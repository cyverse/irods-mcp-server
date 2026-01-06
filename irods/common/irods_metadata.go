package common

var (
	systemAttributes   []string        = []string{"ipc_UUID"}
	systemAttributeMap map[string]bool = map[string]bool{}
)

func GetSystemAttributes() []string {
	return systemAttributes
}

func IsSystemAttribute(attr string) bool {
	if len(systemAttributeMap) != len(systemAttributes) {
		for _, a := range systemAttributes {
			systemAttributeMap[a] = true
		}
	}

	if _, ok := systemAttributeMap[attr]; ok {
		// has it
		return true
	}
	return false
}
