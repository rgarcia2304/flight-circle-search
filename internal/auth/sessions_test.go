package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newTestSessionStore(t *testing.T) (*SessionStore, *miniredis.Miniredis) {
	t.Helper()
	s, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	return NewSessionStore(client), s
}

func TestSessionStore_CreateGet_RoundTrip(t *testing.T) {
	store, _ := newTestSessionStore(t)
	ctx := context.Background()

	id, err := store.Create(ctx, "user@example.com")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if id == "" {
		t.Fatal("Create returned empty session id")
	}

	sess, err := store.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if sess.Email != "user@example.com" {
		t.Errorf("Get email = %q, want %q", sess.Email, "user@example.com")
	}
	if sess.CreatedAt.IsZero() {
		t.Error("Get CreatedAt is zero")
	}
}

func TestSessionStore_Get_UnknownID(t *testing.T) {
	store, _ := newTestSessionStore(t)
	_, err := store.Get(context.Background(), "does-not-exist")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Get err = %v, want ErrNotFound", err)
	}
}

func TestSessionStore_Delete(t *testing.T) {
	store, _ := newTestSessionStore(t)
	ctx := context.Background()

	id, err := store.Create(ctx, "user@example.com")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := store.Delete(ctx, id); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err = store.Get(ctx, id)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Get after Delete err = %v, want ErrNotFound", err)
	}
}

func TestSessionStore_Get_ExpiredSession(t *testing.T) {
	store, mr := newTestSessionStore(t)
	ctx := context.Background()

	id, err := store.Create(ctx, "user@example.com")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	mr.FastForward(SessionTTL + time.Minute)

	_, err = store.Get(ctx, id)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Get after expiry err = %v, want ErrNotFound", err)
	}
}
