package email

import (
	"context"

	"github.com/rgarcia2304/flight-circle-search/internal/config"
)

// Sender delivers a magic-link email to an address.
type Sender interface {
	SendMagicLink(ctx context.Context, to, link string) error
}

const defaultFrom = "login@flightcircle.app"

// NewSender picks a Sender based on config: a real Resend-backed sender when
// RESEND_API_KEY is set, otherwise a stdout fallback for local dev.
func NewSender(cfg *config.Config) Sender {
	if cfg.ResendAPIKey == "" {
		return StdoutSender{}
	}
	return NewResendSender(cfg.ResendAPIKey, defaultFrom)
}
