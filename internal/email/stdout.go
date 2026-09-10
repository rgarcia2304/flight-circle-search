package email

import (
	"context"
	"log"
)

// StdoutSender logs the magic link instead of emailing it. It's the local
// dev fallback used when no RESEND_API_KEY is configured.
type StdoutSender struct{}

func (StdoutSender) SendMagicLink(_ context.Context, to, link string) error {
	log.Printf("magic link for %s: %s", to, link)
	return nil
}
