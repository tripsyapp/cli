package mcpserver

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/tripsyapp/cli/internal/api"
)

func TestTransportationSchemasAndGuidanceExposeSupportedTypes(t *testing.T) {
	session, cleanup := connectTestSession(t, "test-token", http.NotFoundHandler())
	defer cleanup()
	listed, err := session.ListTools(testContext(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"create", "update", "list"} {
		tool := findTool(listed.Tools, "tripsy_transportations_"+action)
		var schema struct {
			Properties map[string]struct {
				Enum       []string `json:"enum"`
				Properties map[string]struct {
					Enum []string `json:"enum"`
				} `json:"properties"`
			} `json:"properties"`
		}
		if err := json.Unmarshal([]byte(toolSchemaString(t, tool)), &schema); err != nil {
			t.Fatal(err)
		}
		got := schema.Properties["transportation_type"].Enum
		if action == "update" {
			got = schema.Properties["data"].Properties["transportation_type"].Enum
		}
		if !reflect.DeepEqual(got, api.TransportationTypes()) {
			t.Fatalf("%s enum = %v, want %v", action, got, api.TransportationTypes())
		}
		for _, want := range []string{"car = Car Rental", "subway = Subway", "transfer = Transfer"} {
			if !strings.Contains(tool.Description, want) || !strings.Contains(toolSchemaString(t, tool), want) {
				t.Fatalf("%s schema and description must include %s", action, want)
			}
		}
	}
	guidance := callTool(t, session, "tripsy_itinerary_guidance", map[string]any{})
	for _, want := range append(api.TransportationTypes(), "car = Car Rental", "subway = Subway", "transfer = Transfer") {
		if !strings.Contains(toolText(guidance), want) {
			t.Fatalf("guidance missing %s: %s", want, toolText(guidance))
		}
	}
}

func TestTransportationToolsRejectInvalidTypesBeforeHTTP(t *testing.T) {
	var calls atomic.Int32
	session, cleanup := connectTestSession(t, "test-token", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls.Add(1)
	}))
	defer cleanup()
	for _, value := range []any{"car_rental", "zzz_invalid", "museum", "golf-clubs", "", "Car", nil, true, 123, []any{"car"}} {
		for _, test := range []struct {
			name string
			args map[string]any
		}{
			{"tripsy_transportations_create", map[string]any{"trip_id": "42", "transportation_type": value}},
			{"tripsy_transportations_create", map[string]any{"trip_id": "42", "data": map[string]any{"transportation_type": value}}},
			{"tripsy_transportations_update", map[string]any{"trip_id": "42", "id": "7", "data": map[string]any{"transportation_type": value}}},
			{"tripsy_transportations_list", map[string]any{"trip_id": "42", "transportation_type": value}},
			{"tripsy_raw_request", map[string]any{"method": "POST", "path": "/v1/trip/42/transportations", "data": map[string]any{"transportation_type": value}}},
			{"tripsy_raw_request", map[string]any{"method": "PATCH", "path": "/v1/trip/42/transportation/7", "data": map[string]any{"transportation_type": value}}},
		} {
			res := callTool(t, session, test.name, test.args)
			if !res.IsError || !strings.Contains(toolText(res), "transportation_type") {
				t.Fatalf("%s with %#v: %s", test.name, test.args, toolText(res))
			}
		}
	}
	if calls.Load() != 0 {
		t.Fatalf("invalid types made %d API requests", calls.Load())
	}
}

func TestTransportationToolsAcceptSupportedTypesAndPartialUpdates(t *testing.T) {
	var calls atomic.Int32
	var want string
	session, cleanup := connectTestSession(t, "test-token", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method == "GET" {
			if got := r.URL.Query().Get("transportationType"); got != want {
				t.Errorf("filter = %s, want %s", got, want)
			}
		} else {
			var fields map[string]any
			if err := json.NewDecoder(r.Body).Decode(&fields); err != nil {
				t.Error(err)
			}
			if want != "" && fields["transportation_type"] != want {
				t.Errorf("type = %v, want %s", fields["transportation_type"], want)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":7,"results":[]}`))
	}))
	defer cleanup()
	for _, slug := range api.TransportationTypes() {
		want = slug
		for _, test := range []struct {
			name string
			args map[string]any
		}{
			{"tripsy_transportations_create", map[string]any{"trip_id": "42", "transportation_type": slug}},
			{"tripsy_transportations_create", map[string]any{"trip_id": "42", "data": map[string]any{"transportation_type": slug}}},
			{"tripsy_transportations_update", map[string]any{"trip_id": "42", "id": "7", "data": map[string]any{"transportation_type": slug, "notes": "Keep arbitrary writable fields"}}},
			{"tripsy_transportations_list", map[string]any{"trip_id": "42", "transportation_type": slug}},
		} {
			res := callTool(t, session, test.name, test.args)
			if res.IsError {
				t.Fatalf("%s %s: %s", test.name, slug, toolText(res))
			}
		}
	}
	want = "car"
	res := callTool(t, session, "tripsy_transportations_create", map[string]any{"trip_id": "42", "transportation_type": "car", "data": map[string]any{"transportation_type": "car_rental"}})
	if res.IsError {
		t.Fatalf("typed fields must still override raw data: %s", toolText(res))
	}
	want = ""
	res = callTool(t, session, "tripsy_transportations_update", map[string]any{"trip_id": "42", "id": "7", "data": map[string]any{"notes": "Keep existing type"}})
	if res.IsError || calls.Load() != int32(len(api.TransportationTypes())*4+2) {
		t.Fatalf("partial update: calls=%d result=%s", calls.Load(), toolText(res))
	}
}
