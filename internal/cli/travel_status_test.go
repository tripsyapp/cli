package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestTravelStatusMovesTripBetweenLists(t *testing.T) {
	travelling := true
	a, cleanup := testAPIApp(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case "GET /v1/me":
			_, _ = w.Write([]byte(`{"id":7}`))
		case "PATCH /v1/trip/42/collaborator/7/permissions":
			var data map[string]any
			if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
				t.Error(err)
			}
			value, ok := data["is_travelling"].(bool)
			if !ok || len(data) != 1 {
				t.Errorf("expected only boolean travel flag, got %#v", data)
			}
			travelling = value
			_ = json.NewEncoder(w).Encode(map[string]any{"user_id": 7, "permissions": data})
		case "GET /v2/trips/":
			_ = json.NewEncoder(w).Encode(map[string]any{"results": []any{map[string]any{"id": 42, "name": "Shared trip", "owner": 8, "guests": []any{map[string]any{"id": 7, "permissions": map[string]any{"is_travelling": travelling}}}}}})
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL)
		}
	})
	defer cleanup()
	for _, tt := range []struct{ value, list, user string }{{"false", "following", "me"}, {"true", "list", "7"}} {
		if err := a.collaborators(context.Background(), []string{"update", tt.user, "--trip", "42", "--is-travelling", tt.value}); err != nil {
			t.Fatal(err)
		}
		a.stdout.(*bytes.Buffer).Reset()
		if err := a.trips(context.Background(), []string{tt.list}); err != nil {
			t.Fatal(err)
		}
		var envelope map[string]any
		if err := json.Unmarshal(a.stdout.(*bytes.Buffer).Bytes(), &envelope); err != nil {
			t.Fatal(err)
		}
		if len(results(envelope["data"])) != 1 {
			t.Fatalf("trip missing from %s list: %s", tt.list, a.stdout)
		}
	}
}

func TestCollaboratorUpdateMeDoesNotWriteWithoutIdentity(t *testing.T) {
	a, cleanup := testAPIApp(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/v1/me" {
			t.Error("unexpected mutation")
		}
		_, _ = w.Write([]byte(`{}`))
	})
	defer cleanup()
	if err := a.collaborators(context.Background(), []string{"update", "me", "--trip", "42", "--is-travelling", "false"}); err == nil || !strings.Contains(err.Error(), "did not include id") {
		t.Fatalf("expected missing identity error, got %v", err)
	}
}
