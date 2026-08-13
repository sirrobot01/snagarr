// Package reconcile keeps the local indexes fresh and derives every item's
// state from them. State is set arithmetic against local tables — the request
// path never waits on Plex, Radarr or Sonarr.
package reconcile

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirrobot01/snagarr/internal/arr"
	"github.com/sirrobot01/snagarr/internal/clients"
	"github.com/sirrobot01/snagarr/internal/config"
	"github.com/sirrobot01/snagarr/internal/media"
	"github.com/sirrobot01/snagarr/internal/notify"
	"github.com/sirrobot01/snagarr/internal/store"
)

// The library is swept incrementally on every pass and fully once a day. Only a
// full sweep can tell a deleted title from one that simply was not touched.
const fullSweepInterval = 24 * time.Hour

// entityTTL matches the PRD's 7-day cache lifetime for TMDB metadata.
const entityTTL = 7 * 24 * time.Hour

type Engine struct {
	store    *store.Store
	settings *config.Manager
	log      *slog.Logger

	mu      sync.Mutex
	running atomic.Bool
	state   syncState
	trigger chan struct{}
}

// syncState is persisted so restarts do not force a full library sweep.
type syncState struct {
	LibraryAt    time.Time `json:"library_at"`
	FullSweepAt  time.Time `json:"full_sweep_at"`
	ArrAt        time.Time `json:"arr_at"`
	CollectionAt time.Time `json:"collection_at"`
}

const syncStateKey = "sync_state"

func New(s *store.Store, settings *config.Manager, log *slog.Logger) *Engine {
	e := &Engine{store: s, settings: settings, log: log, trigger: make(chan struct{}, 1)}
	// Unreadable sync state only costs one extra full sweep.
	if raw, err := s.Setting(context.Background(), syncStateKey); err == nil {
		_ = json.Unmarshal(raw, &e.state)
	}
	return e
}

// Status reports what the UI shows in the footer and on the settings page. A
// sync that has never run reports null rather than the zero time.
type Status struct {
	LibraryAt    *time.Time `json:"library_at"`
	ArrAt        *time.Time `json:"arr_at"`
	CollectionAt *time.Time `json:"collection_at"`
	Running      bool       `json:"running"`
}

func (e *Engine) Status() Status {
	e.mu.Lock()
	defer e.mu.Unlock()
	return Status{
		LibraryAt:    syncedAt(e.state.LibraryAt),
		ArrAt:        syncedAt(e.state.ArrAt),
		CollectionAt: syncedAt(e.state.CollectionAt),
		Running:      e.running.Load(),
	}
}

func syncedAt(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

// Trigger asks for a pass without blocking. A pass already in flight absorbs
// the request.
func (e *Engine) Trigger() {
	select {
	case e.trigger <- struct{}{}:
	default:
	}
}

// Start runs the loop until ctx is cancelled. The interval is read before each
// wait, so changing it in settings takes effect without a restart.
func (e *Engine) Start(ctx context.Context) {
	e.runLogged(ctx)
	for {
		interval := time.Duration(e.settings.Get().General.ReconcileInterval)
		if interval <= 0 {
			interval = 15 * time.Minute
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-e.trigger:
			timer.Stop()
		case <-timer.C:
		}
		e.runLogged(ctx)
	}
}

func (e *Engine) runLogged(ctx context.Context) {
	if err := e.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		e.log.Error("reconcile failed", "error", err)
	}
}

// Run performs one full pass. Individual sync failures are logged and skipped
// rather than aborting: a stale index still answers, and the next pass retries.
func (e *Engine) Run(ctx context.Context) error {
	if !e.running.CompareAndSwap(false, true) {
		return nil
	}
	defer e.running.Store(false)

	started := time.Now()
	settings := e.settings.Get()
	set := clients.Build(settings, e.store.HTTPCache())

	e.syncArr(ctx, set)
	e.syncRequests(ctx, set)
	e.syncLibrary(ctx, set, settings)

	index, err := e.store.LoadStateIndex(ctx)
	if err != nil {
		return err
	}
	available, err := e.applyStates(ctx, set, index, settings)
	if err != nil {
		return err
	}
	e.syncCollection(ctx, set, settings, index)
	e.refreshEntities(ctx, set)

	if err := e.store.PurgeExpiredCache(ctx); err != nil {
		e.log.Warn("could not purge response cache", "error", err)
	}
	e.persistState(ctx)

	e.log.Info("reconcile complete", "took", time.Since(started).Round(time.Millisecond), "became_available", available)
	return nil
}

