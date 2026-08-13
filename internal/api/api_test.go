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
	"strings"
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
	if _, ok := got["tmdb"]["locked"]; !ok {
		t.Error("sections carry no locked flag; the settings UI needs it")
	}
	// Every integration except TMDB is a service now.
	for _, gone := range []string{"library", "radarr", "sonarr", "overseerr", "ntfy", "telegram"} {
		if _, ok := got[gone]; ok {
			t.Errorf("settings still carry the %q section", gone)
		}
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

type servicesResponse struct {
	Services []serviceDTO `json:"services"`
}

// Every member owns their own stack: they may build and change it, and nobody
// else's.
func TestServiceOwnership(t *testing.T) {
	h := newHarness(t)

	resp := h.do(t, http.MethodPost, "/api/v1/services", h.memberToken, map[string]any{
		"kind": "radarr", "name": "Amina's",
		"config": map[string]any{"url": "http://radarr:7878", "api_key": "member-key-4e2a"},
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST /services as member = %d, want 201", resp.StatusCode)
	}
	memberService := decodeBody[serviceDTO](t, resp)
	if memberService.UserID != h.member.ID {
		t.Errorf("service owner = %d, want the calling member %d", memberService.UserID, h.member.ID)
	}
	if secret, _ := memberService.Config["api_key"].(string); !config.IsMasked(secret) {
		t.Errorf("api_key = %q, want it masked", secret)
	}
	if searchOnAdd, _ := memberService.Config["search_on_add"].(bool); !searchOnAdd {
		t.Error("the kind's defaults were not applied to a new service")
	}

	resp = h.do(t, http.MethodPost, "/api/v1/services", h.adminToken, map[string]any{
		"kind": "sonarr", "config": map[string]any{"url": "http://sonarr:8989", "api_key": "admin-key"},
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST /services as admin = %d, want 201", resp.StatusCode)
	}
	adminService := decodeBody[serviceDTO](t, resp)

	listed := decodeBody[servicesResponse](t, h.do(t, http.MethodGet, "/api/v1/services", h.memberToken, nil))
	if len(listed.Services) != 1 || listed.Services[0].ID != memberService.ID {
		t.Errorf("member sees %v, want only their own service", listed.Services)
	}

	// Another member's service is out of reach.
	adminPath := "/api/v1/services/" + strconv.FormatInt(adminService.ID, 10)
	for _, route := range []struct{ method, path string }{
		{http.MethodPatch, adminPath},
		{http.MethodDelete, adminPath},
		{http.MethodPost, adminPath + "/test"},
		{http.MethodGet, adminPath + "/options"},
	} {
		if resp := h.do(t, route.method, route.path, h.memberToken, map[string]any{}); resp.StatusCode != http.StatusForbidden {
			t.Errorf("%s %s as member = %d, want 403", route.method, route.path, resp.StatusCode)
		}
	}

	// An admin reaches everybody's.
	memberPath := "/api/v1/services/" + strconv.FormatInt(memberService.ID, 10)
	if resp := h.do(t, http.MethodPatch, memberPath, h.adminToken, map[string]any{"enabled": false}); resp.StatusCode != http.StatusOK {
		t.Errorf("admin patching a member's service = %d, want 200", resp.StatusCode)
	}
	memberServices := "/api/v1/users/" + strconv.FormatInt(h.member.ID, 10) + "/services"
	if resp := h.do(t, http.MethodGet, memberServices, h.memberToken, nil); resp.StatusCode != http.StatusForbidden {
		t.Errorf("member reading the household view = %d, want 403", resp.StatusCode)
	}
	listed = decodeBody[servicesResponse](t, h.do(t, http.MethodGet, memberServices, h.adminToken, nil))
	if len(listed.Services) != 1 || listed.Services[0].Enabled {
		t.Errorf("admin view of the member's services = %v, want the disabled one", listed.Services)
	}

	if resp := h.do(t, http.MethodDelete, memberPath, h.memberToken, nil); resp.StatusCode != http.StatusNoContent {
		t.Errorf("member deleting their own service = %d, want 204", resp.StatusCode)
	}
}

// A client only ever sees a masked credential, so echoing it back must leave
// the stored one alone.
func TestServiceSecretsRoundTrip(t *testing.T) {
	h := newHarness(t)

	resp := h.do(t, http.MethodPost, "/api/v1/services", h.adminToken, map[string]any{
		"kind":   "radarr",
		"config": map[string]any{"url": "http://radarr:7878", "api_key": "super-secret-4e2a"},
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST /services = %d, want 201", resp.StatusCode)
	}
	created := decodeBody[serviceDTO](t, resp)
	masked, _ := created.Config["api_key"].(string)
	if masked == "super-secret-4e2a" {
		t.Fatal("the API key came back in clear text")
	}

	resp = h.do(t, http.MethodPatch, "/api/v1/services/"+strconv.FormatInt(created.ID, 10), h.adminToken,
		map[string]any{"config": map[string]any{"api_key": masked, "root_folder": "/data/movies"}})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PATCH /services = %d, want 200", resp.StatusCode)
	}
	if got := decodeBody[serviceDTO](t, resp); got.Config["root_folder"] != "/data/movies" {
		t.Errorf("root_folder = %v, want the patched value", got.Config["root_folder"])
	}

	stored, err := h.store.Service(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("read stored service: %v", err)
	}
	if !bytes.Contains(stored.Config, []byte("super-secret-4e2a")) {
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

// The bookmarklet posts from whatever page the user is reading, so the browser
// sends a preflight first. Without an answer to it, desktop capture cannot work.
func TestCrossOriginPreflight(t *testing.T) {
	h := newHarness(t)

	req, err := http.NewRequest(http.MethodOptions, h.server.URL+"/api/v1/capture", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Origin", "https://letterboxd.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "authorization,content-type")

	resp, err := h.server.Client().Do(req)
	if err != nil {
		t.Fatalf("preflight: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("preflight status = %d, want 204", resp.StatusCode)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("Access-Control-Allow-Origin = %q, want *", got)
	}
	if got := resp.Header.Get("Access-Control-Allow-Headers"); !strings.Contains(strings.ToLower(got), "authorization") {
		t.Errorf("Access-Control-Allow-Headers = %q, want it to include authorization", got)
	}
}

// The operator has to paste this into Radarr and Tautulli, so it must be
// readable rather than masked like a credential.
func TestWebhookSecretIsReadable(t *testing.T) {
	h := newHarness(t)

	resp := h.do(t, http.MethodGet, "/api/v1/settings", h.adminToken, nil)
	got := decodeBody[map[string]map[string]any](t, resp)
	secret, _ := got["general"]["webhook_secret"].(string)

	if secret == "" {
		t.Fatal("no webhook secret in the settings response")
	}
	if config.IsMasked(secret) {
		t.Errorf("webhook_secret = %q, want it readable so it can be configured", secret)
	}

	// It must be the value the webhook routes actually accept.
	resp = h.do(t, http.MethodPost, "/api/v1/webhooks/radarr?secret="+secret, "",
		map[string]any{"eventType": "Test"})
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("webhook with the reported secret = %d, want 204", resp.StatusCode)
	}
}

// Per-user services only mean something if a member can spend their own. They
// must not reach an admin's, which is what the admin-only route used to guard.
func TestMemberSendsOnlyToOwnServices(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	adminRadarr := &store.Service{
		UserID: h.admin.ID, Kind: store.KindRadarr, Name: "Default", Enabled: true,
		Config: []byte(`{"url":"http://radarr.lan:7878","api_key":"k"}`),
	}
	if err := h.store.CreateService(ctx, adminRadarr); err != nil {
		t.Fatalf("seed service: %v", err)
	}

	it := &store.Item{
		Title: "Anora", RawInput: "anora", Status: store.StatusNew, Source: store.SourceWeb,
		TMDBID: 1064213, MediaType: media.Movie, CapturedBy: h.member.ID,
	}
	if err := h.store.CreateItem(ctx, it); err != nil {
		t.Fatalf("seed item: %v", err)
	}

	// The member owns no Radarr, so this must fail rather than borrow the admin's.
	resp := h.do(t, http.MethodPost, "/api/v1/items/"+strconv.FormatInt(it.ID, 10)+"/send",
		h.memberToken, map[string]any{"target": "radarr"})
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("member send with no own Radarr = %d, want 503", resp.StatusCode)
	}
}
