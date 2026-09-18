package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/tripsyapp/cli/internal/output"
)

func TestFavoriteGuestsList(t *testing.T) {
	for _, human := range []bool{false, true} {
		t.Run(map[bool]string{false: "json", true: "human"}[human], func(t *testing.T) {
			calls := 0
			a, cleanup := testAPIApp(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != "GET" || r.URL.Path != "/v1/guests/favorites" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL)
				}
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Query().Get("page") == "2" {
					_, _ = w.Write([]byte(`{"count":2,"next":null,"results":[{"id":92,"favorite_user":{"id":8,"name":"Pending Guest","email":"pending@example.com"},"pending":true}]}`))
				} else {
					_, _ = w.Write([]byte(`{"count":2,"next":"/v1/guests/favorites?page=2","results":[{"id":91,"favorite_user":{"id":7,"name":"Confirmed Guest","email":"confirmed@example.com"},"pending":false}]}`))
				}
			})
			defer cleanup()
			if human {
				a.out = output.Options{IsTerminal: true}
			}
			a.args = []string{"guests", "favorites"}
			if err := a.execute(context.Background()); err != nil {
				t.Fatal(err)
			}
			if calls != 2 {
				t.Fatalf("calls = %d", calls)
			}
			text := a.stdout.(*bytes.Buffer).String()
			if human {
				for _, want := range []string{"7  Confirmed Guest", "confirmed", "8  Pending Guest", "pending"} {
					if !strings.Contains(text, want) {
						t.Errorf("output missing %s: %s", want, text)
					}
				}
			} else {
				var envelope map[string]any
				if err := json.Unmarshal([]byte(text), &envelope); err != nil {
					t.Fatal(err)
				}
				items := results(envelope["data"])
				if len(items) != 2 || objectMap(items[0])["pending"] != false || objectMap(items[1])["pending"] != true {
					t.Fatalf("invalid results: %#v", items)
				}
				if objectMap(objectMap(items[0])["favorite_user"])["id"] != float64(7) {
					t.Fatal("lost user id")
				}
			}
		})
	}
}

func TestFavoriteGuestsRejectUnsupportedArguments(t *testing.T) {
	a, cleanup := testAPIApp(t, func(w http.ResponseWriter, r *http.Request) { t.Error("unexpected API request") })
	defer cleanup()
	for _, args := range [][]string{nil, {"unknown"}, {"favorites", "delete"}, {"favorites", "--trip", "42"}} {
		if err := a.guests(context.Background(), args); err == nil {
			t.Errorf("expected error for %v", args)
		}
	}
}
