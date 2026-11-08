package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/tripsyapp/cli/internal/api"
	"github.com/tripsyapp/cli/internal/config"
	"github.com/tripsyapp/cli/internal/mcpserver"
)

const testResourceMetadataURL = "https://mcp.test/.well-known/oauth-protected-resource"

func TestAnonymousMCPDiscoveryThenAuthenticatedCalls(t *testing.T) {
	for _, stateless := range []bool{true, false} {
		for _, path := range []string{"/", "/mcp"} {
			t.Run(fmt.Sprintf("stateless=%t%s", stateless, path), func(t *testing.T) {
				var mu sync.Mutex
				var authorizations []string
				apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					mu.Lock()
					defer mu.Unlock()
					authorizations = append(authorizations, r.Header.Get("Authorization"))
					if r.URL.Path != "/v1/me" {
						t.Errorf("unexpected API path %q", r.URL.Path)
					}
					user := strings.TrimPrefix(r.Header.Get("Authorization"), "Token ")
					if user != "alice" && user != "bob" {
						http.Error(w, "invalid token", http.StatusUnauthorized)
						return
					}
					w.Header().Set("Content-Type", "application/json")
					_, _ = fmt.Fprintf(w, `{"id":%q}`, user)
				}))
				defer apiServer.Close()
				hosted := newDiscoveryTestServer(t, apiServer.URL, stateless)
				defer hosted.Close()

				transport := &switchableBearerTransport{}
				client := mcp.NewClient(&mcp.Implementation{Name: "discovery-test", Version: "test"}, nil)
				// Leave standalone SSE enabled, as it is for ordinary SDK clients.
				session, err := client.Connect(testContext(t), &mcp.StreamableClientTransport{
					Endpoint:   hosted.URL + path,
					HTTPClient: &http.Client{Transport: transport},
				}, nil)
				if err != nil {
					t.Fatalf("anonymous Connect: %v", err)
				}
				defer session.Close()
				tools, err := session.ListTools(testContext(t), nil)
				if err != nil {
					t.Fatalf("anonymous ListTools: %v", err)
				}
				if findTool(tools.Tools, "tripsy_trips_create") == nil || findTool(tools.Tools, "tripsy_raw_request") != nil {
					t.Fatal("unexpected public tool catalog")
				}
				if err := session.Ping(testContext(t), nil); err != nil {
					t.Fatalf("anonymous Ping: %v", err)
				}
				mu.Lock()
				discoveryAPICalls := len(authorizations)
				mu.Unlock()
				if discoveryAPICalls != 0 {
					t.Fatalf("anonymous discovery made %d API requests", discoveryAPICalls)
				}

				for _, user := range []string{"alice", "bob", "alice"} {
					transport.token.Store(user)
					result, err := session.CallTool(testContext(t), &mcp.CallToolParams{Name: "tripsy_me_show"})
					if err != nil {
						t.Fatalf("CallTool as %s: %v", user, err)
					}
					encoded, err := json.Marshal(result.StructuredContent)
					if err != nil || result.IsError || !strings.Contains(string(encoded), `"id":"`+user+`"`) {
						t.Fatalf("CallTool as %s returned %+v (marshal: %v)", user, result, err)
					}
				}
				mu.Lock()
				defer mu.Unlock()
				want := "Token alice,Token alice,Token bob,Token bob,Token alice"
				if got := strings.Join(authorizations, ","); got != want {
					t.Fatalf("API authorizations = %q, want %q", got, want)
				}
			})
		}
	}
}

