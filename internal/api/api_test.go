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
	"github.com/sirrobot01/snagarr/internal/engine"
	"github.com/sirrobot01/snagarr/internal/store"
)

type harness struct {
	server      *httptest.Server
	api         *Server
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
	reconciler := engine.NewReconciler(db, settings, log)
	srv := New(db, settings, engine.NewResolver(db, log), reconciler, http.NotFoundHandler(), log)

	h := &harness{store: db, api: srv, server: httptest.NewServer(srv.Handler())}
	t.Cleanup(h.server.Close)

	h.admin = &store.User{Username: "Mukhtar", Role: store.RoleAdmin}
	if err := db.CreateUser(ctx, h.admin); err != nil {
		t.Fatalf("create admin: %v", err)
	}
	if _, h.adminToken, err = db.CreateToken(ctx, h.admin.ID, "admin", false); err != nil {
		t.Fatalf("admin token: %v", err)
	}

	h.member = &store.User{Username: "Amina", Role: store.RoleMember}
	if err := db.CreateUser(ctx, h.member); err != nil {
		t.Fatalf("create member: %v", err)
	}
	if _, h.memberToken, err = db.CreateToken(ctx, h.member.ID, "member", false); err != nil {
		t.Fatalf("member token: %v", err)
	}
	return h
}

