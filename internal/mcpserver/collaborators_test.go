package mcpserver

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func TestGuestToolSchemasAndAnnotations(t *testing.T) {
	session, cleanup := connectTestSession(t, "test-token", http.NotFoundHandler())
	defer cleanup()
	res, err := session.ListTools(testContext(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"tripsy_collaborators_invite", "tripsy_collaborators_update", "tripsy_collaborators_delete"} {
		tool := findTool(res.Tools, name)
		if tool == nil {
			t.Fatalf("missing tool %s", name)
		}
		if tool.Annotations.ReadOnlyHint {
			t.Errorf("incorrect read-only annotation: %s", name)
		}
		if tool.Annotations.DestructiveHint == nil || *tool.Annotations.DestructiveHint != (name == "tripsy_collaborators_delete") {
			t.Errorf("incorrect destructive annotation: %s", name)
		}
		if name == "tripsy_collaborators_update" {
			for _, field := range []string{"permissions", "can_edit", "receive_notifications"} {
				if !strings.Contains(toolSchemaString(t, tool), field) {
					t.Errorf("update schema missing %s", field)
				}
			}
		}
		if name == "tripsy_collaborators_invite" && !strings.Contains(toolSchemaString(t, tool), "read_only") {
			t.Error("invite schema missing read_only")
		}
	}
}

func TestGuestToolsRequestContracts(t *testing.T) {
	for _, tt := range []struct {
		name, tool, method, path, body string
		args                           map[string]any
	}{
		{"invite defaults", "tripsy_collaborators_invite", "POST", "/v1/guests/invite", `{"trip_id":"42","invited_user_email":"guest@example.com"}`, map[string]any{"trip_id": "42", "invited_user_email": "guest@example.com"}},
		{"invite permissions", "tripsy_collaborators_invite", "POST", "/v1/guests/invite", `{"trip_id":"42","invited_user_email":"guest@example.com","permissions":{"read_only":true,"can_see_documents":false}}`, map[string]any{"trip_id": "42", "invited_user_email": "guest@example.com", "permissions": map[string]any{"read_only": true, "can_see_documents": false}}},
		{"update explicit user", "tripsy_collaborators_update", "PATCH", "/v1/trip/42/collaborator/8/permissions", `{"can_edit":false,"can_see_expenses":true}`, map[string]any{"trip_id": "42", "user_id": "8", "permissions": map[string]any{"can_edit": false, "can_see_expenses": true}}},
		{"delete", "tripsy_collaborators_delete", "DELETE", "/v1/trip/42/collaborator/8", "", map[string]any{"trip_id": "42", "user_id": "8"}},
		{"escaped ids", "tripsy_collaborators_delete", "DELETE", "/v1/trip/42%2F43/collaborator/7%2F8", "", map[string]any{"trip_id": "42/43", "user_id": "7/8"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			session, cleanup := connectTestSession(t, "test-token", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.Header().Set("Content-Type", "application/json")

				if r.Method != tt.method || r.URL.EscapedPath() != tt.path {
					t.Errorf("request = %s %s, want %s %s", r.Method, r.URL.EscapedPath(), tt.method, tt.path)
				}
				if tt.body != "" {
					var got, want any
					if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
						t.Error(err)
					}
					_ = json.Unmarshal([]byte(tt.body), &want)
					if !reflect.DeepEqual(got, want) {
						t.Errorf("body = %#v, want %#v", got, want)
					}
				}
				_, _ = w.Write([]byte(`{"success":true}`))
			}))
			defer cleanup()
			res := callTool(t, session, tt.tool, tt.args)
			if res.IsError {
				t.Fatalf("tool error: %s", toolText(res))
			}
			wantCalls := 1
			if calls != wantCalls {
				t.Errorf("calls = %d, want %d", calls, wantCalls)
			}
			if tt.tool == "tripsy_collaborators_invite" && !strings.Contains(structuredMap(t, res)["summary"].(string), "does not confirm") {
				t.Error("invitation must not claim successful membership")
			}
		})
	}
}

func TestGuestToolsRejectInvalidInput(t *testing.T) {
	for _, tt := range []struct {
		name, tool string
		args       map[string]any
	}{
		{"empty permissions", "tripsy_collaborators_update", map[string]any{"trip_id": "42", "user_id": "7", "permissions": map[string]any{}}},
		{"string boolean", "tripsy_collaborators_update", map[string]any{"trip_id": "42", "user_id": "7", "permissions": map[string]any{"can_edit": "false"}}},
		{"wrong permission", "tripsy_collaborators_update", map[string]any{"trip_id": "42", "user_id": "7", "permissions": map[string]any{"read_only": true}}},
		{"missing user", "tripsy_collaborators_delete", map[string]any{"trip_id": "42"}},
		{"blank user", "tripsy_collaborators_delete", map[string]any{"trip_id": "42", "user_id": " "}},
		{"blank email", "tripsy_collaborators_invite", map[string]any{"trip_id": "42", "invited_user_email": " "}},
		{"blank trip", "tripsy_collaborators_invite", map[string]any{"trip_id": " ", "invited_user_email": "guest@example.com"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			session, cleanup := connectTestSession(t, "test-token", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("unexpected API request") }))
			defer cleanup()
			if res := callTool(t, session, tt.tool, tt.args); !res.IsError {
				t.Fatalf("expected error, got %s", toolText(res))
			}
		})
	}
}

func TestCollaboratorsListCombinesPages(t *testing.T) {
	calls := 0
	session, cleanup := connectTestSession(t, "test-token", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("page") == "2" {
			_, _ = w.Write([]byte(`{"count":2,"next":null,"results":[{"id":8,"joined":false}]}`))
		} else {
			_, _ = w.Write([]byte(`{"count":2,"next":"/v1/trip/42/collaborators?page=2","results":[{"id":7,"joined":true}]}`))
		}
	}))
	defer cleanup()
	res := callTool(t, session, "tripsy_collaborators_list", map[string]any{"trip_id": "42"})
	if res.IsError || calls != 2 {
		t.Fatalf("calls=%d result=%s", calls, toolText(res))
	}
	if len(structuredMap(t, res)["data"].(map[string]any)["results"].([]any)) != 2 {
		t.Fatal("missing collaborator pages")
	}
}
