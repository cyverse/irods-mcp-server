package irods

import (
	"testing"

	"github.com/cyverse/irods-mcp-server/common"
	irods_common "github.com/cyverse/irods-mcp-server/irods/common"
	"github.com/cyverse/irods-mcp-server/irods/model"
	"github.com/stretchr/testify/assert"
)

var (
	mcpServer *IRODSMCPServer
)

func TestTool(t *testing.T) {
	Init()

	t.Run("ListAllowedDirectoriesAndAPIs", testListAllowedDirectoriesAndAPIs)
	t.Run("ListDirectory", testListDirectory)
	t.Run("ListDirectoryRejected", testListDirectoryRejected)
	t.Run("ListDirectoryDetails", testListDirectoryDetails)
	t.Run("ListDirectoryDetailsRejected", testListDirectoryDetailsRejected)
	t.Run("DirectoryTree", testDirectoryTree)
	t.Run("DirectoryTreeRejected", testDirectoryTreeRejected)
	t.Run("SearchFiles", testSearchFiles)
	t.Run("SearchFilesRejected", testSearchFilesRejected)
	t.Run("GetFileInfo", testGetFileInfo)
	t.Run("GetFileInfoForDir", testGetFileInfoForDir)
	t.Run("ReadFile", testReadFile)
}

func Init() {
	svr, err := NewIRODSMCPServer(nil)
	if err != nil {
		panic(err)
	}
	mcpServer = svr
}

func testListAllowedDirectoriesAndAPIs(t *testing.T) {
	myTool := mcpServer.GetTool(ListAllowedDirectoriesName)
	assert.NotNil(t, myTool)

	assert.NotEmpty(t, myTool.GetName())
	assert.NotEmpty(t, myTool.GetDescription())

	ctx := common.AuthForTest()
	req := model.ToolRequest{}

	toolReq, err := req.ToCallToolRequest()
	if err != nil {
		t.Fatalf("failed to create tool request: %v", err)
	}

	handler := myTool.GetHandler()

	res, err := handler(ctx, toolReq)
	if err != nil {
		t.Errorf("failed to call %s: %v", myTool.GetName(), err)
	}
	assert.NotNil(t, res)

	result := model.ToolResponse{}
	err = result.FromCallToolResult(res)
	if err != nil {
		t.Errorf("failed to load result for call %s: %v", myTool.GetName(), err)
	}
	assert.NotEmpty(t, result.Content)

	assert.Contains(t, result.Content[0].Type, "text")
	assert.Contains(t, result.Content[0].Text, "{\"directories\":")
}

func testListDirectory(t *testing.T) {
	myTool := mcpServer.GetTool(ListDirectoryName)
	assert.NotNil(t, myTool)

	assert.NotEmpty(t, myTool.GetName())
	assert.NotEmpty(t, myTool.GetDescription())

	ctx := common.AuthForTest()
	req := model.ToolRequest{
		Params: model.ToolRequestParams{
			Arguments: map[string]interface{}{
				"path": irods_common.GetSharedPath(),
			},
		},
	}

	toolReq, err := req.ToCallToolRequest()
	if err != nil {
		t.Fatalf("failed to create tool request: %v", err)
	}

	handler := myTool.GetHandler()

	res, err := handler(ctx, toolReq)
	if err != nil {
		t.Errorf("failed to call %s: %v", myTool.GetName(), err)
	}
	assert.NotNil(t, res)

	result := model.ToolResponse{}
	err = result.FromCallToolResult(res)
	if err != nil {
		t.Errorf("failed to load result for call %s: %v", myTool.GetName(), err)
	}
	assert.NotEmpty(t, result.Content)

	assert.Contains(t, result.Content[0].Type, "text")
	assert.Contains(t, result.Content[0].Text, "{\"directory_info\":")
}

