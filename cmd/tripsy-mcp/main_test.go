package main

import (
	"net/http"
	"net/http/httptest"
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
