package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// SessionTTL is how long a session remains valid after creation.
const SessionTTL = 30 * 24 * time.Hour

// Session is the data stored for a logged-in user.
type Session struct {
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

// SessionStore manages logged-in sessions backed by Redis.
type SessionStore struct {
	client *redis.Client
}

// NewSessionStore creates a SessionStore.
func NewSessionStore(client *redis.Client) *SessionStore {
	return &SessionStore{client: client}
}

// Create starts a new session for email and returns its opaque session ID.
func (s *SessionStore) Create(ctx context.Context, email string) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate session id: %w", err)
	}
	id := base64.RawURLEncoding.EncodeToString(raw)

	sess := Session{Email: email, CreatedAt: time.Now().UTC()}
	data, err := json.Marshal(sess)
	if err != nil {
		return "", fmt.Errorf("marshal session: %w", err)
	}

	if err := s.client.Set(ctx, sessionKey(id), data, SessionTTL).Err(); err != nil {
		return "", fmt.Errorf("store session: %w", err)
	}
	return id, nil
}

// Get looks up a session by ID. A missing or expired session returns ErrNotFound.
func (s *SessionStore) Get(ctx context.Context, sessionID string) (Session, error) {
	data, err := s.client.Get(ctx, sessionKey(sessionID)).Bytes()
	if err == redis.Nil {
		return Session{}, ErrNotFound
	}
	if err != nil {
		return Session{}, fmt.Errorf("get session: %w", err)
	}
	var sess Session
	if err := json.Unmarshal(data, &sess); err != nil {
		return Session{}, fmt.Errorf("unmarshal session: %w", err)
	}
	return sess, nil
}

// Delete ends a session.
func (s *SessionStore) Delete(ctx context.Context, sessionID string) error {
	if err := s.client.Del(ctx, sessionKey(sessionID)).Err(); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

func sessionKey(id string) string {
	return "session:" + id
}
