package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/sirrobot01/snagarr/internal/media"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	key := make([]byte, 32)
	s, err := Open(filepath.Join(t.TempDir(), "snagarr.db"), key)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func newTestUser(t *testing.T, s *Store) *User {
	t.Helper()
	u := &User{DisplayName: "Mukhtar", Role: RoleAdmin}
	if err := s.CreateUser(context.Background(), u); err != nil {
		t.Fatalf("create user: %v", err)
	}
	return u
}

func TestMigrateIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	key := make([]byte, 32)
	for i := 0; i < 2; i++ {
		s, err := Open(filepath.Join(dir, "snagarr.db"), key)
		if err != nil {
			t.Fatalf("open %d: %v", i, err)
		}
		s.Close()
	}
}

func TestTokenAuthentication(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	u := newTestUser(t, s)

	tok, secret, err := s.CreateToken(ctx, u.ID, "iPhone Shortcut")
	if err != nil {
		t.Fatalf("create token: %v", err)
	}
	if got := secret[:len(TokenPrefix)]; got != TokenPrefix {
		t.Errorf("secret prefix = %q, want %q", got, TokenPrefix)
	}

	got, err := s.Authenticate(ctx, secret)
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	if got.ID != u.ID {
		t.Errorf("authenticated user = %d, want %d", got.ID, u.ID)
	}

	if _, err := s.Authenticate(ctx, "sngr_wrong"); err != ErrNotFound {
		t.Errorf("unknown token error = %v, want ErrNotFound", err)
	}

	if err := s.RevokeToken(ctx, tok.ID); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if _, err := s.Authenticate(ctx, secret); err != ErrNotFound {
		t.Errorf("revoked token error = %v, want ErrNotFound", err)
	}
}

func TestItemLifecycle(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	u := newTestUser(t, s)

	it := &Item{
		Title: "that vampire one w/ dafoe", Status: StatusNeedsReview,
		RawInput: "that vampire one w/ dafoe", Source: SourceTelegram, CapturedBy: u.ID,
	}
	if err := s.CreateItem(ctx, it); err != nil {
		t.Fatalf("create item: %v", err)
	}

	candidates := []Candidate{
		{TMDBID: 426063, MediaType: media.Movie, Title: "Nosferatu", Year: 2024, Score: 0.94},
		{TMDBID: 11800, MediaType: media.Movie, Title: "Shadow of the Vampire", Year: 2000, Score: 0.71},
	}
	if err := s.SetCandidates(ctx, it.ID, candidates); err != nil {
		t.Fatalf("set candidates: %v", err)
	}
	got, err := s.Candidates(ctx, it.ID)
	if err != nil || len(got) != 2 {
		t.Fatalf("candidates = %v, %v; want 2 rows", got, err)
	}
	if got[0].Score < got[1].Score {
		t.Error("candidates are not ordered by descending score")
	}

	if err := s.Resolve(ctx, it.ID, candidates[0]); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	resolved, err := s.Item(ctx, it.ID)
	if err != nil {
		t.Fatalf("get item: %v", err)
	}
	if resolved.Status != StatusNew {
		t.Errorf("status after resolve = %q, want %q", resolved.Status, StatusNew)
	}
	if resolved.TMDBID != 426063 || resolved.Year != 2024 {
		t.Errorf("resolved item = %d (%d), want 426063 (2024)", resolved.TMDBID, resolved.Year)
	}
	if resolved.RawInput != it.RawInput {
		t.Errorf("raw input = %q, want it preserved", resolved.RawInput)
	}
	if resolved.CapturedByName != "Mukhtar" {
		t.Errorf("captured_by name = %q, want attribution joined in", resolved.CapturedByName)
	}

	// Capture is idempotent per TMDB ID.
	dup, err := s.ItemByTMDB(ctx, 426063, media.Movie)
	if err != nil || dup.ID != it.ID {
		t.Errorf("ItemByTMDB = %v, %v; want the existing item", dup, err)
	}

	if err := s.SetArchived(ctx, it.ID, true); err != nil {
		t.Fatalf("archive: %v", err)
	}
	if _, total, _ := s.Items(ctx, ItemFilter{}); total != 0 {
		t.Errorf("default list total = %d, want archived items hidden", total)
	}
	if _, total, _ := s.Items(ctx, ItemFilter{Archived: true}); total != 1 {
		t.Errorf("archived list total = %d, want 1", total)
	}
}

func TestNeedsReviewItemsDoNotCollide(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	// The unique index is partial, so any number of unresolved items coexist.
	for _, raw := range []string{"the one with the whale", "that vampire one"} {
		if err := s.CreateItem(ctx, &Item{
			Title: raw, RawInput: raw, Status: StatusNeedsReview, Source: SourceTelegram,
		}); err != nil {
			t.Fatalf("create %q: %v", raw, err)
		}
	}
	if _, total, _ := s.Items(ctx, ItemFilter{Status: StatusNeedsReview}); total != 2 {
		t.Errorf("needs_review total = %d, want 2", total)
	}
}

