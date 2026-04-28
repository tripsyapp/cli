package mcpserver

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const activityCategoryHint = "Supported activity_type slugs: concert, fit, general, kids, museum, note, relax, restaurant, shopping, theater, tour, event, meeting, bar, cafe, parking, amusementPark, aquarium, atm, bakery, bank, beach, brewery, campground, evCharger, fireStation, fitnessCenter, foodMarket, gasStation, hospital, laundry, library, marina, movieTheater, nationalPark, nightlife, park, pharmacy, police, postOffice, publicTransport, restroom, school, stadium, university, winery, zoo."

const transportationCategoryHint = "Supported transportation_type slugs: airplane, bike, bus, car, roadtrip, cruise, ferry, motorcycle, train, walk."

type emptyInput struct{}

type statusInput struct {
	Verbose bool `json:"verbose,omitempty" jsonschema:"When true and authenticated, include the current Tripsy profile in the status response."`
}

type dataInput struct {
	Data map[string]any `json:"data" jsonschema:"Object of Tripsy API fields to send."`
}

type idInput struct {
	ID string `json:"id" jsonschema:"Tripsy resource id."`
}

type tripIDInput struct {
	TripID string `json:"trip_id" jsonschema:"Tripsy trip id."`
}

type listInput struct {
	Fields        []string `json:"fields,omitempty" jsonschema:"Optional response field allow-list. Sent as the API fields query parameter."`
	FieldsExclude []string `json:"fields_exclude,omitempty" jsonschema:"Optional response field deny-list. Sent as the API fields! query parameter."`
	UpdatedSince  string   `json:"updated_since,omitempty" jsonschema:"Optional ISO-8601 timestamp for incremental list filtering."`
	Deleted       bool     `json:"deleted,omitempty" jsonschema:"When true, list deleted records where the endpoint supports it."`
}

type tripUpdateInput struct {
	ID   string         `json:"id" jsonschema:"Tripsy trip id."`
	Data map[string]any `json:"data" jsonschema:"Object of trip fields to update."`
}

type subresourceListInput struct {
	TripID             string   `json:"trip_id" jsonschema:"Tripsy trip id."`
	Fields             []string `json:"fields,omitempty" jsonschema:"Optional response field allow-list. Sent as the API fields query parameter."`
	FieldsExclude      []string `json:"fields_exclude,omitempty" jsonschema:"Optional response field deny-list. Sent as the API fields! query parameter."`
	UpdatedSince       string   `json:"updated_since,omitempty" jsonschema:"Optional ISO-8601 timestamp for incremental list filtering."`
	Deleted            bool     `json:"deleted,omitempty" jsonschema:"When true, list deleted records where the endpoint supports it."`
	ActivityType       string   `json:"activity_type,omitempty" jsonschema:"Optional activity category slug filter. Only used by activities."`
	TransportationType string   `json:"transportation_type,omitempty" jsonschema:"Optional transportation type slug filter. Only used by transportations."`
}

type tripResourceIDInput struct {
	TripID string `json:"trip_id" jsonschema:"Tripsy trip id."`
	ID     string `json:"id" jsonschema:"Tripsy subresource id."`
}

type tripResourceDataInput struct {
	TripID string         `json:"trip_id" jsonschema:"Tripsy trip id."`
	Data   map[string]any `json:"data" jsonschema:"Object of resource fields to create."`
}

type tripResourceUpdateInput struct {
	TripID string         `json:"trip_id" jsonschema:"Tripsy trip id."`
	ID     string         `json:"id" jsonschema:"Tripsy subresource id."`
	Data   map[string]any `json:"data" jsonschema:"Object of resource fields to update."`
}

type rawRequestInput struct {
	Method string            `json:"method" jsonschema:"HTTP method such as GET, POST, PATCH, or DELETE."`
	Path   string            `json:"path" jsonschema:"Tripsy API path such as /v1/me or /v1/trips."`
	Query  map[string]string `json:"query,omitempty" jsonschema:"Optional query parameters."`
	Data   map[string]any    `json:"data,omitempty" jsonschema:"Optional JSON object request body."`
}

