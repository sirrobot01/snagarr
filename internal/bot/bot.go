// Package bot is the Telegram capture client: household members message the
// bot a title or a link and it lands on the shared list with their name on it.
// The bot long-polls, so all traffic is outbound and no port has to be opened
// — the same promise the rest of Snagarr makes.
package bot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/sirrobot01/snagarr/internal/config"
	"github.com/sirrobot01/snagarr/internal/engine"
	"github.com/sirrobot01/snagarr/internal/integration"
	"github.com/sirrobot01/snagarr/internal/store"
)

const (
	// pollWait is how long one getUpdates call blocks server-side. Delivery is
	// still immediate — Telegram answers the moment something arrives.
	pollWait = 50 * time.Second
	// idleWait paces the re-check for a token when none is configured, so
	// pasting one into settings starts the bot without a restart.
	idleWait = 15 * time.Second
	// errWait keeps a broken token or an outage from becoming a hot loop.
	errWait = 10 * time.Second

	// resolveWait bounds one capture's inline resolution. The capture itself
	// is already safe on disk when resolution starts.
	resolveWait = 45 * time.Second

	posterBase = "https://image.tmdb.org/t/p/w342"
)

type Bot struct {
	store      *store.Store
	settings   *config.Manager
	resolver   *engine.Resolver
	reconciler *engine.Reconciler
	sender     *engine.Sender
	log        *slog.Logger

	client *integration.TelegramClient
	token  string
	offset int64
}

func New(s *store.Store, settings *config.Manager, resolver *engine.Resolver,
	reconciler *engine.Reconciler, sender *engine.Sender, log *slog.Logger) *Bot {
	return &Bot{store: s, settings: settings, resolver: resolver,
		reconciler: reconciler, sender: sender, log: log}
}

// Run polls until ctx ends. The token is re-read every pass, so configuring or
// rotating it in settings takes effect without a restart. Updates are
// acknowledged by offset on the next poll; a crash mid-batch re-delivers
// rather than loses, and the capture path tolerates the replay.
func (b *Bot) Run(ctx context.Context) {
	for ctx.Err() == nil {
		token := b.settings.Get().Telegram.BotToken
		if token == "" {
			b.client, b.token = nil, ""
			nap(ctx, idleWait)
			continue
		}
		if b.client == nil || token != b.token {
			b.client, b.token, b.offset = integration.NewTelegram(token), token, 0
			b.log.Info("telegram bot polling")
		}

		updates, err := b.client.Updates(ctx, b.offset, pollWait)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			b.log.Warn("telegram poll failed", "error", err)
			nap(ctx, errWait)
			continue
		}
		for _, u := range updates {
			b.offset = u.ID + 1
			b.handle(ctx, u)
		}
	}
}

func nap(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}

func (b *Bot) handle(ctx context.Context, u integration.TelegramUpdate) {
	switch {
	case u.Message != nil && strings.TrimSpace(u.Message.Text) != "":
		b.handleMessage(ctx, *u.Message)
	case u.Callback != nil:
		b.handleCallback(ctx, *u.Callback)
	}
}

