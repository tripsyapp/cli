package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/tripsyapp/cli/internal/api"
	"github.com/tripsyapp/cli/internal/config"
)

const (
	serverName  = "tripsy"
	serverTitle = "Tripsy MCP"
)

type Options struct {
	APIBase           string
	Token             string
	ConfigDir         string
	Version           string
	DisableRawRequest bool
	RequestTokenOnly  bool
}

type RuntimeInfo struct {
	APIBase         string `json:"api_base"`
	AuthBackend     string `json:"auth_backend"`
	ConfigDir       string `json:"config_dir"`
	CredentialsPath string `json:"credentials_path"`
	HasToken        bool   `json:"has_token"`
}

type service struct {
	client            *api.Client
	store             *config.Store
	disableRawRequest bool
}

const (
	tokenInfoTripsyTokenKey = "tripsy_token"
	tokenInfoAuthSchemeKey  = "auth_scheme"
)

func New(opts Options) (*mcp.Server, RuntimeInfo, error) {
	store := config.NewStore(opts.ConfigDir)
	credentials, err := store.LoadCredentials()
	if opts.RequestTokenOnly {
		credentials, err = store.LoadNonSecretCredentials()
	}
	if err != nil {
		return nil, RuntimeInfo{}, err
	}

	baseURL := firstNonEmpty(opts.APIBase, os.Getenv("TRIPSY_API_BASE"), credentials.BaseURL, api.DefaultBaseURL)
	token := firstNonEmpty(opts.Token, os.Getenv("TRIPSY_TOKEN"), credentials.Token)
	if opts.RequestTokenOnly {
		token = ""
	}

	client := api.NewClient(baseURL, token)
	info := RuntimeInfo{
		APIBase:         client.BaseURL,
		AuthBackend:     store.AuthBackendName(),
		ConfigDir:       store.Dir,
		CredentialsPath: store.CredentialsPath(),
		HasToken:        strings.TrimSpace(client.Token) != "",
	}
	return NewWithClientOptions(client, store, opts), info, nil
}

func NewWithClient(client *api.Client, store *config.Store, version string) *mcp.Server {
	return NewWithClientOptions(client, store, Options{Version: version})
}

func NewWithClientOptions(client *api.Client, store *config.Store, opts Options) *mcp.Server {
	if strings.TrimSpace(opts.Version) == "" {
		opts.Version = "dev"
	}
	if client == nil {
		client = api.NewClient("", "")
	}
	if store == nil {
		store = config.NewStore("")
	}
	s := &service{client: client, store: store, disableRawRequest: opts.DisableRawRequest}
	server := mcp.NewServer(&mcp.Implementation{
		Name:       serverName,
		Title:      serverTitle,
		Version:    opts.Version,
		WebsiteURL: "https://tripsy.app",
	}, nil)
	s.register(server)
	return server
}

