package api

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"

	"github.com/sirrobot01/snagarr/internal/store"
)

type contextKey int

const userKey contextKey = iota

// authenticate resolves the bearer token to a user. Every API route below
// /api/v1 except the health check and the webhooks goes through it.
func (s *Server) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok || token == "" {
			writeError(w, http.StatusUnauthorized, codeUnauthorized, "a bearer token is required")
			return
		}
		user, err := s.store.Authenticate(r.Context(), strings.TrimSpace(token))
		if err != nil {
			if !errors.Is(err, store.ErrNotFound) {
				s.log.Error("authentication failed", "error", err)
			}
			writeError(w, http.StatusUnauthorized, codeUnauthorized, "the token is not valid")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey, user)))
	})
}

// requireAdmin gates the destructive and configuration routes.
func requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if userFrom(r).Role != store.RoleAdmin {
			writeError(w, http.StatusForbidden, codeForbidden, "this action is restricted to admins")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func userFrom(r *http.Request) *store.User {
	user, _ := r.Context().Value(userKey).(*store.User)
	return user
}

// verifyWebhookSecret guards the webhook routes. Radarr, Tautulli and Emby
// cannot all set headers, so the shared secret travels in the query string.
func (s *Server) verifyWebhookSecret(r *http.Request) bool {
	want := s.settings.Get().General.WebhookSecret
	got := r.URL.Query().Get("secret")
	return want != "" && subtle.ConstantTimeCompare([]byte(want), []byte(got)) == 1
}
