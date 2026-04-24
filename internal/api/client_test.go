package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestRequestSendsTokenAndParsesJSON(t *testing.T) {
	client := NewClient("https://api.test", "test-token")
	client.HTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if got, want := r.Header.Get("Authorization"), "Token test-token"; got != want {
			t.Fatalf("Authorization = %q, want %q", got, want)
		}
		if got, want := r.URL.Path, "/v1/me"; got != want {
			t.Fatalf("path = %q, want %q", got, want)
		}
		return jsonResponse(http.StatusOK, map[string]any{"id": 1, "name": "Test User"}), nil
	})}

	resp, err := client.Request(context.Background(), "GET", "/v1/me", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	data := resp.Data.(map[string]any)
	if data["name"] != "Test User" {
		t.Fatalf("name = %v, want Test User", data["name"])
	}
}

func TestRequestReturnsAPIError(t *testing.T) {
	client := NewClient("https://api.test", "bad")
	client.HTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusUnauthorized, map[string]any{"detail": "bad token"}), nil
	})}

	_, err := client.Request(context.Background(), "GET", "/v1/me", nil, nil)
	apiErr, ok := err.(*Error)
	if !ok {
		t.Fatalf("err = %T, want *Error", err)
	}
	if apiErr.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", apiErr.StatusCode, http.StatusUnauthorized)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func jsonResponse(status int, body any) *http.Response {
	encoded, _ := json.Marshal(body)
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(string(encoded))),
	}
}