func (s *service) register(server *mcp.Server) {
	addTool(server, toolName("tripsy", "status"), "Tripsy Status", "Inspect Tripsy MCP configuration and authentication state without revealing the stored token.", readOnly(), s.status)
	addTool(server, toolName("tripsy", "itinerary", "guidance"), "Tripsy Itinerary Guidance", "Return concise agent guidance for creating high-quality Tripsy trips, including UTC timestamp storage and local timezone display conversion rules, reachable direct Unsplash cover images, item granularity, coordinates, lodging, transportation, transfer rules, and recoverable delete behavior.", readOnly(), s.itineraryGuidance)
	if !s.disableRawRequest {
		addTool(server, toolName("tripsy", "raw_request"), "Raw Tripsy API Request", "Make a raw request to supported Tripsy public API endpoints that do not yet have a dedicated MCP tool. Prefer typed tools when available.", destructive(), s.rawRequest)
	}

	addTool(server, toolName("tripsy", "me", "show"), "Show Current Tripsy User", "Return the authenticated Tripsy profile.", readOnly(), s.meShow)
	addTool(server, toolName("tripsy", "me", "update"), "Update Current Tripsy User", "Update current Tripsy profile fields such as name, timezone, language, or default currency.", idempotentWrite(), s.meUpdate)

	addTool(server, toolName("tripsy", "categories", "list"), "List Custom Categories", "List visible custom activity categories owned by the authenticated user or by users who share active trips with the authenticated user. Custom category slugs can be used as activity_type values on activity endpoints.", readOnly(), s.categoriesList)
	addTool(server, toolName("tripsy", "categories", "show"), "Show Custom Category", "Fetch one visible custom activity category by id.", readOnly(), s.categoryShow)
	addTool(server, toolName("tripsy", "categories", "create"), "Create Custom Category", "Create a custom activity category owned by the authenticated user. The API can generate slug when omitted; if the user already owns the same slug, the API returns an empty 200 OK and does not create a duplicate.", additive(), s.categoryCreate)
	addTool(server, toolName("tripsy", "categories", "update"), "Update Custom Category", "Update a custom activity category owned by the authenticated user by id.", idempotentWrite(), s.categoryUpdate)
	addTool(server, toolName("tripsy", "categories", "delete"), "Delete Custom Category", "Soft-delete a custom activity category owned by the authenticated user by id.", destructive(), s.categoryDelete)

	addTool(server, toolName("tripsy", "trips", "list"), "List Trips", "List Tripsy trips where the authenticated user is travelling, including owned trips and shared trips with current-user is_travelling permission. has_dates is authoritative: when has_dates is false, ignore starts_at and ends_at even if present. Supports fields, excluded fields, deleted records, and updated-since filtering.", readOnly(), s.tripsList)
	addTool(server, toolName("tripsy", "trips", "following", "list"), "List Following Trips", "List Tripsy trips the authenticated user follows but is not travelling on, based on current-user is_travelling permission. has_dates is authoritative: when has_dates is false, ignore starts_at and ends_at even if present. Supports fields, excluded fields, deleted records, and updated-since filtering.", readOnly(), s.tripsFollowingList)
	addTool(server, toolName("tripsy", "trips", "show"), "Show Trip", "Fetch one Tripsy trip by id. A returned trip may be owned by another user and shared through collaboration; inspect owner/collaborators when ownership matters. has_dates is authoritative: when has_dates is false, ignore starts_at and ends_at even if present.", readOnly(), s.tripShow)
	addTool(server, toolName("tripsy", "trips", "create"), "Create Trip", "Create a Tripsy trip. For planned itineraries, include name, timezone, starts_at, ends_at, and cover_image_url. For leisure trips, cover_image_url should be a destination-specific real direct Unsplash CDN URL copied from an image result, in the form https://images.unsplash.com/photo-1562869929-bda0650edb1f?ixid=...&ixlib=rb-4.1.0. The MCP server validates direct Unsplash URL shape; if the client also has external URL access, check that the image URL is reachable and not a 404 before saving it. Do not use unsplash.com/photos/... pages, and do not turn short photo IDs such as nWdsya5_Yms into images.unsplash.com/photo-nWdsya5_Yms URLs. Use date strings for trip starts_at/ends_at such as 2026-06-01.", additive(), s.tripCreate)
	addTool(server, toolName("tripsy", "trips", "update"), "Update Trip", "Update a Tripsy trip by id. If updating cover_image_url, the MCP server validates direct Unsplash URL shape; if the client also has external URL access, check that the image URL is reachable and not a 404 before saving it.", idempotentWrite(), s.tripUpdate)
	addTool(server, toolName("tripsy", "trips", "delete"), "Delete Trip", "Soft-delete a Tripsy trip by id. Delete operations may be executed when requested and can be undone if necessary.", destructive(), s.tripDelete)

	s.registerResource(server, resourceSpec{
		Prefix:         "activities",
		Title:          "Activity",
		PluralTitle:    "Activities",
		ListPath:       "/v1/trip/%s/activities",
		DetailPath:     "/v1/trip/%s/activity/%s",
		ReadListPath:   "/v2/trip/%s/activities/",
		ReadDetailPath: "/v2/trip/%s/activity/%s/",
		FilterName:     "activity_type",
		FilterParam:    "activityType",
		FilterHint:     activityCategoryHint,
		Description:    "Scheduled or unscheduled trip activities. Use one activity per actual stop, reservation, meal, tour, or experience. MCP clients displaying activity dates/times must convert UTC starts_at/ends_at values into the activity timezone.",
		CreateAdvice:   "Set activity_type to the most specific built-in slug or visible custom category slug, include address plus latitude/longitude for map-ready location items, use UTC starts_at/ends_at values with the local timezone for the activity location, and do not bundle multiple stops into one activity. MCP clients displaying activities must convert UTC starts_at/ends_at values into the activity timezone before formatting date/time. Custom category slugs are only valid on Activity objects through activity_type. MCP clients displaying activities must resolve non-built-in activity_type values through tripsy_categories_list so the correct custom activity category name, icon, and color are shown.",
		ExcludeData:    true,
		SkipCreate:     true,
	})
	addTool(server, toolName("tripsy", "activities", "create"), "Create Activity", "Create a Tripsy activity. Use one activity per actual stop, reservation, meal, tour, event, or experience. Timed values are always UTC ISO-8601 timestamps; set timezone to the activity location's local IANA timezone. MCP clients displaying activities must convert UTC starts_at/ends_at values into the activity timezone before formatting date/time. Set activity_type to the most specific built-in slug or visible custom category slug, include address plus latitude/longitude for map-ready location items, and do not invent ad hoc values such as sightseeing. Custom category slugs are only valid on Activity objects through activity_type. MCP clients displaying activities must resolve non-built-in activity_type values through tripsy_categories_list so the correct custom activity category name, icon, and color are shown. "+activityCategoryHint, additive(), s.activityCreate)
	s.registerResource(server, resourceSpec{
		Prefix:         "hostings",
		Title:          "Hosting",
		PluralTitle:    "Hostings",
		ListPath:       "/v1/trip/%s/hostings",
		DetailPath:     "/v1/trip/%s/hosting/%s",
		ReadListPath:   "/v2/trip/%s/hostings/",
		ReadDetailPath: "/v2/trip/%s/hosting/%s/",
		Description:    "Hotel and lodging plans. MCP clients displaying lodging dates/times must convert UTC starts_at/ends_at values into the lodging timezone.",
		CreateAdvice:   "Use hostings for hotels and lodging rather than activities. Include name, address, latitude, longitude, UTC starts_at/ends_at values, and the local timezone for the lodging location when known. MCP clients displaying lodging data must convert UTC starts_at/ends_at values into the lodging timezone before formatting local date/time.",
		ExcludeData:    true,
		SkipCreate:     true,
	})
	addTool(server, toolName("tripsy", "hostings", "create"), "Create Hosting", "Create a Tripsy hosting. Use hostings for hotels and lodging rather than activities. Timed values are always UTC ISO-8601 timestamps; set timezone to the lodging location's local IANA timezone. MCP clients displaying hostings must convert UTC starts_at/ends_at values into the lodging timezone before formatting local date/time. Include name, address, latitude, longitude, starts_at, ends_at, and timezone when known.", additive(), s.hostingCreate)
	s.registerResource(server, resourceSpec{
		Prefix:         "transportations",
		Title:          "Transportation",
		PluralTitle:    "Transportations",
		ListPath:       "/v1/trip/%s/transportations",
		DetailPath:     "/v1/trip/%s/transportation/%s",
		ReadListPath:   "/v2/trip/%s/transportations/",
		ReadDetailPath: "/v2/trip/%s/transportation/%s/",
		FilterName:     "transportation_type",
		FilterParam:    "transportationType",
		FilterHint:     transportationCategoryHint,
		Description:    "Flights, trains, cars, buses, ferries, walks, and other point-to-point travel. MCP clients displaying transportation dates/times must convert UTC departure_at with departure_timezone and UTC arrival_at with arrival_timezone.",
		CreateAdvice:   "Use transportation_type for the segment kind and include departure/arrival coordinates when known. Use UTC departure_at/arrival_at values with local departure_timezone/arrival_timezone fields for display. MCP clients must convert departure_at with departure_timezone and arrival_at with arrival_timezone before formatting local endpoint dates/times. For flights, use transportation_type airplane, set departure_description and arrival_description to airport IATA codes, include the airports' departure/arrival latitudes and longitudes, and omit name unless the user provided one. For transfer activities, use transportation_type roadtrip and include both departure and arrival names/descriptions, addresses, latitudes, and longitudes.",
		ExcludeData:    true,
		SkipCreate:     true,
	})
	addTool(server, toolName("tripsy", "transportations", "create"), "Create Transportation", "Create a Tripsy transportation for point-to-point movement. Timed values are always UTC ISO-8601 timestamps; set departure_timezone and arrival_timezone to each endpoint's local IANA timezone. MCP clients displaying transportations must convert departure_at with departure_timezone and arrival_at with arrival_timezone before formatting local endpoint dates/times. Use transportation_type for the segment kind and include departure/arrival names, addresses, latitudes, and longitudes when known. For flights, use transportation_type airplane, set departure_description and arrival_description to airport IATA codes, include the airports' departure/arrival latitudes and longitudes, and omit name unless the user provided one. For transfer activities, use transportation_type roadtrip and fill both departure and arrival locations completely. "+transportationCategoryHint, additive(), s.transportationCreate)
	s.registerResource(server, resourceSpec{
		Prefix:       "expenses",
		Title:        "Expense",
		PluralTitle:  "Expenses",
		ListPath:     "/v1/trip/%s/expenses",
		DetailPath:   "/v1/trip/%s/expense/%s",
		Description:  "Trip expenses.",
		CreateAdvice: "Use title, price, currency, and date for expense records.",
	})

	addTool(server, toolName("tripsy", "collaborators", "list"), "List Trip Collaborators", "List collaborators and pending invitations for a trip.", readOnly(), s.collaboratorsList)
}

