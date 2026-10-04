package mcpserver

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func TestInboxToolsRegisteredWithoutRawRequests(t *testing.T) {
	session, cleanup := connectTestSessionOptions(t, "test-token", http.NotFoundHandler(), Options{DisableRawRequest: true})
	defer cleanup()
	listed, err := session.ListTools(testContext(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	if findTool(listed.Tools, "tripsy_raw_request") != nil {
		t.Fatal("raw requests should be disabled")
	}
	for _, action := range []string{"list", "show", "update", "delete"} {
		tool := findTool(listed.Tools, "tripsy_inbox_"+action)
		if tool == nil {
			t.Fatalf("missing inbox %s tool", action)
		}
		annotations := tool.Annotations
		if annotations == nil || annotations.ReadOnlyHint != (action == "list" || action == "show") || annotations.DestructiveHint == nil || *annotations.DestructiveHint != (action == "delete" || action == "update") || annotations.OpenWorldHint == nil || *annotations.OpenWorldHint != (action == "update") {
			t.Fatalf("incorrect annotations for %s: %#v", tool.Name, annotations)
		}
		if action == "update" && !annotations.IdempotentHint {
			t.Fatal("inbox update should be idempotent")
		}
	}
	schema := toolSchemaString(t, findTool(listed.Tools, "tripsy_inbox_update"))
	for _, field := range []string{"id", "subject", "trip_id", "activity_id", "hosting_id", "transportation_id"} {
		if !strings.Contains(schema, `"`+field+`"`) {
			t.Fatalf("update schema missing %s: %s", field, schema)
		}
	}
}

func TestInboxListCombinesPages(t *testing.T) {
	calls := 0
	session, cleanup := connectTestSession(t, "test-token", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "GET" || r.URL.Path != "/v1/automation/emails" || r.Header.Get("Authorization") != "Token test-token" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("page") == "2" {
			_, _ = w.Write([]byte(`{"count":2,"next":null,"results":[{"id":56,"subject":"Hotel confirmation","attachments_count":1}]}`))
		} else {
			_, _ = w.Write([]byte(`{"count":2,"next":"/v1/automation/emails?page=2","results":[{"id":55,"subject":"Bus confirmation","body_preview":"Your ticket","attachments_count":2}]}`))
		}
	}))
	defer cleanup()
	res := callTool(t, session, "tripsy_inbox_list", map[string]any{})
	if res.IsError || calls != 2 {
		t.Fatalf("calls=%d result=%s", calls, toolText(res))
	}
	data := structuredMap(t, res)["data"].(map[string]any)
	items := data["results"].([]any)
	if len(items) != 2 || data["next"] != nil || items[0].(map[string]any)["body_preview"] != "Your ticket" || items[1].(map[string]any)["subject"] != "Hotel confirmation" {
		t.Fatalf("invalid inbox results: %#v", data)
	}
}

func TestInboxShowPreservesFullDetailsAndEscapesID(t *testing.T) {
	const detail = `{"id":"a/b","subject":"Bus confirmation","body":"Full booking email","body_html":"<p>Full booking email</p>","attachments":[{"id":9,"filename":"ticket.pdf","file_type":"application/pdf","download_url":"https://example.com/ticket.pdf"}],"extra_metadata":{"reservation":"ABC123"}}`
	session, cleanup := connectTestSession(t, "test-token", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.EscapedPath() != "/v1/automation/emails/a%2Fb" || r.URL.RawQuery != "" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(detail))
	}))
	defer cleanup()
	res := callTool(t, session, "tripsy_inbox_show", map[string]any{"id": " a/b "})
	if res.IsError {
		t.Fatal(toolText(res))
	}
	var want any
	if err := json.Unmarshal([]byte(detail), &want); err != nil {
		t.Fatal(err)
	}
	if got := structuredMap(t, res)["data"]; !reflect.DeepEqual(got, want) {
		t.Fatalf("email details changed: got %#v, want %#v", got, want)
	}
}

func TestInboxUpdateSupportedFields(t *testing.T) {
	for _, payload := range []map[string]any{
		{"subject": "Renamed booking"},
		{"subject": ""},
		{"trip_id": "42"},
		{"activity_id": "101"},
		{"hosting_id": "202"},
		{"transportation_id": "303"},
		{"subject": "Renamed bus ticket", "transportation_id": "303"},
	} {
		name, _ := json.Marshal(payload)
		t.Run(string(name), func(t *testing.T) {
			calls := 0
			session, cleanup := connectTestSession(t, "test-token", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != "PATCH" || r.URL.EscapedPath() != "/v1/automation/emails/a%2Fb" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL)
				}
				var got map[string]any
				if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
					t.Error(err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				if !reflect.DeepEqual(got, payload) {
					t.Errorf("payload = %#v, want %#v", got, payload)
				}
				w.WriteHeader(http.StatusOK)
			}))
			defer cleanup()
			args := map[string]any{"id": "a/b"}
			for key, value := range payload {
				args[key] = value
			}
			res := callTool(t, session, "tripsy_inbox_update", args)
			if res.IsError || calls != 1 {
				t.Fatalf("calls=%d result=%s", calls, toolText(res))
			}
			if got := structuredMap(t, res)["status_code"]; got != float64(http.StatusOK) {
				t.Fatalf("empty-body success status = %v", got)
			}
		})
	}
}

