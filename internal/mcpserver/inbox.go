package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type inboxIDInput struct {
	ID string `json:"id" jsonschema:"Automation email id from tripsy_inbox_list."`
}

type inboxUpdateInput struct {
	ID               string  `json:"id" jsonschema:"Automation email id from tripsy_inbox_list."`
	Subject          *string `json:"subject,omitempty" jsonschema:"Optional replacement subject. Omitted values preserve the existing subject."`
	TripID           *string `json:"trip_id,omitempty" jsonschema:"Move to this editable trip. Supply exactly one move target; omit when moving to an activity, hosting, or transportation."`
	ActivityID       *string `json:"activity_id,omitempty" jsonschema:"Move to this editable activity. Supply exactly one move target; no trip_id is needed."`
	HostingID        *string `json:"hosting_id,omitempty" jsonschema:"Move to this editable hosting (lodging). Supply exactly one move target; no trip_id is needed."`
	TransportationID *string `json:"transportation_id,omitempty" jsonschema:"Move to this editable transportation. Supply exactly one move target; no trip_id is needed."`
}

func (s *service) inboxList(ctx context.Context, req *mcp.CallToolRequest, _ emptyInput) (*mcp.CallToolResult, any, error) {
	return toolOutput(s.doAllPages(ctx, req, "GET", "/v1/automation/emails", nil, nil, "Inbox emails awaiting manual review"))
}

func (s *service) inboxShow(ctx context.Context, req *mcp.CallToolRequest, in inboxIDInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.ID) == "" {
		return nil, nil, fmt.Errorf("id is required")
	}
	return toolOutput(s.do(ctx, req, "GET", "/v1/automation/emails/"+apiPathSegment(in.ID), nil, nil, "Inbox email "+in.ID))
}

func (s *service) inboxUpdate(ctx context.Context, req *mcp.CallToolRequest, in inboxUpdateInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.ID) == "" {
		return nil, nil, fmt.Errorf("id is required")
	}
	payload := map[string]any{}
	if in.Subject != nil {
		payload["subject"] = *in.Subject
	}
	targets := 0
	for field, value := range map[string]*string{
		"trip_id":           in.TripID,
		"activity_id":       in.ActivityID,
		"hosting_id":        in.HostingID,
		"transportation_id": in.TransportationID,
	} {
		if value == nil {
			continue
		}
		id := strings.TrimSpace(*value)
		if id == "" {
			return nil, nil, fmt.Errorf("%s must not be empty", field)
		}
		targets++
		payload[field] = id
	}
	if targets > 1 {
		return nil, nil, fmt.Errorf("supply only one move target: trip_id, activity_id, hosting_id, or transportation_id")
	}
	if len(payload) == 0 {
		return nil, nil, fmt.Errorf("subject or one move target is required")
	}
	return toolOutput(s.do(ctx, req, "PATCH", "/v1/automation/emails/"+apiPathSegment(in.ID), nil, payload, "Inbox email updated"))
}

func (s *service) inboxDelete(ctx context.Context, req *mcp.CallToolRequest, in inboxIDInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.ID) == "" {
		return nil, nil, fmt.Errorf("id is required")
	}
	return toolOutput(s.do(ctx, req, "DELETE", "/v1/automation/emails/"+apiPathSegment(in.ID), nil, nil, "Inbox email deletion request processed"))
}
