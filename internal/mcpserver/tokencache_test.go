package mcpserver

import (
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/auth"
)

func TestTokenCacheReturnsCopies(t *testing.T) {
	cache := newTokenCache(time.Minute)
	cache.put("token", &auth.TokenInfo{
		Scopes: []string{"profile"},
		UserID: "user-1",
		Extra:  map[string]any{tokenInfoTripsyTokenKey: "token"},
	})

	first, ok := cache.get("token")
	if !ok {
		t.Fatal("expected cached token")
	}
	first.Scopes[0] = "mutated"
	first.Extra[tokenInfoTripsyTokenKey] = "mutated"

	second, ok := cache.get("token")
	if !ok {
		t.Fatal("expected cached token")
	}
	if second.Scopes[0] != "profile" {
		t.Fatalf("Scopes[0] = %q, want profile", second.Scopes[0])
	}
	if second.Extra[tokenInfoTripsyTokenKey] != "token" {
		t.Fatalf("cached token extra = %v, want token", second.Extra[tokenInfoTripsyTokenKey])
	}
}
