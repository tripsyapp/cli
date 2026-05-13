package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRegisterMCPHTTPHandlersAddsExactRootAlias(t *testing.T) {
	mux := http.NewServeMux()
	paths := registerMCPHTTPHandlers(mux, "/mcp", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	wantPaths := []string{"/mcp", "/"}
	if len(paths) != len(wantPaths) {
		t.Fatalf("registered paths = %v, want %v", paths, wantPaths)
	}
	for i := range wantPaths {
		if paths[i] != wantPaths[i] {
			t.Fatalf("registered paths = %v, want %v", paths, wantPaths)
		}
	}

	for _, path := range []string{"/", "/mcp"} {
		res := httptest.NewRecorder()
		mux.ServeHTTP(res, httptest.NewRequest(http.MethodPost, path, nil))
		if res.Code != http.StatusNoContent {
			t.Fatalf("%s returned status %d, want %d", path, res.Code, http.StatusNoContent)
		}
	}

	res := httptest.NewRecorder()
	mux.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/anything-else", nil))
	if res.Code != http.StatusNotFound {
		t.Fatalf("unknown path returned status %d, want %d", res.Code, http.StatusNotFound)
	}
}

func TestRegisterMCPHTTPHandlersDoesNotDuplicateRoot(t *testing.T) {
	mux := http.NewServeMux()
	paths := registerMCPHTTPHandlers(mux, "/", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	if len(paths) != 1 || paths[0] != "/" {
		t.Fatalf("registered paths = %v, want [/]", paths)
	}
}

func TestOpenAIAppsChallenge(t *testing.T) {
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/.well-known/openai-apps-challenge", nil)

	openAIAppsChallenge(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
	if got := res.Header().Get("Content-Type"); got != "text/plain; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want text/plain; charset=utf-8", got)
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(body), openAIAppsChallengeToken+"\n"; got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
}

func TestExplicitToolAnnotationsHTTPHandlerNormalizesPostResponses(t *testing.T) {
	handler := explicitToolAnnotationsHTTPHandler{next: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"tools":[{"name":"tripsy_trips_create","annotations":{"destructiveHint":false,"openWorldHint":false}}]}}`))
	})}

	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/mcp", nil))

	if !strings.Contains(res.Body.String(), `"readOnlyHint":false`) {
		t.Fatalf("response body should include explicit readOnlyHint false: %s", res.Body.String())
	}
}
