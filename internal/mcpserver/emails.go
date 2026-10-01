package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (s *service) emailsList(ctx context.Context, req *mcp.CallToolRequest, in attachmentParentInput) (*mcp.CallToolResult, any, error) {
	path, err := attachmentPath(in, "v2", "emails")
	if err != nil {
		return nil, nil, err
	}
	return toolOutput(s.doAllPages(ctx, req, "GET", path, nil, nil, "Attached booking emails"))
}

func (s *service) emailShow(ctx context.Context, req *mcp.CallToolRequest, in attachmentShowInput) (*mcp.CallToolResult, any, error) {
	path, err := attachmentPath(in.attachmentParentInput, "v2", "emails")
	if err != nil {
		return nil, nil, err
	}
	if strings.TrimSpace(in.ID) == "" {
		return nil, nil, fmt.Errorf("id is required")
	}
	return toolOutput(s.do(ctx, req, "GET", path+"/"+apiPathSegment(in.ID), nil, nil, "Attached booking email"))
}
