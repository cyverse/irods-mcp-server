package common

import "github.com/mark3labs/mcp-go/mcp"

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
