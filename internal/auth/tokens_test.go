package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newTestTokenStore(t *testing.T, rateLimitPerDay int) (*TokenStore, *miniredis.Miniredis) {
	t.Helper()
	s, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	return NewTokenStore(client, rateLimitPerDay), s
}

func TestTokenStore_IssueConsume_RoundTrip(t *testing.T) {
	store, _ := newTestTokenStore(t, 5)
	ctx := context.Background()

	token, err := store.Issue(ctx, "user@example.com")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if token == "" {
		t.Fatal("Issue returned empty token")
	}

	email, err := store.Consume(ctx, token)
	if err != nil {
		t.Fatalf("Consume: %v", err)
	}
	if email != "user@example.com" {
		t.Errorf("Consume email = %q, want %q", email, "user@example.com")
	}
}

func TestTokenStore_Consume_SingleUse(t *testing.T) {
	store, _ := newTestTokenStore(t, 5)
	ctx := context.Background()

	token, err := store.Issue(ctx, "user@example.com")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if _, err := store.Consume(ctx, token); err != nil {
		t.Fatalf("first Consume: %v", err)
	}

	_, err = store.Consume(ctx, token)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("second Consume err = %v, want ErrNotFound", err)
	}
}

func TestTokenStore_Consume_UnknownToken(t *testing.T) {
	store, _ := newTestTokenStore(t, 5)
	_, err := store.Consume(context.Background(), "does-not-exist")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Consume err = %v, want ErrNotFound", err)
	}
}

func TestTokenStore_Consume_ExpiredToken(t *testing.T) {
	store, mr := newTestTokenStore(t, 5)
	ctx := context.Background()

	token, err := store.Issue(ctx, "user@example.com")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	mr.FastForward(16 * time.Minute)

	_, err = store.Consume(ctx, token)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Consume err = %v, want ErrNotFound", err)
	}
}

func TestTokenStore_Issue_RateLimited(t *testing.T) {
	store, _ := newTestTokenStore(t, 3)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		if _, err := store.Issue(ctx, "user@example.com"); err != nil {
			t.Fatalf("Issue #%d: %v", i, err)
		}
	}

	_, err := store.Issue(ctx, "user@example.com")
	if !errors.Is(err, ErrRateLimited) {
		t.Errorf("Issue #4 err = %v, want ErrRateLimited", err)
	}

	// A different email is unaffected by the first email's limit.
	if _, err := store.Issue(ctx, "other@example.com"); err != nil {
		t.Errorf("Issue for other email: %v", err)
	}
}

func TestTokenStore_Issue_RateLimitResetsAfterWindow(t *testing.T) {
	store, mr := newTestTokenStore(t, 1)
	ctx := context.Background()

	if _, err := store.Issue(ctx, "user@example.com"); err != nil {
		t.Fatalf("Issue #1: %v", err)
	}
	if _, err := store.Issue(ctx, "user@example.com"); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("Issue #2 err = %v, want ErrRateLimited", err)
	}

	mr.FastForward(25 * time.Hour)

	if _, err := store.Issue(ctx, "user@example.com"); err != nil {
		t.Errorf("Issue after window reset: %v", err)
	}
}
