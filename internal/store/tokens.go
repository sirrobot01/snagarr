package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base32"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

// TokenPrefix marks Snagarr tokens so they are recognisable in logs and secret
// scanners.
const TokenPrefix = "sngr_"

// Token is the metadata of an issued token. The secret itself is never stored,
// only its SHA-256 digest.
type Token struct {
	ID         int64
	UserID     int64
	Name       string
	Prefix     string
	CreatedAt  time.Time
	LastUsedAt time.Time
	Revoked    bool
}

// CreateToken issues a token for a user and returns it alongside the raw
// secret. The secret is readable exactly once — it cannot be recovered later.
func (s *Store) CreateToken(ctx context.Context, userID int64, name string) (*Token, string, error) {
	buf := make([]byte, 20)
	if _, err := rand.Read(buf); err != nil {
		return nil, "", fmt.Errorf("generate token: %w", err)
	}
	secret := TokenPrefix + strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(buf))

	t := &Token{
		UserID:    userID,
		Name:      name,
		Prefix:    secret[:len(TokenPrefix)+4],
		CreatedAt: time.Now().UTC(),
	}
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO tokens (user_id, name, token_hash, prefix, created_at) VALUES (?, ?, ?, ?, ?)`,
		t.UserID, t.Name, hashToken(secret), t.Prefix, t.CreatedAt)
	if err != nil {
		return nil, "", fmt.Errorf("create token: %w", err)
	}
	if t.ID, err = res.LastInsertId(); err != nil {
		return nil, "", err
	}
	return t, secret, nil
}

// Authenticate resolves a raw token to its owner and stamps the token as used.
// An unknown or revoked token returns ErrNotFound.
func (s *Store) Authenticate(ctx context.Context, secret string) (*User, error) {
	var tokenID, userID int64
	err := s.db.QueryRowContext(ctx,
		`SELECT id, user_id FROM tokens WHERE token_hash = ? AND revoked = 0`, hashToken(secret)).
		Scan(&tokenID, &userID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("authenticate: %w", err)
	}
	if _, err := s.db.ExecContext(ctx,
		`UPDATE tokens SET last_used_at = ? WHERE id = ?`, time.Now().UTC(), tokenID); err != nil {
		return nil, fmt.Errorf("stamp token: %w", err)
	}
	return s.User(ctx, userID)
}

func (s *Store) Tokens(ctx context.Context, userID int64) ([]Token, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, user_id, name, prefix, created_at, last_used_at, revoked
		 FROM tokens WHERE user_id = ? ORDER BY id`, userID)
	if err != nil {
		return nil, fmt.Errorf("list tokens: %w", err)
	}
	defer rows.Close()

	var tokens []Token
	for rows.Next() {
		var t Token
		var lastUsed sql.NullTime
		if err := rows.Scan(&t.ID, &t.UserID, &t.Name, &t.Prefix, &t.CreatedAt, &lastUsed, &t.Revoked); err != nil {
			return nil, fmt.Errorf("list tokens: %w", err)
		}
		t.LastUsedAt = lastUsed.Time
		tokens = append(tokens, t)
	}
	return tokens, rows.Err()
}

// RevokeToken keeps the row so the audit trail survives.
func (s *Store) RevokeToken(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `UPDATE tokens SET revoked = 1 WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("revoke token: %w", err)
	}
	return affected(res)
}

func (s *Store) TokenOwner(ctx context.Context, id int64) (int64, error) {
	var userID int64
	err := s.db.QueryRowContext(ctx, `SELECT user_id FROM tokens WHERE id = ?`, id).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	return userID, err
}

// Tokens carry 160 bits of entropy, so a plain digest is enough — there is no
// low-entropy secret here for a slow KDF to protect.
func hashToken(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}