func newEmptyHarness(t *testing.T) *harness {
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
	reconciler := engine.NewReconciler(db, settings, log)
	handler := New(db, settings, engine.NewResolver(db, log), reconciler,
		http.NotFoundHandler(), log).Handler()
	h := &harness{store: db, server: httptest.NewServer(handler)}
	t.Cleanup(h.server.Close)
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

// basic posts with a username and a password, the way a Radarr or Sonarr
// webhook connection does.
func (h *harness) basic(t *testing.T, path, username, password string, body any) *http.Response {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("encode body: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, h.server.URL+path, bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.SetBasicAuth(username, password)
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.server.Client().Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
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

func TestFirstRunRegistrationAndLogin(t *testing.T) {
	h := newEmptyHarness(t)

	status := h.do(t, http.MethodGet, "/api/v1/auth/status", "", nil)
	if status.StatusCode != http.StatusOK {
		t.Fatalf("GET auth status = %d, want 200", status.StatusCode)
	}
	if got := decodeBody[map[string]bool](t, status)["initialized"]; got {
		t.Fatal("fresh installation reports initialized")
	}

	register := h.do(t, http.MethodPost, "/api/v1/auth/register", "", map[string]any{
		"username": "mukhtar", "password": "x",
	})
	if register.StatusCode != http.StatusCreated {
		t.Fatalf("POST register = %d, want 201", register.StatusCode)
	}
	created := decodeBody[struct {
		Token string  `json:"token"`
		User  userRef `json:"user"`
	}](t, register)
	if !strings.HasPrefix(created.Token, store.TokenPrefix) {
		t.Errorf("registration token = %q, want Snagarr token", created.Token)
	}
	if created.User.Role != store.RoleAdmin || created.User.Username != "mukhtar" {
		t.Errorf("registered user = %+v, want first admin", created.User)
	}
	if resp := h.do(t, http.MethodGet, "/api/v1/me", created.Token, nil); resp.StatusCode != http.StatusOK {
		t.Errorf("registration session cannot authenticate: %d", resp.StatusCode)
	}

	second := h.do(t, http.MethodPost, "/api/v1/auth/register", "", map[string]any{
		"username": "someone", "password": "another-long-password",
	})
	if second.StatusCode != http.StatusConflict {
		t.Errorf("second registration = %d, want 409", second.StatusCode)
	}

	login := h.do(t, http.MethodPost, "/api/v1/auth/login", "", map[string]any{
		"username": "MUKHTAR", "password": "x",
	})
	if login.StatusCode != http.StatusOK {
		t.Fatalf("POST login = %d, want 200", login.StatusCode)
	}
	session := decodeBody[map[string]any](t, login)
	if token, _ := session["token"].(string); !strings.HasPrefix(token, store.TokenPrefix) {
		t.Errorf("login token = %q, want Snagarr token", token)
	}

	bad := h.do(t, http.MethodPost, "/api/v1/auth/login", "", map[string]any{
		"username": "mukhtar", "password": "wrong-password",
	})
	if bad.StatusCode != http.StatusUnauthorized {
		t.Errorf("wrong password = %d, want 401", bad.StatusCode)
	}
}

func TestRegistrationValidation(t *testing.T) {
	h := newEmptyHarness(t)
	tests := []struct {
		name string
		body map[string]any
	}{
		{"bad username", map[string]any{"username": "no spaces", "password": "anything"}},
		{"empty password", map[string]any{"username": "mukhtar", "password": ""}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if resp := h.do(t, http.MethodPost, "/api/v1/auth/register", "", tt.body); resp.StatusCode != http.StatusBadRequest {
				t.Errorf("POST register = %d, want 400", resp.StatusCode)
			}
		})
	}
}

func TestAdminCanCreatePasswordAccount(t *testing.T) {
	h := newHarness(t)
	longPassword := strings.Repeat("long password ", 50)
	created := h.do(t, http.MethodPost, "/api/v1/users", h.adminToken, map[string]any{
		"username": "tomi", "password": longPassword, "role": "member",
	})
	if created.StatusCode != http.StatusCreated {
		t.Fatalf("POST user = %d, want 201", created.StatusCode)
	}
	user := decodeBody[userDTO](t, created)
	if user.Username != "tomi" || user.Role != store.RoleMember {
		t.Errorf("created user = %+v", user)
	}

	login := h.do(t, http.MethodPost, "/api/v1/auth/login", "", map[string]any{
		"username": "tomi", "password": longPassword,
	})
	if login.StatusCode != http.StatusOK {
		t.Errorf("new member login = %d, want 200", login.StatusCode)
	}

	duplicate := h.do(t, http.MethodPost, "/api/v1/users", h.adminToken, map[string]any{
		"username": "TOMI", "password": "another-passphrase", "role": "member",
	})
	if duplicate.StatusCode != http.StatusConflict {
		t.Errorf("duplicate username = %d, want 409", duplicate.StatusCode)
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
	if got.CapturedBy == nil || got.CapturedBy.Username != "Amina" {
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
		Source: store.SourceWeb, TMDBID: 426063, MediaType: store.Movie, CapturedBy: h.admin.ID,
	}
	if err := h.store.CreateItem(ctx, adminItem); err != nil {
		t.Fatalf("seed: %v", err)
	}
	memberItem := &store.Item{
		Title: "Severance", RawInput: "severance", Status: store.StatusNew,
		Source: store.SourceTelegram, TMDBID: 95396, MediaType: store.TV, CapturedBy: h.member.ID,
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
		Source: store.SourceWeb, TMDBID: 666277, MediaType: store.Movie, CapturedBy: h.admin.ID,
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
		"tmdb":     map[string]any{"api_key": "super-secret-4e2a"},
		"telegram": map[string]any{"bot_token": "123456:bot-secret-9f1c"},
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
	maskedToken, _ := got["telegram"]["bot_token"].(string)
	if maskedToken == "123456:bot-secret-9f1c" || !config.IsMasked(maskedToken) {
		t.Errorf("bot_token = %q, want it masked", maskedToken)
	}
	if configured, _ := got["telegram"]["configured"].(bool); !configured {
		t.Error("telegram.configured = false after setting a token")
	}
	// Every per-member integration is a service now; only the global
	// catalogue key, the household bot and the knobs stay settings.
	for _, gone := range []string{"library", "radarr", "sonarr", "overseerr", "ntfy"} {
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

func TestWebhookRejectsUnknownCredentials(t *testing.T) {
	h := newHarness(t)

	resp := h.do(t, http.MethodPost, "/api/v1/webhooks/radarr", "",
		map[string]any{"eventType": "Download"})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("webhook with no credential = %d, want 401", resp.StatusCode)
	}

	resp = h.do(t, http.MethodPost, "/api/v1/webhooks/radarr", "sngr_nonsense",
		map[string]any{"eventType": "Download"})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("webhook with an unknown token = %d, want 401", resp.StatusCode)
	}

	resp = h.basic(t, "/api/v1/webhooks/radarr", "Mukhtar", "not-the-password",
		map[string]any{"eventType": "Download"})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("webhook with a wrong password = %d, want 401", resp.StatusCode)
	}
}

// A Radarr or Sonarr webhook connection carries a username and a password, so
// the credential is the one the member signs in with.
func TestWebhookAcceptsBasicAuth(t *testing.T) {
	h := newHarness(t)

	hash, err := hashPassword("open-sesame")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	u := &store.User{Username: "kitchen-pi", PasswordHash: hash, Role: store.RoleMember}
	if err := h.store.CreateUser(context.Background(), u); err != nil {
		t.Fatalf("create user: %v", err)
	}

	resp := h.basic(t, "/api/v1/webhooks/radarr", "kitchen-pi", "open-sesame",
		map[string]any{"eventType": "Download", "movie": map[string]any{"tmdbId": 999999}})
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("webhook with basic auth = %d, want 204", resp.StatusCode)
	}
}

// Emby sets no header of its own, so the token has to be able to travel in the
// query string.
func TestWebhookAcceptsTokenInQuery(t *testing.T) {
	h := newHarness(t)

	resp := h.do(t, http.MethodPost, "/api/v1/webhooks/emby?token="+h.memberToken, "",
		map[string]any{"Item": map[string]any{"Type": "Movie"}})
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("webhook with a query token = %d, want 204", resp.StatusCode)
	}

	resp = h.do(t, http.MethodPost, "/api/v1/webhooks/emby?token=sngr_nonsense", "",
		map[string]any{"Item": map[string]any{"Type": "Movie"}})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("webhook with an unknown query token = %d, want 401", resp.StatusCode)
	}
}