func toolName(parts ...string) string {
	return strings.Join(parts, "_")
}

func addTool[In, Out any](server *mcp.Server, name, title, description string, annotations *mcp.ToolAnnotations, handler mcp.ToolHandlerFor[In, Out]) {
	mcp.AddTool(server, &mcp.Tool{
		Name:         name,
		Title:        title,
		Description:  toolDescription(description, annotations),
		Annotations:  annotations,
		OutputSchema: tripsyToolOutputSchema(),
	}, handler)
}

func tripsyToolOutputSchema() map[string]any {
	return map[string]any{
		"type":        "object",
		"description": "Standard Tripsy MCP tool response envelope.",
		"properties": map[string]any{
			"summary": map[string]any{
				"type":        "string",
				"description": "Short human-readable summary of the Tripsy operation result.",
			},
			"status_code": map[string]any{
				"type":        "integer",
				"description": "HTTP status code returned by the Tripsy API when the tool called the API.",
			},
			"data": map[string]any{
				"description": "Tripsy API response payload or tool-specific structured data.",
			},
		},
		"required": []string{"summary"},
	}
}

func toolDescription(description string, annotations *mcp.ToolAnnotations) string {
	if annotations == nil || annotations.OpenWorldHint == nil || *annotations.OpenWorldHint {
		return description
	}
	reasons := []string{"Closed-world: this tool only interacts with the Tripsy API for the authenticated user's Tripsy account, not arbitrary external services or URLs."}
	if annotations.DestructiveHint != nil && *annotations.DestructiveHint {
		reasons = append(reasons, "Destructive: this tool can remove or broadly mutate Tripsy data, so confirm user intent before calling it.")
	}
	return description + " Safety: " + strings.Join(reasons, " ")
}

