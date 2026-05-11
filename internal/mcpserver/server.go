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

const tokenInfoTripsyTokenKey = "tripsy_token"
const tokenInfoAuthSchemeKey = "auth_scheme"

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
	if !s.disableRawRequest {
		addTool(server, toolName("tripsy", "raw_request"), "Raw Tripsy API Request", "Make a raw request to supported Tripsy public API endpoints that do not yet have a dedicated MCP tool. Prefer typed tools when available.", destructive(), s.rawRequest)
	}

	addTool(server, toolName("tripsy", "me", "show"), "Show Current Tripsy User", "Return the authenticated Tripsy profile.", readOnly(), s.meShow)
	addTool(server, toolName("tripsy", "me", "update"), "Update Current Tripsy User", "Update current Tripsy profile fields such as name, timezone, language, or default currency.", idempotentWrite(), s.meUpdate)

	addTool(server, toolName("tripsy", "trips", "list"), "List Trips", "List Tripsy trips. Supports fields, excluded fields, deleted records, and updated-since filtering.", readOnly(), s.tripsList)
	addTool(server, toolName("tripsy", "trips", "show"), "Show Trip", "Fetch one Tripsy trip by id.", readOnly(), s.tripShow)
	addTool(server, toolName("tripsy", "trips", "create"), "Create Trip", "Create a Tripsy trip. For destination trips, set cover_image_url to a direct images.unsplash.com URL when available.", additive(), s.tripCreate)
	addTool(server, toolName("tripsy", "trips", "update"), "Update Trip", "Update a Tripsy trip by id.", idempotentWrite(), s.tripUpdate)
	addTool(server, toolName("tripsy", "trips", "delete"), "Delete Trip", "Soft-delete a Tripsy trip by id.", destructive(), s.tripDelete)

	s.registerResource(server, resourceSpec{
		Prefix:       "activities",
		Title:        "Activity",
		PluralTitle:  "Activities",
		ListPath:     "/v1/trip/%s/activities",
		DetailPath:   "/v1/trip/%s/activity/%s",
		FilterName:   "activity_type",
		FilterParam:  "activityType",
		FilterHint:   activityCategoryHint,
		Description:  "Scheduled or unscheduled trip activities. Use one activity per actual stop, reservation, meal, tour, or experience.",
		CreateAdvice: "Set activity_type to the most specific supported slug, and include latitude/longitude for map-ready location items.",
		ExcludeData:  true,
	})
	s.registerResource(server, resourceSpec{
		Prefix:       "hostings",
		Title:        "Hosting",
		PluralTitle:  "Hostings",
		ListPath:     "/v1/trip/%s/hostings",
		DetailPath:   "/v1/trip/%s/hosting/%s",
		Description:  "Hotel and lodging plans.",
		CreateAdvice: "Use hostings for hotels and lodging rather than activities. Include address and latitude/longitude when known.",
		ExcludeData:  true,
	})
	s.registerResource(server, resourceSpec{
		Prefix:       "transportations",
		Title:        "Transportation",
		PluralTitle:  "Transportations",
		ListPath:     "/v1/trip/%s/transportations",
		DetailPath:   "/v1/trip/%s/transportation/%s",
		FilterName:   "transportation_type",
		FilterParam:  "transportationType",
		FilterHint:   transportationCategoryHint,
		Description:  "Flights, trains, cars, buses, ferries, walks, and other point-to-point travel.",
		CreateAdvice: "Use transportation_type for the segment kind and include departure/arrival coordinates when known.",
		ExcludeData:  true,
	})
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
		Name:        name,
		Title:       title,
		Description: description,
		Annotations: annotations,
	}, handler)
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
	return &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: boolPtr(false)}
}

func additive() *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{DestructiveHint: boolPtr(false), OpenWorldHint: boolPtr(false)}
}

func idempotentWrite() *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{DestructiveHint: boolPtr(false), IdempotentHint: true, OpenWorldHint: boolPtr(false)}
}

func destructive() *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{DestructiveHint: boolPtr(true), OpenWorldHint: boolPtr(false)}
}

func boolPtr(value bool) *bool {
	return &value
}
