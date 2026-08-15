package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newTelegramFake(t *testing.T, handler http.HandlerFunc) *TelegramClient {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c := NewTelegram("token123")
	c.rest.BaseURL = srv.URL + "/bottoken123"
	return c
}

func TestTelegramUpdatesCarryOffsetAndTimeout(t *testing.T) {
	ctx := context.Background()
	var gotPath, gotOffset, gotTimeout string
	c := newTelegramFake(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotOffset = r.URL.Query().Get("offset")
		gotTimeout = r.URL.Query().Get("timeout")
		_, _ = w.Write([]byte(`{"ok":true,"result":[
			{"update_id":7,"message":{"message_id":1,"from":{"id":42,"username":"amina"},"chat":{"id":42},"text":"sinners"}},
			{"update_id":8,"callback_query":{"id":"cb1","from":{"id":42},"data":"s:3"}}
		]}`))
	})

	updates, err := c.Updates(ctx, 7, 50*time.Second)
	if err != nil {
		t.Fatalf("updates: %v", err)
	}
	if gotPath != "/bottoken123/getUpdates" {
		t.Errorf("path = %q, want the token in the URL", gotPath)
	}
	if gotOffset != "7" || gotTimeout != "50" {
		t.Errorf("offset, timeout = %q, %q; want 7, 50", gotOffset, gotTimeout)
	}
	if len(updates) != 2 {
		t.Fatalf("updates = %d, want 2", len(updates))
	}
	if updates[0].Message == nil || updates[0].Message.Text != "sinners" {
		t.Errorf("first update message = %+v, want text sinners", updates[0].Message)
	}
	if updates[1].Callback == nil || updates[1].Callback.Data != "s:3" {
		t.Errorf("second update callback = %+v, want data s:3", updates[1].Callback)
	}
}

func TestTelegramSendPhotoBuildsKeyboard(t *testing.T) {
	ctx := context.Background()
	var body map[string]any
	c := newTelegramFake(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/bottoken123/sendPhoto" {
			t.Errorf("path = %q, want /sendPhoto", r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":55}}`))
	})

	id, err := c.SendPhoto(ctx, 42, "https://img/poster.jpg", "Sinners (2025)",
		[][]TelegramButton{{{Text: "Send to Radarr", Data: "s:3"}}})
	if err != nil {
		t.Fatalf("send photo: %v", err)
	}
	if id != 55 {
		t.Errorf("message id = %d, want 55", id)
	}
	if body["photo"] != "https://img/poster.jpg" || body["caption"] != "Sinners (2025)" {
		t.Errorf("body = %v, want photo and caption", body)
	}
	raw, _ := json.Marshal(body["reply_markup"])
	var markup struct {
		Keyboard [][]TelegramButton `json:"inline_keyboard"`
	}
	if err := json.Unmarshal(raw, &markup); err != nil {
		t.Fatalf("decode reply_markup: %v", err)
	}
	if len(markup.Keyboard) != 1 || len(markup.Keyboard[0]) != 1 ||
		markup.Keyboard[0][0] != (TelegramButton{Text: "Send to Radarr", Data: "s:3"}) {
		t.Errorf("reply_markup = %s, want one Send to Radarr button", raw)
	}
}

func TestTelegramRewritePicksCaptionForPhotos(t *testing.T) {
	ctx := context.Background()
	var calls []string
	c := newTelegramFake(t, func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.URL.Path)
		_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
	})

	if err := c.Rewrite(ctx, 42, 55, true, "done"); err != nil {
		t.Fatalf("rewrite caption: %v", err)
	}
	if err := c.Rewrite(ctx, 42, 56, false, "done"); err != nil {
		t.Fatalf("rewrite text: %v", err)
	}
	if len(calls) != 2 || calls[0] != "/bottoken123/editMessageCaption" || calls[1] != "/bottoken123/editMessageText" {
		t.Errorf("calls = %v, want caption then text edits", calls)
	}
}

func TestTelegramSurfacesAPIErrors(t *testing.T) {
	ctx := context.Background()
	c := newTelegramFake(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"ok":false,"description":"Unauthorized"}`))
	})

	if _, err := c.Me(ctx); err == nil || err.Error() != "telegram getMe: Unauthorized" {
		t.Errorf("error = %v, want the API description", err)
	}
}

func TestTelegramMe(t *testing.T) {
	ctx := context.Background()
	c := newTelegramFake(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true,"result":{"username":"snagarr_bot"}}`))
	})

	name, err := c.Me(ctx)
	if err != nil {
		t.Fatalf("me: %v", err)
	}
	if name != "@snagarr_bot" {
		t.Errorf("me = %q, want @snagarr_bot", name)
	}
}
