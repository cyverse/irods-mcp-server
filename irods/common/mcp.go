package common

import (
	"encoding/base64"
	"encoding/json"

	"github.com/cockroachdb/errors"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func ToolErrorResult(err error) *mcp.CallToolResult {
	result := mcp.CallToolResult{}
	result.SetError(err)

	return &result
}

func ToolTextResult(text string) *mcp.CallToolResult {
	result := mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: text,
			},
		},
		IsError: false,
	}

	return &result
}

func ToolJSONResult(data any) (*mcp.CallToolResult, error) {
	result := mcp.CallToolResult{}
	jsonData, err := json.Marshal(data)
	if err != nil {
		return ToolErrorResult(errors.Wrapf(err, "failed to marshal data to JSON")), nil
	}
	result.StructuredContent = json.RawMessage(jsonData)

	return &result, nil
}

func ResourceTextResult(uri string, mimetype string, text string) (*mcp.ReadResourceResult, error) {
	result := mcp.ReadResourceResult{}
	result.Contents = []*mcp.ResourceContents{
		{
			URI:      uri,
			MIMEType: mimetype,
			Text:     text,
		},
	}

	return &result, nil
}

func ResourceJSONResult(uri string, data any) (*mcp.ReadResourceResult, error) {
	result := mcp.ReadResourceResult{}
	jsonData, err := json.Marshal(data)
	if err != nil {
		jsonData = []byte(`{"error": "failed to marshal data to JSON"}`)
	}

	result.Contents = []*mcp.ResourceContents{
		{
			URI:      uri,
			MIMEType: "application/json",
			Text:     string(jsonData),
		},
	}

	return &result, nil
}

func ResourceBlobResult(uri string, mimetype string, blob []byte) (*mcp.ReadResourceResult, error) {
	result := mcp.ReadResourceResult{}
	result.Contents = []*mcp.ResourceContents{
		{
			URI:      uri,
			MIMEType: mimetype,
			Blob:     []byte(base64.StdEncoding.EncodeToString(blob)),
		},
	}

	return &result, nil
}

func MarshalInputArguments(tool *mcp.Tool, request *mcp.CallToolRequest, arguments any) error {
	inputSchema, ok := tool.InputSchema.(*jsonschema.Schema)
	if !ok {
		return errors.Newf("input schema is not a JSON schema")
	}

	resolvedSchema, err := inputSchema.Resolve(&jsonschema.ResolveOptions{ValidateDefaults: true})
	if err != nil {
		return errors.Wrapf(err, "failed to resolve input schema")
	}

	var input json.RawMessage
	if request.Params.Arguments != nil {
		input = request.Params.Arguments
	}

	// validate
	v := make(map[string]any)
	if len(input) > 0 {
		if err := json.Unmarshal(input, &v); err != nil {
			return errors.Wrapf(err, "unmarshaling arguments")
		}
	}
	if err := resolvedSchema.ApplyDefaults(&v); err != nil {
		return errors.Wrapf(err, "applying schema defaults")
	}
	if err := resolvedSchema.Validate(&v); err != nil {
		return errors.Wrapf(err, "validating arguments")
	}

	// We must re-marshal with the default values applied.
	input, err = json.Marshal(v)
	if err != nil {
		return errors.Wrapf(err, "marshalling with defaults")
	}

	// Unmarshal and validate args.
	if err := json.Unmarshal(input, arguments); err != nil {
		return errors.Wrapf(err, "failed to unmarshal arguments")
	}

	return nil
}
