package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
	data, _ := json.Marshal(envelope["data"])
	text := string(data)
	for _, want := range []string{
		"cover_image_url",
		"https://images.unsplash.com/photo-",
		"activity-type tour",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("command catalog missing %q in %s", want, text)
		}
	}
	if strings.Contains(text, "activity-type sightseeing") {
		t.Fatalf("command catalog should not recommend unsupported sightseeing category: %s", text)
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

func TestVersionDoesNotReadCredentials(t *testing.T) {
	t.Setenv("TRIPSY_AUTH_BACKEND", "file")
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "credentials.json"), []byte("{not-json"), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"--config-dir", dir, "--version"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "tripsy version") {
		t.Fatalf("stdout = %q, want version output", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %s, want empty", stderr.String())
	}
}

func TestBuildPayloadRejectsNullDataObject(t *testing.T) {
	fs, err := parseFlags([]string{"--data", "null"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := buildPayload(fs, []string{"name"}); err == nil || !strings.Contains(err.Error(), "--data must be a JSON object") {
		t.Fatalf("buildPayload error = %v, want JSON object error", err)
	}
}

func TestRawRequestRejectsExternalURL(t *testing.T) {
	var called bool
	a, cleanup := testAPIApp(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
		t.Errorf("API should not be called for an external request URL")
	})
	defer cleanup()

	err := a.rawRequest(context.Background(), []string{"GET", "https://example.com/v1/me"})
	if err == nil || !strings.Contains(err.Error(), "path must be a Tripsy API path") {
		t.Fatalf("rawRequest error = %v, want path validation error", err)
	}
	if called {
		t.Fatal("API was called, want validation failure before request")
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

func TestTripsCommandsUseV2ForReadsAndV1ForWrites(t *testing.T) {
	for _, tt := range []struct {
		name          string
		args          []string
		method        string
		path          string
		fieldsExclude string
	}{
		{name: "list", args: []string{"list"}, method: http.MethodGet, path: "/v2/trips/"},
		{name: "show", args: []string{"show", "42"}, method: http.MethodGet, path: "/v2/trips/42/"},
		{name: "create", args: []string{"create", "--name", "Copenhagen"}, method: http.MethodPost, path: "/v1/trips", fieldsExclude: "documents,emails"},
		{name: "update", args: []string{"update", "42", "--name", "Copenhagen"}, method: http.MethodPatch, path: "/v1/trips/42", fieldsExclude: "documents,emails"},
		{name: "delete", args: []string{"delete", "42"}, method: http.MethodDelete, path: "/v1/trips/42", fieldsExclude: "documents,emails"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			a, cleanup := testAPIApp(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/v1/me" {
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(`{"id":42}`))
					return
				}
				if r.Method != tt.method {
					t.Errorf("method = %s, want %s", r.Method, tt.method)
				}
				if r.URL.Path != tt.path {
					t.Errorf("path = %s, want %s", r.URL.Path, tt.path)
				}
				if got := r.URL.Query().Get("fields!"); got != tt.fieldsExclude {
					t.Errorf("fields! = %q, want %q", got, tt.fieldsExclude)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"id":42,"name":"Copenhagen","results":[{"id":42,"owner":42,"guests":[{"id":42,"permissions":{"is_travelling":true}}]}]}`))
			})
			defer cleanup()

			if err := a.trips(context.Background(), tt.args); err != nil {
				t.Fatalf("trips(%v) failed: %v", tt.args, err)
			}
		})
	}
}

func TestTripsShowEscapesIDPathSegment(t *testing.T) {
	a, cleanup := testAPIApp(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.RequestURI; !strings.HasPrefix(got, "/v2/trips/..%2Fme/") {
			t.Errorf("request URI = %q, want escaped trip id segment", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"../me","name":"Escaped"}`))
	})
	defer cleanup()

	if err := a.trips(context.Background(), []string{"show", "../me"}); err != nil {
		t.Fatalf("trips show failed: %v", err)
	}
}