type resourceSpec struct {
	Prefix      string
	Title       string
	PluralTitle string
	ListPath    string
	DetailPath  string
	FilterName  string
	FilterParam string
	FilterHint  string

	Description  string
	CreateAdvice string
	ExcludeData  bool
}

func (s *service) status(ctx context.Context, _ *mcp.CallToolRequest, in statusInput) (*mcp.CallToolResult, any, error) {
	data := map[string]any{
		"api_base":         s.client.BaseURL,
		"auth_backend":     s.store.AuthBackendName(),
		"config_dir":       s.store.Dir,
		"credentials_path": s.store.CredentialsPath(),
		"has_token":        strings.TrimSpace(s.client.Token) != "",
	}
	if strings.TrimSpace(s.client.Token) != "" {
		resp, err := s.client.Request(ctx, "GET", "/v1/me", nil, nil)
		if err != nil {
			data["api_check"] = err.Error()
		} else {
			data["api_check"] = "ok"
			if in.Verbose {
				data["me"] = resp.Data
			}
		}
	}
	return nil, map[string]any{"summary": "Tripsy MCP status", "data": data}, nil
}

func (s *service) rawRequest(ctx context.Context, _ *mcp.CallToolRequest, in rawRequestInput) (*mcp.CallToolResult, any, error) {
	method := strings.ToUpper(strings.TrimSpace(in.Method))
	if method == "" {
		return nil, nil, fmt.Errorf("method is required")
	}
	switch method {
	case "GET", "POST", "PATCH", "PUT", "DELETE":
	default:
		return nil, nil, fmt.Errorf("unsupported method %q; expected GET, POST, PATCH, PUT, or DELETE", method)
	}
	path := strings.TrimSpace(in.Path)
	if path == "" {
		return nil, nil, fmt.Errorf("path is required")
	}
	if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") {
		return nil, nil, fmt.Errorf("path must be a Tripsy API path beginning with /")
	}
	if err := allowRawRequestPath(path); err != nil {
		return nil, nil, err
	}
	query := url.Values{}
	for key, value := range in.Query {
		query.Set(key, value)
	}
	var body any
	if len(in.Data) > 0 {
		body = in.Data
	}
	return toolOutput(s.do(ctx, method, path, query, body, "Raw Tripsy API response"))
}

func allowRawRequestPath(apiPath string) error {
	cleaned := path.Clean("/" + strings.TrimLeft(apiPath, "/"))
	blocked := map[string]string{
		"/v1/emails":            "email",
		"/v1/automation/emails": "inbox",
		"/v1/documents":         "document",
		"/v1/storage/uploads":   "upload",
	}
	for prefix, name := range blocked {
		if cleaned == prefix || strings.HasPrefix(cleaned, prefix+"/") {
			return fmt.Errorf("%s endpoints are not exposed by the Tripsy MCP server yet", name)
		}
	}
	if strings.Contains(cleaned, "/documents") {
		return fmt.Errorf("document endpoints are not exposed by the Tripsy MCP server yet")
	}
	return nil
}

func (s *service) meShow(ctx context.Context, _ *mcp.CallToolRequest, _ emptyInput) (*mcp.CallToolResult, any, error) {
	return toolOutput(s.do(ctx, "GET", "/v1/me", nil, nil, "Current user"))
}

func (s *service) meUpdate(ctx context.Context, _ *mcp.CallToolRequest, in dataInput) (*mcp.CallToolResult, any, error) {
	if len(in.Data) == 0 {
		return nil, nil, fmt.Errorf("data is required")
	}
	return toolOutput(s.do(ctx, "PATCH", "/v1/me", nil, in.Data, "Current user updated"))
}

func (s *service) tripsList(ctx context.Context, _ *mcp.CallToolRequest, in listInput) (*mcp.CallToolResult, any, error) {
	return toolOutput(s.do(ctx, "GET", "/v1/trips", tripDataQuery(listQuery(in)), nil, "Trips"))
}

func (s *service) tripShow(ctx context.Context, _ *mcp.CallToolRequest, in idInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.ID) == "" {
		return nil, nil, fmt.Errorf("id is required")
	}
	return toolOutput(s.do(ctx, "GET", "/v1/trips/"+in.ID, tripDataQuery(nil), nil, "Trip "+in.ID))
}