func TestAnonymousMCPRequestsCannotInvokeTools(t *testing.T) {
	var apiCalls atomic.Int32
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		apiCalls.Add(1)
		http.Error(w, "invalid token", http.StatusUnauthorized)
	}))
	defer apiServer.Close()
	hosted := newDiscoveryTestServer(t, apiServer.URL, true)
	defer hosted.Close()

	tests := []struct {
		name, method, body, authorization string
		status                            int
	}{
		{name: "profile call", body: `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"tripsy_me_show"}}`},
		{name: "status call", body: `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"tripsy_status"}}`},
		{name: "guidance call", body: `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"tripsy_itinerary_guidance"}}`},
		{name: "mixed case method", body: `{"jsonrpc":"2.0","id":1,"method":"tools/call","METHOD":"tools/list","params":{"name":"tripsy_status"}}`},
		{name: "duplicate method", body: `{"jsonrpc":"2.0","id":1,"method":"tools/list","method":"tools/call","params":{"name":"tripsy_status"}}`},
		{name: "mixed batch", body: `[{"jsonrpc":"2.0","id":1,"method":"tools/list"},{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"tripsy_status"}}]`},
		{name: "discovery batch", body: `[{"jsonrpc":"2.0","id":1,"method":"tools/list"}]`},
		{name: "unknown method", body: `{"jsonrpc":"2.0","id":1,"method":"future/method"}`},
		{name: "malformed JSON", body: `{`},
		{name: "empty body"},
		{name: "oversized discovery", body: `{"jsonrpc":"2.0","id":1,"method":"tools/list","padding":"` + strings.Repeat("x", 64<<10) + `"}`},
		{name: "SSE probe", method: http.MethodGet, status: http.StatusMethodNotAllowed},
		{name: "session deletion", method: http.MethodDelete},
		{name: "malformed authorization", body: `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`, authorization: "Basic invalid"},
	}
	for _, path := range []string{"/", "/mcp"} {
		for _, tt := range tests {
			t.Run(path+"/"+tt.name, func(t *testing.T) {
				res := sendDiscoveryTestRequest(t, hosted.URL+path, tt.method, tt.body, tt.authorization)
				defer res.Body.Close()
				wantStatus := tt.status
				if wantStatus == 0 {
					wantStatus = http.StatusUnauthorized
				}
				if res.StatusCode != wantStatus {
					body, _ := io.ReadAll(res.Body)
					t.Fatalf("status = %d, want %d: %s", res.StatusCode, wantStatus, body)
				}
				if wantStatus == http.StatusUnauthorized && !strings.Contains(res.Header.Get("WWW-Authenticate"), testResourceMetadataURL) {
					t.Fatalf("missing OAuth challenge: %v", res.Header)
				}
				if wantStatus == http.StatusMethodNotAllowed && res.Header.Get("Allow") != "POST" {
					t.Fatalf("Allow = %q, want POST", res.Header.Get("Allow"))
				}
			})
		}
	}
	if got := apiCalls.Load(); got != 0 {
		t.Fatalf("unauthorized requests made %d API calls", got)
	}
	for _, path := range []string{"/", "/mcp"} {
		res := sendDiscoveryTestRequest(t, hosted.URL+path, http.MethodPost, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`, "Bearer invalid")
		_ = res.Body.Close()
		if res.StatusCode != http.StatusUnauthorized || !strings.Contains(res.Header.Get("WWW-Authenticate"), testResourceMetadataURL) {
			t.Fatalf("invalid supplied discovery token: status %d, headers %v", res.StatusCode, res.Header)
		}
	}
	if got := apiCalls.Load(); got != 2 {
		t.Fatalf("invalid supplied tokens made %d API calls, want 2", got)
	}
}

func newDiscoveryTestServer(t *testing.T, apiBase string, stateless bool) *httptest.Server {
	t.Helper()
	server := mcpserver.NewWithClientOptions(api.NewClient(apiBase, ""), config.NewStore(t.TempDir()), mcpserver.Options{DisableRawRequest: true})
	streamHandler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, &mcp.StreamableHTTPOptions{
		Stateless: stateless,
	})
	handler := explicitToolAnnotationsHTTPHandler{next: normalizeMCPContentType(streamHandler)}
	protected := requireMCPBearerToken(handler, mcpserver.BearerTokenVerifier(apiBase), &auth.RequireBearerTokenOptions{ResourceMetadataURL: testResourceMetadataURL})
	mux := http.NewServeMux()
	registerMCPHTTPHandlers(mux, "/mcp", protected)
	return httptest.NewServer(mux)
}

func sendDiscoveryTestRequest(t *testing.T, endpoint, method, body, authorization string) *http.Response {
	t.Helper()
	if method == "" {
		method = http.MethodPost
	}
	req, err := http.NewRequestWithContext(testContext(t), method, endpoint, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

type switchableBearerTransport struct {
	token atomic.Value
}

func (transport *switchableBearerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	if token, ok := transport.token.Load().(string); ok && token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return http.DefaultTransport.RoundTrip(req)
}