func (e *Engine) syncArr(ctx context.Context, set clients.Set) {
	if set.Radarr != nil {
		items, err := set.Radarr.List(ctx)
		if err != nil {
			e.log.Warn("radarr sync failed", "error", err)
		} else if err := e.store.ReplaceArrIndex(ctx, "radarr", toArrEntries("radarr", items)); err != nil {
			e.log.Warn("radarr index write failed", "error", err)
		} else {
			e.mark(&e.state.ArrAt)
		}
	}
	if set.Sonarr != nil {
		items, err := set.Sonarr.List(ctx)
		if err != nil {
			e.log.Warn("sonarr sync failed", "error", err)
		} else if err := e.store.ReplaceArrIndex(ctx, "sonarr", toArrEntries("sonarr", items)); err != nil {
			e.log.Warn("sonarr index write failed", "error", err)
		} else {
			e.mark(&e.state.ArrAt)
		}
	}
}

func (e *Engine) syncRequests(ctx context.Context, set clients.Set) {
	if set.Overseerr == nil {
		return
	}
	requests, err := set.Overseerr.List(ctx)
	if err != nil {
		e.log.Warn("overseerr sync failed", "error", err)
		return
	}
	entries := make([]store.RequestEntry, 0, len(requests))
	for _, r := range requests {
		entries = append(entries, store.RequestEntry{
			RequestID: r.ID, TMDBID: int64(r.TMDBID), MediaType: r.Type, Status: r.Status,
		})
	}
	if err := e.store.ReplaceRequests(ctx, entries); err != nil {
		e.log.Warn("request index write failed", "error", err)
	}
}

func (e *Engine) syncLibrary(ctx context.Context, set clients.Set, settings config.Settings) {
	if set.Library == nil {
		return
	}
	e.mu.Lock()
	lastFull, lastSync := e.state.FullSweepAt, e.state.LibraryAt
	e.mu.Unlock()

	full := time.Since(lastFull) > fullSweepInterval
	since := lastSync
	if full {
		since = time.Time{}
	}

	sweepStart := time.Now().UTC()
	items, err := set.Library.Items(ctx, settings.Library.SectionIDs, since)
	if err != nil {
		e.log.Warn("library sync failed", "error", err)
		return
	}

	entries := make([]store.LibraryEntry, 0, len(items))
	for _, it := range items {
		entries = append(entries, store.LibraryEntry{
			Provider: settings.Library.Provider, ProviderItemID: it.ProviderItemID,
			TMDBID: int64(it.TMDBID), IMDBID: it.IMDBID, TVDBID: int64(it.TVDBID),
			MediaType: it.Type, Title: it.Title, Year: it.Year, AddedAt: it.AddedAt,
		})
	}
	if err := e.store.UpsertLibrary(ctx, entries); err != nil {
		e.log.Warn("library index write failed", "error", err)
		return
	}

	if full {
		removed, err := e.store.TombstoneLibrary(ctx, settings.Library.Provider, sweepStart)
		if err != nil {
			e.log.Warn("library tombstone failed", "error", err)
		} else if removed > 0 {
			e.log.Info("library titles removed", "count", removed)
		}
		e.mark(&e.state.FullSweepAt)
	}
	e.mu.Lock()
	e.state.LibraryAt = sweepStart
	e.mu.Unlock()
	e.log.Debug("library synced", "titles", len(entries), "full", full)
}