func (s *service) tripCreate(ctx context.Context, _ *mcp.CallToolRequest, in dataInput) (*mcp.CallToolResult, any, error) {
	if len(in.Data) == 0 {
		return nil, nil, fmt.Errorf("data is required")
	}
	return toolOutput(s.do(ctx, "POST", "/v1/trips", tripDataQuery(nil), in.Data, "Trip created"))
}

func (s *service) tripUpdate(ctx context.Context, _ *mcp.CallToolRequest, in tripUpdateInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.ID) == "" {
		return nil, nil, fmt.Errorf("id is required")
	}
	if len(in.Data) == 0 {
		return nil, nil, fmt.Errorf("data is required")
	}
	return toolOutput(s.do(ctx, "PATCH", "/v1/trips/"+in.ID, tripDataQuery(nil), in.Data, "Trip updated"))
}

func (s *service) tripDelete(ctx context.Context, _ *mcp.CallToolRequest, in idInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.ID) == "" {
		return nil, nil, fmt.Errorf("id is required")
	}
	return toolOutput(s.do(ctx, "DELETE", "/v1/trips/"+in.ID, tripDataQuery(nil), nil, "Trip deleted"))
}

func (s *service) registerResource(server *mcp.Server, spec resourceSpec) {
	pluralTitle := firstNonEmpty(spec.PluralTitle, spec.Title+"s")
	filterText := ""
	if spec.FilterHint != "" {
		filterText = " " + spec.FilterHint
	}
	addTool(server, toolName("tripsy", spec.Prefix, "list"), "List "+pluralTitle, spec.Description+" Supports common list filters."+filterText, readOnly(), func(ctx context.Context, req *mcp.CallToolRequest, in subresourceListInput) (*mcp.CallToolResult, any, error) {
		if strings.TrimSpace(in.TripID) == "" {
			return nil, nil, fmt.Errorf("trip_id is required")
		}
		query := subresourceListQuery(in)
		if spec.FilterParam != "" {
			switch spec.FilterName {
			case "activity_type":
				if in.ActivityType != "" {
					query.Set(spec.FilterParam, in.ActivityType)
				}
			case "transportation_type":
				if in.TransportationType != "" {
					query.Set(spec.FilterParam, in.TransportationType)
				}
			}
		}
		return toolOutput(s.do(ctx, "GET", fmt.Sprintf(spec.ListPath, in.TripID), spec.responseQuery(query), nil, pluralTitle))
	})
	addTool(server, toolName("tripsy", spec.Prefix, "show"), "Show "+spec.Title, "Fetch one Tripsy "+strings.ToLower(spec.Title)+" by id.", readOnly(), func(ctx context.Context, req *mcp.CallToolRequest, in tripResourceIDInput) (*mcp.CallToolResult, any, error) {
		if strings.TrimSpace(in.TripID) == "" {
			return nil, nil, fmt.Errorf("trip_id is required")
		}
		if strings.TrimSpace(in.ID) == "" {
			return nil, nil, fmt.Errorf("id is required")
		}
		return toolOutput(s.do(ctx, "GET", fmt.Sprintf(spec.DetailPath, in.TripID, in.ID), spec.responseQuery(nil), nil, spec.Title+" "+in.ID))
	})
	addTool(server, toolName("tripsy", spec.Prefix, "create"), "Create "+spec.Title, "Create a Tripsy "+strings.ToLower(spec.Title)+". "+spec.CreateAdvice, additive(), func(ctx context.Context, req *mcp.CallToolRequest, in tripResourceDataInput) (*mcp.CallToolResult, any, error) {
		if strings.TrimSpace(in.TripID) == "" {
			return nil, nil, fmt.Errorf("trip_id is required")
		}
		if len(in.Data) == 0 {
			return nil, nil, fmt.Errorf("data is required")
		}
		return toolOutput(s.do(ctx, "POST", fmt.Sprintf(spec.ListPath, in.TripID), spec.responseQuery(nil), in.Data, spec.Title+" created"))
	})
	addTool(server, toolName("tripsy", spec.Prefix, "update"), "Update "+spec.Title, "Update a Tripsy "+strings.ToLower(spec.Title)+" by id.", idempotentWrite(), func(ctx context.Context, req *mcp.CallToolRequest, in tripResourceUpdateInput) (*mcp.CallToolResult, any, error) {
		if strings.TrimSpace(in.TripID) == "" {
			return nil, nil, fmt.Errorf("trip_id is required")
		}
		if strings.TrimSpace(in.ID) == "" {
			return nil, nil, fmt.Errorf("id is required")
		}
		if len(in.Data) == 0 {
			return nil, nil, fmt.Errorf("data is required")
		}
		return toolOutput(s.do(ctx, "PATCH", fmt.Sprintf(spec.DetailPath, in.TripID, in.ID), spec.responseQuery(nil), in.Data, spec.Title+" updated"))
	})
	addTool(server, toolName("tripsy", spec.Prefix, "delete"), "Delete "+spec.Title, "Delete a Tripsy "+strings.ToLower(spec.Title)+" by id.", destructive(), func(ctx context.Context, req *mcp.CallToolRequest, in tripResourceIDInput) (*mcp.CallToolResult, any, error) {
		if strings.TrimSpace(in.TripID) == "" {
			return nil, nil, fmt.Errorf("trip_id is required")
		}
		if strings.TrimSpace(in.ID) == "" {
			return nil, nil, fmt.Errorf("id is required")
		}
		return toolOutput(s.do(ctx, "DELETE", fmt.Sprintf(spec.DetailPath, in.TripID, in.ID), spec.responseQuery(nil), nil, spec.Title+" deleted"))
	})
}

