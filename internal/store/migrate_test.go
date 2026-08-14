package store

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

// An install that predates section tracking must gain the column and be told to
// sweep again, or its collections would drain until the next daily sweep.
func TestMigrationAddsSectionAndForcesSweep(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "old.db")
	key := make([]byte, 32)

	s, err := Open(path, key)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := s.db.Exec(`PRAGMA user_version = 1`); err != nil {
		t.Fatalf("rewind version: %v", err)
	}
	if _, err := s.db.Exec(`ALTER TABLE library_index DROP COLUMN section_id`); err != nil {
		t.Fatalf("drop column: %v", err)
	}
	if _, err := s.db.Exec(`ALTER TABLE tokens DROP COLUMN session`); err != nil {
		t.Fatalf("drop column: %v", err)
	}
	if err := s.SetSetting(ctx, "sync_state", []byte(`{"arr_at":"2026-01-01T00:00:00Z"}`)); err != nil {
		t.Fatalf("seed sync state: %v", err)
	}
	s.Close()

	s, err = Open(path, key)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer s.Close()

	var version int
	if err := s.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatalf("version: %v", err)
	}
	if version != len(migrations) {
		t.Errorf("user_version = %d, want %d", version, len(migrations))
	}
	if _, err := s.db.Exec(`SELECT section_id FROM library_index`); err != nil {
		t.Errorf("section_id missing after migration: %v", err)
	}
	if _, err := s.Setting(ctx, "sync_state"); err == nil {
		t.Error("sync_state survived the migration; the next pass would not sweep in full")
	}
}

// Sessions created by builds that predate the session column must expire like
// the ones the login handler issues today. The migration recognises them by
// the only marker those rows carry: the name the handler gave them.
func TestMigrationMarksExistingBrowserSessions(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "old.db")
	key := make([]byte, 32)

	s, err := Open(path, key)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := s.db.Exec(`PRAGMA user_version = 2`); err != nil {
		t.Fatalf("rewind version: %v", err)
	}
	if _, err := s.db.Exec(`ALTER TABLE tokens DROP COLUMN session`); err != nil {
		t.Fatalf("drop column: %v", err)
	}
	u := newTestUser(t, s, "Mukhtar", RoleAdmin)
	for i, name := range []string{"Browser session", "iPhone Shortcut"} {
		if _, err := s.db.ExecContext(ctx,
			`INSERT INTO tokens (user_id, name, token_hash, prefix, created_at) VALUES (?, ?, ?, ?, ?)`,
			u.ID, name, fmt.Sprintf("hash-%d", i), "sngr_", time.Now().UTC()); err != nil {
			t.Fatalf("seed %s: %v", name, err)
		}
	}
	s.Close()

	s, err = Open(path, key)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer s.Close()

	tokens, err := s.Tokens(ctx, u.ID)
	if err != nil {
		t.Fatalf("list tokens: %v", err)
	}
	want := map[string]bool{"Browser session": true, "iPhone Shortcut": false}
	for _, tok := range tokens {
		if tok.Session != want[tok.Name] {
			t.Errorf("%s session = %v, want %v", tok.Name, tok.Session, want[tok.Name])
		}
	}
}
