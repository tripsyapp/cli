package mcpserver

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	maxInlineDocumentBytes = 8 * 1024 * 1024
	maxDocumentBytes       = 25 * 1024 * 1024
)

type documentAttachInput struct {
	attachmentParentInput
	URL         string `json:"url" jsonschema:"HTTP(S) link, or exact object_key from tripsy_documents_upload_prepare after uploading bytes."`
	FileType    string `json:"file_type,omitempty" jsonschema:"url (default) for a web link, or the exact MIME content_type used when preparing the upload."`
	Title       string `json:"title,omitempty" jsonschema:"Document title."`
	Description string `json:"description,omitempty" jsonschema:"Document description."`
	ThumbURL    string `json:"thumb_url,omitempty" jsonschema:"Optional thumbnail URL."`
	FaviconURL  string `json:"favicon_url,omitempty" jsonschema:"Optional favicon URL."`
	UploadToken string `json:"upload_token,omitempty" jsonschema:"Required for file documents: upload_token returned by tripsy_documents_upload_prepare."`
}

type documentUpdateInput struct {
	ID               string  `json:"id" jsonschema:"Document id."`
	Title            *string `json:"title,omitempty" jsonschema:"Replacement title; empty clears it."`
	Description      *string `json:"description,omitempty" jsonschema:"Replacement description; empty clears it."`
	ThumbURL         *string `json:"thumb_url,omitempty" jsonschema:"Replacement thumbnail URL; empty clears it."`
	FaviconURL       *string `json:"favicon_url,omitempty" jsonschema:"Replacement favicon URL; empty clears it."`
	TripID           *string `json:"trip_id,omitempty" jsonschema:"Move to this trip. Supply exactly one destination."`
	ActivityID       *string `json:"activity_id,omitempty" jsonschema:"Move to this activity. Omit trip_id."`
	HostingID        *string `json:"hosting_id,omitempty" jsonschema:"Move to this hosting. Omit trip_id."`
	TransportationID *string `json:"transportation_id,omitempty" jsonschema:"Move to this transportation. Omit trip_id."`
}

type documentUploadPrepareInput struct {
	attachmentParentInput
	Filename      string `json:"filename" jsonschema:"File basename including extension; never a path on the MCP server."`
	ContentType   string `json:"content_type" jsonschema:"File MIME type, for example application/pdf."`
	ContentLength int64  `json:"content_length" jsonschema:"Exact size in bytes, between 1 and 26214400 (25 MiB)."`
}

type documentUploadInput struct {
	attachmentParentInput
	Filename      string `json:"filename" jsonschema:"File basename including extension."`
	ContentType   string `json:"content_type" jsonschema:"File MIME type."`
	ContentBase64 string `json:"content_base64" jsonschema:"Standard base64 encoded document bytes, at most 8 MiB decoded. No local file paths or URLs are accepted."`
	Title         string `json:"title,omitempty" jsonschema:"Title; defaults to filename."`
	Description   string `json:"description,omitempty" jsonschema:"Optional description."`
}

func (s *service) documentsList(ctx context.Context, req *mcp.CallToolRequest, in attachmentParentInput) (*mcp.CallToolResult, any, error) {
	path, err := attachmentPath(in, "v2", "documents")
	if err != nil {
		return nil, nil, err
	}
	return toolOutput(s.doAllPages(ctx, req, "GET", path, nil, nil, "Attached documents"))
}

func (s *service) documentShow(ctx context.Context, req *mcp.CallToolRequest, in attachmentShowInput) (*mcp.CallToolResult, any, error) {
	path, err := attachmentPath(in.attachmentParentInput, "v2", "documents")
	if err != nil {
		return nil, nil, err
	}
	if strings.TrimSpace(in.ID) == "" {
		return nil, nil, fmt.Errorf("id is required")
	}
	return toolOutput(s.do(ctx, req, "GET", path+"/"+apiPathSegment(in.ID), nil, nil, "Document metadata"))
}

