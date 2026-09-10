package email

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rgarcia2304/flight-circle-search/internal/config"
)

func TestNewSender_PicksStdoutWhenNoAPIKey(t *testing.T) {
	sender := NewSender(&config.Config{ResendAPIKey: ""})
	if _, ok := sender.(StdoutSender); !ok {
		t.Errorf("NewSender() = %T, want StdoutSender", sender)
	}
}

func TestNewSender_PicksResendWhenAPIKeySet(t *testing.T) {
	sender := NewSender(&config.Config{ResendAPIKey: "test-key"})
	if _, ok := sender.(*ResendSender); !ok {
		t.Errorf("NewSender() = %T, want *ResendSender", sender)
	}
}

func TestStdoutSender_DoesNotError(t *testing.T) {
	if err := (StdoutSender{}).SendMagicLink(context.Background(), "user@example.com", "https://example.com/callback?token=abc"); err != nil {
		t.Errorf("SendMagicLink: %v", err)
	}
}

func TestResendSender_SendsExpectedRequest(t *testing.T) {
	var gotAuth string
	var gotBody resendRequest

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode request body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sender := NewResendSender("secret-key", "login@flightcircle.app")
	sender.client = srv.Client()
	sender.apiURL = srv.URL

	err := sender.SendMagicLink(context.Background(), "user@example.com", "https://example.com/callback?token=abc")
	if err != nil {
		t.Fatalf("SendMagicLink: %v", err)
	}

	if gotAuth != "Bearer secret-key" {
		t.Errorf("Authorization header = %q, want %q", gotAuth, "Bearer secret-key")
	}
	if gotBody.To != "user@example.com" {
		t.Errorf("To = %q, want %q", gotBody.To, "user@example.com")
	}
	if gotBody.From != "login@flightcircle.app" {
		t.Errorf("From = %q, want %q", gotBody.From, "login@flightcircle.app")
	}
	if !strings.Contains(gotBody.HTML, "https://example.com/callback?token=abc") {
		t.Errorf("HTML body missing link: %q", gotBody.HTML)
	}
}

func TestResendSender_NonSuccessStatusReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"invalid api key"}`))
	}))
	defer srv.Close()

	sender := NewResendSender("bad-key", "login@flightcircle.app")
	sender.client = srv.Client()
	sender.apiURL = srv.URL

	err := sender.SendMagicLink(context.Background(), "user@example.com", "https://example.com/callback?token=abc")
	if err == nil {
		t.Fatal("SendMagicLink err = nil, want error")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error = %v, want it to mention status 401", err)
	}
}
