package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sirrobot01/snagarr/internal/config"
	"github.com/sirrobot01/snagarr/internal/engine"
	"github.com/sirrobot01/snagarr/internal/integration"
	"github.com/sirrobot01/snagarr/internal/store"
)

// telegramCall is one request the bot made to the fake Bot API.
type telegramCall struct {
	Method string
	Body   map[string]any
}

type harness struct {
	bot    *Bot
	store  *store.Store
	member *store.User
	admin  *store.User

	calls *[]telegramCall
}

// newHarness stands up a bot against fake Telegram and TMDB servers. The TMDB
// fake resolves "sinners" confidently and parks "obsession" as ambiguous.
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
	if _, err := settings.Apply(ctx, []byte(`{"tmdb":{"api_key":"k"},"telegram":{"bot_token":"tok"}}`)); err != nil {
		t.Fatalf("apply settings: %v", err)
	}

	calls := &[]telegramCall{}
	telegram := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := map[string]any{}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		*calls = append(*calls, telegramCall{Method: r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:], Body: body})
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":1}}`))
	}))
	t.Cleanup(telegram.Close)

	tmdb := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/search/multi" && strings.Contains(r.URL.Query().Get("query"), "obsession"):
			_, _ = w.Write([]byte(`{"results":[
				{"id":100,"media_type":"movie","title":"Obsession","release_date":"2026-01-01"},
				{"id":200,"media_type":"movie","title":"Obsession","release_date":"1976-01-01"}
			]}`))
		case r.URL.Path == "/search/multi":
			_, _ = w.Write([]byte(`{"results":[
				{"id":1233413,"media_type":"movie","title":"Sinners","release_date":"2025-04-18","poster_path":"/p.jpg","popularity":90}
			]}`))
		case strings.HasPrefix(r.URL.Path, "/movie/"):
			id := strings.TrimPrefix(r.URL.Path, "/movie/")
			title := "Sinners"
			if id == "100" || id == "200" {
				title = "Obsession"
			}
			fmt.Fprintf(w, `{"id":%s,"title":%q,"release_date":"2025-04-18","poster_path":"/p.jpg"}`, id, title)
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	t.Cleanup(tmdb.Close)

	oldTMDB, oldTelegram := integration.TMDBBaseURL, integration.TelegramBaseURL
	integration.TMDBBaseURL, integration.TelegramBaseURL = tmdb.URL, telegram.URL
	t.Cleanup(func() { integration.TMDBBaseURL, integration.TelegramBaseURL = oldTMDB, oldTelegram })

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	reconciler := engine.NewReconciler(db, settings, log)
	b := New(db, settings, engine.NewResolver(db, log), reconciler,
		engine.NewSender(db, settings, reconciler, log), log)
	b.client = integration.NewTelegram("tok")

	admin := &store.User{Username: "Mukhtar", Role: store.RoleAdmin, TelegramUserID: 11}
	if err := db.CreateUser(ctx, admin); err != nil {
		t.Fatalf("create admin: %v", err)
	}
	member := &store.User{Username: "Amina", Role: store.RoleMember, TelegramUserID: 42}
	if err := db.CreateUser(ctx, member); err != nil {
		t.Fatalf("create member: %v", err)
	}
	return &harness{bot: b, store: db, member: member, admin: admin, calls: calls}
}

func message(fromID int64, text string) integration.TelegramMessage {
	return integration.TelegramMessage{
		ID:   1,
		From: integration.TelegramUser{ID: fromID, Username: "amina"},
		Chat: integration.TelegramChat{ID: fromID},
		Text: text,
	}
}

func (h *harness) lastCall(t *testing.T) telegramCall {
	t.Helper()
	if len(*h.calls) == 0 {
		t.Fatal("the bot sent nothing")
	}
	return (*h.calls)[len(*h.calls)-1]
}

func (h *harness) items(t *testing.T) []store.Item {
	t.Helper()
	items, _, err := h.store.Items(context.Background(), store.ItemFilter{})
	if err != nil {
		t.Fatalf("list items: %v", err)
	}
	return items
}

func TestUnknownTelegramAccountIsRefused(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t)

	h.bot.handleMessage(ctx, message(999, "sinners"))

	call := h.lastCall(t)
	if call.Method != "sendMessage" {
		t.Fatalf("reply method = %s, want sendMessage", call.Method)
	}
	text, _ := call.Body["text"].(string)
	if !strings.Contains(text, "isn't linked") || !strings.Contains(text, "999") {
		t.Errorf("reply = %q, want a not-linked message carrying the ID", text)
	}
	if got := h.items(t); len(got) != 0 {
		t.Errorf("items = %d, want none captured for a stranger", len(got))
	}
}

func TestMessageCapturesAndRepliesWithPoster(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t)

	h.bot.handleMessage(ctx, message(42, "sinners"))

	items := h.items(t)
	if len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}
	it := items[0]
	if it.Title != "Sinners" || it.TMDBID != 1233413 || it.Source != store.SourceTelegram {
		t.Errorf("item = %+v, want resolved Sinners from telegram", it)
	}
	if it.CapturedBy != h.member.ID {
		t.Errorf("captured_by = %d, want the member %d", it.CapturedBy, h.member.ID)
	}

	call := h.lastCall(t)
	if call.Method != "sendPhoto" {
		t.Fatalf("reply method = %s, want sendPhoto", call.Method)
	}
	caption, _ := call.Body["caption"].(string)
	if !strings.Contains(caption, "Sinners (2025)") || !strings.Contains(caption, "snagged by Amina") {
		t.Errorf("caption = %q, want title, year and attribution", caption)
	}
	markup, _ := json.Marshal(call.Body["reply_markup"])
	if !strings.Contains(string(markup), fmt.Sprintf(`"s:%d"`, it.ID)) {
		t.Errorf("buttons = %s, want a send button", markup)
	}
}

