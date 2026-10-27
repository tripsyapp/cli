package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

func TestTransportationTypesMatchIOSModel(t *testing.T) {
	want := []string{"airplane", "bike", "bus", "car", "roadtrip", "cruise", "ferry", "motorcycle", "subway", "train", "transfer", "walk"}
	if got := TransportationTypes(); !reflect.DeepEqual(got, want) {
		t.Fatalf("TransportationTypes() = %v, want %v", got, want)
	}
}

func TestTransportationWritesAcceptSupportedTypes(t *testing.T) {
	client := NewClient("https://api.test", "test-token")
	var want string
	client.HTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var fields map[string]any
		if err := json.NewDecoder(r.Body).Decode(&fields); err != nil {
			t.Fatal(err)
		}
		if fields["transportation_type"] != want {
			t.Fatalf("transportation_type = %v, want %s", fields["transportation_type"], want)
		}
		return jsonResponse(http.StatusOK, fields), nil
	})}
	for _, slug := range TransportationTypes() {
		want = slug
		for _, method := range []string{"POST", "PATCH", "PUT"} {
			if _, err := client.Request(context.Background(), method, "/v1/trip/42/transportation/7", nil, map[string]any{"transportation_type": slug}); err != nil {
				t.Fatalf("%s %s: %v", method, slug, err)
			}
		}
	}
}

func TestTransportationWritesRejectInvalidTypesBeforeHTTP(t *testing.T) {
	client := NewClient("https://api.test", "test-token")
	client.HTTPClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("invalid transportation must not reach the API")
		return nil, nil
	})}
	for _, value := range []any{"car_rental", "zzz_invalid", "museum", "golf-clubs", "", "Car", " subway ", nil, true, 123, []any{"car"}, map[string]any{"slug": "car"}} {
		fields := map[string]any{"transportation_type": value}
		encoded, err := json.Marshal(fields)
		if err != nil {
			t.Fatal(err)
		}
		for _, body := range []any{fields, string(encoded), encoded} {
			for _, test := range []struct{ method, path string }{
				{"POST", "/v1/trip/42/transportations"},
				{"patch", "/v1/trip/42/transportation/7/"},
				{"PUT", "/v2/trip/42/transportation/7"},
				{"POST", "https://api.test/v1/trip/a%2Fb/transportations/?extra=1"},
			} {
				_, err := client.Request(context.Background(), test.method, test.path, nil, body)
				if err == nil || !strings.Contains(err.Error(), "transportation_type") || !strings.Contains(err.Error(), "allowed values:") {
					t.Fatalf("%s %s with %s: error = %v", test.method, test.path, encoded, err)
				}
				for _, slug := range TransportationTypes() {
					if !strings.Contains(err.Error(), slug) {
						t.Fatalf("error missing allowed slug %s: %v", slug, err)
					}
				}
			}
		}
	}
}

func TestTransportationTypeValidationPreservesOtherFieldsAndResources(t *testing.T) {
	client := NewClient("https://api.test", "test-token")
	calls := 0
	client.HTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		return jsonResponse(http.StatusOK, map[string]any{"ok": true}), nil
	})}
	for _, test := range []struct {
		method, path string
		body         any
	}{
		{"PATCH", "/v1/trip/42/transportation/7", map[string]any{"notes": "Updated"}},
		{"POST", "/v1/trip/42/transportations", map[string]any{"name": "Untyped"}},
		{"GET", "/v2/trip/42/transportation/7/", nil},
		{"DELETE", "/v1/trip/42/transportation/7", nil},
		{"POST", "/v1/trip/42/activities", map[string]any{"activity_type": "golf-clubs"}},
		{"PATCH", "/v1/trip/42/transportation/7/documents/9", map[string]any{"transportation_type": "custom"}},
	} {
		if _, err := client.Request(context.Background(), test.method, test.path, nil, test.body); err != nil {
			t.Fatalf("%s %s: %v", test.method, test.path, err)
		}
	}
	if calls != 6 {
		t.Fatalf("calls = %d, want 6", calls)
	}
}

func TestTransportationListFilters(t *testing.T) {
	client := NewClient("https://api.test", "test-token")
	var got string
	client.HTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		got = r.URL.Query().Get("transportationType")
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"results":[]}`))}, nil
	})}
	for _, slug := range TransportationTypes() {
		got = ""
		_, err := client.Request(context.Background(), "GET", "/v2/trip/42/transportations/", url.Values{"transportationType": {slug}}, nil)
		if err != nil || got != slug {
			t.Fatalf("filter %s: got %q, error = %v", slug, got, err)
		}
	}
	for _, target := range []string{
		"/v1/trip/42/transportations?transportationType=car_rental",
		"/v2/trip/42/transportations/?transportationType=",
		"/v2/trip/42/transportations?transportationType=car&transportationType=invalid",
	} {
		got = ""
		if _, err := client.Request(context.Background(), "GET", target, nil, nil); err == nil || got != "" {
			t.Fatalf("invalid filter %s: got %q, error = %v", target, got, err)
		}
	}
}
