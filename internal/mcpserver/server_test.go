package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/tripsyapp/cli/internal/api"
	"github.com/tripsyapp/cli/internal/config"
)

func TestListToolsIncludesCoreTripsySurface(t *testing.T) {
	session, cleanup := connectTestSession(t, "test-token", http.NotFoundHandler())
	defer cleanup()

	res, err := session.ListTools(testContext(t), nil)
	if err != nil {
		t.Fatalf("ListTools() failed: %v", err)
	}

	for _, name := range []string{
		"tripsy_status",
		"tripsy_trips_create",
		"tripsy_activities_create",
		"tripsy_collaborators_list",
		"tripsy_raw_request",
	} {
		if findTool(res.Tools, name) == nil {
			t.Fatalf("tool %q was not registered", name)
		}
	}

	for _, name := range []string{
		"tripsy_emails_list",
		"tripsy_inbox_list",
		"tripsy_documents_attach",
		"tripsy_documents_upload",
		"tripsy_uploads_create",
	} {
		if findTool(res.Tools, name) != nil {
			t.Fatalf("tool %q should not be registered yet", name)
		}
	}

	allowedToolName := regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
	for _, tool := range res.Tools {
		if !allowedToolName.MatchString(tool.Name) {
			t.Fatalf("tool name %q contains characters rejected by common MCP clients", tool.Name)
		}
	}

	activitiesList := findTool(res.Tools, "tripsy_activities_list")
	if activitiesList == nil {
		t.Fatal("tripsy_activities_list was not registered")
	}
	if activitiesList.Title != "List Activities" {
		t.Fatalf("activities list title = %q, want List Activities", activitiesList.Title)
	}
	if activitiesList.Annotations == nil || !activitiesList.Annotations.ReadOnlyHint {
		t.Fatal("activities list should be marked read-only")
	}

	tripsCreate := findTool(res.Tools, "tripsy_trips_create")
	if !strings.Contains(tripsCreate.Description, "cover_image_url") || !strings.Contains(tripsCreate.Description, "images.unsplash.com") {
		t.Fatalf("trips create description should mention Unsplash cover_image_url guidance: %q", tripsCreate.Description)
	}

	activitiesCreate := findTool(res.Tools, "tripsy_activities_create")
	if !strings.Contains(activitiesCreate.Description, "activity_type") || !strings.Contains(activitiesCreate.Description, "latitude/longitude") {
		t.Fatalf("activities create description should mention category and coordinates guidance: %q", activitiesCreate.Description)
	}
}

func TestTripCreateSendsAuthenticatedTripsyRequest(t *testing.T) {
	var called atomic.Int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called.Add(1)
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/v1/trips" {
			t.Errorf("path = %s, want /v1/trips", r.URL.Path)
		}
		if got := r.URL.Query().Get("fields!"); got != "documents,emails" {
			t.Errorf("fields! = %q, want documents,emails", got)
		}
		if got := r.Header.Get("Authorization"); got != "Token test-token" {
			t.Errorf("Authorization = %q, want Token test-token", got)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode body: %v", err)
		}
		if body["name"] != "Copenhagen" {
			t.Errorf("body name = %v, want Copenhagen", body["name"])
		}
		if got := fmt.Sprint(body["cover_image_url"]); !strings.HasPrefix(got, "https://images.unsplash.com/photo-") {
			t.Errorf("cover_image_url = %q, want direct Unsplash image URL", got)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":42,"name":"Copenhagen"}`))
	})
	session, cleanup := connectTestSession(t, "test-token", handler)
	defer cleanup()

	res := callTool(t, session, "tripsy_trips_create", map[string]any{
		"data": map[string]any{
			"name":            "Copenhagen",
			"timezone":        "Europe/Copenhagen",
			"cover_image_url": "https://images.unsplash.com/photo-1723596807374-5cfbe183a820?ixid=abc&ixlib=rb-4.1.0",
		},
	})
	if res.IsError {
		t.Fatalf("tool returned error: %s", toolText(res))
	}
	if called.Load() != 1 {
		t.Fatalf("handler called %d times, want 1", called.Load())
	}

	structured := structuredMap(t, res)
	if got := fmt.Sprint(structured["status_code"]); got != "201" {
		t.Fatalf("status_code = %s, want 201", got)
	}
	if structured["summary"] != "Trip created" {
		t.Fatalf("summary = %v, want Trip created", structured["summary"])
	}
	data := structured["data"].(map[string]any)
	if data["name"] != "Copenhagen" {
		t.Fatalf("data.name = %v, want Copenhagen", data["name"])
	}
}

