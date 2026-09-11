package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/mail"
	"net/url"
	"strings"

	"github.com/rgarcia2304/flight-circle-search/internal/auth"
	"github.com/rgarcia2304/flight-circle-search/internal/email"
)

const sessionCookieName = "session"

// AuthHandlers implements the magic-link auth HTTP endpoints.
type AuthHandlers struct {
	tokens        *auth.TokenStore
	sessions      *auth.SessionStore
	sender        email.Sender
	appOrigin     string
	cookieSecure  bool
	allowedEmails map[string]struct{} // nil/empty = no restriction
}

// NewAuthHandlers creates AuthHandlers. appOrigin is the frontend's origin
// (e.g. https://app.example.com); the magic link emailed to users points
// there, and it also determines whether session cookies are marked Secure.
// allowedEmails, when non-empty, restricts who can request a magic link at
// all (case-insensitive) — meant for gating a demo deployment to specific
// people rather than leaving it open to anyone who finds the URL.
func NewAuthHandlers(tokens *auth.TokenStore, sessions *auth.SessionStore, sender email.Sender, appOrigin string, allowedEmails []string) *AuthHandlers {
	var allowed map[string]struct{}
	if len(allowedEmails) > 0 {
		allowed = make(map[string]struct{}, len(allowedEmails))
		for _, e := range allowedEmails {
			allowed[strings.ToLower(e)] = struct{}{}
		}
	}
	return &AuthHandlers{
		tokens:        tokens,
		sessions:      sessions,
		sender:        sender,
		appOrigin:     appOrigin,
		cookieSecure:  strings.HasPrefix(appOrigin, "https://"),
		allowedEmails: allowed,
	}
}

// isAllowed reports whether email may request a magic link. An empty
// allowlist means no restriction.
func (h *AuthHandlers) isAllowed(email string) bool {
	if len(h.allowedEmails) == 0 {
		return true
	}
	_, ok := h.allowedEmails[strings.ToLower(email)]
	return ok
}

type magicLinkRequest struct {
	Email string `json:"email"`
}

// RequestMagicLink handles POST /v1/auth/magic-link.
func (h *AuthHandlers) RequestMagicLink(w http.ResponseWriter, r *http.Request) {
	var req magicLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	addr, err := mail.ParseAddress(req.Email)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_email", "email must be a valid address")
		return
	}

	if !h.isAllowed(addr.Address) {
		// Same response as success — don't let the endpoint be used to probe
		// which addresses are allowlisted.
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "sent"})
		return
	}

	token, err := h.tokens.Issue(r.Context(), addr.Address)
	if err != nil {
		if errors.Is(err, auth.ErrRateLimited) {
			writeError(w, http.StatusTooManyRequests, "rate_limited", "too many magic-link requests, try again later")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}

	link := h.appOrigin + "/auth/callback?token=" + url.QueryEscape(token)
	// Errors are intentionally not surfaced to the caller: the response
	// must not reveal whether the address exists or delivery succeeded.
	_ = h.sender.SendMagicLink(r.Context(), addr.Address, link)

	writeJSON(w, http.StatusAccepted, map[string]string{"status": "sent"})
}

// IssueTestToken handles POST /v1/auth/test-token — it skips the allowlist
// and email delivery, returning a raw magic-link token directly.
//
// Only ever registered when E2E_TEST_MODE is set (see NewServer); never
// wired up in a real deployment. Exists so live e2e tests can authenticate
// against a running backend over HTTP instead of duplicating the token
// store's Redis key format in TypeScript.
func (h *AuthHandlers) IssueTestToken(w http.ResponseWriter, r *http.Request) {
	var req magicLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	addr, err := mail.ParseAddress(req.Email)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_email", "email must be a valid address")
		return
	}
	token, err := h.tokens.Issue(r.Context(), addr.Address)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}

type callbackResponse struct {
	Email string `json:"email"`
}

// Callback handles GET /v1/auth/callback?token=... — it exchanges a
// magic-link token for a session, setting the session cookie.
func (h *AuthHandlers) Callback(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		writeError(w, http.StatusBadRequest, "missing_token", "token is required")
		return
	}

	addr, err := h.tokens.Consume(r.Context(), token)
	if err != nil {
		if errors.Is(err, auth.ErrNotFound) {
			writeError(w, http.StatusUnauthorized, "invalid_or_expired_link", "this link is invalid or has expired")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}

	sessionID, err := h.sessions.Create(r.Context(), addr)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}

	h.setSessionCookie(w, sessionID, int(auth.SessionTTL.Seconds()))
	writeJSON(w, http.StatusOK, callbackResponse{Email: addr})
}

// Logout handles POST /v1/auth/logout.
func (h *AuthHandlers) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(sessionCookieName)
	if err == nil {
		_ = h.sessions.Delete(r.Context(), cookie.Value)
	}
	h.setSessionCookie(w, "", -1)
	w.WriteHeader(http.StatusNoContent)
}

// Session handles GET /v1/auth/session. It's wrapped in RequireAuth, so a
// valid Session is always present in the request context here.
func (h *AuthHandlers) Session(w http.ResponseWriter, r *http.Request) {
	sess, ok := SessionFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", ErrUnauthorized.Error())
		return
	}
	writeJSON(w, http.StatusOK, callbackResponse{Email: sess.Email})
}

func (h *AuthHandlers) setSessionCookie(w http.ResponseWriter, value string, maxAge int) {
	// The frontend and API are cross-site in production (Firebase Hosting +
	// Cloud Run), so a credentialed fetch needs SameSite=None — which browsers
	// only honor alongside Secure. Locally (http, same registrable domain,
	// different port) Lax is what actually works: SameSite=None without
	// Secure is rejected outright.
	sameSite := http.SameSiteLaxMode
	if h.cookieSecure {
		sameSite = http.SameSiteNoneMode
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: sameSite,
	})
}