func TestInboxRejectsInvalidInputsWithoutAPIRequest(t *testing.T) {
	session, cleanup := connectTestSession(t, "test-token", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("invalid input should not reach the API")
	}))
	defer cleanup()
	for _, action := range []string{"show", "update", "delete"} {
		res := callTool(t, session, "tripsy_inbox_"+action, map[string]any{"id": " "})
		if !res.IsError || !strings.Contains(toolText(res), "id is required") {
			t.Fatalf("invalid %s id: %s", action, toolText(res))
		}
	}
	if res := callTool(t, session, "tripsy_inbox_update", map[string]any{"id": "55"}); !res.IsError || !strings.Contains(toolText(res), "subject or one move target") {
		t.Fatalf("empty update: %s", toolText(res))
	}
	fields := []string{"trip_id", "activity_id", "hosting_id", "transportation_id"}
	for i, field := range fields {
		res := callTool(t, session, "tripsy_inbox_update", map[string]any{"id": "55", field: " "})
		if !res.IsError || !strings.Contains(toolText(res), field+" must not be empty") {
			t.Fatalf("empty %s: %s", field, toolText(res))
		}
		for _, other := range fields[i+1:] {
			res := callTool(t, session, "tripsy_inbox_update", map[string]any{"id": "55", field: "42", other: "303"})
			if !res.IsError || !strings.Contains(toolText(res), "only one move target") {
				t.Fatalf("multiple targets %s and %s: %s", field, other, toolText(res))
			}
		}
	}
}

func TestInboxToolsRequireAuthentication(t *testing.T) {
	session, cleanup := connectTestSession(t, "", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("unauthenticated input should not reach the API")
	}))
	defer cleanup()
	for _, action := range []string{"list", "show", "update", "delete"} {
		args := map[string]any{}
		if action != "list" {
			args["id"] = "55"
		}
		if action == "update" {
			args["subject"] = "Renamed booking"
		}
		if res := callTool(t, session, "tripsy_inbox_"+action, args); !res.IsError || !strings.Contains(toolText(res), "not authenticated") {
			t.Fatalf("unauthenticated %s: %s", action, toolText(res))
		}
	}
}

func TestInboxAPIErrors(t *testing.T) {
	for _, tt := range []struct {
		action string
		status int
		args   map[string]any
	}{
		{"show", http.StatusNotFound, map[string]any{"id": "55"}},
		{"update", http.StatusForbidden, map[string]any{"id": "55", "transportation_id": "303"}},
	} {
		t.Run(tt.action, func(t *testing.T) {
			session, cleanup := connectTestSession(t, "test-token", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(`{"detail":"Not allowed"}`))
			}))
			defer cleanup()
			if res := callTool(t, session, "tripsy_inbox_"+tt.action, tt.args); !res.IsError || !strings.Contains(toolText(res), "Not allowed") {
				t.Fatalf("API error was lost: %s", toolText(res))
			}
		})
	}
}

func TestInboxDelete(t *testing.T) {
	calls := 0
	session, cleanup := connectTestSession(t, "test-token", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "DELETE" || r.URL.EscapedPath() != "/v1/automation/emails/a%2Fb" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer cleanup()
	res := callTool(t, session, "tripsy_inbox_delete", map[string]any{"id": "a/b"})
	if res.IsError || calls != 1 || structuredMap(t, res)["data"].(map[string]any)["success"] != true {
		t.Fatalf("calls=%d result=%s", calls, toolText(res))
	}
}

func TestInboxBookingWorkflowWithoutRawRequests(t *testing.T) {
	created, moved := false, false
	session, cleanup := connectTestSessionOptions(t, "test-token", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case "GET /v1/automation/emails":
			if moved {
				_, _ = w.Write([]byte(`{"count":0,"next":null,"results":[]}`))
			} else {
				_, _ = w.Write([]byte(`{"count":1,"next":null,"results":[{"id":55,"subject":"Bus confirmation"}]}`))
			}
		case "GET /v1/automation/emails/55":
			_, _ = w.Write([]byte(`{"id":55,"body":"Bus to Pforzheim, reservation ABC123","attachments":[{"filename":"ticket.pdf"}]}`))
		case "POST /v1/trip/42/transportations":
			created = true
			_, _ = w.Write([]byte(`{"id":303,"provider_reservation_code":"ABC123"}`))
		case "PATCH /v1/automation/emails/55":
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Error(err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if !created || !reflect.DeepEqual(payload, map[string]any{"transportation_id": "303"}) {
				t.Errorf("move should attach only to the created transportation: %#v", payload)
			}
			moved = true
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected workflow request: %s %s", r.Method, r.URL)
			w.WriteHeader(http.StatusNotFound)
		}
	}), Options{DisableRawRequest: true})
	defer cleanup()
	for _, step := range []struct {
		tool string
		args map[string]any
	}{
		{"tripsy_inbox_list", map[string]any{}},
		{"tripsy_inbox_show", map[string]any{"id": "55"}},
		{"tripsy_transportations_create", map[string]any{"trip_id": "42", "transportation_type": "bus", "provider_reservation_code": "ABC123"}},
		{"tripsy_inbox_update", map[string]any{"id": "55", "transportation_id": "303"}},
	} {
		if res := callTool(t, session, step.tool, step.args); res.IsError {
			t.Fatalf("%s: %s", step.tool, toolText(res))
		}
	}
	res := callTool(t, session, "tripsy_inbox_list", map[string]any{})
	if res.IsError || !created || !moved {
		t.Fatalf("workflow did not complete: %s", toolText(res))
	}
	if items := structuredMap(t, res)["data"].(map[string]any)["results"].([]any); len(items) != 0 {
		t.Fatalf("moved email remained in inbox: %#v", items)
	}
}
