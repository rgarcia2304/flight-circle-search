package config

import (
    "testing"
    "github.com/stretchr/testify/assert"
)

var configEnvVars = []string{
    "DATABASE_URL", "SESSION_SIGNING_KEY", "REDIS_URL", "API_ADDR",
    "APP_ORIGIN", "HUBS", "CACHE_TTL_HOURS", "RATE_LIMIT_PER_DAY",
    "RESEND_API_KEY", "TRAVELPAYOUTS_API_KEY",
}

// clearConfigEnv clears the config env vars for the duration of the test
// (e.g. TRAVELPAYOUTS_API_KEY set in a dev's shell profile) so tests observe
// Load()'s defaults. t.Setenv restores the original value automatically.
func clearConfigEnv(t *testing.T) {
    t.Helper()
    for _, key := range configEnvVars {
        t.Setenv(key, "")
    }
}

func TestLoad_Defaults(t *testing.T) {
    clearConfigEnv(t)

    cfg, err := Load()
    if assert.NoError(t, err) {
        assert.Equal(t, "postgres://dev:dev@localhost:5432/flightsearch", cfg.DatabaseURL)
        assert.Equal(t, "redis://localhost:6379", cfg.RedisURL)
        assert.Equal(t, ":8080", cfg.APIAddr)
        assert.Equal(t, "http://localhost:5173", cfg.AppOrigin)
        assert.Nil(t, cfg.Hubs)
        assert.Equal(t, 24, cfg.CacheTTLHours)
        assert.Equal(t, 5, cfg.RateLimitPerDay)
        assert.Equal(t, "", cfg.ResendAPIKey)
        assert.Equal(t, "", cfg.SessionSigningKey)
        assert.Equal(t, "", cfg.TravelpayoutsAPIKey)
    }
}

func TestLoad_WithEnv(t *testing.T) {
    clearConfigEnv(t)
    t.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
    t.Setenv("SESSION_SIGNING_KEY", "secret")
    t.Setenv("REDIS_URL", "redis://redis:6379")
    t.Setenv("API_ADDR", ":9090")
    t.Setenv("APP_ORIGIN", "http://example.com")
    t.Setenv("HUBS", "LHR, FRA, AMS")
    t.Setenv("CACHE_TTL_HOURS", "48")
    t.Setenv("RATE_LIMIT_PER_DAY", "100")
    t.Setenv("RESEND_API_KEY", "key123")
    t.Setenv("TRAVELPAYOUTS_API_KEY", "tp123")

    cfg, err := Load()
    if assert.NoError(t, err) {
        assert.Equal(t, "postgres://test:test@localhost/test", cfg.DatabaseURL)
        assert.Equal(t, "redis://redis:6379", cfg.RedisURL)
        assert.Equal(t, ":9090", cfg.APIAddr)
        assert.Equal(t, "http://example.com", cfg.AppOrigin)
        assert.Equal(t, []string{"LHR", "FRA", "AMS"}, cfg.Hubs)
        assert.Equal(t, 48, cfg.CacheTTLHours)
        assert.Equal(t, 100, cfg.RateLimitPerDay)
        assert.Equal(t, "key123", cfg.ResendAPIKey)
        assert.Equal(t, "secret", cfg.SessionSigningKey)
        assert.Equal(t, "tp123", cfg.TravelpayoutsAPIKey)
    }
}

func TestValidate_MissingRequiredFields(t *testing.T) {
    cfg := &Config{}
    err := cfg.Validate()
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "DATABASE_URL is required")

    cfg.DatabaseURL = "postgres://dev:dev@localhost:5432/flightsearch"
    err = cfg.Validate()
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "SESSION_SIGNING_KEY is required")

    cfg.SessionSigningKey = "secret"
    assert.NoError(t, cfg.Validate())
}
