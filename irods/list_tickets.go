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
	ListTicketsName = irods_common.IRODSAPIPrefix + "list_tickets"
)

type ListTickets struct {
	mcpServer *IRODSMCPServer
	config    *common.Config
}

func NewListTickets(svr *IRODSMCPServer) ToolAPI {
	return &ListTickets{
		mcpServer: svr,
		config:    svr.GetConfig(),
	}
}

func (t *ListTickets) GetName() string {
	return ListTicketsName
}

func (t *ListTickets) GetDescription() string {
	return `Get a list of iRODS tickets. Return information about the tickets, such as their IDs and expiration times, in JSON format.
	Anonymous users are not allowed to list tickets.`
}

func (t *ListTickets) GetTool() mcp.Tool {
	return mcp.NewTool(
		t.GetName(),
		mcp.WithDescription(t.GetDescription()),
	)
}

func (t *ListTickets) GetHandler() server.ToolHandlerFunc {
	return t.Handler
}

func (t *ListTickets) GetAccessiblePaths(authValue *common.AuthValue) []string {
	return []string{}
}

func (t *ListTickets) Handler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// auth
	authValue, err := common.GetAuthValue(ctx)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get auth value")
	}

	if authValue.IsAnonymous() {
		outputErr := errors.Errorf("anonymous user is not allowed to list tickets")
		return irods_common.OutputMCPError(outputErr)
	}

	// make a irods filesystem client
	fs, err := t.mcpServer.GetIRODSFSClientFromAuthValue(&authValue)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create a irods fs client")
	}

	// list
	content, err := t.listTickets(fs)
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to list tickets")
		return irods_common.OutputMCPError(outputErr)
	}

	return mcp.NewToolResultText(content), nil
}

func (t *ListTickets) listTickets(fs *irodsclient_fs.FileSystem) (string, error) {
	outputTickets := []model.TicketWithRestrictions{}

	tickets, err := fs.ListTickets()
	if err != nil {
		return "", errors.Wrapf(err, "failed to list tickets")
	}

	for _, ticket := range tickets {
		restrictions, err := fs.GetTicketRestrictions(ticket.ID)
		if err != nil {
			return "", errors.Wrapf(err, "failed to get ticket restrictions for %q", ticket.Name)
		}

		ticketEntry := model.TicketWithRestrictions{
			Ticket:       ticket,
			Restrictions: restrictions,
		}

		outputTickets = append(outputTickets, ticketEntry)
	}

	jsonBytes, err := json.Marshal(outputTickets)
	if err != nil {
		return "", errors.Wrapf(err, "failed to marshal JSON")
	}

	return string(jsonBytes), nil
}
