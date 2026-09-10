package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const resendAPIURL = "https://api.resend.com/emails"

// ResendSender sends magic-link emails via the Resend API
// (https://resend.com/docs/api-reference/emails/send-email).
type ResendSender struct {
	apiKey string
	from   string
	apiURL string
	client *http.Client
}

// NewResendSender creates a ResendSender using the given API key and from address.
func NewResendSender(apiKey, from string) *ResendSender {
	return &ResendSender{
		apiKey: apiKey,
		from:   from,
		apiURL: resendAPIURL,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

type resendRequest struct {
	From    string `json:"from"`
	To      string `json:"to"`
	Subject string `json:"subject"`
	HTML    string `json:"html"`
}

func (s *ResendSender) SendMagicLink(ctx context.Context, to, link string) error {
	body := resendRequest{
		From:    s.from,
		To:      to,
		Subject: "Your sign-in link",
		HTML:    fmt.Sprintf(`<p>Click <a href="%s">here</a> to sign in. This link expires in 15 minutes.</p>`, link),
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal resend request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.apiURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build resend request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("send resend request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("resend api error: status %d: %s", resp.StatusCode, respBody)
	}
	return nil
}