// applyStates recomputes every snagged item's status from the local indexes and
// notifies on the transitions that matter.
func (e *Engine) applyStates(ctx context.Context, set clients.Set, index *store.StateIndex, settings config.Settings) (int, error) {
	items, err := e.store.SnaggedItems(ctx)
	if err != nil {
		return 0, err
	}
	watched, err := e.store.WatchedItemIDs(ctx)
	if err != nil {
		return 0, err
	}

	var becameAvailable int
	for _, it := range items {
		if it.Status == store.StatusNeedsReview {
			continue
		}
		next := index.State(store.TitleKey{TMDBID: it.TMDBID, MediaType: it.MediaType})
		if watched[it.ID] {
			next = store.StatusWatched
		}
		if next == it.Status {
			continue
		}

		var availableAt time.Time
		if next == store.StatusAvailable {
			availableAt = time.Now().UTC()
		}
		if err := e.store.SetStatus(ctx, it.ID, next, availableAt); err != nil {
			e.log.Warn("could not update item state", "item", it.ID, "error", err)
			continue
		}
		e.log.Debug("item state changed", "item", it.ID, "title", it.Title, "from", it.Status, "to", next)

		if next == store.StatusAvailable && it.NotifiedAt.IsZero() {
			becameAvailable++
			e.notifyAvailable(ctx, set, settings, it)
		}
	}
	return becameAvailable, nil
}

// notifyAvailable carries the original capture context, which is the whole
// point of the nudge: the reminder has to say why this title is on the list.
func (e *Engine) notifyAvailable(ctx context.Context, set clients.Set, settings config.Settings, it store.Item) {
	if set.Ntfy == nil {
		return
	}
	body := it.Title + " is ready"
	if it.CapturedByName != "" {
		body += " — snagged by " + it.CapturedByName
	}
	if !it.CapturedAt.IsZero() {
		body += ", " + it.CapturedAt.Format("2 Jan")
	}
	if it.Source != "" {
		body += ", from " + string(it.Source)
	}

	msg := notify.Message{
		Title:    "Ready to watch",
		Body:     body,
		Tags:     []string{"clapper"},
		Priority: settings.Ntfy.Priority,
	}
	if settings.General.PublicURL != "" {
		msg.ClickURL = settings.General.PublicURL
	}
	if err := set.Ntfy.Send(ctx, msg); err != nil {
		e.log.Warn("availability notification failed", "item", it.ID, "error", err)
		return
	}
	if err := e.store.MarkNotified(ctx, it.ID); err != nil {
		e.log.Warn("could not stamp notification", "item", it.ID, "error", err)
	}
}

// syncCollection applies (snagged ∩ available) − watched to the media server.
// This is the only place Snagarr mutates the library.
func (e *Engine) syncCollection(ctx context.Context, set clients.Set, settings config.Settings, index *store.StateIndex) {
	if set.Library == nil || settings.Library.CollectionName == "" {
		return
	}
	items, err := e.store.SnaggedItems(ctx)
	if err != nil {
		e.log.Warn("could not read snagged items", "error", err)
		return
	}

	var members []string
	for _, it := range items {
		if it.Status != store.StatusAvailable {
			continue
		}
		if id, ok := index.Library[store.TitleKey{TMDBID: it.TMDBID, MediaType: it.MediaType}]; ok {
			members = append(members, id)
		}
	}

	if err := set.Library.SyncCollection(ctx, settings.Library.CollectionName, members); err != nil {
		e.log.Warn("collection sync failed", "collection", settings.Library.CollectionName, "error", err)
		return
	}
	e.mark(&e.state.CollectionAt)
	e.log.Debug("collection synced", "collection", settings.Library.CollectionName, "members", len(members))
}

