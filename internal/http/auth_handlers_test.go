package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/rgarcia2304/flight-circle-search/internal/auth"
)

type fakeSender struct {
	mu    sync.Mutex
	links map[string]string
}

func newFakeSender() *fakeSender {
	return &fakeSender{links: make(map[string]string)}
}

func (f *fakeSender) SendMagicLink(_ context.Context, to, link string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.links[to] = link
	return nil
}

func (f *fakeSender) linkFor(to string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.links[to]
}

func newTestAuthHandlers(t *testing.T) (*AuthHandlers, *fakeSender) {
	t.Helper()
	return newTestAuthHandlersWithAllowlist(t, nil)
}

func newTestAuthHandlersWithAllowlist(t *testing.T, allowedEmails []string) (*AuthHandlers, *fakeSender) {
	t.Helper()
	s, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	tokens := auth.NewTokenStore(client, 5)
	sessions := auth.NewSessionStore(client)
	sender := newFakeSender()

	return NewAuthHandlers(tokens, sessions, sender, "http://localhost:5173", allowedEmails), sender
}

func TestAuthHandlers_FullFlow_RequestCallbackLogout(t *testing.T) {
	h, sender := newTestAuthHandlers(t)

	// 1. Request a magic link.
	reqBody := strings.NewReader(`{"email":"user@example.com"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/magic-link", reqBody)
	rec := httptest.NewRecorder()
	h.RequestMagicLink(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("RequestMagicLink status = %d, want %d, body=%s", rec.Code, http.StatusAccepted, rec.Body.String())
	}

	link := sender.linkFor("user@example.com")
	if link == "" {
		t.Fatal("no magic link was sent")
	}
	if !strings.HasPrefix(link, "http://localhost:5173/auth/callback?token=") {
		t.Errorf("link = %q, want it to start with the frontend callback URL", link)
	}
	token := strings.TrimPrefix(link, "http://localhost:5173/auth/callback?token=")

	// 2. Exchange the token for a session via the callback.
	cbReq := httptest.NewRequest(http.MethodGet, "/v1/auth/callback?token="+token, nil)
	cbRec := httptest.NewRecorder()
	h.Callback(cbRec, cbReq)

	if cbRec.Code != http.StatusOK {
		t.Fatalf("Callback status = %d, want %d, body=%s", cbRec.Code, http.StatusOK, cbRec.Body.String())
	}
	var cbResp callbackResponse
	if err := json.NewDecoder(cbRec.Body).Decode(&cbResp); err != nil {
		t.Fatalf("decode callback response: %v", err)
	}
	if cbResp.Email != "user@example.com" {
		t.Errorf("callback email = %q, want %q", cbResp.Email, "user@example.com")
	}

	result := cbRec.Result()
	cookies := result.Cookies()
	if len(cookies) != 1 || cookies[0].Name != sessionCookieName {
		t.Fatalf("expected a %q cookie, got %v", sessionCookieName, cookies)
	}
	sessionValue := cookies[0].Value
	if sessionValue == "" {
		t.Fatal("session cookie value is empty")
	}
	if cookies[0].HttpOnly != true {
		t.Error("session cookie is not HttpOnly")
	}
	if cookies[0].Secure {
		t.Error("session cookie should not be Secure for an http:// appOrigin")
	}

	// 3. The same token cannot be used twice.
	cbReq2 := httptest.NewRequest(http.MethodGet, "/v1/auth/callback?token="+token, nil)
	cbRec2 := httptest.NewRecorder()
	h.Callback(cbRec2, cbReq2)
	if cbRec2.Code != http.StatusUnauthorized {
		t.Errorf("second Callback status = %d, want %d", cbRec2.Code, http.StatusUnauthorized)
	}

	// 4. Logout clears the session.
	logoutReq := httptest.NewRequest(http.MethodPost, "/v1/auth/logout", nil)
	logoutReq.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionValue})
	logoutRec := httptest.NewRecorder()
	h.Logout(logoutRec, logoutReq)

	if logoutRec.Code != http.StatusNoContent {
		t.Fatalf("Logout status = %d, want %d", logoutRec.Code, http.StatusNoContent)
	}
	logoutCookies := logoutRec.Result().Cookies()
	if len(logoutCookies) != 1 || logoutCookies[0].MaxAge >= 0 {
		t.Errorf("expected logout to clear the cookie, got %v", logoutCookies)
	}

	// 5. The session is gone from the store.
	if _, err := h.sessions.Get(context.Background(), sessionValue); err == nil {
		t.Error("session still exists after logout")
	}
}

func TestAuthHandlers_RequestMagicLink_InvalidEmail(t *testing.T) {
	h, _ := newTestAuthHandlers(t)

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/magic-link", strings.NewReader(`{"email":"not-an-email"}`))
	rec := httptest.NewRecorder()
	h.RequestMagicLink(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestAuthHandlers_RequestMagicLink_RateLimited(t *testing.T) {
	h, _ := newTestAuthHandlers(t)
	body := `{"email":"user@example.com"}`

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/auth/magic-link", strings.NewReader(body))
		rec := httptest.NewRecorder()
		h.RequestMagicLink(rec, req)
		if rec.Code != http.StatusAccepted {
			t.Fatalf("request #%d status = %d, want %d", i, rec.Code, http.StatusAccepted)
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/magic-link", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.RequestMagicLink(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
}

func TestAuthHandlers_RequestMagicLink_AllowlistBlocksUnlistedEmail(t *testing.T) {
	h, sender := newTestAuthHandlersWithAllowlist(t, []string{"allowed@example.com"})

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/magic-link", strings.NewReader(`{"email":"stranger@example.com"}`))
	rec := httptest.NewRecorder()
	h.RequestMagicLink(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Errorf("status = %d, want %d (must look identical to success)", rec.Code, http.StatusAccepted)
	}
	if link := sender.linkFor("stranger@example.com"); link != "" {
		t.Errorf("expected no magic link to be sent to a non-allowlisted email, got %q", link)
	}
}

func TestAuthHandlers_RequestMagicLink_AllowlistAllowsListedEmail(t *testing.T) {
	h, sender := newTestAuthHandlersWithAllowlist(t, []string{"Allowed@Example.com"})

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/magic-link", strings.NewReader(`{"email":"allowed@example.com"}`))
	rec := httptest.NewRecorder()
	h.RequestMagicLink(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusAccepted)
	}
	if link := sender.linkFor("allowed@example.com"); link == "" {
		t.Error("expected a magic link to be sent to an allowlisted email (case-insensitive match)")
	}
}

func TestAuthHandlers_RequestMagicLink_EmptyAllowlistAllowsAnyEmail(t *testing.T) {
	h, sender := newTestAuthHandlers(t) // nil allowlist

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/magic-link", strings.NewReader(`{"email":"anyone@example.com"}`))
	rec := httptest.NewRecorder()
	h.RequestMagicLink(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusAccepted)
	}
	if link := sender.linkFor("anyone@example.com"); link == "" {
		t.Error("expected an empty allowlist to allow any email")
	}
}

func TestAuthHandlers_Callback_UnknownToken(t *testing.T) {
	h, _ := newTestAuthHandlers(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/auth/callback?token=bogus", nil)
	rec := httptest.NewRecorder()
	h.Callback(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestAuthHandlers_Callback_MissingToken(t *testing.T) {
	h, _ := newTestAuthHandlers(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/auth/callback", nil)
	rec := httptest.NewRecorder()
	h.Callback(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestNewAuthHandlers_SecureCookieForHTTPSOrigin(t *testing.T) {
	h := NewAuthHandlers(nil, nil, nil, "https://app.example.com", nil)
	if !h.cookieSecure {
		t.Error("cookieSecure = false, want true for https:// appOrigin")
	}
}

func TestAuthHandlers_Session_ReturnsEmailForValidSession(t *testing.T) {
	h, _ := newTestAuthHandlers(t)
	ctx := context.Background()

	sessionID, err := h.sessions.Create(ctx, "user@example.com")
	if err != nil {
		t.Fatalf("Create session: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/auth/session", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	rec := httptest.NewRecorder()
	RequireAuth(h.sessions)(http.HandlerFunc(h.Session)).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var resp callbackResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Email != "user@example.com" {
		t.Errorf("email = %q, want %q", resp.Email, "user@example.com")
	}
}

func TestAuthHandlers_Session_Returns401WithoutCookie(t *testing.T) {
	h, _ := newTestAuthHandlers(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/auth/session", nil)
	rec := httptest.NewRecorder()
	RequireAuth(h.sessions)(http.HandlerFunc(h.Session)).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestSetSessionCookie_SameSiteLaxForHTTPOrigin(t *testing.T) {
	h := NewAuthHandlers(nil, nil, nil, "http://localhost:5173", nil)
	rec := httptest.NewRecorder()
	h.setSessionCookie(rec, "sid", 3600)

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}
	if cookies[0].SameSite != http.SameSiteLaxMode {
		t.Errorf("SameSite = %v, want Lax", cookies[0].SameSite)
	}
	if cookies[0].Secure {
		t.Error("Secure = true, want false for http:// origin")
	}
}

func TestSetSessionCookie_SameSiteNoneForHTTPSOrigin(t *testing.T) {
	h := NewAuthHandlers(nil, nil, nil, "https://app.example.com", nil)
	rec := httptest.NewRecorder()
	h.setSessionCookie(rec, "sid", 3600)

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}
	if cookies[0].SameSite != http.SameSiteNoneMode {
		t.Errorf("SameSite = %v, want None", cookies[0].SameSite)
	}
	if !cookies[0].Secure {
		t.Error("Secure = false, want true for https:// origin")
	}
}

func TestAuthHandlers_IssueTestToken_UsableAgainstCallback(t *testing.T) {
	h, sender := newTestAuthHandlers(t)

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/test-token", strings.NewReader(`{"email":"e2e@example.com"}`))
	rec := httptest.NewRecorder()
	h.IssueTestToken(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("IssueTestToken status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var body struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Token == "" {
		t.Fatal("expected non-empty token")
	}

	// It bypasses the allowlist and never touches the sender.
	if sender.linkFor("e2e@example.com") != "" {
		t.Error("IssueTestToken should not send an email")
	}

	// The token is real: it should work against the actual callback handler.
	cbReq := httptest.NewRequest(http.MethodGet, "/v1/auth/callback?token="+body.Token, nil)
	cbRec := httptest.NewRecorder()
	h.Callback(cbRec, cbReq)
	if cbRec.Code != http.StatusOK {
		t.Fatalf("Callback status = %d, want %d, body=%s", cbRec.Code, http.StatusOK, cbRec.Body.String())
	}
}

func TestAuthHandlers_IssueTestToken_InvalidEmail(t *testing.T) {
	h, _ := newTestAuthHandlers(t)

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/test-token", strings.NewReader(`{"email":"not-an-email"}`))
	rec := httptest.NewRecorder()
	h.IssueTestToken(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}
