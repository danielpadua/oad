package auth

import (
	"container/list"
	"context"
	"sync"
	"time"
)

type cachedEntry struct {
	key       string
	identity  *Identity
	expiresAt time.Time
}

// IdentityCache wraps IdentityResolver with an in-process LRU cache.
// Keyed by "<provider>:<sub>". Thread-safe.
type IdentityCache struct {
	mu       sync.Mutex
	resolver *IdentityResolver
	capacity int
	ttl      time.Duration
	items    map[string]*list.Element
	lru      *list.List
}

// NewIdentityCache creates a cache backed by the given resolver.
func NewIdentityCache(resolver *IdentityResolver, capacity int, ttl time.Duration) *IdentityCache {
	return &IdentityCache{
		resolver: resolver,
		capacity: capacity,
		ttl:      ttl,
		items:    make(map[string]*list.Element),
		lru:      list.New(),
	}
}

// Resolve returns a cached Identity or calls the resolver on miss/expiry.
// Errors are not cached — a failing lookup always retries on the next call.
func (c *IdentityCache) Resolve(ctx context.Context, provider, sub string) (*Identity, error) {
	key := provider + ":" + sub

	c.mu.Lock()
	if el, ok := c.items[key]; ok {
		entry := el.Value.(*cachedEntry)
		if time.Now().Before(entry.expiresAt) {
			c.lru.MoveToFront(el)
			identity := entry.identity
			c.mu.Unlock()
			return identity, nil
		}
		c.lru.Remove(el)
		delete(c.items, key)
	}
	c.mu.Unlock()

	identity, err := c.resolver.Resolve(ctx, provider, sub)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	for c.lru.Len() >= c.capacity {
		oldest := c.lru.Back()
		if oldest == nil {
			break
		}
		evicted := c.lru.Remove(oldest).(*cachedEntry)
		delete(c.items, evicted.key)
	}
	entry := &cachedEntry{key: key, identity: identity, expiresAt: time.Now().Add(c.ttl)}
	el := c.lru.PushFront(entry)
	c.items[key] = el
	c.mu.Unlock()

	return identity, nil
}

// Invalidate removes an entry from the cache by (provider, sub).
// Safe to call when the entry does not exist.
func (c *IdentityCache) Invalidate(provider, sub string) {
	key := provider + ":" + sub
	c.mu.Lock()
	if el, ok := c.items[key]; ok {
		c.lru.Remove(el)
		delete(c.items, key)
	}
	c.mu.Unlock()
}
