package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func TestCollaboratorCommands(t *testing.T) {
	for _, tt := range []struct {
		name, method, path, body string
		args                     []string
	}{
		{"list", "GET", "/v1/trip/42/collaborators", "", []string{"list", "--trip", "42"}},
		{"legacy list", "GET", "/v1/trip/42/collaborators", "", []string{"--trip", "42"}},
		{"legacy positional", "GET", "/v1/trip/42/collaborators", "", []string{"42"}},
		{"invite defaults", "POST", "/v1/guests/invite", `{"trip_id":"42","invited_user_email":"guest@example.com","permissions":{}}`, []string{"invite", "--trip", "42", "--email", "guest@example.com"}},
		{"invite permissions", "POST", "/v1/guests/invite", `{"trip_id":"42","invited_user_email":"guest@example.com","permissions":{"read_only":true,"is_travelling":false}}`, []string{"invite", "--trip", "42", "--email", "guest@example.com", "--read-only", "true", "--is-travelling", "false"}},
		{"update false", "PATCH", "/v1/trip/42/collaborator/7/permissions", `{"can_edit":false}`, []string{"update", "7", "--trip", "42", "--can-edit", "false"}},
		{"update data", "PATCH", "/v1/trip/42/collaborator/7/permissions", `{"can_see_expenses":false,"can_add_guests":true}`, []string{"update", "7", "--trip", "42", "--data", `{"can_see_expenses":false}`, "--set", "can_add_guests=true"}},
		{"delete", "DELETE", "/v1/trip/42/collaborator/7", "", []string{"delete", "7", "--trip", "42"}},
		{"escaped ids", "PATCH", "/v1/trip/42%2F43/collaborator/7%2F8/permissions", `{"can_edit":true}`, []string{"update", "7/8", "--trip", "42/43", "--can-edit", "true"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var requests int
			a, cleanup := testAPIApp(t, func(w http.ResponseWriter, r *http.Request) {
				requests++
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
			})
			defer cleanup()
			if err := a.collaborators(context.Background(), tt.args); err != nil {
				t.Fatal(err)
			}
			wantRequests := 1
			if requests != wantRequests {
				t.Errorf("requests = %d, want %d", requests, wantRequests)
			}
			if tt.method == "POST" && !strings.Contains(a.stdout.(*bytes.Buffer).String(), "success does not confirm") {
				t.Error("invitation output should explain the generic success response")
			}
		})
	}
}

func TestCollaboratorInvalidInputMakesNoRequest(t *testing.T) {
	for _, args := range [][]string{
		{"update", "7", "--trip", "42"},
		{"update", "7", "--trip", "42", "--can-edit", "no"},
		{"update", "7", "--trip", "42", "--data", `{"can_edit":null}`},
		{"update", "7", "--trip", "42", "--data", `{"read_only":true}`},
		{"update", "7", "--trip", "42", "--is-owner", "true"},
		{"update", "--trip", "42", "--can-edit", "false"},
		{"invite", "--trip", "42"},
		{"invite", "--trip", "42", "--email", "guest@example.com", "--can-edit", "false"},
		{"delete", "7"},
		{"delete", "7", "8", "--trip", "42"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			a, cleanup := testAPIApp(t, func(w http.ResponseWriter, r *http.Request) { t.Error("unexpected API request") })
			defer cleanup()
			if err := a.collaborators(context.Background(), args); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestCollaboratorPermissionDenied(t *testing.T) {
	a, cleanup := testAPIApp(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"Permission denied"}`))
	})
	defer cleanup()
	if err := a.collaborators(context.Background(), []string{"delete", "7", "--trip", "42"}); err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("expected API error, got %v", err)
	}
	if a.stdout.(*bytes.Buffer).Len() != 0 {
		t.Fatal("failed removal reported success")
	}
}

func TestCollaboratorsListCombinesPages(t *testing.T) {
	calls := 0
	a, cleanup := testAPIApp(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("page") == "2" {
			_, _ = w.Write([]byte(`{"count":2,"next":null,"results":[{"id":8,"joined":false}]}`))
		} else {
			_, _ = w.Write([]byte(`{"count":2,"next":"/v1/trip/42/collaborators?page=2","results":[{"id":7,"joined":true}]}`))
		}
	})
	defer cleanup()
	if err := a.collaborators(context.Background(), []string{"list", "--trip", "42"}); err != nil {
		t.Fatal(err)
	}
	var envelope map[string]any
	if err := json.Unmarshal(a.stdout.(*bytes.Buffer).Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if calls != 2 || len(results(envelope["data"])) != 2 {
		t.Fatal("missing collaborator pages")
	}
}
