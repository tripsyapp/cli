package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/tripsyapp/cli/internal/api"
	"github.com/tripsyapp/cli/internal/config"
	"github.com/tripsyapp/cli/internal/mcpserver"
)

func TestRegisterMCPHTTPHandlersAddsExactRootAlias(t *testing.T) {
	mux := http.NewServeMux()
	paths := registerMCPHTTPHandlers(mux, "/mcp", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	wantPaths := []string{"/mcp", "/"}
	if len(paths) != len(wantPaths) {
		t.Fatalf("registered paths = %v, want %v", paths, wantPaths)
	}
	for i := range wantPaths {
		if paths[i] != wantPaths[i] {
			t.Fatalf("registered paths = %v, want %v", paths, wantPaths)
		}
	}

	for _, path := range []string{"/", "/mcp"} {
		res := httptest.NewRecorder()
		mux.ServeHTTP(res, httptest.NewRequest(http.MethodPost, path, nil))
		if res.Code != http.StatusNoContent {
			t.Fatalf("%s returned status %d, want %d", path, res.Code, http.StatusNoContent)
		}
	}

	res := httptest.NewRecorder()
	mux.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/anything-else", nil))
	if res.Code != http.StatusNotFound {
		t.Fatalf("unknown path returned status %d, want %d", res.Code, http.StatusNotFound)
	}
}

func TestRegisterMCPHTTPHandlersDoesNotDuplicateRoot(t *testing.T) {
	mux := http.NewServeMux()
	paths := registerMCPHTTPHandlers(mux, "/", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	if len(paths) != 1 || paths[0] != "/" {
		t.Fatalf("registered paths = %v, want [/]", paths)
	}
}

func TestNewHTTPServerHasDefensiveTimeouts(t *testing.T) {
	server := newHTTPServer("127.0.0.1:0", http.NotFoundHandler())
	if server.ReadHeaderTimeout <= 0 {
		t.Fatal("ReadHeaderTimeout must be configured")
	}
	if server.IdleTimeout <= 0 {
		t.Fatal("IdleTimeout must be configured")
	}
	if server.Handler == nil {
		t.Fatal("Handler must be configured")
	}
}

func TestRedirectBrowserToMCPDocumentation(t *testing.T) {
	nextCalls := 0
	handler := redirectBrowserToMCPDocumentation(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalls++
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept", "text/html,application/xhtml+xml;q=0.9,*/*;q=0.8")
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusFound)
	}
	if got := res.Header().Get("Location"); got != mcpDocumentationURL {
		t.Fatalf("Location = %q, want %q", got, mcpDocumentationURL)
	}
	if got := res.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
	vary := strings.Join(res.Header().Values("Vary"), ",")
	if !strings.Contains(vary, "Accept") || !strings.Contains(vary, "Authorization") {
		t.Fatalf("Vary = %q, want Accept and Authorization", vary)
	}
	if nextCalls != 0 {
		t.Fatalf("next handler called %d times, want 0", nextCalls)
	}
}

func TestRedirectBrowserToMCPDocumentationPreservesMachineRequests(t *testing.T) {
	tests := []struct {
		name          string
		method        string
		path          string
		accept        string
		authorization string
	}{
		{name: "MCP POST", method: http.MethodPost, path: "/", accept: "application/json, text/event-stream"},
		{name: "MCP SSE GET", method: http.MethodGet, path: "/", accept: "text/event-stream"},
		{name: "legacy MCP path", method: http.MethodGet, path: "/mcp", accept: "text/html"},
		{name: "authenticated browser GET", method: http.MethodGet, path: "/", accept: "text/html", authorization: "Bearer token"},
		{name: "generic client", method: http.MethodGet, path: "/", accept: "*/*"},
		{name: "HTML explicitly rejected", method: http.MethodGet, path: "/", accept: "text/html;q=0, text/event-stream"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := redirectBrowserToMCPDocumentation(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("WWW-Authenticate", `Bearer resource_metadata="https://mcp.test/.well-known/oauth-protected-resource"`)
				w.WriteHeader(http.StatusUnauthorized)
			}))
			req := httptest.NewRequest(tt.method, tt.path, nil)
			req.Header.Set("Accept", tt.accept)
			if tt.authorization != "" {
				req.Header.Set("Authorization", tt.authorization)
			}
			res := httptest.NewRecorder()

			handler.ServeHTTP(res, req)

			if res.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d", res.Code, http.StatusUnauthorized)
			}
			if got := res.Header().Get("WWW-Authenticate"); got == "" {
				t.Fatal("WWW-Authenticate challenge was not preserved")
			}
		})
	}
}

func TestOpenAIAppsChallenge(t *testing.T) {
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/.well-known/openai-apps-challenge", nil)

	openAIAppsChallenge(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
	if got := res.Header().Get("Content-Type"); got != "text/plain; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want text/plain; charset=utf-8", got)
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(body), openAIAppsChallengeResponse+"\n"; got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
}

func TestExplicitToolAnnotationsHTTPHandlerNormalizesPostResponses(t *testing.T) {
	handler := explicitToolAnnotationsHTTPHandler{next: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"tools":[{"name":"tripsy_trips_create","annotations":{"destructiveHint":false,"openWorldHint":false}}]}}`))
	})}

	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/mcp", nil))

	if !strings.Contains(res.Body.String(), `"readOnlyHint":false`) {
		t.Fatalf("response body should include explicit readOnlyHint false: %s", res.Body.String())
	}
}

func TestStreamableHTTPListToolsThroughHostedHandlerStack(t *testing.T) {
	server := mcpserver.NewWithClientOptions(api.NewClient("https://api.test", ""), config.NewStore(t.TempDir()), mcpserver.Options{DisableRawRequest: true})
	streamHandler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return server
	}, &mcp.StreamableHTTPOptions{Stateless: true})
	httpHandler := explicitToolAnnotationsHTTPHandler{next: streamHandler}
	verifier := func(_ context.Context, token string, _ *http.Request) (*auth.TokenInfo, error) {
		if token != "test-token" {
			return nil, auth.ErrInvalidToken
		}
		return &auth.TokenInfo{
			Expiration: time.Now().Add(time.Hour),
			UserID:     "test-user",
		}, nil
	}
	httpServer := httptest.NewServer(auth.RequireBearerToken(verifier, nil)(httpHandler))
	defer httpServer.Close()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "test"}, nil)
	session, err := client.Connect(testContext(t), &mcp.StreamableClientTransport{
		Endpoint:             httpServer.URL,
		HTTPClient:           &http.Client{Transport: bearerTokenTransport{token: "test-token"}},
		DisableStandaloneSSE: true,
	}, nil)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer session.Close()

	res, err := session.ListTools(testContext(t), nil)
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}
	if findTool(res.Tools, "tripsy_trips_create") == nil {
		t.Fatal("tripsy_trips_create was not returned over streamable HTTP")
	}
	if findTool(res.Tools, "tripsy_raw_request") != nil {
		t.Fatal("tripsy_raw_request should be disabled for hosted HTTP")
	}
}

type bearerTokenTransport struct {
	token string
}

func (t bearerTokenTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("Authorization", "Bearer "+t.token)
	return http.DefaultTransport.RoundTrip(req)
}

func testContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func findTool(tools []*mcp.Tool, name string) *mcp.Tool {
	for _, tool := range tools {
		if tool.Name == name {
			return tool
		}
	}
	return nil
}