func testListDirectoryRejected(t *testing.T) {
	myTool := mcpServer.GetTool(ListDirectoryName)
	assert.NotNil(t, myTool)

	assert.NotEmpty(t, myTool.GetName())
	assert.NotEmpty(t, myTool.GetDescription())

	ctx := common.AuthForTest()
	req := model.ToolRequest{
		Params: model.ToolRequestParams{
			Arguments: map[string]interface{}{
				"path": irods_common.GetHomePath(),
			},
		},
	}

	toolReq, err := req.ToCallToolRequest()
	if err != nil {
		t.Fatalf("failed to create tool request: %v", err)
	}

	handler := myTool.GetHandler()

	res, err := handler(ctx, toolReq)
	if err != nil {
		t.Errorf("failed to call %s: %v", myTool.GetName(), err)
	}
	assert.NotNil(t, res)

	result := model.ToolResponse{}
	err = result.FromCallToolResult(res)
	if err != nil {
		t.Errorf("failed to load result for call %s: %v", myTool.GetName(), err)
	}
	assert.NotEmpty(t, result.Content)

	assert.Contains(t, result.Content[0].Type, "text")
	assert.Contains(t, result.Content[0].Text, "not permitted")
}

func testListDirectoryDetails(t *testing.T) {
	myTool := mcpServer.GetTool(ListDirectoryDetailsName)
	assert.NotNil(t, myTool)

	assert.NotEmpty(t, myTool.GetName())
	assert.NotEmpty(t, myTool.GetDescription())

	ctx := common.AuthForTest()
	req := model.ToolRequest{
		Params: model.ToolRequestParams{
			Arguments: map[string]interface{}{
				"path": irods_common.GetSharedPath(),
			},
		},
	}

	toolReq, err := req.ToCallToolRequest()
	if err != nil {
		t.Fatalf("failed to create tool request: %v", err)
	}

	handler := myTool.GetHandler()

	res, err := handler(ctx, toolReq)
	if err != nil {
		t.Errorf("failed to call %s: %v", myTool.GetName(), err)
	}
	assert.NotNil(t, res)

	result := model.ToolResponse{}
	err = result.FromCallToolResult(res)
	if err != nil {
		t.Errorf("failed to load result for call %s: %v", myTool.GetName(), err)
	}
	assert.NotEmpty(t, result.Content)

	assert.Contains(t, result.Content[0].Type, "text")
	assert.Contains(t, result.Content[0].Text, "{\"directory_info\":")
}

func testListDirectoryDetailsRejected(t *testing.T) {
	myTool := mcpServer.GetTool(ListDirectoryDetailsName)
	assert.NotNil(t, myTool)

	assert.NotEmpty(t, myTool.GetName())
	assert.NotEmpty(t, myTool.GetDescription())

	ctx := common.AuthForTest()
	req := model.ToolRequest{
		Params: model.ToolRequestParams{
			Arguments: map[string]interface{}{
				"path": irods_common.GetHomePath(),
			},
		},
	}

	toolReq, err := req.ToCallToolRequest()
	if err != nil {
		t.Fatalf("failed to create tool request: %v", err)
	}

	handler := myTool.GetHandler()

	res, err := handler(ctx, toolReq)
	if err != nil {
		t.Errorf("failed to call %s: %v", myTool.GetName(), err)
	}
	assert.NotNil(t, res)

	result := model.ToolResponse{}
	err = result.FromCallToolResult(res)
	if err != nil {
		t.Errorf("failed to load result for call %s: %v", myTool.GetName(), err)
	}
	assert.NotEmpty(t, result.Content)

	assert.Contains(t, result.Content[0].Type, "text")
	assert.Contains(t, result.Content[0].Text, "not permitted")
}

func testDirectoryTree(t *testing.T) {
	myTool := mcpServer.GetTool(DirectoryTreeName)
	assert.NotNil(t, myTool)

	assert.NotEmpty(t, myTool.GetName())
	assert.NotEmpty(t, myTool.GetDescription())

	ctx := common.AuthForTest()
	req := model.ToolRequest{
		Params: model.ToolRequestParams{
			Arguments: map[string]interface{}{
				"path":  irods_common.GetSharedPath() + "/terraref",
				"depth": 2,
			},
		},
	}

	toolReq, err := req.ToCallToolRequest()
	if err != nil {
		t.Fatalf("failed to create tool request: %v", err)
	}

	handler := myTool.GetHandler()

	res, err := handler(ctx, toolReq)
	if err != nil {
		t.Errorf("failed to call %s: %v", myTool.GetName(), err)
	}
	assert.NotNil(t, res)

	result := model.ToolResponse{}
	err = result.FromCallToolResult(res)
	if err != nil {
		t.Errorf("failed to load result for call %s: %v", myTool.GetName(), err)
	}
	assert.NotEmpty(t, result.Content)

	assert.Contains(t, result.Content[0].Type, "text")
	assert.Contains(t, result.Content[0].Text, "{\"directory_info\":")
}

