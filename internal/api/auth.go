package api

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"net/http"
	"regexp"
	"strings"

	"github.com/sirrobot01/snagarr/internal/store"
	"golang.org/x/crypto/bcrypt"
)

type contextKey int

const userKey contextKey = iota

var usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{2,31}$`)

func (s *Server) authStatus(w http.ResponseWriter, r *http.Request) {
	initialized, err := s.store.PasswordAuthInitialized(r.Context())
	if err != nil {
		s.writeStoreError(w, err, "account status")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{
		"initialized":    initialized,
		"setup_required": !s.settings.Get().TMDB.Configured(),
	})
}

func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decode(w, r, &req) {
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if !usernamePattern.MatchString(req.Username) {
		writeError(w, http.StatusBadRequest, codeBadRequest, "username must be 3–32 letters, numbers, dots, dashes, or underscores")
		return
	}
	if req.Password == "" {
		writeError(w, http.StatusBadRequest, codeBadRequest, "password is required")
		return
	}
	hash, err := hashPassword(req.Password)
	if err != nil {
		s.log.Error("password hashing failed", "error", err)
		writeError(w, http.StatusInternalServerError, codeInternal, "account could not be created")
		return
	}
	u := &store.User{Username: req.Username, PasswordHash: hash}
	secret, err := s.store.RegisterFirstAdmin(r.Context(), u)
	if errors.Is(err, store.ErrConflict) {
		writeError(w, http.StatusConflict, codeConflict, "this Snagarr installation is already registered")
		return
	}
	if err != nil {
		s.writeStoreError(w, err, "account")
		return
	}
	// Environment-provided integrations now have an owner and can be seeded.
	if err := s.settings.SeedServices(r.Context(), s.store); err != nil {
		s.log.Error("seed services after registration", "error", err)
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"token": secret,
		"user":  userRef{ID: u.ID, Username: u.Username, Role: u.Role},
	})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decode(w, r, &req) {
		return
	}
	u, err := s.store.UserByUsername(r.Context(), strings.TrimSpace(req.Username))
	if err != nil || u.PasswordHash == "" || !passwordMatches(u.PasswordHash, req.Password) {
		writeError(w, http.StatusUnauthorized, codeUnauthorized, "username or password is incorrect")
		return
	}
	_, secret, err := s.store.CreateToken(r.Context(), u.ID, "Browser session")
	if err != nil {
		s.writeStoreError(w, err, "session")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"token": secret,
		"user":  userRef{ID: u.ID, Username: u.Username, Role: u.Role},
	})
}

// Pre-hashing removes bcrypt's 72-byte input ceiling without imposing a
// product-level password length policy. The raw comparison keeps local hashes
// created by earlier development builds usable.
func hashPassword(password string) (string, error) {
	digest := sha256.Sum256([]byte(password))
	hash, err := bcrypt.GenerateFromPassword(digest[:], bcrypt.DefaultCost)
	return string(hash), err
}

func passwordMatches(hash, password string) bool {
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil {
		return true
	}
	digest := sha256.Sum256([]byte(password))
	return bcrypt.CompareHashAndPassword([]byte(hash), digest[:]) == nil
}

// authenticate resolves the bearer token to a user. Every protected API route
// below /api/v1 goes through it; health, account entry and webhooks are public.
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

// allowCrossOrigin lets the bookmarklet and other page-embedded clients reach
// the API from any origin. A wildcard is safe here because authentication is a
// bearer token: there are no cookies for a hostile page to ride on, and it must
// already hold a token to get anything back.
func allowCrossOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Max-Age", "86400")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
