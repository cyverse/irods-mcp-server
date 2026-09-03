package model

import (
	"encoding/json"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ToolRequestParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

type ToolRequest struct {
	Params ToolRequestParams `json:"params"`
}

func (t *ToolRequest) ToCallToolRequest() (mcp.CallToolRequest, error) {
	argsJSON, err := json.Marshal(t.Params.Arguments)
	if err != nil {
		return mcp.CallToolRequest{}, err
	}
	var req mcp.CallToolRequest
	req.Params = &mcp.CallToolParamsRaw{
		Name:      t.Params.Name,
		Arguments: argsJSON,
	}
	return req, nil
}

type ToolResponseContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type ToolResponse struct {
	Content []ToolResponseContent `json:"content"`
}

func (t *ToolResponse) FromCallToolResult(result *mcp.CallToolResult) error {
	t.Content = make([]ToolResponseContent, 0, len(result.Content))
	for _, c := range result.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			t.Content = append(t.Content, ToolResponseContent{Type: "text", Text: tc.Text})
		}
	}
	return nil
}
