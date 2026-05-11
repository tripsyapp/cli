package mcpserver

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/auth"
)

type tokenCacheEntry struct {
	info       *auth.TokenInfo
	expiration time.Time
}

type tokenCache struct {
	ttl time.Duration
	mu  sync.RWMutex
	m   map[string]tokenCacheEntry
}

func newTokenCache(ttl time.Duration) *tokenCache {
	return &tokenCache{ttl: ttl, m: make(map[string]tokenCacheEntry)}
}

func (c *tokenCache) key(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (c *tokenCache) get(token string) (*auth.TokenInfo, bool) {
	key := c.key(token)
	c.mu.RLock()
	entry, ok := c.m[key]
	c.mu.RUnlock()
	if !ok {
		return nil, false
	}
	if time.Now().After(entry.expiration) {
		c.mu.Lock()
		delete(c.m, key)
		c.mu.Unlock()
		return nil, false
	}
	return entry.info, true
}

func (c *tokenCache) put(token string, info *auth.TokenInfo) {
	if info == nil {
		return
	}
	expiration := time.Now().Add(c.ttl)
	if !info.Expiration.IsZero() && info.Expiration.Before(expiration) {
		expiration = info.Expiration
	}
	c.mu.Lock()
	c.m[c.key(token)] = tokenCacheEntry{info: info, expiration: expiration}
	c.mu.Unlock()
}

func cachedVerifier(verifier auth.TokenVerifier, ttl time.Duration) auth.TokenVerifier {
	cache := newTokenCache(ttl)
	return func(ctx context.Context, token string, r *http.Request) (*auth.TokenInfo, error) {
		trimmed := strings.TrimSpace(token)
		if trimmed == "" {
			return verifier(ctx, token, r)
		}
		if info, ok := cache.get(trimmed); ok {
			return info, nil
		}
		info, err := verifier(ctx, token, r)
		if err != nil {
			return nil, err
		}
		cache.put(trimmed, info)
		return info, nil
	}
}