func (s *service) documentGet(ctx context.Context, req *mcp.CallToolRequest, in idInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.ID) == "" {
		return nil, nil, fmt.Errorf("id is required")
	}
	return toolOutput(s.do(ctx, req, "GET", "/v1/documents/"+apiPathSegment(in.ID)+"/get", nil, nil, "Temporary document download URL"))
}

func (s *service) documentAttach(ctx context.Context, req *mcp.CallToolRequest, in documentAttachInput) (*mcp.CallToolResult, any, error) {
	path, err := attachmentPath(in.attachmentParentInput, "v1", "documents")
	if err != nil {
		return nil, nil, err
	}
	fileType := firstNonEmpty(in.FileType, "url")
	if fileType == "url" {
		u, err := url.Parse(in.URL)
		if err != nil || u.Host == "" || u.User != nil || (u.Scheme != "https" && u.Scheme != "http") {
			return nil, nil, fmt.Errorf("link documents require an HTTP(S) URL")
		}
	} else if !validDocumentObjectKey(in.URL) || strings.TrimSpace(in.UploadToken) == "" {
		return nil, nil, fmt.Errorf("file documents require object_key and upload_token from tripsy_documents_upload_prepare")
	}
	payload := map[string]any{"url": in.URL, "file_type": fileType, "title": in.Title, "description": in.Description, "thumb_url": in.ThumbURL, "favicon_url": in.FaviconURL}
	if in.UploadToken != "" {
		payload["upload_token"] = in.UploadToken
	}
	return toolOutput(s.do(ctx, req, "POST", path, nil, payload, "Document attached"))
}

func (s *service) documentUpdate(ctx context.Context, req *mcp.CallToolRequest, in documentUpdateInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.ID) == "" {
		return nil, nil, fmt.Errorf("id is required")
	}
	payload := map[string]any{}
	for field, value := range map[string]*string{"title": in.Title, "description": in.Description, "thumb_url": in.ThumbURL, "favicon_url": in.FaviconURL} {
		if value != nil {
			payload[field] = *value
		}
	}
	if err := addMoveTargets(payload, map[string]*string{"trip_id": in.TripID, "activity_id": in.ActivityID, "hosting_id": in.HostingID, "transportation_id": in.TransportationID}); err != nil {
		return nil, nil, err
	}
	if len(payload) == 0 {
		return nil, nil, fmt.Errorf("metadata or one move target is required")
	}
	return toolOutput(s.do(ctx, req, "PATCH", "/v1/documents/"+apiPathSegment(in.ID), nil, payload, "Document updated"))
}

func (s *service) documentDelete(ctx context.Context, req *mcp.CallToolRequest, in attachmentShowInput) (*mcp.CallToolResult, any, error) {
	path, err := attachmentPath(in.attachmentParentInput, "v1", "documents")
	if err != nil {
		return nil, nil, err
	}
	if strings.TrimSpace(in.ID) == "" {
		return nil, nil, fmt.Errorf("id is required")
	}
	return toolOutput(s.do(ctx, req, "DELETE", path+"/"+apiPathSegment(in.ID), nil, nil, "Document deleted"))
}

func documentUploadPayload(in documentUploadPrepareInput) (map[string]any, error) {
	if _, err := attachmentPath(in.attachmentParentInput, "v1", "documents"); err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.Filename) == "" || strings.ContainsAny(in.Filename, "/\\") {
		return nil, fmt.Errorf("filename must be a file basename")
	}
	if strings.TrimSpace(in.ContentType) == "" {
		return nil, fmt.Errorf("content_type is required")
	}
	if in.ContentLength < 1 || in.ContentLength > maxDocumentBytes {
		return nil, fmt.Errorf("content_length must be between 1 and 26214400 bytes")
	}
	parentType := firstNonEmpty(strings.TrimSpace(in.ParentType), "trip")
	parentID := strings.TrimSpace(in.ParentID)
	if parentType == "trip" {
		parentID = strings.TrimSpace(in.TripID)
	}
	return map[string]any{"purpose": "document", "visibility": "private", "parent_type": parentType, "parent_id": parentID, "filename": in.Filename, "content_type": in.ContentType, "content_length": in.ContentLength}, nil
}

