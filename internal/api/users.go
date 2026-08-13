package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/sirrobot01/snagarr/internal/store"
)

type userDTO struct {
	ID             int64      `json:"id"`
	Username       string     `json:"username"`
	Role           store.Role `json:"role"`
	TelegramUserID *int64     `json:"telegram_user_id"`
	TokenCount     int        `json:"token_count"`
	CreatedAt      time.Time  `json:"created_at"`
}

func newUserDTO(u store.User) userDTO {
	return userDTO{
		ID: u.ID, Username: u.Username, Role: u.Role,
		TelegramUserID: nullableInt(u.TelegramUserID),
		TokenCount:     u.TokenCount, CreatedAt: u.CreatedAt,
	}
}

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.store.Users(r.Context())
	if err != nil {
		s.writeStoreError(w, err, "users")
		return
	}
	dtos := make([]userDTO, 0, len(users))
	for _, u := range users {
		dtos = append(dtos, newUserDTO(u))
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": dtos})
}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username       string     `json:"username"`
		Password       string     `json:"password"`
		Role           store.Role `json:"role"`
		TelegramUserID int64      `json:"telegram_user_id"`
	}
	if !decode(w, r, &req) {
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Role == "" {
		req.Role = store.RoleMember
	}
	if !req.Role.Valid() {
		writeError(w, http.StatusBadRequest, codeBadRequest, "role must be admin or member")
		return
	}
	if !usernamePattern.MatchString(req.Username) {
		writeError(w, http.StatusBadRequest, codeBadRequest, "username must be 3–32 letters, numbers, dots, dashes, or underscores")
		return
	}
	if req.Password == "" {
		writeError(w, http.StatusBadRequest, codeBadRequest, "password is required")
		return
	}
	if _, err := s.store.UserByUsername(r.Context(), req.Username); err == nil {
		writeError(w, http.StatusConflict, codeConflict, "that username is already in use")
		return
	} else if !errors.Is(err, store.ErrNotFound) {
		s.writeStoreError(w, err, "users")
		return
	}
	passwordHash, err := hashPassword(req.Password)
	if err != nil {
		s.log.Error("password hashing failed", "error", err)
		writeError(w, http.StatusInternalServerError, codeInternal, "user could not be created")
		return
	}

	u := &store.User{
		Username: req.Username, PasswordHash: passwordHash,
		Role: req.Role, TelegramUserID: req.TelegramUserID,
	}
	if err := s.store.CreateUser(r.Context(), u); err != nil {
		s.writeStoreError(w, err, "user")
		return
	}
	writeJSON(w, http.StatusCreated, newUserDTO(*u))
}

func (s *Server) updateUser(w http.ResponseWriter, r *http.Request) {
	u, ok := s.loadUser(w, r)
	if !ok {
		return
	}
	req := struct {
		Role           *store.Role `json:"role"`
		TelegramUserID *int64      `json:"telegram_user_id"`
	}{}
	if !decode(w, r, &req) {
		return
	}

	if req.TelegramUserID != nil {
		u.TelegramUserID = *req.TelegramUserID
	}
	if req.Role != nil {
		if !req.Role.Valid() {
			writeError(w, http.StatusBadRequest, codeBadRequest, "role must be admin or member")
			return
		}
		if u.Role == store.RoleAdmin && *req.Role != store.RoleAdmin && !s.hasAnotherAdmin(w, r) {
			return
		}
		u.Role = *req.Role
	}

	if err := s.store.UpdateUser(r.Context(), u); err != nil {
		s.writeStoreError(w, err, "user")
		return
	}
	writeJSON(w, http.StatusOK, newUserDTO(*u))
}

func (s *Server) deleteUser(w http.ResponseWriter, r *http.Request) {
	u, ok := s.loadUser(w, r)
	if !ok {
		return
	}
	if u.Role == store.RoleAdmin && !s.hasAnotherAdmin(w, r) {
		return
	}
	if err := s.store.DeleteUser(r.Context(), u.ID); err != nil {
		s.writeStoreError(w, err, "user")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// hasAnotherAdmin stops the last admin locking everyone out of settings.
func (s *Server) hasAnotherAdmin(w http.ResponseWriter, r *http.Request) bool {
	count, err := s.store.AdminCount(r.Context())
	if err != nil {
		s.writeStoreError(w, err, "users")
		return false
	}
	if count <= 1 {
		writeError(w, http.StatusConflict, codeConflict, "the last admin cannot be removed or demoted")
		return false
	}
	return true
}

func (s *Server) listTokens(w http.ResponseWriter, r *http.Request) {
	u, ok := s.loadUser(w, r)
	if !ok {
		return
	}
	tokens, err := s.store.Tokens(r.Context(), u.ID)
	if err != nil {
		s.writeStoreError(w, err, "tokens")
		return
	}
	dtos := make([]map[string]any, 0, len(tokens))
	for _, t := range tokens {
		dtos = append(dtos, map[string]any{
			"id": t.ID, "name": t.Name, "prefix": t.Prefix,
			"created_at": t.CreatedAt, "last_used_at": nullableTime(t.LastUsedAt),
			"revoked": t.Revoked,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"tokens": dtos})
}

// createToken returns the only copy of the secret that will ever exist.
func (s *Server) createToken(w http.ResponseWriter, r *http.Request) {
	u, ok := s.loadUser(w, r)
	if !ok {
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if !decode(w, r, &req) {
		return
	}
	if req.Name == "" {
		req.Name = "New token"
	}

	token, secret, err := s.store.CreateToken(r.Context(), u.ID, req.Name)
	if err != nil {
		s.writeStoreError(w, err, "token")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id": token.ID, "name": token.Name, "token": secret, "created_at": token.CreatedAt,
	})
}

func (s *Server) revokeToken(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, codeBadRequest, "token id must be a number")
		return
	}
	if err := s.store.RevokeToken(r.Context(), id); err != nil {
		s.writeStoreError(w, err, "token")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) loadUser(w http.ResponseWriter, r *http.Request) (*store.User, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, codeBadRequest, "user id must be a number")
		return nil, false
	}
	u, err := s.store.User(r.Context(), id)
	if err != nil {
		s.writeStoreError(w, err, "user")
		return nil, false
	}
	return u, true
}