// Unmatched webhook payloads must still answer 204, or the sender retries.
func TestWebhookAcceptsUnmatchedPayload(t *testing.T) {
	h := newHarness(t)

	resp := h.do(t, http.MethodPost, "/api/v1/webhooks/radarr", h.adminToken,
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
		TMDBID: 1064213, MediaType: store.Movie, CapturedBy: h.member.ID,
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

// The settings dialog creates nothing until Save, so a test has to run against
// credentials that exist only in the browser.
func TestTestDraftNeedsNoRecord(t *testing.T) {
	h := newHarness(t)

	resp := h.do(t, http.MethodPost, "/api/v1/services/test", h.memberToken, map[string]any{
		"kind":   "radarr",
		"config": map[string]any{"url": "http://127.0.0.1:1", "api_key": "nope"},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /services/test = %d, want 200", resp.StatusCode)
	}
	got := decodeBody[map[string]any](t, resp)
	if ok, _ := got["ok"].(bool); ok {
		t.Error("an unreachable Radarr reported ok")
	}
	if msg, _ := got["message"].(string); msg == "" {
		t.Error("a failed test carried no message")
	}

	services, err := h.store.UserServices(context.Background(), h.member.ID)
	if err != nil {
		t.Fatalf("read services: %v", err)
	}
	if len(services) != 0 {
		t.Errorf("testing created %d service(s), want none", len(services))
	}
}

// A masked secret means "the one already stored", so editing the URL alone
// still tests against the real key.
func TestTestDraftResolvesMaskedSecrets(t *testing.T) {
	h := newHarness(t)

	resp := h.do(t, http.MethodPost, "/api/v1/services", h.memberToken, map[string]any{
		"kind":   "radarr",
		"config": map[string]any{"url": "http://127.0.0.1:1", "api_key": "super-secret-4e2a"},
	})
	created := decodeBody[serviceDTO](t, resp)
	masked, _ := created.Config["api_key"].(string)

	resp = h.do(t, http.MethodPost, "/api/v1/services/test", h.memberToken, map[string]any{
		"id":     created.ID,
		"kind":   "radarr",
		"config": map[string]any{"url": "http://127.0.0.1:2", "api_key": masked},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /services/test = %d, want 200", resp.StatusCode)
	}

	stored, err := h.store.Service(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("read stored service: %v", err)
	}
	if !bytes.Contains(stored.Config, []byte("super-secret-4e2a")) {
		t.Error("the test overwrote the stored secret")
	}
	if bytes.Contains(stored.Config, []byte("127.0.0.1:2")) {
		t.Error("the test saved the pending edit")
	}
}

// fakeArr answers the two calls a Radarr add makes, and counts the adds.
func fakeArr(added *int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/v3/movie/lookup"):
			_, _ = w.Write([]byte(`[{"tmdbId":603,"title":"The Matrix","year":1999}]`))
		case r.URL.Path == "/api/v3/movie" && r.Method == http.MethodPost:
			*added++
			_, _ = w.Write([]byte(`{"id":7,"tmdbId":603,"title":"The Matrix","monitored":true,"hasFile":false}`))
		default:
			http.NotFound(w, r)
		}
	}))
}

