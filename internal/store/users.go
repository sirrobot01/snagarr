package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Role decides what an account or client token may do. The household shares a
// list; roles keep members away from install-wide and destructive actions.
type Role string

const (
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
)

func (r Role) Valid() bool { return r == RoleAdmin || r == RoleMember }

// User is a household member. TelegramUserID is 0 when unset.
type User struct {
	ID             int64
	Username       string
	PasswordHash   string
	Role           Role
	TelegramUserID int64
	CreatedAt      time.Time
	TokenCount     int
}

func (s *Store) CreateUser(ctx context.Context, u *User) error {
	u.CreatedAt = time.Now().UTC()
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO users (username, password_hash, role, telegram_user_id, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		u.Username, u.PasswordHash, u.Role, nullInt(u.TelegramUserID), u.CreatedAt)
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	u.ID, err = res.LastInsertId()
	return err
}

const userColumns = `u.id, u.username, u.password_hash, u.role, u.telegram_user_id, u.created_at,
	(SELECT COUNT(*) FROM tokens t WHERE t.user_id = u.id AND t.revoked = 0)`

func scanUser(row interface{ Scan(...any) error }) (*User, error) {
	var u User
	var telegram sql.NullInt64
	if err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &telegram, &u.CreatedAt, &u.TokenCount); err != nil {
		return nil, err
	}
	u.TelegramUserID = telegram.Int64
	return &u, nil
}

// UserByUsername resolves an interactive account name case-insensitively.
func (s *Store) UserByUsername(ctx context.Context, username string) (*User, error) {
	u, err := scanUser(s.db.QueryRowContext(ctx,
		`SELECT `+userColumns+` FROM users u WHERE u.username = ? COLLATE NOCASE`, username))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get user by username: %w", err)
	}
	return u, nil
}

// PasswordAuthInitialized reports whether this installation already has an
// account.
func (s *Store) PasswordAuthInitialized(ctx context.Context) (bool, error) {
	var exists bool
	if err := s.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM users)`).Scan(&exists); err != nil {
		return false, fmt.Errorf("check account setup: %w", err)
	}
	return exists, nil
}

// RegisterFirstAdmin creates the initial account and its browser session in a
// single transaction, making the public registration route one-time even if
// two requests arrive together.
func (s *Store) RegisterFirstAdmin(ctx context.Context, u *User) (string, error) {
	secret, err := newTokenSecret()
	if err != nil {
		return "", err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("begin registration: %w", err)
	}
	defer func(tx *sql.Tx) {
		_ = tx.Rollback()
	}(tx)

	var initialized bool
	if err := tx.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM users)`).Scan(&initialized); err != nil {
		return "", fmt.Errorf("check registration: %w", err)
	}
	if initialized {
		return "", ErrConflict
	}

	u.Role = RoleAdmin
	u.CreatedAt = time.Now().UTC()
	res, err := tx.ExecContext(ctx,
		`INSERT INTO users (username, password_hash, role, created_at) VALUES (?, ?, ?, ?)`,
		u.Username, u.PasswordHash, u.Role, u.CreatedAt)
	if err != nil {
		return "", fmt.Errorf("register admin: %w", err)
	}
	if u.ID, err = res.LastInsertId(); err != nil {
		return "", err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO tokens (user_id, name, token_hash, prefix, created_at) VALUES (?, ?, ?, ?, ?)`,
		u.ID, "Browser session", hashToken(secret), secret[:len(TokenPrefix)+4], u.CreatedAt); err != nil {
		return "", fmt.Errorf("create registration session: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("finish registration: %w", err)
	}
	return secret, nil
}

func (s *Store) User(ctx context.Context, id int64) (*User, error) {
	u, err := scanUser(s.db.QueryRowContext(ctx, `SELECT `+userColumns+` FROM users u WHERE u.id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	return u, nil
}

// UserByTelegram maps an inbound Telegram message to a household member. An
// unknown ID returns ErrNotFound, which is what makes the user table double as
// the bot's allow-list.
func (s *Store) UserByTelegram(ctx context.Context, telegramID int64) (*User, error) {
	u, err := scanUser(s.db.QueryRowContext(ctx,
		`SELECT `+userColumns+` FROM users u WHERE u.telegram_user_id = ?`, telegramID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get user by telegram id: %w", err)
	}
	return u, nil
}

func (s *Store) Users(ctx context.Context) ([]User, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+userColumns+` FROM users u ORDER BY u.id`)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, fmt.Errorf("list users: %w", err)
		}
		users = append(users, *u)
	}
	return users, rows.Err()
}

func (s *Store) UpdateUser(ctx context.Context, u *User) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE users SET username = ?, password_hash = ?, role = ?, telegram_user_id = ? WHERE id = ?`,
		u.Username, u.PasswordHash, u.Role, nullInt(u.TelegramUserID), u.ID)
	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}
	return affected(res)
}

func (s *Store) DeleteUser(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	return affected(res)
}

// AdminCount guards the last-admin rule: the API refuses to delete or demote
// the only remaining admin.
func (s *Store) AdminCount(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE role = ?`, RoleAdmin).Scan(&n)
	return n, err
}

func affected(res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
