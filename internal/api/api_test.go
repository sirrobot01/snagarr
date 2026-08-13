package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/sirrobot01/snagarr/internal/config"
	"github.com/sirrobot01/snagarr/internal/media"
	"github.com/sirrobot01/snagarr/internal/reconcile"
	"github.com/sirrobot01/snagarr/internal/resolver"
	"github.com/sirrobot01/snagarr/internal/store"
)

type harness struct {
	server      *httptest.Server
	store       *store.Store
	adminToken  string
	memberToken string
	admin       *store.User
	member      *store.User
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	ctx := context.Background()

	db, err := store.Open(filepath.Join(t.TempDir(), "snagarr.db"), make([]byte, 32))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	settings, err := config.NewManager(ctx, db)
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	engine := reconcile.New(db, settings, log)
	handler := New(db, settings, resolver.New(db, log), engine,
		http.NotFoundHandler(), log).Handler()

	h := &harness{store: db, server: httptest.NewServer(handler)}
	t.Cleanup(h.server.Close)

	h.admin = &store.User{DisplayName: "Mukhtar", Role: store.RoleAdmin}
	if err := db.CreateUser(ctx, h.admin); err != nil {
		t.Fatalf("create admin: %v", err)
	}
	if _, h.adminToken, err = db.CreateToken(ctx, h.admin.ID, "admin"); err != nil {
		t.Fatalf("admin token: %v", err)
	}

	h.member = &store.User{DisplayName: "Amina", Role: store.RoleMember}
	if err := db.CreateUser(ctx, h.member); err != nil {
		t.Fatalf("create member: %v", err)
	}
	if _, h.memberToken, err = db.CreateToken(ctx, h.member.ID, "member"); err != nil {
		t.Fatalf("member token: %v", err)
	}
	return h
}

func (h *harness) do(t *testing.T, method, path, token string, body any) *http.Response {
	t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("encode body: %v", err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, h.server.URL+path, reader)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.server.Client().Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

func decodeBody[T any](t *testing.T, resp *http.Response) T {
	t.Helper()
	var v T
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return v
}

func TestAuthentication(t *testing.T) {
	h := newHarness(t)

	tests := []struct {
		name   string
		token  string
		status int
	}{
		{"no token", "", http.StatusUnauthorized},
		{"unknown token", "sngr_nonsense", http.StatusUnauthorized},
		{"valid token", h.adminToken, http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := h.do(t, http.MethodGet, "/api/v1/me", tt.token, nil)
			if resp.StatusCode != tt.status {
				t.Errorf("GET /me = %d, want %d", resp.StatusCode, tt.status)
			}
		})
	}
}

func TestHealthNeedsNoToken(t *testing.T) {
	h := newHarness(t)
	if resp := h.do(t, http.MethodGet, "/api/v1/health", "", nil); resp.StatusCode != http.StatusOK {
		t.Errorf("GET /health = %d, want 200", resp.StatusCode)
	}
}

func TestMembersCannotReachAdminRoutes(t *testing.T) {
	h := newHarness(t)

	adminOnly := []struct{ method, path string }{
		{http.MethodGet, "/api/v1/users"},
		{http.MethodGet, "/api/v1/settings"},
		{http.MethodPost, "/api/v1/admin/sync"},
	}
	for _, route := range adminOnly {
		t.Run(route.path, func(t *testing.T) {
			if resp := h.do(t, route.method, route.path, h.memberToken, nil); resp.StatusCode != http.StatusForbidden {
				t.Errorf("%s %s as member = %d, want 403", route.method, route.path, resp.StatusCode)
			}
			if resp := h.do(t, route.method, route.path, h.adminToken, nil); resp.StatusCode == http.StatusForbidden {
				t.Errorf("%s %s as admin = 403, want it allowed", route.method, route.path)
			}
		})
	}
}

// A capture that cannot be resolved must still be stored with its raw text —
// this is the "no capture is ever lost" guarantee.
func TestCaptureParksUnresolvableInput(t *testing.T) {
	h := newHarness(t)

	resp := h.do(t, http.MethodPost, "/api/v1/capture", h.memberToken, map[string]any{
		"query": "that vampire one w/ dafoe", "source": "telegram",
	})
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("POST /capture = %d, want 202", resp.StatusCode)
	}

	got := decodeBody[itemDTO](t, resp)
	if got.Status != store.StatusNeedsReview {
		t.Errorf("status = %q, want needs_review", got.Status)
	}
	if got.RawInput != "that vampire one w/ dafoe" {
		t.Errorf("raw_input = %q, want the original text", got.RawInput)
	}
	if got.Source != store.SourceTelegram {
		t.Errorf("source = %q, want telegram", got.Source)
	}
	if got.CapturedBy == nil || got.CapturedBy.DisplayName != "Amina" {
		t.Errorf("captured_by = %+v, want Amina attributed", got.CapturedBy)
	}
}

func TestCaptureRejectsEmptyInput(t *testing.T) {
	h := newHarness(t)
	resp := h.do(t, http.MethodPost, "/api/v1/capture", h.adminToken, map[string]any{})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("empty capture = %d, want 400", resp.StatusCode)
	}
}