func TestStateIndexArithmetic(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	if err := s.UpsertLibrary(ctx, []LibraryEntry{
		{Provider: "plex", ProviderItemID: "1001", TMDBID: 100, MediaType: media.Movie, Title: "Sinners", Year: 2025},
	}); err != nil {
		t.Fatalf("upsert library: %v", err)
	}
	if err := s.ReplaceArrIndex(ctx, "radarr", []ArrEntry{
		{Source: "radarr", ArrID: 1, TMDBID: 200, Title: "Anora", Monitored: true},
		{Source: "radarr", ArrID: 2, TMDBID: 300, Title: "The Substance", Monitored: true, HasFile: true},
	}); err != nil {
		t.Fatalf("replace arr index: %v", err)
	}
	if err := s.ReplaceRequests(ctx, []RequestEntry{
		{RequestID: 418, TMDBID: 400, MediaType: media.TV, Status: "pending"},
	}); err != nil {
		t.Fatalf("replace requests: %v", err)
	}

	idx, err := s.LoadStateIndex(ctx)
	if err != nil {
		t.Fatalf("load state index: %v", err)
	}

	tests := []struct {
		name string
		key  TitleKey
		want Status
	}{
		{"in library", TitleKey{100, media.Movie}, StatusAvailable},
		{"monitored without a file", TitleKey{200, media.Movie}, StatusMonitored},
		{"monitored with a file", TitleKey{300, media.Movie}, StatusAvailable},
		{"requested", TitleKey{400, media.TV}, StatusRequested},
		{"unknown", TitleKey{999, media.Movie}, StatusNew},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := idx.State(tt.key); got != tt.want {
				t.Errorf("State(%v) = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}

func TestTombstoneLibraryDropsUnseenTitles(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	if err := s.UpsertLibrary(ctx, []LibraryEntry{
		{Provider: "plex", ProviderItemID: "1", TMDBID: 1, MediaType: media.Movie, Title: "Kept"},
		{Provider: "plex", ProviderItemID: "2", TMDBID: 2, MediaType: media.Movie, Title: "Deleted"},
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	time.Sleep(10 * time.Millisecond)
	sweepStart := time.Now().UTC()
	time.Sleep(10 * time.Millisecond)

	if err := s.UpsertLibrary(ctx, []LibraryEntry{
		{Provider: "plex", ProviderItemID: "1", TMDBID: 1, MediaType: media.Movie, Title: "Kept"},
	}); err != nil {
		t.Fatalf("resweep: %v", err)
	}
	removed, err := s.TombstoneLibrary(ctx, "plex", sweepStart)
	if err != nil {
		t.Fatalf("tombstone: %v", err)
	}
	if removed != 1 {
		t.Errorf("removed = %d, want 1", removed)
	}
	if n, _ := s.LibraryCount(ctx); n != 1 {
		t.Errorf("library count = %d, want 1", n)
	}
}

func TestSearchLibrary(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	if err := s.UpsertLibrary(ctx, []LibraryEntry{
		{Provider: "plex", ProviderItemID: "1", TMDBID: 1, MediaType: media.Movie, Title: "Sinners", Year: 2025},
		{Provider: "plex", ProviderItemID: "2", TMDBID: 2, MediaType: media.TV, Title: "Severance", Year: 2022},
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	tests := []struct {
		query string
		want  string
	}{
		{"sinn", "Sinners"},
		{"Severance", "Severance"},
		{`sinn"*(`, "Sinners"}, // FTS5 syntax characters must not reach the matcher
	}
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			got, err := s.SearchLibrary(ctx, tt.query, 10)
			if err != nil {
				t.Fatalf("search %q: %v", tt.query, err)
			}
			if len(got) != 1 || got[0].Title != tt.want {
				t.Errorf("search %q = %v, want one %q", tt.query, got, tt.want)
			}
		})
	}

	if got, err := s.SearchLibrary(ctx, "   ", 10); err != nil || got != nil {
		t.Errorf("blank search = %v, %v; want no results and no error", got, err)
	}
}

func TestSettingsAreEncryptedAtRest(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	secret := []byte(`{"api_key":"4e2a-super-secret"}`)
	if err := s.SetSetting(ctx, "tmdb", secret); err != nil {
		t.Fatalf("set setting: %v", err)
	}

	var stored []byte
	if err := s.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, "tmdb").Scan(&stored); err != nil {
		t.Fatalf("read raw: %v", err)
	}
	if string(stored) == string(secret) {
		t.Error("setting was stored in clear text")
	}

	got, err := s.Setting(ctx, "tmdb")
	if err != nil {
		t.Fatalf("get setting: %v", err)
	}
	if string(got) != string(secret) {
		t.Errorf("decrypted = %q, want %q", got, secret)
	}

	if _, err := s.Setting(ctx, "missing"); err != ErrNotFound {
		t.Errorf("missing setting error = %v, want ErrNotFound", err)
	}
}

func TestCounts(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	seed := []struct {
		status   Status
		tmdbID   int64
		archived bool
	}{
		{StatusAvailable, 1, false},
		{StatusWatched, 2, false},
		{StatusMonitored, 3, false},
		{StatusNew, 4, false},
		{StatusNeedsReview, 0, false},
		{StatusAvailable, 5, true},
	}
	for _, sd := range seed {
		it := &Item{Title: "x", RawInput: "x", Status: sd.status, Source: SourceWeb, TMDBID: sd.tmdbID, MediaType: media.Movie}
		if err := s.CreateItem(ctx, it); err != nil {
			t.Fatalf("seed: %v", err)
		}
		if sd.archived {
			if err := s.SetArchived(ctx, it.ID, true); err != nil {
				t.Fatalf("archive: %v", err)
			}
		}
	}

	got, err := s.Counts(ctx)
	if err != nil {
		t.Fatalf("counts: %v", err)
	}
	want := Counts{Total: 5, Ready: 2, Pending: 2, NeedsReview: 1, Archived: 1}
	if got != want {
		t.Errorf("counts = %+v, want %+v", got, want)
	}
}
