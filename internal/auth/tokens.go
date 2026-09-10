package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const tokenTTL = 15 * time.Minute

const rateLimitWindow = 24 * time.Hour

// consumeScript atomically reads and deletes a key so a magic-link token
// can never be replayed, even under concurrent callback requests.
var consumeScript = redis.NewScript(`
local v = redis.call('GET', KEYS[1])
if v then
	redis.call('DEL', KEYS[1])
end
return v
`)

// TokenStore issues and consumes single-use magic-link tokens backed by Redis.
type TokenStore struct {
	client          *redis.Client
	rateLimitPerDay int
}

// NewTokenStore creates a TokenStore. rateLimitPerDay caps how many magic
// links a single email address may request per rolling 24h window.
func NewTokenStore(client *redis.Client, rateLimitPerDay int) *TokenStore {
	return &TokenStore{client: client, rateLimitPerDay: rateLimitPerDay}
}

// Issue generates a new magic-link token for email, subject to the
// per-email rate limit. The raw token is returned to the caller; only its
// SHA-256 hash is ever persisted.
func (s *TokenStore) Issue(ctx context.Context, email string) (string, error) {
	rateKey := "authrate:" + email
	count, err := s.client.Incr(ctx, rateKey).Result()
	if err != nil {
		return "", fmt.Errorf("incr rate limit: %w", err)
	}
	if count == 1 {
		if err := s.client.Expire(ctx, rateKey, rateLimitWindow).Err(); err != nil {
			return "", fmt.Errorf("set rate limit ttl: %w", err)
		}
	}
	if s.rateLimitPerDay > 0 && count > int64(s.rateLimitPerDay) {
		return "", ErrRateLimited
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)

	if err := s.client.Set(ctx, tokenKey(token), email, tokenTTL).Err(); err != nil {
		return "", fmt.Errorf("store token: %w", err)
	}
	return token, nil
}

// Consume validates and invalidates token, returning the email it was
// issued for. A token can only be consumed once; a missing or expired
// token returns ErrNotFound.
func (s *TokenStore) Consume(ctx context.Context, token string) (string, error) {
	res, err := consumeScript.Run(ctx, s.client, []string{tokenKey(token)}).Result()
	if err == redis.Nil {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("consume token: %w", err)
	}
	email, ok := res.(string)
	if !ok || email == "" {
		return "", ErrNotFound
	}
	return email, nil
}

func tokenKey(token string) string {
	sum := sha256.Sum256([]byte(token))
	return "authtoken:" + hex.EncodeToString(sum[:])
}