func TestAmbiguousCaptureOffersCandidates(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t)

	h.bot.handleMessage(ctx, message(42, "obsession"))

	items := h.items(t)
	if len(items) != 1 || items[0].Status != store.StatusNeedsReview {
		t.Fatalf("items = %+v, want one needs_review item", items)
	}

	call := h.lastCall(t)
	markup, _ := json.Marshal(call.Body["reply_markup"])
	if !strings.Contains(string(markup), "Obsession (2026)") || !strings.Contains(string(markup), "None of these") {
		t.Errorf("buttons = %s, want both candidates and an out", markup)
	}
	if !strings.Contains(string(markup), fmt.Sprintf("r:%d:100:m", items[0].ID)) {
		t.Errorf("buttons = %s, want resolve callback data", markup)
	}
}

func TestCallbackResolvesCandidate(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t)

	h.bot.handleMessage(ctx, message(42, "obsession"))
	it := h.items(t)[0]

	h.bot.handleCallback(ctx, integration.TelegramCallback{
		ID:   "cb1",
		From: integration.TelegramUser{ID: 42},
		Message: &integration.TelegramMessage{ID: 5,
			Chat: integration.TelegramChat{ID: 42}},
		Data: fmt.Sprintf("r:%d:100:m", it.ID),
	})

	saved, err := h.store.Item(ctx, it.ID)
	if err != nil {
		t.Fatalf("item: %v", err)
	}
	if saved.TMDBID != 100 || saved.Status == store.StatusNeedsReview {
		t.Errorf("item = %+v, want it resolved to 100", saved)
	}
}

func TestCallbackSendUsesTheMembersOwnRadarr(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t)

	var added bool
	radarr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/lookup"):
			_, _ = w.Write([]byte(`[{"tmdbId":1233413,"title":"Sinners"}]`))
		case r.Method == http.MethodPost:
			added = true
			_, _ = w.Write([]byte(`{"id":9,"tmdbId":1233413,"title":"Sinners","monitored":true}`))
		default:
			_, _ = w.Write([]byte(`[]`))
		}
	}))
	t.Cleanup(radarr.Close)

	cfg, _ := json.Marshal(map[string]any{"url": radarr.URL, "api_key": "k"})
	svc := &store.Service{UserID: h.member.ID, Kind: store.KindRadarr, Name: "radarr", Enabled: true, Config: cfg}
	if err := h.store.CreateService(ctx, svc); err != nil {
		t.Fatalf("create service: %v", err)
	}

	h.bot.handleMessage(ctx, message(42, "sinners"))
	it := h.items(t)[0]

	// Auto-send already ran on capture; reset so the button press is what we
	// observe.
	if err := h.store.SetStatus(ctx, it.ID, store.StatusNew, it.AvailableAt); err != nil {
		t.Fatalf("reset status: %v", err)
	}
	added = false

	h.bot.handleCallback(ctx, integration.TelegramCallback{
		ID:   "cb1",
		From: integration.TelegramUser{ID: 42},
		Data: fmt.Sprintf("s:%d", it.ID),
	})

	if !added {
		t.Error("radarr never received the add")
	}
	saved, err := h.store.Item(ctx, it.ID)
	if err != nil {
		t.Fatalf("item: %v", err)
	}
	if saved.Status != store.StatusMonitored {
		t.Errorf("status = %s, want monitored", saved.Status)
	}
}

func TestCallbackRefusesAnotherMembersItem(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t)

	// The admin captures; a member presses the button.
	h.bot.handleMessage(ctx, message(11, "sinners"))
	it := h.items(t)[0]

	h.bot.handleCallback(ctx, integration.TelegramCallback{
		ID:   "cb1",
		From: integration.TelegramUser{ID: 42},
		Data: fmt.Sprintf("d:%d", it.ID),
	})

	call := h.lastCall(t)
	if call.Method != "answerCallbackQuery" {
		t.Fatalf("last call = %s, want answerCallbackQuery", call.Method)
	}
	text, _ := call.Body["text"].(string)
	if !strings.Contains(text, "admin") {
		t.Errorf("answer = %q, want a permission refusal", text)
	}
	if _, err := h.store.Item(ctx, it.ID); err != nil {
		t.Errorf("item vanished despite the refusal: %v", err)
	}
}

func TestDuplicateCaptureIsReported(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t)

	h.bot.handleMessage(ctx, message(42, "sinners"))
	h.bot.handleMessage(ctx, message(42, "sinners"))

	if got := h.items(t); len(got) != 1 {
		t.Fatalf("items = %d, want the duplicate collapsed", len(got))
	}
	call := h.lastCall(t)
	text, _ := call.Body["text"].(string)
	if !strings.Contains(text, "already on the household list") {
		t.Errorf("reply = %q, want the duplicate reported", text)
	}
}