func (b *Bot) handleMessage(ctx context.Context, msg integration.TelegramMessage) {
	member, err := b.store.UserByTelegram(ctx, msg.From.ID)
	if errors.Is(err, store.ErrNotFound) {
		// The reply carries the numeric ID because that is exactly what the
		// admin needs to type into the household table.
		b.reply(ctx, msg.Chat.ID,
			fmt.Sprintf("This Telegram account isn't linked to a Snagarr member. An admin can add your Telegram ID in Settings → Household. Your ID: %d", msg.From.ID), nil)
		return
	}
	if err != nil {
		b.log.Warn("telegram member lookup failed", "error", err)
		return
	}

	text := strings.TrimSpace(msg.Text)
	if strings.HasPrefix(text, "/") {
		b.reply(ctx, msg.Chat.ID, "Send me a title or a link and I'll snag it for the household list.", nil)
		return
	}

	it := &store.Item{
		Title: text, RawInput: text, Status: store.StatusNeedsReview,
		Source: store.SourceTelegram, CapturedBy: member.ID,
	}
	if err := b.store.CreateItem(ctx, it); err != nil {
		b.log.Warn("telegram capture failed", "error", err)
		b.reply(ctx, msg.Chat.ID, "Something went wrong saving that. Try again.", nil)
		return
	}

	catalogue := b.tmdb()
	if catalogue == nil {
		b.reply(ctx, msg.Chat.ID, "Snagged. TMDB isn't configured yet, so it's parked in Needs Review.", nil)
		return
	}

	rctx, cancel := context.WithTimeout(ctx, resolveWait)
	defer cancel()
	if err := b.resolver.Resolve(rctx, catalogue, it.ID); err != nil {
		b.log.Warn("telegram capture could not be resolved", "item", it.ID, "error", err)
		b.reply(ctx, msg.Chat.ID, "Snagged — I couldn't identify it right now, so it's parked in Needs Review.", nil)
		return
	}

	saved, err := b.store.Item(ctx, it.ID)
	if errors.Is(err, store.ErrNotFound) {
		// The resolver yielded to an existing item for the same title.
		b.reply(ctx, msg.Chat.ID, "That one's already on the household list.", nil)
		return
	}
	if err != nil {
		b.log.Warn("telegram capture read-back failed", "item", it.ID, "error", err)
		return
	}

	if saved.Status == store.StatusNeedsReview {
		b.offerCandidates(ctx, msg.Chat.ID, saved)
		return
	}

	if err := b.reconciler.RefreshItem(ctx, saved.ID); err != nil {
		b.log.Warn("could not compute state after resolve", "item", saved.ID, "error", err)
	}
	b.sender.AutoSend(ctx, saved.ID)
	if saved, err = b.store.Item(ctx, saved.ID); err != nil {
		b.log.Warn("telegram capture read-back failed", "item", it.ID, "error", err)
		return
	}
	b.replyWithItem(ctx, msg.Chat.ID, saved)
}

// offerCandidates turns an ambiguous capture into the one-tap disambiguation
// the web's Needs Review card offers, as inline buttons.
func (b *Bot) offerCandidates(ctx context.Context, chatID int64, it *store.Item) {
	candidates, err := b.store.Candidates(ctx, it.ID)
	if err != nil || len(candidates) == 0 {
		b.reply(ctx, chatID, fmt.Sprintf("Snagged — no confident match for %q. It's parked in Needs Review.", it.RawInput), nil)
		return
	}

	rows := make([][]integration.TelegramButton, 0, len(candidates)+1)
	for _, c := range candidates {
		label := c.Title
		if c.Year != 0 {
			label = fmt.Sprintf("%s (%d)", c.Title, c.Year)
		}
		rows = append(rows, []integration.TelegramButton{{
			Text: label,
			Data: fmt.Sprintf("r:%d:%d:%s", it.ID, c.TMDBID, mediaCode(c.MediaType)),
		}})
	}
	rows = append(rows, []integration.TelegramButton{{Text: "None of these", Data: fmt.Sprintf("x:%d", it.ID)}})
	b.reply(ctx, chatID, fmt.Sprintf("Which one is %q?", it.RawInput), rows)
}

func (b *Bot) replyWithItem(ctx context.Context, chatID int64, it *store.Item) {
	caption := itemLine(it)
	buttons := actionButtons(it)
	if it.PosterPath != "" {
		if _, err := b.client.SendPhoto(ctx, chatID, posterBase+it.PosterPath, caption, buttons); err == nil {
			return
		}
		// Telegram fetches the poster itself; when it cannot, the words still
		// have to arrive.
	}
	b.reply(ctx, chatID, caption, buttons)
}

