package store

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
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

func newTestUser(t *testing.T, s *Store, name string, role Role) *User {
	t.Helper()
	u := &User{DisplayName: name, Role: role}
	if err := s.CreateUser(context.Background(), u); err != nil {
		t.Fatalf("create user: %v", err)
	}
	return u
}

// newTestService owns the index rows a test writes; every index row needs one.
func newTestService(t *testing.T, s *Store, userID int64, kind ServiceKind, name string) *Service {
	t.Helper()
	svc := &Service{
		UserID: userID, Kind: kind, Name: name, Enabled: true,
		Config: []byte(`{"url":"http://localhost","api_key":"k","token":"k"}`),
	}
	if err := s.CreateService(context.Background(), svc); err != nil {
		t.Fatalf("create service: %v", err)
	}
	return svc
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
	u := newTestUser(t, s, "Mukhtar", RoleAdmin)

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
	u := newTestUser(t, s, "Mukhtar", RoleAdmin)

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
	u := newTestUser(t, s, "Mukhtar", RoleAdmin)
	plex := newTestService(t, s, u.ID, KindPlex, "Default")
	radarr := newTestService(t, s, u.ID, KindRadarr, "Default")
	overseerr := newTestService(t, s, u.ID, KindOverseerr, "Default")

	if err := s.UpsertLibrary(ctx, plex.ID, []LibraryEntry{
		{ProviderItemID: "1001", TMDBID: 100, MediaType: media.Movie, Title: "Sinners", Year: 2025},
	}); err != nil {
		t.Fatalf("upsert library: %v", err)
	}
	if err := s.ReplaceArrIndex(ctx, radarr.ID, []ArrEntry{
		{ArrID: 1, TMDBID: 200, Title: "Anora", Monitored: true},
		{ArrID: 2, TMDBID: 300, Title: "The Substance", Monitored: true, HasFile: true},
	}); err != nil {
		t.Fatalf("replace arr index: %v", err)
	}
	if err := s.ReplaceRequests(ctx, overseerr.ID, []RequestEntry{
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
	u := newTestUser(t, s, "Mukhtar", RoleAdmin)
	plex := newTestService(t, s, u.ID, KindPlex, "Default")

	if err := s.UpsertLibrary(ctx, plex.ID, []LibraryEntry{
		{ProviderItemID: "1", TMDBID: 1, MediaType: media.Movie, Title: "Kept"},
		{ProviderItemID: "2", TMDBID: 2, MediaType: media.Movie, Title: "Deleted"},
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	time.Sleep(10 * time.Millisecond)
	sweepStart := time.Now().UTC()
	time.Sleep(10 * time.Millisecond)

	if err := s.UpsertLibrary(ctx, plex.ID, []LibraryEntry{
		{ProviderItemID: "1", TMDBID: 1, MediaType: media.Movie, Title: "Kept"},
	}); err != nil {
		t.Fatalf("resweep: %v", err)
	}
	removed, err := s.TombstoneLibrary(ctx, plex.ID, sweepStart)
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
	u := newTestUser(t, s, "Mukhtar", RoleAdmin)
	plex := newTestService(t, s, u.ID, KindPlex, "Default")

	if err := s.UpsertLibrary(ctx, plex.ID, []LibraryEntry{
		{ProviderItemID: "1", TMDBID: 1, MediaType: media.Movie, Title: "Sinners", Year: 2025},
		{ProviderItemID: "2", TMDBID: 2, MediaType: media.TV, Title: "Severance", Year: 2022},
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

// The household shares one list, so a title only one member holds still counts
// for everybody.
func TestStateIndexUnionsTheHousehold(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	a := newTestUser(t, s, "Mukhtar", RoleAdmin)
	b := newTestUser(t, s, "Amina", RoleMember)

	plex := newTestService(t, s, a.ID, KindPlex, "Living room")
	radarr := newTestService(t, s, b.ID, KindRadarr, "Mine")

	if err := s.UpsertLibrary(ctx, plex.ID, []LibraryEntry{
		{ProviderItemID: "1001", TMDBID: 100, MediaType: media.Movie, Title: "Sinners"},
	}); err != nil {
		t.Fatalf("upsert library: %v", err)
	}
	if err := s.ReplaceArrIndex(ctx, radarr.ID, []ArrEntry{
		{ArrID: 7, TMDBID: 200, Title: "Anora", Monitored: true},
	}); err != nil {
		t.Fatalf("replace arr index: %v", err)
	}

	idx, err := s.LoadStateIndex(ctx)
	if err != nil {
		t.Fatalf("load state index: %v", err)
	}
	if got := idx.State(TitleKey{100, media.Movie}); got != StatusAvailable {
		t.Errorf("state of A's library title = %q, want available", got)
	}
	if got := idx.State(TitleKey{200, media.Movie}); got != StatusMonitored {
		t.Errorf("state of B's monitored title = %q, want monitored", got)
	}

	// Collections are personal, so membership stays attributed to its server.
	members, err := s.LibraryMembers(ctx)
	if err != nil {
		t.Fatalf("library members: %v", err)
	}
	if got := members[plex.ID][TitleKey{100, media.Movie}]; got != "1001" {
		t.Errorf("plex member id = %q, want 1001", got)
	}
	if len(members) != 1 {
		t.Errorf("library members cover %d services, want only the one with titles", len(members))
	}
}

func TestServiceLifecycle(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	u := newTestUser(t, s, "Mukhtar", RoleAdmin)

	svc := &Service{
		UserID: u.ID, Kind: KindRadarr, Name: "Default", Enabled: true,
		Config: []byte(`{"url":"http://radarr:7878","api_key":"super-secret-4e2a"}`),
	}
	if err := s.CreateService(ctx, svc); err != nil {
		t.Fatalf("create service: %v", err)
	}

	var stored []byte
	if err := s.db.QueryRowContext(ctx, `SELECT config FROM services WHERE id = ?`, svc.ID).Scan(&stored); err != nil {
		t.Fatalf("read raw: %v", err)
	}
	if bytes.Contains(stored, []byte("super-secret-4e2a")) {
		t.Error("the service config was stored in clear text")
	}

	got, err := s.Service(ctx, svc.ID)
	if err != nil {
		t.Fatalf("get service: %v", err)
	}
	if string(got.Config) != string(svc.Config) {
		t.Errorf("decrypted config = %s, want %s", got.Config, svc.Config)
	}

	svc.Name = "Kids"
	svc.Enabled = false
	if err := s.UpdateService(ctx, svc); err != nil {
		t.Fatalf("update service: %v", err)
	}
	enabled, err := s.Services(ctx)
	if err != nil {
		t.Fatalf("list services: %v", err)
	}
	if len(enabled) != 0 {
		t.Errorf("enabled services = %d, want a disabled one left out", len(enabled))
	}
	owned, err := s.UserServices(ctx, u.ID)
	if err != nil || len(owned) != 1 || owned[0].Name != "Kids" {
		t.Fatalf("user services = %v, %v; want the renamed one", owned, err)
	}

	if err := s.DeleteService(ctx, svc.ID); err != nil {
		t.Fatalf("delete service: %v", err)
	}
	if _, err := s.Service(ctx, svc.ID); err != ErrNotFound {
		t.Errorf("get deleted service = %v, want ErrNotFound", err)
	}
}

// The release before services shipped kept every integration in one settings
// blob. Migration moves the configured ones onto the operator who set them up.
func TestSettingsMigrateOntoTheFirstAdmin(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "snagarr.db")
	key := make([]byte, 32)

	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatalf("open raw database: %v", err)
	}
	if _, err := db.Exec(migrations[0]); err != nil {
		t.Fatalf("apply migration 1: %v", err)
	}
	if _, err := db.Exec(`PRAGMA user_version = 1`); err != nil {
		t.Fatalf("set version: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO users (display_name, role, created_at) VALUES (?, ?, ?)`,
		"Admin", RoleAdmin, time.Now().UTC()); err != nil {
		t.Fatalf("seed admin: %v", err)
	}
	legacy := &Store{db: db, key: key}
	blob := []byte(`{
		"tmdb":{"api_key":"tmdb-key"},
		"library":{"provider":"plex","url":"http://plex:32400","token":"px","collection_name":"Snagged"},
		"radarr":{"url":"http://radarr:7878","api_key":"radarr-key","root_folder":"/movies"},
		"sonarr":{"url":"","api_key":""},
		"general":{"public_url":"http://home"}}`)
	if err := legacy.SetSetting(ctx, settingsKey, blob); err != nil {
		t.Fatalf("seed settings: %v", err)
	}
	db.Close()

	s, err := Open(path, key)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	services, err := s.Services(ctx)
	if err != nil {
		t.Fatalf("list services: %v", err)
	}
	byKind := map[ServiceKind]Service{}
	for _, svc := range services {
		byKind[svc.Kind] = svc
	}
	if len(services) != 2 {
		t.Fatalf("migrated %d services, want plex and radarr only: %v", len(services), services)
	}
	radarr, ok := byKind[KindRadarr]
	if !ok {
		t.Fatal("the configured Radarr did not migrate")
	}
	if radarr.UserID != 1 || radarr.Name != "Default" {
		t.Errorf("migrated Radarr = user %d %q, want user 1 \"Default\"", radarr.UserID, radarr.Name)
	}
	var radarrConfig map[string]any
	if err := json.Unmarshal(radarr.Config, &radarrConfig); err != nil {
		t.Fatalf("decode migrated config: %v", err)
	}
	if radarrConfig["api_key"] != "radarr-key" || radarrConfig["root_folder"] != "/movies" {
		t.Errorf("migrated Radarr config = %v, want the settings values", radarrConfig)
	}
	if _, ok := byKind[KindPlex]; !ok {
		t.Error("the library section did not migrate to a plex service")
	}
	if _, ok := byKind[KindSonarr]; ok {
		t.Error("an empty Sonarr section became a service")
	}

	raw, err := s.Setting(ctx, settingsKey)
	if err != nil {
		t.Fatalf("read migrated settings: %v", err)
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatalf("decode migrated settings: %v", err)
	}
	for _, section := range []string{"radarr", "sonarr", "library"} {
		if _, ok := document[section]; ok {
			t.Errorf("settings still carry the %q section", section)
		}
	}
	if _, ok := document["tmdb"]; !ok {
		t.Error("tmdb left the settings document; it stays global")
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
