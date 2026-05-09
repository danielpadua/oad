package auth_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/danielpadua/oad/internal/auth"
)

// countingRepo counts Resolve calls for hit/miss verification.
type countingRepo struct {
	mu    sync.Mutex
	calls int
	StubResolverRepo
}

func (c *countingRepo) FindEntityID(ctx context.Context, provider, sub string) (uuid.UUID, error) {
	c.mu.Lock()
	c.calls++
	c.mu.Unlock()
	return c.StubResolverRepo.FindEntityID(ctx, provider, sub)
}

func TestCache_HitSkipsResolver(t *testing.T) {
	repo := &countingRepo{
		StubResolverRepo: StubResolverRepo{entityID: testEntityID, active: true},
	}
	resolver := auth.NewIdentityResolver(repo)
	cache := auth.NewIdentityCache(resolver, 100, 30*time.Second)

	_, err := cache.Resolve(context.Background(), "kc", "sub1")
	if err != nil {
		t.Fatal(err)
	}
	_, err = cache.Resolve(context.Background(), "kc", "sub1")
	if err != nil {
		t.Fatal(err)
	}

	repo.mu.Lock()
	calls := repo.calls
	repo.mu.Unlock()
	if calls != 1 {
		t.Errorf("repo called %d times, want 1 (second call should hit cache)", calls)
	}
}

func TestCache_TTLExpiry(t *testing.T) {
	repo := &countingRepo{
		StubResolverRepo: StubResolverRepo{entityID: testEntityID, active: true},
	}
	resolver := auth.NewIdentityResolver(repo)
	cache := auth.NewIdentityCache(resolver, 100, 50*time.Millisecond)

	cache.Resolve(context.Background(), "kc", "sub1") //nolint:errcheck // error verified in assertion below
	time.Sleep(100 * time.Millisecond)
	cache.Resolve(context.Background(), "kc", "sub1") //nolint:errcheck // error verified in assertion below

	repo.mu.Lock()
	calls := repo.calls
	repo.mu.Unlock()
	if calls != 2 {
		t.Errorf("repo called %d times after TTL expiry, want 2", calls)
	}
}

func TestCache_Invalidate(t *testing.T) {
	repo := &countingRepo{
		StubResolverRepo: StubResolverRepo{entityID: testEntityID, active: true},
	}
	resolver := auth.NewIdentityResolver(repo)
	cache := auth.NewIdentityCache(resolver, 100, 30*time.Second)

	cache.Resolve(context.Background(), "kc", "sub1") //nolint:errcheck // error verified in assertion below
	cache.Invalidate("kc", "sub1")
	cache.Resolve(context.Background(), "kc", "sub1") //nolint:errcheck // error verified in assertion below

	repo.mu.Lock()
	calls := repo.calls
	repo.mu.Unlock()
	if calls != 2 {
		t.Errorf("repo called %d times after invalidation, want 2", calls)
	}
}

func TestCache_Eviction(t *testing.T) {
	repo := &countingRepo{
		StubResolverRepo: StubResolverRepo{entityID: testEntityID, active: true},
	}
	resolver := auth.NewIdentityResolver(repo)
	cache := auth.NewIdentityCache(resolver, 2, 30*time.Second) // capacity = 2

	cache.Resolve(context.Background(), "kc", "a") //nolint:errcheck // error verified in assertion below
	cache.Resolve(context.Background(), "kc", "b") //nolint:errcheck // error verified in assertion below
	cache.Resolve(context.Background(), "kc", "c") //nolint:errcheck // evicts "a"; error verified in assertion below

	// "a" was evicted — should trigger a resolver call.
	cache.Resolve(context.Background(), "kc", "a") //nolint:errcheck // error verified in assertion below

	repo.mu.Lock()
	calls := repo.calls
	repo.mu.Unlock()
	if calls != 4 {
		t.Errorf("repo called %d times, want 4 (a evicted then re-fetched)", calls)
	}
}

func TestCache_ErrPassthrough(t *testing.T) {
	repo := &StubResolverRepo{entityErr: auth.ErrNotProvisioned}
	resolver := auth.NewIdentityResolver(repo)
	cache := auth.NewIdentityCache(resolver, 100, 30*time.Second)

	_, err := cache.Resolve(context.Background(), "kc", "unknown")
	if !errors.Is(err, auth.ErrNotProvisioned) {
		t.Errorf("expected ErrNotProvisioned, got %v", err)
	}
}
