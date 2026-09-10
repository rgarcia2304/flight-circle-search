package http

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rgarcia2304/flight-circle-search/internal/auth"
)

type contextKey int

const (
	sessionContextKey contextKey = iota
	requestIDContextKey
)

// SessionFromContext returns the authenticated session set by RequireAuth, if any.
func SessionFromContext(ctx context.Context) (auth.Session, bool) {
	sess, ok := ctx.Value(sessionContextKey).(auth.Session)
	return sess, ok
}

// RequestIDFromContext returns the request ID set by RequestID, if any.
func RequestIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(requestIDContextKey).(string)
	return id, ok
}

// RequireAuth rejects requests without a valid session cookie with 401,
// and otherwise makes the session available via SessionFromContext.
func RequireAuth(sessions *auth.SessionStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(sessionCookieName)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "unauthorized", ErrUnauthorized.Error())
				return
			}
			sess, err := sessions.Get(r.Context(), cookie.Value)
			if err != nil {
				if errors.Is(err, auth.ErrNotFound) {
					writeError(w, http.StatusUnauthorized, "unauthorized", ErrUnauthorized.Error())
					return
				}
				writeError(w, http.StatusInternalServerError, "internal", err.Error())
				return
			}
			ctx := context.WithValue(r.Context(), sessionContextKey, sess)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequestID assigns a unique ID to each request, exposing it via the
// X-Request-ID response header and RequestIDFromContext.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := uuid.NewString()
		w.Header().Set("X-Request-ID", id)
		ctx := context.WithValue(r.Context(), requestIDContextKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (rec *statusRecorder) WriteHeader(status int) {
	rec.status = status
	rec.ResponseWriter.WriteHeader(status)
}

// AccessLog logs method, path, status, duration and request ID for every request.
func AccessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		requestID, _ := RequestIDFromContext(r.Context())
		slog.Info("http_request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"request_id", requestID,
		)
	})
}

// SecurityHeaders adds baseline security headers to every response.
func SecurityHeaders(appOrigin string) func(http.Handler) http.Handler {
	isHTTPS := strings.HasPrefix(appOrigin, "https://")
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			if isHTTPS {
				w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
			}
			next.ServeHTTP(w, r)
		})
	}
}

// CORS allows credentialed cross-origin requests from exactly appOrigin.
// A wildcard origin can't be used here since credentialed requests require
// the browser to see an exact origin match.
func CORS(appOrigin string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", appOrigin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Set("Vary", "Origin")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
