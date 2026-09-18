package mcpserver

import (
	"net/http"
	"testing"
)

func TestFavoriteGuestsTool(t *testing.T) {
	calls := 0
	session, cleanup := connectTestSession(t, "test-token", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "GET" || r.URL.Path != "/v1/guests/favorites" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("page") == "2" {
			_, _ = w.Write([]byte(`{"count":2,"next":null,"results":[{"id":92,"favorite_user":{"id":8},"pending":true}]}`))
		} else {
			_, _ = w.Write([]byte(`{"count":2,"next":"/v1/guests/favorites?page=2","results":[{"id":91,"favorite_user":{"id":7},"pending":false}]}`))
		}
	}))
	defer cleanup()
	listed, err := session.ListTools(testContext(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	tool := findTool(listed.Tools, "tripsy_guests_favorites_list")
	if tool == nil || !tool.Annotations.ReadOnlyHint || tool.Annotations.DestructiveHint == nil || *tool.Annotations.DestructiveHint {
		t.Fatal("missing read-only favorite guest tool")
	}
	res := callTool(t, session, tool.Name, map[string]any{})
	if res.IsError || calls != 2 {
		t.Fatalf("calls=%d result=%s", calls, toolText(res))
	}
	items := structuredMap(t, res)["data"].(map[string]any)["results"].([]any)
	if len(items) != 2 || items[0].(map[string]any)["pending"] != false || items[1].(map[string]any)["pending"] != true {
		t.Fatalf("invalid results: %#v", items)
	}
}

func TestFavoriteGuestsRequireAuthentication(t *testing.T) {
	session, cleanup := connectTestSession(t, "", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("unexpected API request") }))
	defer cleanup()
	if res := callTool(t, session, "tripsy_guests_favorites_list", map[string]any{}); !res.IsError {
		t.Fatal("unauthenticated request succeeded")
	}
}