func (b *Bot) handleCallback(ctx context.Context, cb integration.TelegramCallback) {
	member, err := b.store.UserByTelegram(ctx, cb.From.ID)
	if err != nil {
		b.answer(ctx, cb.ID, "This Telegram account isn't linked to a Snagarr member.")
		return
	}

	parts := strings.Split(cb.Data, ":")
	kind := parts[0]
	if len(parts) < 2 {
		b.answer(ctx, cb.ID, "")
		return
	}
	id, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		b.answer(ctx, cb.ID, "")
		return
	}

	it, err := b.store.Item(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		b.answer(ctx, cb.ID, "That item is gone.")
		b.rewrite(ctx, cb, "This item is no longer on the list.")
		return
	}
	if err != nil {
		b.answer(ctx, cb.ID, "Something went wrong.")
		return
	}
	// Mirrors the API's rule: a member acts on their own captures, an admin on
	// anybody's.
	if member.Role != store.RoleAdmin && it.CapturedBy != member.ID {
		b.answer(ctx, cb.ID, "Only an admin can act on another member's snag.")
		return
	}

	switch kind {
	case "s":
		b.callbackSend(ctx, cb, member, it)
	case "r":
		b.callbackResolve(ctx, cb, parts, it)
	case "x":
		b.answer(ctx, cb.ID, "Kept in Needs Review.")
		b.rewrite(ctx, cb, fmt.Sprintf("%q is parked in Needs Review — resolve it in the web app.", it.RawInput))
	case "d":
		if err := b.store.DeleteItem(ctx, it.ID); err != nil && !errors.Is(err, store.ErrNotFound) {
			b.answer(ctx, cb.ID, "Something went wrong.")
			return
		}
		b.answer(ctx, cb.ID, "Removed.")
		b.rewrite(ctx, cb, fmt.Sprintf("%s — removed.", it.Title))
	default:
		b.answer(ctx, cb.ID, "")
	}
}

func (b *Bot) callbackSend(ctx context.Context, cb integration.TelegramCallback, member *store.User, it *store.Item) {
	target := "radarr"
	if it.MediaType == store.TV {
		target = "sonarr"
	}
	house, err := b.sender.Household(ctx)
	if err != nil {
		b.answer(ctx, cb.ID, "Something went wrong reading the services.")
		return
	}
	owners := b.sender.Owners(ctx, member, it.CapturedBy)

	status, err := b.sender.Send(ctx, house, *it, target, owners)
	if errors.Is(err, engine.ErrNoTarget) {
		b.answer(ctx, cb.ID, fmt.Sprintf("Nobody has a %s connected.", target))
		return
	}
	if err != nil {
		b.answer(ctx, cb.ID, fmt.Sprintf("%s rejected the request.", target))
		b.log.Warn("telegram send failed", "item", it.ID, "target", target, "error", err)
		return
	}
	if err := b.store.SetStatus(ctx, it.ID, status, time.Time{}); err != nil {
		b.log.Warn("telegram send could not record the state", "item", it.ID, "error", err)
	}
	b.reconciler.Trigger()
	b.answer(ctx, cb.ID, "Sent to "+target)
	b.rewrite(ctx, cb, fmt.Sprintf("%s — sent to %s.", titleLine(it), target))
}