func TestTripsListCombinesV2PaginatedResults(t *testing.T) {
	var paths []string
	a, cleanup := testAPIApp(t, func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.RequestURI())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.RequestURI() {
		case "/v1/me":
			_, _ = w.Write([]byte(`{"id":7}`))
		case "/v2/trips/?updatedSince=2026-05-01T00%3A00%3A00Z":
			_, _ = w.Write([]byte(`{"count":2,"next":"/v2/trips/?page=2","previous":null,"results":[{"id":1,"name":"One","owner":7,"guests":[{"id":7,"permissions":{"is_travelling":true}}]}]}`))
		case "/v2/trips/?page=2":
			_, _ = w.Write([]byte(`{"count":2,"next":null,"previous":"/v2/trips/","results":[{"id":2,"name":"Two","owner":7,"guests":[{"id":7,"permissions":{"is_travelling":true}}]}]}`))
		default:
			t.Fatalf("unexpected request URI: %s", r.URL.RequestURI())
		}
	})
	defer cleanup()

	if err := a.trips(context.Background(), []string{"list", "--updated-since", "2026-05-01T00:00:00Z"}); err != nil {
		t.Fatalf("trips list failed: %v", err)
	}

	stdout := a.stdout.(*bytes.Buffer)
	var envelope map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	data := envelope["data"].(map[string]any)
	items := data["results"].([]any)
	if len(items) != 2 {
		t.Fatalf("results len = %d, want 2", len(items))
	}
	if data["next"] != nil {
		t.Fatalf("next = %v, want nil after aggregation", data["next"])
	}
	if got, want := strings.Join(paths, ","), "/v2/trips/?updatedSince=2026-05-01T00%3A00%3A00Z,/v2/trips/?page=2,/v1/me"; got != want {
		t.Fatalf("paths = %s, want %s", got, want)
	}
}

func TestTripsListFiltersTravellingTrips(t *testing.T) {
	a, cleanup := testAPIApp(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.RequestURI() {
		case "/v1/me":
			_, _ = w.Write([]byte(`{"id":7}`))
		case "/v2/trips/":
			_, _ = w.Write([]byte(`{"count":3,"next":null,"previous":null,"results":[{"id":1,"name":"Owner","owner":7,"guests":[{"id":7,"permissions":{"is_travelling":true}}]},{"id":2,"name":"Following","owner":8,"guests":[{"id":7,"permissions":{"is_travelling":false}}]},{"id":3,"name":"Travelling guest","owner":8,"guests":[{"id":7,"permissions":{"is_travelling":true}}]}]}`))
		default:
			t.Fatalf("unexpected request URI: %s", r.URL.RequestURI())
		}
	})
	defer cleanup()

	if err := a.trips(context.Background(), []string{"list"}); err != nil {
		t.Fatalf("trips list failed: %v", err)
	}

	stdout := a.stdout.(*bytes.Buffer)
	var envelope map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	data := envelope["data"].(map[string]any)
	items := data["results"].([]any)
	if len(items) != 2 {
		t.Fatalf("results len = %d, want 2", len(items))
	}
	if data["count"] != float64(2) {
		t.Fatalf("count = %v, want 2", data["count"])
	}
}

func TestTripsFollowingFiltersNonTravellingTrips(t *testing.T) {
	a, cleanup := testAPIApp(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.RequestURI() {
		case "/v1/me":
			_, _ = w.Write([]byte(`{"id":7}`))
		case "/v2/trips/":
			_, _ = w.Write([]byte(`{"count":2,"next":null,"previous":null,"results":[{"id":1,"name":"Following","owner":8,"guests":[{"id":7,"permissions":{"is_travelling":false}}]},{"id":2,"name":"Travelling","owner":8,"guests":[{"id":7,"permissions":{"is_travelling":true}}]}]}`))
		default:
			t.Fatalf("unexpected request URI: %s", r.URL.RequestURI())
		}
	})
	defer cleanup()

	if err := a.trips(context.Background(), []string{"following"}); err != nil {
		t.Fatalf("trips following failed: %v", err)
	}

	stdout := a.stdout.(*bytes.Buffer)
	var envelope map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	data := envelope["data"].(map[string]any)
	items := data["results"].([]any)
	if len(items) != 1 {
		t.Fatalf("results len = %d, want 1", len(items))
	}
	if got := items[0].(map[string]any)["name"]; got != "Following" {
		t.Fatalf("name = %v, want Following", got)
	}
}

func TestDocumentAttachPathEscapesSegments(t *testing.T) {
	got := documentAttachPath("trip/42", "activity", "activity/9")
	want := "/v1/trip/trip%2F42/activity/activity%2F9/documents"
	if got != want {
		t.Fatalf("documentAttachPath = %q, want %q", got, want)
	}
}

func TestTripsCreateRejectsShortUnsplashPhotoID(t *testing.T) {
	var called bool
	a, cleanup := testAPIApp(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
		t.Errorf("API should not be called with an invalid cover_image_url")
	})
	defer cleanup()

	err := a.trips(context.Background(), []string{
		"create",
		"--name", "Copenhagen",
		"--cover-image-url", "https://images.unsplash.com/photo-nWdsya5_Yms?ixlib=rb-4.1.0",
	})
	if err == nil {
		t.Fatal("trips create error = nil, want invalid cover_image_url error")
	}
	if !strings.Contains(err.Error(), "numeric Unsplash photo asset path") || !strings.Contains(err.Error(), "photo-nWdsya5_Yms") {
		t.Fatalf("error = %q, want specific Unsplash URL guidance", err.Error())
	}
	if called {
		t.Fatal("API was called, want validation failure before request")
	}
}

