package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tripsyapp/cli/internal/api"
	"github.com/tripsyapp/cli/internal/output"
)

func TestCommandsJSON(t *testing.T) {
	t.Setenv("TRIPSY_AUTH_BACKEND", "file")
	var stdout, stderr bytes.Buffer
	code := Run([]string{"commands", "--json", "--config-dir", t.TempDir()}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code = %d, stderr = %s", code, stderr.String())
	}

	var envelope map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope["ok"] != true {
		t.Fatalf("ok = %v, want true", envelope["ok"])
	}
	if _, ok := envelope["data"].([]any); !ok {
		t.Fatalf("data = %T, want []any", envelope["data"])
	}
}

func TestRequestRequiresMethodAndPath(t *testing.T) {
	t.Setenv("TRIPSY_AUTH_BACKEND", "file")
	var stdout, stderr bytes.Buffer
	code := Run([]string{"request", "GET", "--config-dir", t.TempDir()}, nil, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
	if !bytes.Contains(stderr.Bytes(), []byte("request requires METHOD and PATH")) {
		t.Fatalf("stderr = %s", stderr.String())
	}
}

func TestFormatFullObjectShowsDocumentedAndExtraFields(t *testing.T) {
	got := formatFullObject("Activity", map[string]any{
		"id":            202,
		"name":          "Colosseum Tour",
		"activity_type": "sightseeing",
		"notes":         "Bring tickets",
		"address":       "Piazza del Colosseo",
		"latitude":      41.8902,
		"longitude":     12.4922,
		"period":        nil,
		"documents":     []any{},
		"custom_field":  "kept",
	}, activityDetailFields...)

	for _, want := range []string{
		"Activity",
		"id: 202",
		"name: Colosseum Tour",
		"activity_type: sightseeing",
		"notes: Bring tickets",
		"address: Piazza del Colosseo",
		"latitude: 41.8902",
		"longitude: 12.4922",
		"period: null",
		"documents:",
		"custom_field: kept",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("formatFullObject missing %q in:\n%s", want, got)
		}
	}
}

func TestTripsCommandsExcludeDocumentsAndEmails(t *testing.T) {
	for _, tt := range []struct {
		name   string
		args   []string
		method string
		path   string
	}{
		{name: "list", args: []string{"list"}, method: http.MethodGet, path: "/v1/trips"},
		{name: "show", args: []string{"show", "42"}, method: http.MethodGet, path: "/v1/trips/42"},
		{name: "create", args: []string{"create", "--name", "Copenhagen"}, method: http.MethodPost, path: "/v1/trips"},
		{name: "update", args: []string{"update", "42", "--name", "Copenhagen"}, method: http.MethodPatch, path: "/v1/trips/42"},
		{name: "delete", args: []string{"delete", "42"}, method: http.MethodDelete, path: "/v1/trips/42"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			a, cleanup := testAPIApp(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != tt.method {
					t.Errorf("method = %s, want %s", r.Method, tt.method)
				}
				if r.URL.Path != tt.path {
					t.Errorf("path = %s, want %s", r.URL.Path, tt.path)
				}
				if got := r.URL.Query().Get("fields!"); got != "documents,emails" {
					t.Errorf("fields! = %q, want documents,emails", got)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"id":42,"name":"Copenhagen","results":[]}`))
			})
			defer cleanup()

			if err := a.trips(context.Background(), tt.args); err != nil {
				t.Fatalf("trips(%v) failed: %v", tt.args, err)
			}
		})
	}
}

func TestTripSubresourceCommandsExcludeDocumentsAndEmails(t *testing.T) {
	for _, spec := range []resourceSpec{activityResource, hostingResource, transportationResource} {
		t.Run(spec.Plural, func(t *testing.T) {
			for _, tt := range []struct {
				name   string
				args   []string
				method string
				path   string
			}{
				{name: "list", args: []string{"list", "--trip", "42", "--fields-exclude", "notes"}, method: http.MethodGet, path: spec.listPath("42")},
				{name: "show", args: []string{"show", "--trip", "42", "9"}, method: http.MethodGet, path: spec.detailPath("42", "9")},
				{name: "create", args: []string{"create", "--trip", "42", "--name", "Reservation"}, method: http.MethodPost, path: spec.listPath("42")},
				{name: "update", args: []string{"update", "--trip", "42", "9", "--name", "Reservation"}, method: http.MethodPatch, path: spec.detailPath("42", "9")},
				{name: "delete", args: []string{"delete", "--trip", "42", "9"}, method: http.MethodDelete, path: spec.detailPath("42", "9")},
			} {
				t.Run(tt.name, func(t *testing.T) {
					a, cleanup := testAPIApp(t, func(w http.ResponseWriter, r *http.Request) {
						if r.Method != tt.method {
							t.Errorf("method = %s, want %s", r.Method, tt.method)
						}
						if r.URL.Path != tt.path {
							t.Errorf("path = %s, want %s", r.URL.Path, tt.path)
						}
						wantFieldsExclude := "documents,emails"
						if tt.name == "list" {
							wantFieldsExclude = "documents,emails,notes"
						}
						if got := r.URL.Query().Get("fields!"); got != wantFieldsExclude {
							t.Errorf("fields! = %q, want %s", got, wantFieldsExclude)
						}
						w.Header().Set("Content-Type", "application/json")
						_, _ = w.Write([]byte(`{"id":9,"name":"Reservation","results":[]}`))
					})
					defer cleanup()

					if err := a.resource(context.Background(), spec, tt.args); err != nil {
						t.Fatalf("%s(%v) failed: %v", spec.Plural, tt.args, err)
					}
				})
			}
		})
	}
}

func TestExpensesDoNotGetTripDataFieldExclusions(t *testing.T) {
	a, cleanup := testAPIApp(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("fields!"); got != "" {
			t.Errorf("fields! = %q, want empty", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":9,"title":"Lunch"}`))
	})
	defer cleanup()

	if err := a.resource(context.Background(), expenseResource, []string{"show", "--trip", "42", "9"}); err != nil {
		t.Fatalf("expenses show failed: %v", err)
	}
}

func (spec resourceSpec) listPath(tripID string) string {
	return strings.ReplaceAll(spec.ListPath, "%s", tripID)
}

func (spec resourceSpec) detailPath(tripID, id string) string {
	path := strings.Replace(spec.DetailPath, "%s", tripID, 1)
	return strings.Replace(path, "%s", id, 1)
}

func testAPIApp(t *testing.T, handler http.HandlerFunc) (*app, func()) {
	t.Helper()
	server := httptest.NewServer(handler)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	return &app{
		stdout: stdout,
		stderr: stderr,
		client: api.NewClient(server.URL, "test-token"),
		out:    output.Options{JSON: true},
	}, server.Close
}