func (s *service) do(ctx context.Context, req *mcp.CallToolRequest, method, path string, query url.Values, body any, summary string) (any, error) {
	client := s.clientForRequest(req)
	if err := requireToken(client); err != nil {
		return nil, err
	}
	resp, err := client.Request(ctx, method, path, query, body)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(summary) == "" {
		summary = fmt.Sprintf("HTTP %d", resp.StatusCode)
	}
	return envelope(resp, summary), nil
}

func (s *service) doAllPages(ctx context.Context, req *mcp.CallToolRequest, method, path string, query url.Values, body any, summary string) (any, error) {
	client := s.clientForRequest(req)
	if err := requireToken(client); err != nil {
		return nil, err
	}
	resp, err := client.RequestAllPages(ctx, method, path, query, body)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(summary) == "" {
		summary = fmt.Sprintf("HTTP %d", resp.StatusCode)
	}
	return envelope(resp, summary), nil
}

func (s *service) clientForRequest(req *mcp.CallToolRequest) *api.Client {
	if s == nil {
		return nil
	}
	if req == nil || req.Extra == nil || req.Extra.TokenInfo == nil || req.Extra.TokenInfo.Extra == nil {
		return s.client
	}
	token, _ := req.Extra.TokenInfo.Extra[tokenInfoTripsyTokenKey].(string)
	if strings.TrimSpace(token) == "" {
		return s.client
	}
	authScheme, _ := req.Extra.TokenInfo.Extra[tokenInfoAuthSchemeKey].(string)
	baseURL := ""
	var httpClient *http.Client
	if s.client != nil {
		baseURL = s.client.BaseURL
		httpClient = s.client.HTTPClient
	}
	client := api.NewClient(baseURL, token)
	client.AuthScheme = firstNonEmpty(authScheme, "Token")
	client.HTTPClient = httpClient
	return client
}

