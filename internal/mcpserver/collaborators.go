package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/tripsyapp/cli/internal/guests"
)

type collaboratorInviteInput struct {
	TripID           string                    `json:"trip_id" jsonschema:"Tripsy trip id."`
	InvitedUserEmail string                    `json:"invited_user_email" jsonschema:"Email address of the guest to invite."`
	Permissions      *guests.InvitePermissions `json:"permissions,omitempty" jsonschema:"Optional invitation permissions. Omitted fields use the API defaults, limited by the inviter's permissions."`
}

type collaboratorUpdateInput struct {
	TripID      string                   `json:"trip_id" jsonschema:"Tripsy trip id."`
	UserID      string                   `json:"user_id" jsonschema:"Guest user id from collaborators, or me to resolve the authenticated user."`
	Permissions guests.UpdatePermissions `json:"permissions" jsonschema:"Permissions to change. At least one field is required. Omitted fields are preserved. For your own travel or notification preference, send only that single field."`
}

type collaboratorDeleteInput struct {
	TripID string `json:"trip_id" jsonschema:"Tripsy trip id."`
	UserID string `json:"user_id" jsonschema:"Guest user id from tripsy_collaborators_list, not a favorite record or invitation id."`
}

func (s *service) collaboratorInvite(ctx context.Context, req *mcp.CallToolRequest, in collaboratorInviteInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.TripID) == "" || strings.TrimSpace(in.InvitedUserEmail) == "" {
		return nil, nil, fmt.Errorf("trip_id and invited_user_email are required")
	}
	payload := map[string]any{"trip_id": strings.TrimSpace(in.TripID), "invited_user_email": strings.TrimSpace(in.InvitedUserEmail)}
	if in.Permissions != nil {
		payload["permissions"] = in.Permissions
	}
	return toolOutput(s.do(ctx, req, "POST", "/v1/guests/invite", nil, payload, "Invitation request processed. This does not confirm a guest was added; check tripsy_collaborators_list for membership or pending status."))
}

func (s *service) collaboratorUpdate(ctx context.Context, req *mcp.CallToolRequest, in collaboratorUpdateInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.TripID) == "" || strings.TrimSpace(in.UserID) == "" {
		return nil, nil, fmt.Errorf("trip_id and user_id are required")
	}
	if in.Permissions.Empty() {
		return nil, nil, fmt.Errorf("permissions must contain at least one field")
	}
	userID := strings.TrimSpace(in.UserID)
	if userID == "me" {
		client := s.clientForRequest(req)
		if err := requireToken(client); err != nil {
			return nil, nil, err
		}
		resp, err := client.Request(ctx, "GET", "/v1/me", nil, nil)
		if err != nil {
			return nil, nil, err
		}
		userID = valueString(resp.Data, "id")
		if userID == "" {
			return nil, nil, fmt.Errorf("current user response did not include id")
		}
	}
	return toolOutput(s.do(ctx, req, "PATCH", "/v1/trip/"+apiPathSegment(in.TripID)+"/collaborator/"+apiPathSegment(userID)+"/permissions", nil, in.Permissions, "Collaborator permissions updated"))
}

func (s *service) collaboratorDelete(ctx context.Context, req *mcp.CallToolRequest, in collaboratorDeleteInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.TripID) == "" || strings.TrimSpace(in.UserID) == "" {
		return nil, nil, fmt.Errorf("trip_id and user_id are required")
	}
	return toolOutput(s.do(ctx, req, "DELETE", "/v1/trip/"+apiPathSegment(in.TripID)+"/collaborator/"+apiPathSegment(in.UserID), nil, nil, "Collaborator removal request processed"))
}
