package mcpserver

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestTravelStatusUpdateMe(t *testing.T) {
	for _, travelling := range []bool{false, true} {
		t.Run(map[bool]string{false: "following", true: "travelling"}[travelling], func(t *testing.T) {
			calls := 0
			session, cleanup := connectTestSession(t, "test-token", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.Header().Set("Content-Type", "application/json")
				if r.Method == "GET" && r.URL.Path == "/v1/me" {
					_, _ = w.Write([]byte(`{"id":7}`))
					return
				}
				if r.Method != "PATCH" || r.URL.Path != "/v1/trip/42/collaborator/7/permissions" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL)
				}
				var data map[string]any
				if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
					t.Error(err)
				}
				if len(data) != 1 || data["is_travelling"] != travelling {
					t.Errorf("unexpected permissions: %#v", data)
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"user_id": 7, "permissions": data})
			}))
			defer cleanup()
			tools, err := session.ListTools(testContext(t), nil)
			if err != nil {
				t.Fatal(err)
			}
			schema := toolSchemaString(t, findTool(tools.Tools, "tripsy_collaborators_update"))
			if !strings.Contains(schema, "is_travelling") || !strings.Contains(schema, "me to resolve") {
				t.Fatal("missing travel schema guidance")
			}
			res := callTool(t, session, "tripsy_collaborators_update", map[string]any{"trip_id": "42", "user_id": "me", "permissions": map[string]any{"is_travelling": travelling}})
			if res.IsError || calls != 2 {
				t.Fatalf("calls=%d result=%s", calls, toolText(res))
			}
		})
	}
}

func TestGuestToolAuthenticationAndAPIErrors(t *testing.T) {
	for _, tt := range []struct {
		name, token, body string
		status            int
	}{
		{"unauthenticated", "", "", 0},
		{"missing identity", "test-token", `{}`, 200},
		{"identity forbidden", "test-token", `{"error":"Permission denied"}`, 403},
	} {
		t.Run(tt.name, func(t *testing.T) {
			session, cleanup := connectTestSession(t, tt.token, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.token == "" || r.Method != "GET" || r.URL.Path != "/v1/me" {
					t.Error("unexpected request")
				}
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer cleanup()
			res := callTool(t, session, "tripsy_collaborators_update", map[string]any{"trip_id": "42", "user_id": "me", "permissions": map[string]any{"is_travelling": false}})
			if !res.IsError {
				t.Fatal("failed identity resolution must not report successful update")
			}
		})
	}
	session, cleanup := connectTestSession(t, "test-token", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"Permission denied"}`))
	}))
	defer cleanup()
	res := callTool(t, session, "tripsy_collaborators_delete", map[string]any{"trip_id": "42", "user_id": "7"})
	if !res.IsError || !strings.Contains(toolText(res), "403") {
		t.Fatalf("expected permission error, got %s", toolText(res))
	}
}