func (b *Bot) callbackResolve(ctx context.Context, cb integration.TelegramCallback, parts []string, it *store.Item) {
	if len(parts) < 4 {
		b.answer(ctx, cb.ID, "")
		return
	}
	tmdbID, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		b.answer(ctx, cb.ID, "")
		return
	}
	mediaType := store.Movie
	if parts[3] == "t" {
		mediaType = store.TV
	}

	catalogue := b.tmdb()
	if catalogue == nil {
		b.answer(ctx, cb.ID, "TMDB is not configured.")
		return
	}
	details, err := catalogue.Details(ctx, mediaType, int(tmdbID))
	if err != nil {
		b.answer(ctx, cb.ID, "TMDB lookup failed.")
		return
	}

	err = b.store.Resolve(ctx, it.ID, store.Candidate{
		TMDBID: int64(details.TMDBID), MediaType: details.Type, Title: details.Title,
		Year: details.Year, PosterPath: details.PosterPath,
	})
	if errors.Is(err, store.ErrConflict) {
		_ = b.store.DeleteItem(ctx, it.ID)
		b.answer(ctx, cb.ID, "Already on the list.")
		b.rewrite(ctx, cb, fmt.Sprintf("%s is already on the household list.", details.Title))
		return
	}
	if err != nil {
		b.answer(ctx, cb.ID, "Something went wrong.")
		return
	}
	if err := b.store.SetCandidates(ctx, it.ID, nil); err != nil {
		b.log.Warn("could not clear candidates", "item", it.ID, "error", err)
	}
	if err := b.store.PutEntity(ctx, store.Entity{
		TMDBID: int64(details.TMDBID), MediaType: details.Type, Title: details.Title,
		Year: details.Year, PosterPath: details.PosterPath, BackdropPath: details.BackdropPath,
		Overview: details.Overview, Genres: details.Genres, Runtime: details.Runtime,
		Popularity: details.Popularity,
	}); err != nil {
		b.log.Warn("could not cache metadata", "tmdb_id", details.TMDBID, "error", err)
	}
	if err := b.reconciler.RefreshItem(ctx, it.ID); err != nil {
		b.log.Warn("could not compute state after resolve", "item", it.ID, "error", err)
	}
	b.sender.AutoSend(ctx, it.ID)

	saved, err := b.store.Item(ctx, it.ID)
	if err != nil {
		b.answer(ctx, cb.ID, "Snagged.")
		return
	}
	b.answer(ctx, cb.ID, "Snagged — "+saved.Title)
	b.rewrite(ctx, cb, itemLine(saved))
}

/* ── Small pieces ────────────────────────────────────────────────────────── */

func (b *Bot) reply(ctx context.Context, chatID int64, text string, buttons [][]integration.TelegramButton) {
	if _, err := b.client.Send(ctx, chatID, text, buttons); err != nil {
		b.log.Warn("telegram reply failed", "error", err)
	}
}

func (b *Bot) answer(ctx context.Context, callbackID, text string) {
	if err := b.client.AnswerCallback(ctx, callbackID, text); err != nil {
		b.log.Debug("telegram callback answer failed", "error", err)
	}
}

// rewrite settles the message whose button was pressed: outcome text, no more
// buttons. A missing original (old chats) is fine to skip.
func (b *Bot) rewrite(ctx context.Context, cb integration.TelegramCallback, text string) {
	if cb.Message == nil {
		return
	}
	if err := b.client.Rewrite(ctx, cb.Message.Chat.ID, cb.Message.ID, len(cb.Message.Photo) > 0, text); err != nil {
		b.log.Debug("telegram rewrite failed", "error", err)
	}
}

func (b *Bot) tmdb() *integration.TMDBClient {
	return integration.TMDB(b.settings.Get(), b.store.HTTPCache())
}

func titleLine(it *store.Item) string {
	if it.Year != 0 {
		return fmt.Sprintf("%s (%d)", it.Title, it.Year)
	}
	return it.Title
}

func itemLine(it *store.Item) string {
	line := titleLine(it) + "\n" + stateLabel(it.Status)
	if it.CapturedByUsername != "" {
		line += " · snagged by " + it.CapturedByUsername
	}
	return line
}

func stateLabel(s store.Status) string {
	switch s {
	case store.StatusAvailable:
		return "In library"
	case store.StatusWatched:
		return "In library"
	case store.StatusMonitored:
		return "Monitored"
	case store.StatusRequested:
		return "Requested"
	case store.StatusNeedsReview:
		return "Needs review"
	default:
		return "Snagged"
	}
}

func actionButtons(it *store.Item) [][]integration.TelegramButton {
	var rows [][]integration.TelegramButton
	if it.Status == store.StatusNew {
		label := "Send to Radarr"
		if it.MediaType == store.TV {
			label = "Send to Sonarr"
		}
		rows = append(rows, []integration.TelegramButton{{Text: label, Data: fmt.Sprintf("s:%d", it.ID)}})
	}
	rows = append(rows, []integration.TelegramButton{{Text: "Remove", Data: fmt.Sprintf("d:%d", it.ID)}})
	return rows
}

func mediaCode(t store.MediaType) string {
	if t == store.TV {
		return "t"
	}
	return "m"
}
