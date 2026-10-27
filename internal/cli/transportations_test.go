package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tripsyapp/cli/internal/api"
)

func TestTransportationCommandsRejectInvalidTypes(t *testing.T) {
	t.Setenv("TRIPSY_AUTH_BACKEND", "file")
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("invalid transportation must not reach the API")
	}))
	defer server.Close()
	for _, args := range [][]string{
		{"transportations", "create", "--trip", "42", "--transportation-type", "car_rental"},
		{"transportations", "update", "7", "--trip", "42", "--transportation-type", "zzz_invalid"},
		{"transportation", "patch", "7", "--trip", "42", "--set", "transportation_type=car_rental"},
		{"transportations", "new", "--trip", "42", "--data", `{"transportation_type":null}`},
		{"transportations", "update", "7", "--trip", "42", "--data", `{"transportation_type":123}`},
		{"transportations", "create", "--trip", "42", "--data", `{"transportation_type":""}`},
		{"transportations", "list", "--trip", "42", "--transportation-type", "car_rental"},
		{"request", "POST", "/v1/trip/42/transportations", "--data", `{"transportation_type":"car_rental"}`},
		{"request", "PATCH", "/v1/trip/42/transportation/7", "--data", `{"transportation_type":"zzz_invalid"}`},
	} {
		t.Run(strings.Join(args[:2], " ")+strings.Join(args[2:], " "), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			command := append([]string{"--token", "test-token", "--config-dir", t.TempDir(), "--api-base", server.URL}, args...)
			if code := Run(command, nil, &stdout, &stderr); code == 0 {
				t.Fatal("invalid type should fail")
			}
			if !strings.Contains(stderr.String(), "transportation_type") || !strings.Contains(stderr.String(), "allowed values:") {
				t.Fatalf("missing validation guidance: %s", &stderr)
			}
		})
	}
}

func TestTransportationCommandsAcceptSupportedTypes(t *testing.T) {
	t.Setenv("TRIPSY_AUTH_BACKEND", "file")
	var want string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var fields map[string]any
		if err := json.NewDecoder(r.Body).Decode(&fields); err != nil {
			t.Error(err)
		}
		if fields["transportation_type"] != want {
			t.Errorf("type = %v, want %s", fields["transportation_type"], want)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":7}`))
	}))
	defer server.Close()
	for _, slug := range api.TransportationTypes() {
		want = slug
		for _, args := range [][]string{
			{"create", "--trip", "42", "--transportation-type", slug},
			{"update", "7", "--trip", "42", "--set", "transportation_type=" + slug},
		} {
			var stdout, stderr bytes.Buffer
			command := append([]string{"--token", "test-token", "--config-dir", t.TempDir(), "--api-base", server.URL, "transportations"}, args...)
			if code := Run(command, nil, &stdout, &stderr); code != 0 {
				t.Fatalf("%v: code = %d, error = %s", args, code, &stderr)
			}
		}
	}
}

func TestTransportationHelpListsSupportedTypes(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"transportations", "--help"}, nil, &stdout, &stderr); code != 0 {
		t.Fatalf("help failed: %s", &stderr)
	}
	for _, want := range append(api.TransportationTypes(), "car = Car Rental", "transfer = Transfer") {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("help missing %s: %s", want, &stdout)
		}
	}
}