func requireToken(client *api.Client) error {
	if client == nil || strings.TrimSpace(client.Token) == "" {
		return errors.New("not authenticated; run `tripsy auth login`, `tripsy auth token set TOKEN`, or start the MCP server with TRIPSY_TOKEN")
	}
	return nil
}

const tokenVerifierCacheTTL = 5 * time.Minute

func BearerTokenVerifier(baseURL string) auth.TokenVerifier {
	verifier := func(ctx context.Context, token string, _ *http.Request) (*auth.TokenInfo, error) {
		token = strings.TrimSpace(token)
		if token == "" {
			return nil, fmt.Errorf("%w: empty Tripsy token", auth.ErrInvalidToken)
		}
		client := api.NewClient(baseURL, token)
		resp, err := client.Request(ctx, "GET", "/v1/me", nil, nil)
		if err != nil {
			var apiErr *api.Error
			if errors.As(err, &apiErr) && (apiErr.StatusCode == http.StatusUnauthorized || apiErr.StatusCode == http.StatusForbidden) {
				return nil, fmt.Errorf("%w: Tripsy token rejected", auth.ErrInvalidToken)
			}
			return nil, err
		}
		return &auth.TokenInfo{
			Expiration: time.Now().Add(tokenVerifierCacheTTL),
			UserID:     userID(resp.Data),
			Extra: map[string]any{
				tokenInfoTripsyTokenKey: token,
				tokenInfoAuthSchemeKey:  "Token",
			},
		}, nil
	}
	return cachedVerifier(verifier, tokenVerifierCacheTTL)
}

func OAuthBearerTokenVerifier(userinfoEndpoint string) auth.TokenVerifier {
	verifier := func(ctx context.Context, token string, _ *http.Request) (*auth.TokenInfo, error) {
		token = strings.TrimSpace(token)
		if token == "" {
			return nil, fmt.Errorf("%w: empty OAuth access token", auth.ErrInvalidToken)
		}
		client := api.NewClient("", token)
		client.AuthScheme = "Bearer"
		resp, err := client.Request(ctx, "GET", userinfoEndpoint, nil, nil)
		if err != nil {
			var apiErr *api.Error
			if errors.As(err, &apiErr) && (apiErr.StatusCode == http.StatusUnauthorized || apiErr.StatusCode == http.StatusForbidden) {
				return nil, fmt.Errorf("%w: OAuth token rejected", auth.ErrInvalidToken)
			}
			return nil, err
		}
		return &auth.TokenInfo{
			Expiration: time.Now().Add(tokenVerifierCacheTTL),
			UserID:     userID(resp.Data),
			Extra: map[string]any{
				tokenInfoTripsyTokenKey: token,
				tokenInfoAuthSchemeKey:  "Bearer",
			},
		}, nil
	}
	return cachedVerifier(verifier, tokenVerifierCacheTTL)
}

func userID(data any) string {
	values, ok := data.(map[string]any)
	if !ok {
		return ""
	}
	for _, key := range []string{"sub", "id", "uuid", "email", "username"} {
		value := strings.TrimSpace(fmt.Sprint(values[key]))
		if value != "" && value != "<nil>" {
			return value
		}
	}
	return ""
}

func envelope(resp *api.Response, summary string) map[string]any {
	if resp == nil {
		return map[string]any{"summary": summary}
	}
	return map[string]any{
		"status_code": resp.StatusCode,
		"summary":     summary,
		"data":        resp.Data,
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func readOnly() *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{ReadOnlyHint: true, DestructiveHint: boolPtr(false), IdempotentHint: true, OpenWorldHint: boolPtr(false)}
}

func additive() *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{ReadOnlyHint: false, DestructiveHint: boolPtr(false), OpenWorldHint: boolPtr(false)}
}

func idempotentWrite() *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{ReadOnlyHint: false, DestructiveHint: boolPtr(false), IdempotentHint: true, OpenWorldHint: boolPtr(false)}
}

func destructive() *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{ReadOnlyHint: false, DestructiveHint: boolPtr(true), OpenWorldHint: boolPtr(false)}
}

func boolPtr(value bool) *bool {
	return &value
}
