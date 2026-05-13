package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
	"github.com/tripsyapp/cli/internal/cli"
	"github.com/tripsyapp/cli/internal/mcpserver"
)

func main() {
	var opts mcpserver.Options
	var transport string
	var httpAddr string
	var httpPath string
	var httpStateless bool
	var httpJSON bool
	var httpRequireBearer bool
	var publicURL string
	var oauthIssuer string
	var oauthUserinfoEndpoint string
	var oauthScopes string
	var showVersion bool

	flags := flag.NewFlagSet("tripsy-mcp", flag.ExitOnError)
	flags.StringVar(&opts.APIBase, "api-base", "", "Tripsy API base URL; defaults to TRIPSY_API_BASE, stored config, or https://api.tripsy.app")
	flags.StringVar(&opts.Token, "token", "", "Tripsy API token for stdio mode; HTTP mode always requires client bearer tokens")
	flags.StringVar(&opts.ConfigDir, "config-dir", "", "Tripsy CLI config directory; defaults to TRIPSY_CONFIG_DIR or ~/.config/tripsy-cli")
	flags.BoolVar(&opts.DisableRawRequest, "disable-raw-request", false, "do not expose the broad tripsy_raw_request MCP tool")
	flags.StringVar(&transport, "transport", "stdio", "MCP transport: stdio or http")
	flags.StringVar(&httpAddr, "http-addr", "127.0.0.1:8787", "listen address for --transport=http")
	flags.StringVar(&httpPath, "http-path", "/mcp", "HTTP endpoint path for --transport=http")
	flags.BoolVar(&httpStateless, "http-stateless", false, "run streamable HTTP without MCP session retention")
	flags.BoolVar(&httpJSON, "http-json-response", false, "prefer application/json responses for streamable HTTP")
	flags.BoolVar(&httpRequireBearer, "http-require-bearer", false, "deprecated; HTTP mode always requires Authorization: Bearer from the client")
	flags.StringVar(&publicURL, "public-url", os.Getenv("TRIPSY_MCP_PUBLIC_URL"), "public HTTPS base URL for OAuth discovery, such as https://mcp.tripsy.app")
	flags.StringVar(&oauthIssuer, "oauth-issuer", os.Getenv("TRIPSY_OAUTH_ISSUER"), "OAuth issuer for hosted MCP discovery, such as https://my.tripsy.app")
	flags.StringVar(&oauthUserinfoEndpoint, "oauth-userinfo-endpoint", os.Getenv("TRIPSY_OAUTH_USERINFO_ENDPOINT"), "OAuth userinfo endpoint for validating bearer access tokens; defaults to <oauth-issuer>/oauth/userinfo")
	flags.StringVar(&oauthScopes, "oauth-scopes", firstNonEmpty(os.Getenv("TRIPSY_OAUTH_SCOPES"), "profile email"), "space-separated OAuth scopes advertised by the hosted MCP server")
	flags.BoolVar(&showVersion, "version", false, "print version and exit")
	flags.BoolVar(&showVersion, "v", false, "print version and exit")
	_ = flags.Parse(os.Args[1:])

	opts.Version = cli.Version
	httpTransport := isHTTPTransport(transport)
	if httpTransport {
		httpRequireBearer = true
		opts.RequestTokenOnly = true
	}
	if showVersion {
		fmt.Println(versionString())
		return
	}

	server, info, err := mcpserver.New(opts)
	if err != nil {
		log.Fatal(err)
	}

	switch strings.ToLower(strings.TrimSpace(transport)) {
	case "", "stdio":
		if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
			log.Fatal(err)
		}
	case "http", "streamable-http":
		runHTTP(server, info, httpAddr, httpPath, httpStateless, httpJSON, httpRequireBearer, hostedOAuthConfig{
			publicURL:            publicURL,
			issuer:               oauthIssuer,
			userinfoEndpoint:     oauthUserinfoEndpoint,
			scopes:               strings.Fields(oauthScopes),
			resourceMetadataPath: "/.well-known/oauth-protected-resource",
		})
	default:
		log.Fatalf("unsupported transport %q; expected stdio or http", transport)
	}
}