func seedSnag(t *testing.T, h *harness, owner int64) *store.Item {
	t.Helper()
	it := &store.Item{
		TMDBID: 603, MediaType: store.Movie, Title: "The Matrix", Status: store.StatusNew,
		RawInput: "the matrix", Source: store.SourceWeb, CapturedBy: owner,
	}
	if err := h.store.CreateItem(context.Background(), it); err != nil {
		t.Fatalf("seed item: %v", err)
	}
	return it
}

// "Add to *arr by default" spends the capturer's own download manager, so a
// snagged title is already on its way without a second action.
func TestAutoSendUsesTheCapturersArr(t *testing.T) {
	h := newHarness(t)
	added := 0
	arr := fakeArr(&added)
	defer arr.Close()

	resp := h.do(t, http.MethodPost, "/api/v1/services", h.memberToken, map[string]any{
		"kind":   "radarr",
		"config": map[string]any{"url": arr.URL, "api_key": "key"},
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST /services = %d, want 201", resp.StatusCode)
	}

	it := seedSnag(t, h, h.member.ID)
	h.api.sender.AutoSend(context.Background(), it.ID)

	if added != 1 {
		t.Errorf("radarr adds = %d, want 1", added)
	}
	got, err := h.store.Item(context.Background(), it.ID)
	if err != nil {
		t.Fatalf("read item: %v", err)
	}
	if got.Status != store.StatusMonitored {
		t.Errorf("status = %q, want monitored", got.Status)
	}
}

func TestAutoSendOffLeavesTheItemAlone(t *testing.T) {
	h := newHarness(t)
	added := 0
	arr := fakeArr(&added)
	defer arr.Close()

	h.do(t, http.MethodPost, "/api/v1/services", h.memberToken, map[string]any{
		"kind":   "radarr",
		"config": map[string]any{"url": arr.URL, "api_key": "key"},
	})
	resp := h.do(t, http.MethodPut, "/api/v1/settings", h.adminToken, map[string]any{
		"general": map[string]any{"auto_send": false},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT /settings = %d, want 200", resp.StatusCode)
	}

	it := seedSnag(t, h, h.member.ID)
	h.api.sender.AutoSend(context.Background(), it.ID)

	if added != 0 {
		t.Errorf("radarr adds = %d, want none while automatic sending is off", added)
	}
}

// A member with no Radarr of their own must not spend an admin's.
func TestAutoSendNeverSpendsAnotherMembersService(t *testing.T) {
	h := newHarness(t)
	added := 0
	arr := fakeArr(&added)
	defer arr.Close()

	h.do(t, http.MethodPost, "/api/v1/services", h.adminToken, map[string]any{
		"kind":   "radarr",
		"config": map[string]any{"url": arr.URL, "api_key": "key"},
	})

	it := seedSnag(t, h, h.member.ID)
	h.api.sender.AutoSend(context.Background(), it.ID)

	if added != 0 {
		t.Errorf("radarr adds = %d, want none", added)
	}
	got, _ := h.store.Item(context.Background(), it.ID)
	if got.Status != store.StatusNew {
		t.Errorf("status = %q, want it left alone", got.Status)
	}
}

// The default has to be on, or "by default" means nothing.
func TestAutoSendIsOnByDefault(t *testing.T) {
	h := newHarness(t)

	resp := h.do(t, http.MethodGet, "/api/v1/settings", h.adminToken, nil)
	got := decodeBody[map[string]map[string]any](t, resp)
	if on, _ := got["general"]["auto_send"].(bool); !on {
		t.Error("auto_send is off on a fresh install")
	}
}

// The quality profile and root folder lists are needed while a connection is
// being filled in, which is before it exists.
func TestDraftOptionsNeedNoRecord(t *testing.T) {
	arr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v3/qualityprofile":
			_, _ = w.Write([]byte(`[{"id":4,"name":"HD-1080p"}]`))
		case "/api/v3/rootfolder":
			_, _ = w.Write([]byte(`[{"path":"/movies","freeSpace":812340000000}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer arr.Close()

	h := newHarness(t)
	resp := h.do(t, http.MethodPost, "/api/v1/services/options", h.memberToken, map[string]any{
		"kind":   "radarr",
		"config": map[string]any{"url": arr.URL, "api_key": "key"},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /services/options = %d, want 200", resp.StatusCode)
	}

	got := decodeBody[struct {
		QualityProfiles []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"quality_profiles"`
		RootFolders []struct {
			Path string `json:"path"`
		} `json:"root_folders"`
	}](t, resp)

	if len(got.QualityProfiles) != 1 || got.QualityProfiles[0].Name != "HD-1080p" {
		t.Errorf("quality profiles = %+v, want the one the server offered", got.QualityProfiles)
	}
	if len(got.RootFolders) != 1 || got.RootFolders[0].Path != "/movies" {
		t.Errorf("root folders = %+v, want the one the server offered", got.RootFolders)
	}

	services, err := h.store.UserServices(context.Background(), h.member.ID)
	if err != nil {
		t.Fatalf("read services: %v", err)
	}
	if len(services) != 0 {
		t.Errorf("looking options up created %d service(s), want none", len(services))
	}
}

func TestLoginRateLimit(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t)
	hash, err := hashPassword("correct horse")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	h.admin.PasswordHash = hash
	if err := h.store.UpdateUser(ctx, h.admin); err != nil {
		t.Fatalf("set password: %v", err)
	}

	login := func(password string) *http.Response {
		return h.do(t, http.MethodPost, "/api/v1/auth/login", "",
			map[string]string{"username": "Mukhtar", "password": password})
	}
	for i := 0; i < failureLimit; i++ {
		if resp := login("wrong"); resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("failure %d status = %d, want 401", i, resp.StatusCode)
		}
	}
	if resp := login("correct horse"); resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("after %d failures status = %d, want 429", failureLimit, resp.StatusCode)
	}
	// The webhook limiter is a separate one, so imports still authenticate.
	resp := h.basic(t, "/api/v1/webhooks/radarr", "Mukhtar", "correct horse",
		map[string]any{"eventType": "Test"})
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("webhook during login lockout status = %d, want 204", resp.StatusCode)
	}
}