func (e *Engine) refreshEntities(ctx context.Context, set clients.Set) {
	if set.TMDB == nil {
		return
	}
	stale, err := e.store.StaleSnaggedTitles(ctx, entityTTL)
	if err != nil {
		e.log.Warn("could not list stale metadata", "error", err)
		return
	}
	for _, key := range stale {
		details, err := set.TMDB.Details(ctx, key.MediaType, int(key.TMDBID))
		if err != nil {
			e.log.Debug("metadata refresh failed", "tmdb_id", key.TMDBID, "error", err)
			continue
		}
		if err := e.store.PutEntity(ctx, store.Entity{
			TMDBID: int64(details.TMDBID), MediaType: details.Type, Title: details.Title,
			Year: details.Year, PosterPath: details.PosterPath, BackdropPath: details.BackdropPath,
			Overview: details.Overview, Genres: details.Genres, Runtime: details.Runtime,
			Popularity: details.Popularity,
		}); err != nil {
			e.log.Warn("could not cache metadata", "tmdb_id", key.TMDBID, "error", err)
		}
	}
}

// RefreshItem recomputes one item's state, for the paths that must feel
// immediate: resolving a capture, sending to an *arr, or an inbound webhook.
func (e *Engine) RefreshItem(ctx context.Context, itemID int64) error {
	it, err := e.store.Item(ctx, itemID)
	if err != nil {
		return err
	}
	if it.TMDBID == 0 || it.Status == store.StatusNeedsReview {
		return nil
	}
	index, err := e.store.LoadStateIndex(ctx)
	if err != nil {
		return err
	}
	next := index.State(store.TitleKey{TMDBID: it.TMDBID, MediaType: it.MediaType})
	if next == it.Status {
		return nil
	}
	var availableAt time.Time
	if next == store.StatusAvailable {
		availableAt = time.Now().UTC()
	}
	return e.store.SetStatus(ctx, itemID, next, availableAt)
}

// MarkAvailable is the webhook fast path: Radarr and Sonarr tell us a file
// landed long before the next scheduled sync would notice.
func (e *Engine) MarkAvailable(ctx context.Context, tmdbID int64, t media.Type) error {
	it, err := e.store.ItemByTMDB(ctx, tmdbID, t)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if it.Status == store.StatusAvailable || it.Status == store.StatusWatched {
		return nil
	}
	if err := e.store.SetStatus(ctx, it.ID, store.StatusAvailable, time.Now().UTC()); err != nil {
		return err
	}

	settings := e.settings.Get()
	set := clients.Build(settings, e.store.HTTPCache())
	if it.NotifiedAt.IsZero() {
		e.notifyAvailable(ctx, set, settings, *it)
	}
	e.Trigger()
	return nil
}

// MarkWatched is the playback webhook path. The item leaves the Snagged
// collection on the next pass, so the collection stays live state.
func (e *Engine) MarkWatched(ctx context.Context, tmdbID int64, t media.Type, source string) error {
	it, err := e.store.ItemByTMDB(ctx, tmdbID, t)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := e.store.MarkWatched(ctx, it.ID, 0, source); err != nil {
		return err
	}
	if err := e.store.SetStatus(ctx, it.ID, store.StatusWatched, time.Time{}); err != nil {
		return err
	}
	e.Trigger()
	return nil
}

func (e *Engine) mark(field *time.Time) {
	e.mu.Lock()
	defer e.mu.Unlock()
	*field = time.Now().UTC()
}

func (e *Engine) persistState(ctx context.Context) {
	e.mu.Lock()
	raw, err := json.Marshal(e.state)
	e.mu.Unlock()
	if err != nil {
		return
	}
	if err := e.store.SetSetting(ctx, syncStateKey, raw); err != nil {
		e.log.Warn("could not persist sync state", "error", err)
	}
}

func toArrEntries(source string, items []arr.Item) []store.ArrEntry {
	entries := make([]store.ArrEntry, 0, len(items))
	for _, it := range items {
		entries = append(entries, store.ArrEntry{
			Source: source, ArrID: it.ID, TMDBID: int64(it.TMDBID), TVDBID: int64(it.TVDBID),
			IMDBID: it.IMDBID, Title: it.Title, Year: it.Year,
			Monitored: it.Monitored, HasFile: it.HasFile, QualityProfileID: it.QualityProfileID,
		})
	}
	return entries
}
