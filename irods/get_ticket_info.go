package irods

import (
	"context"
	"encoding/json"

	"github.com/cockroachdb/errors"
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	"github.com/cyverse/irods-mcp-server/common"
	irods_common "github.com/cyverse/irods-mcp-server/irods/common"
	"github.com/cyverse/irods-mcp-server/irods/model"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	GetTicketInfoName = irods_common.IRODSAPIPrefix + "get_ticket_info"
)

type GetTicketInfo struct {
	mcpServer *IRODSMCPServer
	config    *common.Config
}

func NewGetTicketInfo(svr *IRODSMCPServer) ToolAPI {
	return &GetTicketInfo{
		mcpServer: svr,
		config:    svr.GetConfig(),
	}
}

func (t *GetTicketInfo) GetName() string {
	return GetTicketInfoName
}

func (t *GetTicketInfo) GetDescription() string {
	return `Get information about a specific iRODS ticket, such as its ID and expiration time, in JSON format.
	Anonymous users are not allowed to get ticket information.`
}

func (t *GetTicketInfo) GetTool() mcp.Tool {
	return mcp.NewTool(
		t.GetName(),
		mcp.WithDescription(t.GetDescription()),
		mcp.WithString(
			"name",
			mcp.Required(),
			mcp.Description("The name of the iRODS ticket to get information about"),
		),
	)
}

func (t *GetTicketInfo) GetHandler() server.ToolHandlerFunc {
	return t.Handler
}

func (t *GetTicketInfo) GetAccessiblePaths(authValue *common.AuthValue) []string {
	return []string{}
}

func (t *GetTicketInfo) Handler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	arguments := request.GetArguments()

	inputName, err := irods_common.GetInputStringArgument(arguments, "name", true)
	if err != nil {
		outputErr := errors.New("failed to get name from arguments")
		return irods_common.OutputMCPError(outputErr)
	}

	// auth
	authValue, err := common.GetAuthValue(ctx)
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to get auth value")
		return irods_common.OutputMCPError(outputErr)
	}

	if authValue.IsAnonymous() {
		outputErr := errors.New("anonymous user is not allowed to list tickets")
		return irods_common.OutputMCPError(outputErr)
	}

	// make a irods filesystem client
	fs, err := t.mcpServer.GetIRODSFSClientFromAuthValue(&authValue)
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to create a irods fs client")
		return irods_common.OutputMCPError(outputErr)
	}

	// get
	content, err := t.getTicketInfo(fs, inputName)
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to get ticket info for %q", inputName)
		return irods_common.OutputMCPError(outputErr)
	}

	return mcp.NewToolResultText(content), nil
}

func (t *GetTicketInfo) getTicketInfo(fs *irodsclient_fs.FileSystem, ticketName string) (string, error) {
	ticket, err := fs.GetTicket(ticketName)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get ticket")
	}

	restrictions, err := fs.GetTicketRestrictions(ticket.ID)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get ticket restrictions for %q", ticket.Name)
	}

	outputTicket := model.TicketWithRestrictions{
		Ticket:       ticket,
		Restrictions: restrictions,
	}

	jsonBytes, err := json.Marshal(outputTicket)
	if err != nil {
		return "", errors.Wrapf(err, "failed to marshal JSON")
	}

	return string(jsonBytes), nil
}