func TestWebhookRateLimit(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t)
	hash, err := hashPassword("correct horse")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	h.admin.PasswordHash = hash
	if err := h.store.UpdateUser(ctx, h.admin); err != nil {
		t.Fatalf("set password: %v", err)
	}

	body := map[string]any{"eventType": "Test"}
	for i := 0; i < failureLimit; i++ {
		if resp := h.basic(t, "/api/v1/webhooks/radarr", "Mukhtar", "wrong", body); resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("failure %d status = %d, want 401", i, resp.StatusCode)
		}
	}
	if resp := h.basic(t, "/api/v1/webhooks/radarr", "Mukhtar", "correct horse", body); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("during webhook lockout status = %d, want 401", resp.StatusCode)
	}
	// Tokens are not limited, and the sign-in limiter is untouched.
	if resp := h.do(t, http.MethodPost, "/api/v1/webhooks/radarr", h.adminToken, body); resp.StatusCode != http.StatusNoContent {
		t.Errorf("token webhook during lockout status = %d, want 204", resp.StatusCode)
	}
	resp := h.do(t, http.MethodPost, "/api/v1/auth/login", "",
		map[string]string{"username": "Mukhtar", "password": "correct horse"})
	if resp.StatusCode != http.StatusOK {
		t.Errorf("login during webhook lockout status = %d, want 200", resp.StatusCode)
	}
}

// A playback webhook that names a start, pause, resume or progress event must
// not mark the title watched — pressing play is the opposite of finishing.
func TestPlaybackWebhookFiltersInProgressEvents(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t)
	it := &store.Item{Title: "Sinners", RawInput: "sinners", Status: store.StatusAvailable,
		Source: store.SourceWeb, TMDBID: 1233413, MediaType: store.Movie}
	if err := h.store.CreateItem(ctx, it); err != nil {
		t.Fatalf("seed item: %v", err)
	}

	status := func() store.Status {
		t.Helper()
		got, err := h.store.Item(ctx, it.ID)
		if err != nil {
			t.Fatalf("read item: %v", err)
		}
		return got.Status
	}

	for _, event := range []map[string]any{
		{"event": "media.play"},
		{"event": "playback.start"},
		{"action": "pause"},
		{"NotificationType": "PlaybackProgress"},
		{"NotificationType": "PlaybackStart"},
	} {
		event["tmdb_id"] = "1233413"
		event["media_type"] = "movie"
		resp := h.do(t, http.MethodPost, "/api/v1/webhooks/tautulli", h.adminToken, event)
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("webhook status = %d, want 204", resp.StatusCode)
		}
		if got := status(); got != store.StatusAvailable {
			t.Fatalf("after %v item status = %s, want still available", event, got)
		}
	}

	// A stop event is a watch.
	resp := h.do(t, http.MethodPost, "/api/v1/webhooks/emby", h.adminToken, map[string]any{
		"event": "playback.stop",
		"Item":  map[string]any{"Type": "Movie", "ProviderIds": map[string]string{"Tmdb": "1233413"}},
	})
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("stop webhook status = %d, want 204", resp.StatusCode)
	}
	if got := status(); got != store.StatusWatched {
		t.Errorf("after stop item status = %s, want watched", got)
	}
}