func TestItemOwnershipRules(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	adminItem := &store.Item{
		Title: "Nosferatu", RawInput: "nosferatu", Status: store.StatusNew,
		Source: store.SourceWeb, TMDBID: 426063, MediaType: media.Movie, CapturedBy: h.admin.ID,
	}
	if err := h.store.CreateItem(ctx, adminItem); err != nil {
		t.Fatalf("seed: %v", err)
	}
	memberItem := &store.Item{
		Title: "Severance", RawInput: "severance", Status: store.StatusNew,
		Source: store.SourceTelegram, TMDBID: 95396, MediaType: media.TV, CapturedBy: h.member.ID,
	}
	if err := h.store.CreateItem(ctx, memberItem); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// A member may undo their own capture, which is what the undo toast does.
	path := "/api/v1/items/" + strconv.FormatInt(memberItem.ID, 10)
	if resp := h.do(t, http.MethodDelete, path, h.memberToken, nil); resp.StatusCode != http.StatusNoContent {
		t.Errorf("member deleting own item = %d, want 204", resp.StatusCode)
	}

	// Someone else's item is off limits.
	path = "/api/v1/items/" + strconv.FormatInt(adminItem.ID, 10)
	if resp := h.do(t, http.MethodDelete, path, h.memberToken, nil); resp.StatusCode != http.StatusForbidden {
		t.Errorf("member deleting another item = %d, want 403", resp.StatusCode)
	}
	if resp := h.do(t, http.MethodDelete, path, h.adminToken, nil); resp.StatusCode != http.StatusNoContent {
		t.Errorf("admin deleting any item = %d, want 204", resp.StatusCode)
	}
}

func TestListFiltersArchivedItems(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	it := &store.Item{
		Title: "Past Lives", RawInput: "past lives", Status: store.StatusAvailable,
		Source: store.SourceWeb, TMDBID: 666277, MediaType: media.Movie, CapturedBy: h.admin.ID,
	}
	if err := h.store.CreateItem(ctx, it); err != nil {
		t.Fatalf("seed: %v", err)
	}

	resp := h.do(t, http.MethodPost, "/api/v1/items/"+strconv.FormatInt(it.ID, 10)+"/archive", h.adminToken,
		map[string]any{"archived": true})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("archive = %d, want 200", resp.StatusCode)
	}
	if got := decodeBody[itemDTO](t, resp); !got.Archived {
		t.Error("archived flag did not come back set")
	}

	type listResponse struct {
		Items []itemDTO `json:"items"`
		Total int       `json:"total"`
	}
	if got := decodeBody[listResponse](t, h.do(t, http.MethodGet, "/api/v1/items", h.adminToken, nil)); got.Total != 0 {
		t.Errorf("default list total = %d, want archived items hidden", got.Total)
	}
	if got := decodeBody[listResponse](t, h.do(t, http.MethodGet, "/api/v1/items?archived=true", h.adminToken, nil)); got.Total != 1 {
		t.Errorf("archived list total = %d, want 1", got.Total)
	}
}

func TestSettingsMaskSecrets(t *testing.T) {
	h := newHarness(t)

	resp := h.do(t, http.MethodPut, "/api/v1/settings", h.adminToken, map[string]any{
		"tmdb": map[string]any{"api_key": "super-secret-4e2a"},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT /settings = %d, want 200", resp.StatusCode)
	}

	got := decodeBody[map[string]map[string]any](t, resp)
	masked, _ := got["tmdb"]["api_key"].(string)
	if masked == "super-secret-4e2a" {
		t.Error("the API key came back in clear text")
	}
	if !config.IsMasked(masked) {
		t.Errorf("api_key = %q, want it masked", masked)
	}
	if configured, _ := got["tmdb"]["configured"].(bool); !configured {
		t.Error("tmdb.configured = false after setting a key")
	}
	if _, ok := got["radarr"]["locked"]; !ok {
		t.Error("sections carry no locked flag; the settings UI needs it")
	}

	// Echoing the mask back must not overwrite the stored secret.
	resp = h.do(t, http.MethodPut, "/api/v1/settings", h.adminToken, map[string]any{
		"tmdb": map[string]any{"api_key": masked},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("second PUT /settings = %d, want 200", resp.StatusCode)
	}
	stored, err := h.store.Setting(context.Background(), "settings")
	if err != nil {
		t.Fatalf("read stored settings: %v", err)
	}
	if !bytes.Contains(stored, []byte("super-secret-4e2a")) {
		t.Error("echoing the mask back overwrote the stored secret")
	}
}

func TestWebhookRejectsWrongSecret(t *testing.T) {
	h := newHarness(t)

	resp := h.do(t, http.MethodPost, "/api/v1/webhooks/radarr?secret=wrong", "",
		map[string]any{"eventType": "Download"})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("wrong webhook secret = %d, want 401", resp.StatusCode)
	}
}

// Unmatched webhook payloads must still answer 204, or the sender retries.
func TestWebhookAcceptsUnmatchedPayload(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	settings, err := config.NewManager(ctx, h.store)
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	secret := settings.Get().General.WebhookSecret
	if secret == "" {
		t.Fatal("no webhook secret was generated on first run")
	}

	resp := h.do(t, http.MethodPost, "/api/v1/webhooks/radarr?secret="+secret, "",
		map[string]any{"eventType": "Download", "movie": map[string]any{"tmdbId": 999999}})
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("unmatched webhook = %d, want 204", resp.StatusCode)
	}
}
