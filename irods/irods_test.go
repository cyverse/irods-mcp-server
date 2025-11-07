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
	mcpConfig *common.Config
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
	t.Run("SearchFilesByAVU", testSearchFilesByAVU)
	t.Run("GetFileInfo", testGetFileInfo)
	t.Run("GetFileInfoForDir", testGetFileInfoForDir)
	t.Run("ReadFile", testReadFile)
}

func getTestServerConfig() *common.Config {
	config := common.NewDefaultConfig()
	config.Config.Host = "data.cyverse.org"
	config.Config.Port = 1247
	config.Config.ZoneName = "iplant"
	config.Config.Username = "anonymous"
	config.IRODSSharedDirName = "shared"

	return config
}

func Init() {
	config := getTestServerConfig()

	svr, err := NewIRODSMCPServer(nil, config)
	if err != nil {
		panic(err)
	}
	mcpServer = svr
	mcpConfig = config
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
	authVal, err := common.GetAuthValue(ctx)
	if err != nil {
		t.Fatalf("failed to get auth value: %v", err)
	}

	mcpAccount, err := mcpServer.GetIRODSAccountFromAuthValue(&authVal)
	if err != nil {
		t.Fatalf("failed to get irods account from auth value: %v", err)
	}

	req := model.ToolRequest{
		Params: model.ToolRequestParams{
			Arguments: map[string]interface{}{
				"path": irods_common.GetSharedPath(mcpConfig, mcpAccount) + "/terraref",
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
	authVal, err := common.GetAuthValue(ctx)
	if err != nil {
		t.Fatalf("failed to get auth value: %v", err)
	}

	mcpAccount, err := mcpServer.GetIRODSAccountFromAuthValue(&authVal)
	if err != nil {
		t.Fatalf("failed to get irods account from auth value: %v", err)
	}

	req := model.ToolRequest{
		Params: model.ToolRequestParams{
			Arguments: map[string]interface{}{
				"path": irods_common.GetHomePath(mcpConfig, mcpAccount),
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
	authVal, err := common.GetAuthValue(ctx)
	if err != nil {
		t.Fatalf("failed to get auth value: %v", err)
	}

	mcpAccount, err := mcpServer.GetIRODSAccountFromAuthValue(&authVal)
	if err != nil {
		t.Fatalf("failed to get irods account from auth value: %v", err)
	}

	req := model.ToolRequest{
		Params: model.ToolRequestParams{
			Arguments: map[string]interface{}{
				"path": irods_common.GetSharedPath(mcpConfig, mcpAccount),
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
	authVal, err := common.GetAuthValue(ctx)
	if err != nil {
		t.Fatalf("failed to get auth value: %v", err)
	}

	mcpAccount, err := mcpServer.GetIRODSAccountFromAuthValue(&authVal)
	if err != nil {
		t.Fatalf("failed to get irods account from auth value: %v", err)
	}

	req := model.ToolRequest{
		Params: model.ToolRequestParams{
			Arguments: map[string]interface{}{
				"path": irods_common.GetHomePath(mcpConfig, mcpAccount),
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
	authVal, err := common.GetAuthValue(ctx)
	if err != nil {
		t.Fatalf("failed to get auth value: %v", err)
	}

	mcpAccount, err := mcpServer.GetIRODSAccountFromAuthValue(&authVal)
	if err != nil {
		t.Fatalf("failed to get irods account from auth value: %v", err)
	}

	req := model.ToolRequest{
		Params: model.ToolRequestParams{
			Arguments: map[string]interface{}{
				"path":  irods_common.GetSharedPath(mcpConfig, mcpAccount) + "/terraref",
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
	authVal, err := common.GetAuthValue(ctx)
	if err != nil {
		t.Fatalf("failed to get auth value: %v", err)
	}

	mcpAccount, err := mcpServer.GetIRODSAccountFromAuthValue(&authVal)
	if err != nil {
		t.Fatalf("failed to get irods account from auth value: %v", err)
	}

	req := model.ToolRequest{
		Params: model.ToolRequestParams{
			Arguments: map[string]interface{}{
				"path":  irods_common.GetSharedPath(mcpConfig, mcpAccount),
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
	authVal, err := common.GetAuthValue(ctx)
	if err != nil {
		t.Fatalf("failed to get auth value: %v", err)
	}

	mcpAccount, err := mcpServer.GetIRODSAccountFromAuthValue(&authVal)
	if err != nil {
		t.Fatalf("failed to get irods account from auth value: %v", err)
	}

	req := model.ToolRequest{
		Params: model.ToolRequestParams{
			Arguments: map[string]interface{}{
				"path": irods_common.GetSharedPath(mcpConfig, mcpAccount) + "/terraref/README*",
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
	authVal, err := common.GetAuthValue(ctx)
	if err != nil {
		t.Fatalf("failed to get auth value: %v", err)
	}

	mcpAccount, err := mcpServer.GetIRODSAccountFromAuthValue(&authVal)
	if err != nil {
		t.Fatalf("failed to get irods account from auth value: %v", err)
	}

	req := model.ToolRequest{
		Params: model.ToolRequestParams{
			Arguments: map[string]interface{}{
				"path": irods_common.GetHomePath(mcpConfig, mcpAccount) + "/*",
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

func testSearchFilesByAVU(t *testing.T) {
	myTool := mcpServer.GetTool(SearchFilesByAVUName)
	assert.NotNil(t, myTool)

	assert.NotEmpty(t, myTool.GetName())
	assert.NotEmpty(t, myTool.GetDescription())

	ctx := common.AuthForTest()
	req := model.ToolRequest{
		Params: model.ToolRequestParams{
			Arguments: map[string]interface{}{
				"attribute": "ipc_UUID",
				"value":     "fb9894d8-ecce-11e6-a7b0-000e1e0af2dc",
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
	assert.Contains(t, result.Content[0].Text, "\"search_attribute\":")
	assert.Contains(t, result.Content[0].Text, "\"search_value\":")
	assert.Contains(t, result.Content[0].Text, "\"matching_entries\":")
}

func testGetFileInfo(t *testing.T) {
	myTool := mcpServer.GetTool(GetFileInfoName)
	assert.NotNil(t, myTool)

	assert.NotEmpty(t, myTool.GetName())
	assert.NotEmpty(t, myTool.GetDescription())

	ctx := common.AuthForTest()
	authVal, err := common.GetAuthValue(ctx)
	if err != nil {
		t.Fatalf("failed to get auth value: %v", err)
	}

	mcpAccount, err := mcpServer.GetIRODSAccountFromAuthValue(&authVal)
	if err != nil {
		t.Fatalf("failed to get irods account from auth value: %v", err)
	}

	req := model.ToolRequest{
		Params: model.ToolRequestParams{
			Arguments: map[string]interface{}{
				"path": irods_common.GetSharedPath(mcpConfig, mcpAccount) + "/terraref/README.txt",
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
	authVal, err := common.GetAuthValue(ctx)
	if err != nil {
		t.Fatalf("failed to get auth value: %v", err)
	}

	mcpAccount, err := mcpServer.GetIRODSAccountFromAuthValue(&authVal)
	if err != nil {
		t.Fatalf("failed to get irods account from auth value: %v", err)
	}

	req := model.ToolRequest{
		Params: model.ToolRequestParams{
			Arguments: map[string]interface{}{
				"path": irods_common.GetSharedPath(mcpConfig, mcpAccount) + "/terraref",
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
	authVal, err := common.GetAuthValue(ctx)
	if err != nil {
		t.Fatalf("failed to get auth value: %v", err)
	}

	mcpAccount, err := mcpServer.GetIRODSAccountFromAuthValue(&authVal)
	if err != nil {
		t.Fatalf("failed to get irods account from auth value: %v", err)
	}

	req := model.ToolRequest{
		Params: model.ToolRequestParams{
			Arguments: map[string]interface{}{
				"path": irods_common.GetSharedPath(mcpConfig, mcpAccount) + "/terraref/README.txt",
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