// The documented Tautulli and Jellyfin templates carry no event name at all,
// so an eventless payload must keep marking the title watched.
func TestPlaybackWebhookAcceptsEventlessPayload(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t)
	it := &store.Item{Title: "Sinners", RawInput: "sinners", Status: store.StatusAvailable,
		Source: store.SourceWeb, TMDBID: 1233413, MediaType: store.Movie}
	if err := h.store.CreateItem(ctx, it); err != nil {
		t.Fatalf("seed item: %v", err)
	}

	resp := h.do(t, http.MethodPost, "/api/v1/webhooks/tautulli", h.adminToken,
		map[string]any{"tmdb_id": "1233413", "media_type": "movie"})
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("webhook status = %d, want 204", resp.StatusCode)
	}
	got, err := h.store.Item(ctx, it.ID)
	if err != nil {
		t.Fatalf("read item: %v", err)
	}
	if got.Status != store.StatusWatched {
		t.Errorf("item status = %s, want watched", got.Status)
	}
}

// Import webhooks act on the four import event types and nothing else.
func TestImportWebhookFiltersEventTypes(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t)
	it := &store.Item{Title: "Sinners", RawInput: "sinners", Status: store.StatusMonitored,
		Source: store.SourceWeb, TMDBID: 1233413, MediaType: store.Movie}
	if err := h.store.CreateItem(ctx, it); err != nil {
		t.Fatalf("seed item: %v", err)
	}

	body := map[string]any{"eventType": "Grab", "movie": map[string]any{"tmdbId": 1233413}}
	if resp := h.do(t, http.MethodPost, "/api/v1/webhooks/radarr", h.adminToken, body); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("grab webhook status = %d, want 204", resp.StatusCode)
	}
	got, err := h.store.Item(ctx, it.ID)
	if err != nil {
		t.Fatalf("read item: %v", err)
	}
	if got.Status != store.StatusMonitored {
		t.Fatalf("after Grab status = %s, want still monitored", got.Status)
	}

	body["eventType"] = "Download"
	if resp := h.do(t, http.MethodPost, "/api/v1/webhooks/radarr", h.adminToken, body); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("download webhook status = %d, want 204", resp.StatusCode)
	}
	if got, err = h.store.Item(ctx, it.ID); err != nil {
		t.Fatalf("read item: %v", err)
	}
	if got.Status != store.StatusAvailable {
		t.Errorf("after Download status = %s, want available", got.Status)
	}
}

// A build carrying the shared TMDB key needs no key from the operator: setup
// is not required, the card reads as configured, and the key itself never
// travels to a client.
func TestSettingsReportBuiltinTMDBKey(t *testing.T) {
	old := config.DefaultTMDBKey
	t.Cleanup(func() { config.DefaultTMDBKey = old })
	config.DefaultTMDBKey = "builtin-secret"

	h := newHarness(t)

	resp := h.do(t, http.MethodGet, "/api/v1/settings", h.adminToken, nil)
	body := decodeBody[map[string]map[string]any](t, resp)
	if body["tmdb"]["builtin_key"] != true {
		t.Errorf("tmdb.builtin_key = %v, want true", body["tmdb"]["builtin_key"])
	}
	if body["tmdb"]["configured"] != true {
		t.Errorf("tmdb.configured = %v, want true", body["tmdb"]["configured"])
	}
	if body["tmdb"]["api_key"] != "" {
		t.Errorf("tmdb.api_key = %v; the built-in key must never reach a client", body["tmdb"]["api_key"])
	}

	resp = h.do(t, http.MethodGet, "/api/v1/auth/status", "", nil)
	status := decodeBody[map[string]bool](t, resp)
	if status["setup_required"] {
		t.Error("setup_required = true; the built-in key should cover setup")
	}
}
