package irods

import (
	"context"

	"github.com/cockroachdb/errors"
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	"github.com/cyverse/irods-mcp-server/common"
	irods_common "github.com/cyverse/irods-mcp-server/irods/common"
	"github.com/cyverse/irods-mcp-server/irods/model"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	GetTicketInfoName = irods_common.IRODSAPIPrefix + "get_ticket_info"
)

type GetTicketInfoInputArgs struct {
	Name string `json:"name"`
}

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

func (t *GetTicketInfo) GetTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        t.GetName(),
		Description: t.GetDescription(),
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"name": {
					Type:        "string",
					Description: "The name of the iRODS ticket to get information about.",
				},
			},
			Required: []string{"name"},
		},
	}
}

func (t *GetTicketInfo) GetHandler() mcp.ToolHandler {
	return t.Handler
}

func (t *GetTicketInfo) GetAccessiblePaths(authValue *common.AuthValue) []string {
	return []string{}
}

func (t *GetTicketInfo) Handler(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// arguments
	args := GetTicketInfoInputArgs{}
	err := irods_common.MarshalInputArguments(t.GetTool(), request, &args)
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to marshal input arguments")
		return irods_common.ToolErrorResult(outputErr), nil
	}

	// auth
	authValue, err := common.GetAuthValue(ctx)
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to get auth value")
		return irods_common.ToolErrorResult(outputErr), nil
	}

	if authValue.IsAnonymous() {
		outputErr := errors.New("anonymous user is not allowed to list tickets")
		return irods_common.ToolErrorResult(outputErr), nil
	}

	// make a irods filesystem client
	fs, err := t.mcpServer.GetIRODSFSClientFromAuthValue(&authValue)
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to create a irods fs client")
		return irods_common.ToolErrorResult(outputErr), nil
	}

	// get
	content, err := t.getTicketInfo(fs, args.Name)
	if err != nil {
		outputErr := errors.Wrapf(err, "failed to get ticket info for %q", args.Name)
		return irods_common.ToolErrorResult(outputErr), nil
	}

	return irods_common.ToolJSONResult(*content)
}

func (t *GetTicketInfo) getTicketInfo(fs *irodsclient_fs.FileSystem, ticketName string) (*model.TicketWithRestrictions, error) {
	ticket, err := fs.GetTicket(ticketName)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get ticket")
	}

	restrictions, err := fs.GetTicketRestrictions(ticket.ID)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get ticket restrictions for %q", ticket.Name)
	}

	outputTicket := &model.TicketWithRestrictions{
		Ticket:       ticket,
		Restrictions: restrictions,
	}

	return outputTicket, nil
}