func isHTTPTransport(transport string) bool {
	switch strings.ToLower(strings.TrimSpace(transport)) {
	case "http", "streamable-http":
		return true
	default:
		return false
	}
}

type hostedOAuthConfig struct {
	publicURL            string
	issuer               string
	userinfoEndpoint     string
	scopes               []string
	resourceMetadataPath string
}

func runHTTP(server *mcp.Server, info mcpserver.RuntimeInfo, addr, path string, stateless, jsonResponse, requireBearer bool, oauthConfig hostedOAuthConfig) {
	path = normalizeHTTPPath(path)
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return server
	}, &mcp.StreamableHTTPOptions{
		Stateless:      stateless,
		JSONResponse:   jsonResponse,
		Logger:         slog.New(slog.NewTextHandler(os.Stderr, nil)),
		SessionTimeout: 30 * time.Minute,
	})
	var httpHandler http.Handler = handler
	if requireBearer {
		verifier := mcpserver.BearerTokenVerifier(info.APIBase)
		authOptions := &auth.RequireBearerTokenOptions{}
		if oauthConfig.enabled() {
			verifier = mcpserver.OAuthBearerTokenVerifier(oauthConfig.userinfoURL())
			authOptions.ResourceMetadataURL = oauthConfig.resourceMetadataURL()
		}
		httpHandler = auth.RequireBearerToken(verifier, authOptions)(httpHandler)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthz)
	if oauthConfig.enabled() {
		mux.Handle(oauthConfig.resourceMetadataPath, auth.ProtectedResourceMetadataHandler(&oauthex.ProtectedResourceMetadata{
			Resource:               joinURLPath(oauthConfig.publicURL, "/"),
			AuthorizationServers:   []string{strings.TrimRight(oauthConfig.issuer, "/")},
			ScopesSupported:        oauthConfig.scopes,
			BearerMethodsSupported: []string{"header"},
			ResourceName:           "Tripsy MCP",
			ResourceDocumentation:  "https://tripsy.app",
			ResourcePolicyURI:      "https://tripsy.app/privacy",
			ResourceTOSURI:         "https://tripsy.app/terms",
		}))
	}
	paths := registerMCPHTTPHandlers(mux, path, httpHandler)
	log.Printf("Tripsy MCP listening on http://%s%s (aliases=%s api_base=%s auth_backend=%s has_token=%t require_bearer=%t)", addr, path, strings.Join(paths, ","), info.APIBase, info.AuthBackend, info.HasToken, requireBearer)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func normalizeHTTPPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return path
}

func registerMCPHTTPHandlers(mux *http.ServeMux, path string, handler http.Handler) []string {
	path = normalizeHTTPPath(path)
	registered := make([]string, 0, 2)
	register := func(endpoint string) {
		pattern := endpoint
		if endpoint == "/" {
			pattern = "/{$}"
		}
		mux.Handle(pattern, handler)
		registered = append(registered, endpoint)
	}

	register(path)
	if path != "/" {
		register("/")
	}
	return registered
}

func (c hostedOAuthConfig) enabled() bool {
	return strings.TrimSpace(c.publicURL) != "" && strings.TrimSpace(c.issuer) != ""
}

func (c hostedOAuthConfig) resourceMetadataURL() string {
	return joinURLPath(c.publicURL, c.resourceMetadataPath)
}

func (c hostedOAuthConfig) userinfoURL() string {
	if strings.TrimSpace(c.userinfoEndpoint) != "" {
		return strings.TrimSpace(c.userinfoEndpoint)
	}
	return joinURLPath(c.issuer, "/oauth/userinfo")
}

func joinURLPath(base, path string) string {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	u, err := url.Parse(base)
	if err != nil {
		return base + path
	}
	u.Path = strings.TrimRight(u.Path, "/") + path
	return u.String()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok\n"))
}

func versionString() string {
	value := "tripsy-mcp version " + cli.Version
	if cli.Commit != "" {
		value += " (" + cli.Commit + ")"
	}
	if cli.Date != "" {
		value += " " + cli.Date
	}
	return value
}
