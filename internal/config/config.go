package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	DatabaseURL        string
	RedisURL           string
	APIAddr            string
	AppOrigin          string
	Hubs               []string
	AllowedEmails      []string
	CacheTTLHours      int
	RateLimitPerDay    int
	ResendAPIKey       string
	SessionSigningKey  string
	TravelpayoutsAPIKey string
	E2ETestMode        bool
}

func Load() (*Config, error) {
	cfg := &Config{
		DatabaseURL:        env("DATABASE_URL", "postgres://dev:dev@localhost:5432/flightsearch"),
		RedisURL:           env("REDIS_URL", "redis://localhost:6379"),
		APIAddr:            apiAddr(),
		AppOrigin:          env("APP_ORIGIN", "http://localhost:5173"),
		Hubs:               parseHubs(env("HUBS", "")),
		AllowedEmails:      parseAllowedEmails(env("ALLOWED_EMAILS", "")),
		CacheTTLHours:      intOrDefault("CACHE_TTL_HOURS", 24),
		RateLimitPerDay:    intOrDefault("RATE_LIMIT_PER_DAY", 5),
		ResendAPIKey:       env("RESEND_API_KEY", ""),
		SessionSigningKey:  env("SESSION_SIGNING_KEY", ""),
		TravelpayoutsAPIKey: env("TRAVELPAYOUTS_API_KEY", ""),
		E2ETestMode:        env("E2E_TEST_MODE", "") == "true",
	}

	return cfg, nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// apiAddr resolves the HTTP listen address. Cloud Run sets PORT (not a
// value we choose) and requires the container to listen on it; local dev
// uses API_ADDR or the :8080 default instead.
func apiAddr() string {
	if port := os.Getenv("PORT"); port != "" {
		return ":" + port
	}
	return env("API_ADDR", ":8080")
}

func intOrDefault(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil {
			return n
		}
	}
	return fallback
}

func parseHubs(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var hubs []string
	for _, p := range parts {
		h := strings.TrimSpace(p)
		if h != "" {
			hubs = append(hubs, h)
		}
	}
	return hubs
}

// parseAllowedEmails parses a comma-separated ALLOWED_EMAILS list, normalized
// to lowercase for case-insensitive matching. An empty list means no
// restriction — anyone can request a magic link (the local-dev default).
func parseAllowedEmails(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var emails []string
	for _, p := range parts {
		e := strings.ToLower(strings.TrimSpace(p))
		if e != "" {
			emails = append(emails, e)
		}
	}
	return emails
}

func (c *Config) Validate() error {
	if c.DatabaseURL == "" {
		return err("DATABASE_URL is required")
	}
	if c.SessionSigningKey == "" {
		return err("SESSION_SIGNING_KEY is required")
	}
	return nil
}

func err(msg string) error {
	return &configError{msg: msg}
}

type configError struct {
	msg string
}

func (e *configError) Error() string {
	return e.msg
}

// DefaultHubs is the fallback world hub list when HUBS env var is not set.
var DefaultHubs = []string{
	"LHR", "FRA", "AMS", "CDG", "IST", "DUB", "MAD", "MUC", "ZRH", "BCN",
	"YYZ", "YVR", "MEX", "DFW", "ORD", "ATL", "JFK", "LAX", "SFO",
	"GRU", "EZE",
	"DXB", "DOH",
	"PEK", "PVG", "HND", "NRT", "ICN", "BKK", "DEL", "BOM",
	"BRU", "MIL",
	"CAI",
}

func DefaultCacheTTL() time.Duration {
	return 24 * time.Hour
}

func LoadMust() *Config {
	cfg, err := Load()
	if err != nil {
		panic(err)
	}
	return cfg
}
