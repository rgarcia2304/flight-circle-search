package email

import (
	"context"

	"github.com/rgarcia2304/flight-circle-search/internal/config"
)

// Sender delivers a magic-link email to an address.
type Sender interface {
	SendMagicLink(ctx context.Context, to, link string) error
}

// defaultFrom uses Resend's built-in test sender, which works without any
// domain verification. It's restricted to sending to the Resend account
// owner's own address — fine for a demo deployment gated by an email
// allowlist, not suitable for sending to arbitrary third-party recipients.
const defaultFrom = "onboarding@resend.dev"

// NewSender picks a Sender based on config: a real Resend-backed sender when
// RESEND_API_KEY is set, otherwise a stdout fallback for local dev.
func NewSender(cfg *config.Config) Sender {
	if cfg.ResendAPIKey == "" {
		return StdoutSender{}
	}
	return NewResendSender(cfg.ResendAPIKey, defaultFrom)
}