func (s *service) documentUploadPrepare(ctx context.Context, req *mcp.CallToolRequest, in documentUploadPrepareInput) (*mcp.CallToolResult, any, error) {
	payload, err := documentUploadPayload(in)
	if err != nil {
		return nil, nil, err
	}
	return toolOutput(s.do(ctx, req, "POST", "/v1/storage/uploads", nil, payload, "Private document upload prepared"))
}

func (s *service) documentUpload(ctx context.Context, req *mcp.CallToolRequest, in documentUploadInput) (*mcp.CallToolResult, any, error) {
	if len(in.ContentBase64) > base64.StdEncoding.EncodedLen(maxInlineDocumentBytes) {
		return nil, nil, fmt.Errorf("inline documents must be at most 8 MiB; use upload_prepare for larger files")
	}
	content, err := base64.StdEncoding.Strict().DecodeString(in.ContentBase64)
	if err != nil {
		return nil, nil, fmt.Errorf("content_base64 must be standard base64")
	}
	if len(content) > maxInlineDocumentBytes {
		return nil, nil, fmt.Errorf("inline documents must be at most 8 MiB")
	}
	payload, err := documentUploadPayload(documentUploadPrepareInput{attachmentParentInput: in.attachmentParentInput, Filename: in.Filename, ContentType: in.ContentType, ContentLength: int64(len(content))})
	if err != nil {
		return nil, nil, err
	}
	client := s.clientForRequest(req)
	if err := requireToken(client); err != nil {
		return nil, nil, err
	}
	response, err := client.Request(ctx, "POST", "/v1/storage/uploads", nil, payload)
	if err != nil {
		return nil, nil, err
	}
	info, ok := response.Data.(map[string]any)
	if !ok {
		return nil, nil, fmt.Errorf("invalid storage upload response")
	}
	value := func(key string) string { str, _ := info[key].(string); return str }
	key, receipt := value("object_key"), value("upload_token")
	if !validDocumentObjectKey(key) || receipt == "" || value("visibility") != "private" || value("method") != "PUT" {
		return nil, nil, fmt.Errorf("API did not return a private document upload receipt")
	}
	headers := map[string]string{}
	if fields, ok := info["headers"].(map[string]any); ok {
		for field, val := range fields {
			str, ok := val.(string)
			if !ok {
				return nil, nil, fmt.Errorf("invalid upload headers")
			}
			headers[field] = str
		}
	}
	if err := client.UploadDocumentBytes(ctx, value("upload_url"), value("bucket"), headers, content); err != nil {
		return nil, nil, err
	}
	attach := documentAttachInput{attachmentParentInput: in.attachmentParentInput, URL: key, FileType: in.ContentType, UploadToken: receipt, Title: firstNonEmpty(in.Title, in.Filename), Description: in.Description}
	result, output, err := s.documentAttach(ctx, req, attach)
	if err != nil {
		recovery := map[string]any{"error": err.Error(), "uploaded": true, "retry_tool": "tripsy_documents_attach", "retry_arguments": attach, "guidance": "File bytes were uploaded. Retry attachment with these arguments before the receipt expires; do not upload the bytes again."}
		return &mcp.CallToolResult{IsError: true}, map[string]any{"summary": "File uploaded but attachment failed", "data": recovery}, nil
	}
	return result, output, nil
}

func validDocumentObjectKey(key string) bool {
	return strings.TrimSpace(key) != "" && !strings.HasPrefix(key, "/") && !strings.Contains(key, "://") && !strings.ContainsAny(key, "\\\r\n\t")
}
