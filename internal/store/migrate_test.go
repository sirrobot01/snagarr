package store

import (
	"context"
	"path/filepath"
	"testing"
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