func TestActivitiesListSendsFilters(t *testing.T) {
	var called atomic.Int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called.Add(1)
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/v1/trip/42/activities" {
			t.Errorf("path = %s, want /v1/trip/42/activities", r.URL.Path)
		}
		query := r.URL.Query()
		if got := query.Get("fields"); got != "id,name" {
			t.Errorf("fields = %q, want id,name", got)
		}
		if got := query.Get("fields!"); got != "documents,emails" {
			t.Errorf("fields! = %q, want documents,emails", got)
		}
		if got := query.Get("activityType"); got != "museum" {
			t.Errorf("activityType = %q, want museum", got)
		}
		if got := query.Get("deleted"); got != "true" {
			t.Errorf("deleted = %q, want true", got)
		}
		if got := query.Get("updatedSince"); got != "2026-04-01T00:00:00Z" {
			t.Errorf("updatedSince = %q, want timestamp", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[]}`))
	})
	session, cleanup := connectTestSession(t, "test-token", handler)
	defer cleanup()

	res := callTool(t, session, "tripsy_activities_list", map[string]any{
		"trip_id":        "42",
		"fields":         []string{"name", "id"},
		"fields_exclude": []string{"documents"},
		"activity_type":  "museum",
		"deleted":        true,
		"updated_since":  "2026-04-01T00:00:00Z",
	})
	if res.IsError {
		t.Fatalf("tool returned error: %s", toolText(res))
	}
	if called.Load() != 1 {
		t.Fatalf("handler called %d times, want 1", called.Load())
	}
	if got := structuredMap(t, res)["summary"]; got != "Activities" {
		t.Fatalf("summary = %v, want Activities", got)
	}
}

func TestToolsRequireTokenBeforeCallingAPI(t *testing.T) {
	var called atomic.Int32
	session, cleanup := connectTestSession(t, "", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called.Add(1)
		t.Errorf("API should not be called without a token")
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer cleanup()

	res := callTool(t, session, "tripsy_trips_list", map[string]any{})
	if !res.IsError {
		t.Fatalf("expected tool error for missing token, got: %s", toolText(res))
	}
	if !strings.Contains(toolText(res), "not authenticated") {
		t.Fatalf("error text = %q, want not authenticated guidance", toolText(res))
	}
	if called.Load() != 0 {
		t.Fatalf("handler called %d times, want 0", called.Load())
	}
}

func TestRawRequestRejectsExternalURL(t *testing.T) {
	var called atomic.Int32
	session, cleanup := connectTestSession(t, "test-token", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called.Add(1)
		t.Errorf("API should not be called for an external raw URL")
	}))
	defer cleanup()

	res := callTool(t, session, "tripsy_raw_request", map[string]any{
		"method": "GET",
		"path":   "https://example.com/v1/me",
	})
	if !res.IsError {
		t.Fatalf("expected tool error for external raw URL, got: %s", toolText(res))
	}
	if !strings.Contains(toolText(res), "Tripsy API path") {
		t.Fatalf("error text = %q, want Tripsy API path guidance", toolText(res))
	}
	if called.Load() != 0 {
		t.Fatalf("handler called %d times, want 0", called.Load())
	}
}

func TestRawRequestRejectsWithheldCapabilities(t *testing.T) {
	session, cleanup := connectTestSession(t, "test-token", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("API should not be called for withheld MCP capabilities")
	}))
	defer cleanup()

	for _, tt := range []struct {
		path string
		want string
	}{
		{path: "/v1/emails", want: "email endpoints"},
		{path: "/v1/automation/emails/123", want: "inbox endpoints"},
		{path: "/v1/documents/123/get", want: "document endpoints"},
		{path: "/v1/trip/42/activity/9/documents", want: "document endpoints"},
		{path: "/v1/storage/uploads", want: "upload endpoints"},
	} {
		t.Run(tt.path, func(t *testing.T) {
			res := callTool(t, session, "tripsy_raw_request", map[string]any{
				"method": "GET",
				"path":   tt.path,
			})
			if !res.IsError {
				t.Fatalf("expected tool error for %s, got: %s", tt.path, toolText(res))
			}
			if !strings.Contains(toolText(res), tt.want) {
				t.Fatalf("error text = %q, want %q", toolText(res), tt.want)
			}
		})
	}
}

func connectTestSession(t *testing.T, token string, handler http.Handler) (*mcp.ClientSession, func()) {
	t.Helper()

	apiServer := httptest.NewServer(handler)
	server := NewWithClient(api.NewClient(apiServer.URL, token), config.NewStore(t.TempDir()), "test")
	serverTransport, clientTransport := mcp.NewInMemoryTransports()

	serverSession, err := server.Connect(testContext(t), serverTransport, nil)
	if err != nil {
		apiServer.Close()
		t.Fatalf("server.Connect() failed: %v", err)
	}

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "test"}, nil)
	clientSession, err := client.Connect(testContext(t), clientTransport, nil)
	if err != nil {
		serverSession.Close()
		apiServer.Close()
		t.Fatalf("client.Connect() failed: %v", err)
	}

	cleanup := func() {
		clientSession.Close()
		serverSession.Close()
		apiServer.Close()
	}
	return clientSession, cleanup
}

func testContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func callTool(t *testing.T, session *mcp.ClientSession, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	res, err := session.CallTool(testContext(t), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("CallTool(%q) failed: %v", name, err)
	}
	return res
}

func findTool(tools []*mcp.Tool, name string) *mcp.Tool {
	for _, tool := range tools {
		if tool.Name == name {
			return tool
		}
	}
	return nil
}

func structuredMap(t *testing.T, res *mcp.CallToolResult) map[string]any {
	t.Helper()
	switch value := res.StructuredContent.(type) {
	case map[string]any:
		return value
	case json.RawMessage:
		var out map[string]any
		if err := json.Unmarshal(value, &out); err != nil {
			t.Fatalf("unmarshal structured content: %v", err)
		}
		return out
	case []byte:
		var out map[string]any
		if err := json.Unmarshal(value, &out); err != nil {
			t.Fatalf("unmarshal structured content: %v", err)
		}
		return out
	default:
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatalf("marshal structured content %T: %v", value, err)
		}
		var out map[string]any
		if err := json.Unmarshal(data, &out); err != nil {
			t.Fatalf("unmarshal structured content %T: %v", value, err)
		}
		return out
	}
}

func toolText(res *mcp.CallToolResult) string {
	for _, content := range res.Content {
		if text, ok := content.(*mcp.TextContent); ok {
			return text.Text
		}
	}
	return ""
}
