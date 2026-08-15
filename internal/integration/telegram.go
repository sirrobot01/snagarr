package integration

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"time"
)

// TelegramBaseURL is a variable so tests can point the client at a fake
// server; the token is appended per Bot API convention.
var TelegramBaseURL = "https://api.telegram.org"

// TelegramClient calls the Telegram Bot API. All traffic is outbound — long
// polling means the bot works with no inbound port, which is what lets it run
// behind NAT and CGNAT like everything else in Snagarr.
type TelegramClient struct {
	rest client
}

// NewTelegram returns a client for one bot token.
func NewTelegram(token string) *TelegramClient {
	return &TelegramClient{rest: client{BaseURL: TelegramBaseURL + "/bot" + token}}
}

// TelegramUpdate is one inbound event: a message or a button press.
type TelegramUpdate struct {
	ID       int64             `json:"update_id"`
	Message  *TelegramMessage  `json:"message"`
	Callback *TelegramCallback `json:"callback_query"`
}

type TelegramMessage struct {
	ID      int64        `json:"message_id"`
	From    TelegramUser `json:"from"`
	Chat    TelegramChat `json:"chat"`
	Text    string       `json:"text"`
	Caption string       `json:"caption"`
	// Non-empty when the message carries a photo, which decides whether an
	// edit goes through editMessageCaption or editMessageText.
	Photo []struct{} `json:"photo"`
}

type TelegramUser struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
}

type TelegramChat struct {
	ID int64 `json:"id"`
}

type TelegramCallback struct {
	ID      string           `json:"id"`
	From    TelegramUser     `json:"from"`
	Message *TelegramMessage `json:"message"`
	Data    string           `json:"data"`
}

// TelegramButton is one inline keyboard button. Data travels back verbatim in
// the callback and is capped at 64 bytes by the Bot API.
type TelegramButton struct {
	Text string `json:"text"`
	Data string `json:"callback_data"`
}

// telegramEnvelope is the Bot API's uniform response wrapper.
type telegramEnvelope struct {
	OK          bool   `json:"ok"`
	Description string `json:"description"`
}

func (e telegramEnvelope) err(call string) error {
	if e.OK {
		return nil
	}
	return fmt.Errorf("telegram %s: %s", call, e.Description)
}

// Updates long-polls for new events. The call blocks server-side up to wait,
// answering the moment something arrives; offset acknowledges everything
// before it, so a crash re-delivers rather than loses.
func (c *TelegramClient) Updates(ctx context.Context, offset int64, wait time.Duration) ([]TelegramUpdate, error) {
	q := url.Values{
		"timeout": {strconv.Itoa(int(wait.Seconds()))},
		// Everything else the API can deliver (edits, member events) is noise
		// to a capture bot.
		"allowed_updates": {`["message","callback_query"]`},
	}
	if offset != 0 {
		q.Set("offset", strconv.FormatInt(offset, 10))
	}
	var r struct {
		telegramEnvelope
		Result []TelegramUpdate `json:"result"`
	}
	if err := c.rest.Get(ctx, "/getUpdates", q, &r); err != nil {
		return nil, err
	}
	return r.Result, r.err("getUpdates")
}

func keyboard(buttons [][]TelegramButton) map[string]any {
	if len(buttons) == 0 {
		return nil
	}
	return map[string]any{"inline_keyboard": buttons}
}

// Send posts a plain text message and returns its ID.
func (c *TelegramClient) Send(ctx context.Context, chatID int64, text string, buttons [][]TelegramButton) (int64, error) {
	body := map[string]any{"chat_id": chatID, "text": text}
	if kb := keyboard(buttons); kb != nil {
		body["reply_markup"] = kb
	}
	var r struct {
		telegramEnvelope
		Result TelegramMessage `json:"result"`
	}
	if err := c.rest.Post(ctx, "/sendMessage", body, &r); err != nil {
		return 0, err
	}
	return r.Result.ID, r.err("sendMessage")
}

// SendPhoto posts a photo by URL with a caption. Telegram fetches the URL
// itself, so the poster never passes through Snagarr.
func (c *TelegramClient) SendPhoto(ctx context.Context, chatID int64, photoURL, caption string, buttons [][]TelegramButton) (int64, error) {
	body := map[string]any{"chat_id": chatID, "photo": photoURL, "caption": caption}
	if kb := keyboard(buttons); kb != nil {
		body["reply_markup"] = kb
	}
	var r struct {
		telegramEnvelope
		Result TelegramMessage `json:"result"`
	}
	if err := c.rest.Post(ctx, "/sendPhoto", body, &r); err != nil {
		return 0, err
	}
	return r.Result.ID, r.err("sendPhoto")
}

// Rewrite replaces a sent message's text or caption and drops its buttons —
// how a message settles once its buttons have done their work. hasPhoto picks
// the right call: captions and text edit through different methods.
func (c *TelegramClient) Rewrite(ctx context.Context, chatID, messageID int64, hasPhoto bool, text string) error {
	body := map[string]any{"chat_id": chatID, "message_id": messageID}
	call := "/editMessageText"
	if hasPhoto {
		call = "/editMessageCaption"
		body["caption"] = text
	} else {
		body["text"] = text
	}
	var r telegramEnvelope
	if err := c.rest.Post(ctx, call, body, &r); err != nil {
		return err
	}
	return r.err(call)
}

// AnswerCallback acknowledges a button press. text, when set, shows as a small
// toast in the member's Telegram client.
func (c *TelegramClient) AnswerCallback(ctx context.Context, callbackID, text string) error {
	body := map[string]any{"callback_query_id": callbackID}
	if text != "" {
		body["text"] = text
	}
	var r telegramEnvelope
	if err := c.rest.Post(ctx, "/answerCallbackQuery", body, &r); err != nil {
		return err
	}
	return r.err("answerCallbackQuery")
}

// Me identifies the bot, which is what the settings Test button shows.
func (c *TelegramClient) Me(ctx context.Context) (string, error) {
	var r struct {
		telegramEnvelope
		Result struct {
			Username string `json:"username"`
		} `json:"result"`
	}
	if err := c.rest.Get(ctx, "/getMe", nil, &r); err != nil {
		return "", err
	}
	if err := r.err("getMe"); err != nil {
		return "", err
	}
	return "@" + r.Result.Username, nil
}
