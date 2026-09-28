package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (s *service) favoriteGuestsList(ctx context.Context, req *mcp.CallToolRequest, _ emptyInput) (*mcp.CallToolResult, any, error) {
	return toolOutput(s.doAllPages(ctx, req, "GET", "/v1/guests/favorites", nil, nil, "Favorite guests and pending invitations. pending distinguishes pending invitations from confirmed favorites; favorite_user.id is the user id."))
}
