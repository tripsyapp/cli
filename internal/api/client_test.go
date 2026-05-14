package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
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

func TestRequestCanSendBearerToken(t *testing.T) {
	client := NewClient("https://api.test", "oauth-token")
	client.AuthScheme = "Bearer"
	client.HTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if got, want := r.Header.Get("Authorization"), "Bearer oauth-token"; got != want {
			t.Fatalf("Authorization = %q, want %q", got, want)
		}
		return jsonResponse(http.StatusOK, map[string]any{"ok": true}), nil
	})}

	if _, err := client.Request(context.Background(), "GET", "/v1/me", nil, nil); err != nil {
		t.Fatal(err)
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

func TestRequestAllPagesFollowsPaginationAndCombinesResults(t *testing.T) {
	var paths []string
	client := NewClient("https://api.test", "test-token")
	client.HTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		paths = append(paths, r.URL.RequestURI())
		switch r.URL.RequestURI() {
		case "/v2/trips/?updatedSince=2026-05-01T00%3A00%3A00Z":
			return jsonResponse(http.StatusOK, map[string]any{
				"count":    2,
				"next":     "https://api.test/v2/trips/?page=2",
				"previous": nil,
				"results":  []any{map[string]any{"id": 1}},
			}), nil
		case "/v2/trips/?page=2":
			return jsonResponse(http.StatusOK, map[string]any{
				"count":    2,
				"next":     nil,
				"previous": "https://api.test/v2/trips/",
				"results":  []any{map[string]any{"id": 2}},
			}), nil
		default:
			t.Fatalf("unexpected request URI: %s", r.URL.RequestURI())
			return nil, nil
		}
	})}

	query := make(url.Values)
	query.Set("updatedSince", "2026-05-01T00:00:00Z")
	resp, err := client.RequestAllPages(context.Background(), "GET", "/v2/trips/", query, nil)
	if err != nil {
		t.Fatal(err)
	}
	root := resp.Data.(map[string]any)
	items := root["results"].([]any)
	if len(items) != 2 {
		t.Fatalf("results len = %d, want 2", len(items))
	}
	if root["next"] != nil {
		t.Fatalf("next = %v, want nil after aggregation", root["next"])
	}
	if got, want := strings.Join(paths, ","), "/v2/trips/?updatedSince=2026-05-01T00%3A00%3A00Z,/v2/trips/?page=2"; got != want {
		t.Fatalf("paths = %s, want %s", got, want)
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
