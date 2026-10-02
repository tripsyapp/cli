package mcpserver

import (
	"net/http"
	"strings"
	"testing"
)

func TestAttachedEmailTools(t *testing.T) {
	for _, parent := range []string{"trip", "activity", "hosting", "transportation"} {
		t.Run(parent, func(t *testing.T) {
			base := "/v2/trip/12"
			input := map[string]any{"trip_id": "12", "parent_type": parent}
			if parent != "trip" {
				base += "/" + parent + "/34"
				input["parent_id"] = "34"
			}
			calls := 0
			session, cleanup := connectTestSessionOptions(t, "test-token", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Header.Get("Authorization") != "Token test-token" || r.Method != "GET" {
					t.Errorf("unexpected authentication or method")
				}
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == base+"/emails/56" {
					_, _ = w.Write([]byte(`{"id":56,"content":"<p>Original reservation</p>","attachments":[{"filename":"ticket.pdf","payload":"dGlja2V0"}]}`))
				} else if r.URL.Path == base+"/emails" {
					if r.URL.Query().Get("page") == "2" {
						_, _ = w.Write([]byte(`{"results":[{"id":57}],"next":null}`))
					} else {
						_, _ = w.Write([]byte(`{"results":[{"id":56}],"next":"` + base + `/emails?page=2"}`))
					}
				} else {
					t.Errorf("unexpected path %s", r.URL)
					http.NotFound(w, r)
				}
			}), Options{DisableRawRequest: true})
			defer cleanup()
			listed, err := session.ListTools(testContext(t), nil)
			if err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"tripsy_emails_list", "tripsy_emails_show"} {
				tool := findTool(listed.Tools, name)
				if tool == nil || !tool.Annotations.ReadOnlyHint {
					t.Fatalf("missing read tool %s", name)
				}
			}
			res := callTool(t, session, "tripsy_emails_list", input)
			if res.IsError {
				t.Fatal(toolText(res))
			}
			data := structuredMap(t, res)["data"].(map[string]any)
			if len(data["results"].([]any)) != 2 {
				t.Fatal(data)
			}
			input["id"] = "56"
			res = callTool(t, session, "tripsy_emails_show", input)
			if res.IsError || !strings.Contains(toolText(res), "dGlja2V0") {
				t.Fatal(toolText(res))
			}
			if calls != 3 {
				t.Fatalf("calls=%d", calls)
			}
		})
	}
}

func TestAttachmentInputsAndPermissionErrors(t *testing.T) {
	calls := 0
	session, cleanup := connectTestSession(t, "test-token", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"detail":"Document access denied"}`))
	}))
	defer cleanup()
	for _, input := range []map[string]any{{"trip_id": ""}, {"trip_id": "1", "parent_type": "unknown"}, {"trip_id": "1", "parent_type": "activity"}, {"trip_id": "1", "parent_id": "2"}} {
		if res := callTool(t, session, "tripsy_emails_list", input); !res.IsError {
			t.Fatal("invalid parent accepted")
		}
	}
	if calls != 0 {
		t.Fatal("invalid inputs called API")
	}
	res := callTool(t, session, "tripsy_emails_show", map[string]any{"trip_id": "1", "id": "2"})
	if !res.IsError || !strings.Contains(toolText(res), "403") {
		t.Fatal(toolText(res))
	}
}