func testDirectoryTreeRejected(t *testing.T) {
	myTool := mcpServer.GetTool(DirectoryTreeName)
	assert.NotNil(t, myTool)

	assert.NotEmpty(t, myTool.GetName())
	assert.NotEmpty(t, myTool.GetDescription())

	ctx := common.AuthForTest()
	req := model.ToolRequest{
		Params: model.ToolRequestParams{
			Arguments: map[string]interface{}{
				"path":  irods_common.GetSharedPath(),
				"depth": 2,
			},
		},
	}

	toolReq, err := req.ToCallToolRequest()
	if err != nil {
		t.Fatalf("failed to create tool request: %v", err)
	}

	handler := myTool.GetHandler()

	res, err := handler(ctx, toolReq)
	if err != nil {
		t.Errorf("failed to call %s: %v", myTool.GetName(), err)
	}
	assert.NotNil(t, res)

	result := model.ToolResponse{}
	err = result.FromCallToolResult(res)
	if err != nil {
		t.Errorf("failed to load result for call %s: %v", myTool.GetName(), err)
	}
	assert.NotEmpty(t, result.Content)

	assert.Contains(t, result.Content[0].Type, "text")
	assert.Contains(t, result.Content[0].Text, "not permitted")
}

func testSearchFiles(t *testing.T) {
	myTool := mcpServer.GetTool(SearchFilesName)
	assert.NotNil(t, myTool)

	assert.NotEmpty(t, myTool.GetName())
	assert.NotEmpty(t, myTool.GetDescription())

	ctx := common.AuthForTest()
	req := model.ToolRequest{
		Params: model.ToolRequestParams{
			Arguments: map[string]interface{}{
				"path": irods_common.GetSharedPath() + "/terraref/README*",
			},
		},
	}

	toolReq, err := req.ToCallToolRequest()
	if err != nil {
		t.Fatalf("failed to create tool request: %v", err)
	}

	handler := myTool.GetHandler()

	res, err := handler(ctx, toolReq)
	if err != nil {
		t.Errorf("failed to call %s: %v", myTool.GetName(), err)
	}
	assert.NotNil(t, res)

	result := model.ToolResponse{}
	err = result.FromCallToolResult(res)
	if err != nil {
		t.Errorf("failed to load result for call %s: %v", myTool.GetName(), err)
	}
	assert.NotEmpty(t, result.Content)

	assert.Contains(t, result.Content[0].Type, "text")
	assert.Contains(t, result.Content[0].Text, "\"matching_entries\":")
}

func testSearchFilesRejected(t *testing.T) {
	myTool := mcpServer.GetTool(SearchFilesName)
	assert.NotNil(t, myTool)

	assert.NotEmpty(t, myTool.GetName())
	assert.NotEmpty(t, myTool.GetDescription())

	ctx := common.AuthForTest()
	req := model.ToolRequest{
		Params: model.ToolRequestParams{
			Arguments: map[string]interface{}{
				"path": irods_common.GetSharedPath() + "/README*",
			},
		},
	}

	toolReq, err := req.ToCallToolRequest()
	if err != nil {
		t.Fatalf("failed to create tool request: %v", err)
	}

	handler := myTool.GetHandler()

	res, err := handler(ctx, toolReq)
	if err != nil {
		t.Errorf("failed to call %s: %v", myTool.GetName(), err)
	}
	assert.NotNil(t, res)

	result := model.ToolResponse{}
	err = result.FromCallToolResult(res)
	if err != nil {
		t.Errorf("failed to load result for call %s: %v", myTool.GetName(), err)
	}
	assert.NotEmpty(t, result.Content)

	assert.Contains(t, result.Content[0].Type, "text")
	assert.Contains(t, result.Content[0].Text, "not permitted")
}