func TestTripSubresourceCommandsUseV2ForReadsAndV1ForWrites(t *testing.T) {
	for _, spec := range []resourceSpec{activityResource, hostingResource, transportationResource} {
		t.Run(spec.Plural, func(t *testing.T) {
			for _, tt := range []struct {
				name          string
				args          []string
				method        string
				path          string
				fieldsExclude string
			}{
				{name: "list", args: []string{"list", "--trip", "42", "--fields-exclude", "notes"}, method: http.MethodGet, path: spec.formatReadListPath("42"), fieldsExclude: "notes"},
				{name: "show", args: []string{"show", "--trip", "42", "9"}, method: http.MethodGet, path: spec.formatReadDetailPath("42", "9")},
				{name: "create", args: []string{"create", "--trip", "42", "--name", "Reservation"}, method: http.MethodPost, path: spec.listPath("42"), fieldsExclude: "documents,emails"},
				{name: "update", args: []string{"update", "--trip", "42", "9", "--name", "Reservation"}, method: http.MethodPatch, path: spec.detailPath("42", "9"), fieldsExclude: "documents,emails"},
				{name: "delete", args: []string{"delete", "--trip", "42", "9"}, method: http.MethodDelete, path: spec.detailPath("42", "9"), fieldsExclude: "documents,emails"},
			} {
				t.Run(tt.name, func(t *testing.T) {
					a, cleanup := testAPIApp(t, func(w http.ResponseWriter, r *http.Request) {
						if r.Method != tt.method {
							t.Errorf("method = %s, want %s", r.Method, tt.method)
						}
						if r.URL.Path != tt.path {
							t.Errorf("path = %s, want %s", r.URL.Path, tt.path)
						}
						if got := r.URL.Query().Get("fields!"); got != tt.fieldsExclude {
							t.Errorf("fields! = %q, want %q", got, tt.fieldsExclude)
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

func TestTripSubresourceCreatePreservesDatetimeFlagValues(t *testing.T) {
	for _, tt := range []struct {
		name string
		spec resourceSpec
		args []string
		want map[string]any
	}{
		{
			name: "hosting",
			spec: hostingResource,
			args: []string{
				"create",
				"--trip", "42",
				"--name", "TEST",
				"--starts-at", "2026-12-01T10:00:00Z",
				"--ends-at", "2026-12-02T10:00:00Z",
				"--timezone", "Europe/Rome",
			},
			want: map[string]any{
				"name":      "TEST",
				"starts_at": "2026-12-01T10:00:00Z",
				"ends_at":   "2026-12-02T10:00:00Z",
				"timezone":  "Europe/Rome",
			},
		},
		{
			name: "activity",
			spec: activityResource,
			args: []string{
				"create",
				"--trip", "42",
				"--name", "Tour",
				"--starts-at", "2026-12-01T10:00:00+02:00",
				"--ends-at", "2026-12-01T12:00:00+02:00",
				"--timezone", "Europe/Rome",
			},
			want: map[string]any{
				"name":      "Tour",
				"starts_at": "2026-12-01T10:00:00+02:00",
				"ends_at":   "2026-12-01T12:00:00+02:00",
				"timezone":  "Europe/Rome",
			},
		},
		{
			name: "transportation",
			spec: transportationResource,
			args: []string{
				"create",
				"--trip", "42",
				"--name", "Flight",
				"--departure-at", "2026-12-01T10:00:00.123456Z",
				"--arrival-at", "2026-12-01T12:00:00.123456Z",
				"--departure-timezone", "Europe/Rome",
				"--arrival-timezone", "Europe/Rome",
			},
			want: map[string]any{
				"name":               "Flight",
				"departure_at":       "2026-12-01T10:00:00.123456Z",
				"arrival_at":         "2026-12-01T12:00:00.123456Z",
				"departure_timezone": "Europe/Rome",
				"arrival_timezone":   "Europe/Rome",
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			a, cleanup := testAPIApp(t, func(w http.ResponseWriter, r *http.Request) {
				if got := r.Header.Get("Content-Type"); got != "application/json" {
					t.Errorf("Content-Type = %q, want application/json", got)
				}
				raw, err := io.ReadAll(r.Body)
				if err != nil {
					t.Fatal(err)
				}
				var payload map[string]any
				if err := json.Unmarshal(raw, &payload); err != nil {
					t.Fatalf("request body is not JSON: %s", raw)
				}
				for key, want := range tt.want {
					if payload[key] != want {
						t.Errorf("%s = %v, want %v", key, payload[key], want)
					}
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"id":9,"name":"ok"}`))
			})
			defer cleanup()

			if err := a.resource(context.Background(), tt.spec, tt.args); err != nil {
				t.Fatalf("%s create failed: %v", tt.spec.Plural, err)
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