func (s *service) collaboratorsList(ctx context.Context, _ *mcp.CallToolRequest, in tripIDInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.TripID) == "" {
		return nil, nil, fmt.Errorf("trip_id is required")
	}
	return toolOutput(s.do(ctx, "GET", "/v1/trip/"+in.TripID+"/collaborators", nil, nil, "Collaborators"))
}

func listQuery(in listInput) url.Values {
	query := url.Values{}
	addListQuery(query, in.Fields, in.FieldsExclude, in.UpdatedSince, in.Deleted)
	return query
}

func subresourceListQuery(in subresourceListInput) url.Values {
	query := url.Values{}
	addListQuery(query, in.Fields, in.FieldsExclude, in.UpdatedSince, in.Deleted)
	return query
}

func addListQuery(query url.Values, fields, fieldsExclude []string, updatedSince string, deleted bool) {
	if deleted {
		query.Set("deleted", "true")
	}
	if updatedSince != "" {
		query.Set("updatedSince", updatedSince)
	}
	if len(fields) > 0 {
		query.Set("fields", joinFields(fields))
	}
	addFieldsExclude(query, fieldsExclude)
}

func joinFields(fields []string) string {
	normalized := make([]string, 0, len(fields))
	seen := map[string]bool{}
	for _, field := range fields {
		for _, part := range strings.Split(field, ",") {
			part = strings.TrimSpace(part)
			if part != "" && !seen[part] {
				seen[part] = true
				normalized = append(normalized, part)
			}
		}
	}
	sort.Strings(normalized)
	return strings.Join(normalized, ",")
}

var defaultTripDataFieldsExclude = []string{"documents", "emails"}

func (spec resourceSpec) responseQuery(query url.Values) url.Values {
	if !spec.ExcludeData {
		return query
	}
	return tripDataQuery(query)
}

func tripDataQuery(query url.Values) url.Values {
	if query == nil {
		query = url.Values{}
	}
	addFieldsExclude(query, defaultTripDataFieldsExclude)
	return query
}

func addFieldsExclude(query url.Values, fields []string) {
	if len(fields) == 0 {
		return
	}
	values := append([]string{}, query["fields!"]...)
	values = append(values, fields...)
	if joined := joinFields(values); joined != "" {
		query.Set("fields!", joined)
	}
}

func toolOutput(value any, err error) (*mcp.CallToolResult, any, error) {
	return nil, value, err
}