func testGetFileInfo(t *testing.T) {
	myTool := mcpServer.GetTool(GetFileInfoName)
	assert.NotNil(t, myTool)

	assert.NotEmpty(t, myTool.GetName())
	assert.NotEmpty(t, myTool.GetDescription())

	ctx := common.AuthForTest()
	req := model.ToolRequest{
		Params: model.ToolRequestParams{
			Arguments: map[string]interface{}{
				"path": irods_common.GetSharedPath() + "/terraref/README.txt",
			},
		},
	}

	toolReq, err := req.ToCallToolRequest()
	if err != nil {
		t.Fatalf("failed to create tool request: %v", err)
	}

	handler := myTool.GetHandler()

	res, err := handler(ctx, toolReq)
	if err != nil {
		t.Errorf("failed to call %s: %v", myTool.GetName(), err)
	}
	assert.NotNil(t, res)

	result := model.ToolResponse{}
	err = result.FromCallToolResult(res)
	if err != nil {
		t.Errorf("failed to load result for call %s: %v", myTool.GetName(), err)
	}
	assert.NotEmpty(t, result.Content)

	assert.Contains(t, result.Content[0].Type, "text")
	assert.Contains(t, result.Content[0].Text, "{\"mime_type\":\"text/plain;")
}

func testGetFileInfoForDir(t *testing.T) {
	myTool := mcpServer.GetTool(GetFileInfoName)
	assert.NotNil(t, myTool)

	assert.NotEmpty(t, myTool.GetName())
	assert.NotEmpty(t, myTool.GetDescription())

	ctx := common.AuthForTest()
	req := model.ToolRequest{
		Params: model.ToolRequestParams{
			Arguments: map[string]interface{}{
				"path": irods_common.GetSharedPath() + "/terraref",
			},
		},
	}

	toolReq, err := req.ToCallToolRequest()
	if err != nil {
		t.Fatalf("failed to create tool request: %v", err)
	}

	handler := myTool.GetHandler()

	res, err := handler(ctx, toolReq)
	if err != nil {
		t.Errorf("failed to call %s: %v", myTool.GetName(), err)
	}
	assert.NotNil(t, res)

	result := model.ToolResponse{}
	err = result.FromCallToolResult(res)
	if err != nil {
		t.Errorf("failed to load result for call %s: %v", myTool.GetName(), err)
	}
	assert.NotEmpty(t, result.Content)

	assert.Contains(t, result.Content[0].Type, "text")
	assert.Contains(t, result.Content[0].Text, "\"mime_type\":\"Directory\"")
}

func testReadFile(t *testing.T) {
	myTool := mcpServer.GetTool(ReadFileName)
	assert.NotNil(t, myTool)

	assert.NotEmpty(t, myTool.GetName())
	assert.NotEmpty(t, myTool.GetDescription())

	ctx := common.AuthForTest()
	req := model.ToolRequest{
		Params: model.ToolRequestParams{
			Arguments: map[string]interface{}{
				"path": irods_common.GetSharedPath() + "/terraref/README.txt",
			},
		},
	}

	toolReq, err := req.ToCallToolRequest()
	if err != nil {
		t.Fatalf("failed to create tool request: %v", err)
	}

	handler := myTool.GetHandler()

	res, err := handler(ctx, toolReq)
	if err != nil {
		t.Errorf("failed to call %s: %v", myTool.GetName(), err)
	}
	assert.NotNil(t, res)

	result := model.ToolResponse{}
	err = result.FromCallToolResult(res)
	if err != nil {
		t.Errorf("failed to load result for call %s: %v", myTool.GetName(), err)
	}
	assert.NotEmpty(t, result.Content)

	assert.Contains(t, result.Content[0].Type, "text")
	assert.Contains(t, result.Content[0].Text, "README")
}
