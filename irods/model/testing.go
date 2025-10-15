package model

import (
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"
)

type ToolRequestParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

type ToolRequest struct {
	Params ToolRequestParams `json:"params"`
}

func (t *ToolRequest) ToCallToolRequest() (mcp.CallToolRequest, error) {
	var req mcp.CallToolRequest

	jsonBytes, err := json.Marshal(t)
	if err != nil {
		return mcp.CallToolRequest{}, err
	}

	err = json.Unmarshal(jsonBytes, &req)
	if err != nil {
		return mcp.CallToolRequest{}, err
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
	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return err
	}

	err = json.Unmarshal(jsonBytes, t)
	if err != nil {
		return err
	}
	return nil
}
