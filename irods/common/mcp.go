package common

import (
	"github.com/cockroachdb/errors"
	"github.com/mark3labs/mcp-go/mcp"
)

func OutputMCPError(err error) (*mcp.CallToolResult, error) {
	result := mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: err.Error(),
			},
		},
		IsError: true,
	}

	return &result, nil
}

func GetInputStringArgument(arguments map[string]any, key string, required bool) (string, error) {
	value, ok := arguments[key]
	if !ok {
		if required {
			return "", errors.Newf("argument %q is required", key)
		}

		return "", nil
	}

	stringVal, ok := value.(string)
	if !ok {
		return "", errors.Newf("value for key %q is not a string", key)
	}

	if stringVal == "" && required {
		return "", errors.Newf("argument %q is required", key)
	}

	return stringVal, nil
}

func GetInputNumberArgument(arguments map[string]any, key string) (float64, error) {
	value, ok := arguments[key]
	if !ok {
		return 0, nil
	}

	numberVal, ok := value.(float64)
	if !ok {
		return 0, errors.Newf("value for key %q is not a number", key)
	}

	return numberVal, nil
}

func GetInputBooleanArgument(arguments map[string]any, key string) (bool, error) {
	value, ok := arguments[key]
	if !ok {
		return false, nil
	}

	booleanVal, ok := value.(bool)
	if !ok {
		return false, errors.Newf("value for key %q is not a boolean", key)
	}

	return booleanVal, nil
}
