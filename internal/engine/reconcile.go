package engine

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirrobot01/snagarr/internal/config"
	"github.com/sirrobot01/snagarr/internal/integration"
	"github.com/sirrobot01/snagarr/internal/store"
)

// The library is swept incrementally on every pass and fully once a day. Only a
// full sweep can tell a deleted title from one that simply was not touched.
const fullSweepInterval = 24 * time.Hour

// entityTTL matches the PRD's 7-day cache lifetime for TMDB metadata.
const entityTTL = 7 * 24 * time.Hour

// Reconciler keeps the local indexes fresh and derives every item's state
// from them.
type Reconciler struct {
	store    *store.Store
	settings *config.Manager
	log      *slog.Logger

	mu      sync.Mutex
	running atomic.Bool
	state   syncState
	trigger chan struct{}
}

// syncState is persisted so restarts do not force a full library sweep. The
// library stamps are per service: a media server a member connects today must
// get its own first full sweep, whatever the others did yesterday.
type syncState struct {
	Library      map[int64]librarySync `json:"library"`
	ArrAt        time.Time             `json:"arr_at"`
	CollectionAt time.Time             `json:"collection_at"`
}

type librarySync struct {
	SyncedAt    time.Time `json:"synced_at"`
	FullSweepAt time.Time `json:"full_sweep_at"`
}

const syncStateKey = "sync_state"

func NewReconciler(s *store.Store, settings *config.Manager, log *slog.Logger) *Reconciler {
	e := &Reconciler{store: s, settings: settings, log: log, trigger: make(chan struct{}, 1)}
	// Unreadable sync state only costs one extra full sweep.
	if raw, err := s.Setting(context.Background(), syncStateKey); err == nil {
		_ = json.Unmarshal(raw, &e.state)
	}
	if e.state.Library == nil {
		e.state.Library = map[int64]librarySync{}
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

func (e *Reconciler) Status() Status {
	e.mu.Lock()
	defer e.mu.Unlock()

	// The household has one footer, so the most recent sweep speaks for all of
	// the media servers.
	var library time.Time
	for _, l := range e.state.Library {
		if l.SyncedAt.After(library) {
			library = l.SyncedAt
		}
	}
	return Status{
		LibraryAt:    syncedAt(library),
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
func (e *Reconciler) Trigger() {
	select {
	case e.trigger <- struct{}{}:
	default:
	}
}

// Start runs the loop until ctx is cancelled. The interval is read before each
// wait, so changing it in settings takes effect without a restart.
func (e *Reconciler) Start(ctx context.Context) {
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

func (e *Reconciler) runLogged(ctx context.Context) {
	if err := e.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		e.log.Error("reconcile failed", "error", err)
	}
}

// Run performs one full pass over every member's services. Individual sync
// failures are logged and skipped rather than aborting: a stale index still
// answers, and the next pass retries.
func (e *Reconciler) Run(ctx context.Context) error {
	if !e.running.CompareAndSwap(false, true) {
		return nil
	}
	defer e.running.Store(false)

	started := time.Now()
	settings := e.settings.Get()
	house, err := e.household(ctx)
	if err != nil {
		return err
	}

	e.syncArr(ctx, house)
	e.syncRequests(ctx, house)
	e.syncLibrary(ctx, house)

	index, err := e.store.LoadStateIndex(ctx)
	if err != nil {
		return err
	}
	available, err := e.applyStates(ctx, house, index, settings)
	if err != nil {
		return err
	}
	e.syncCollection(ctx, house)
	e.refreshEntities(ctx, settings)

	if err := e.store.PurgeExpiredCache(ctx); err != nil {
		e.log.Warn("could not purge response cache", "error", err)
	}
	e.persistState(ctx)

	e.log.Info("reconcile complete", "took", time.Since(started).Round(time.Millisecond), "became_available", available)
	return nil
}

// household builds every enabled service in the install. A record that cannot
// be decoded is reported and left out, so one bad row never costs a pass.
func (e *Reconciler) household(ctx context.Context) (integration.Household, error) {
	services, err := e.store.Services(ctx)
	if err != nil {
		return integration.Household{}, err
	}
	house, err := integration.BuildHousehold(services)
	if err != nil {
		e.log.Warn("some services could not be built", "error", err)
	}
	return house, nil
}

func (e *Reconciler) syncArr(ctx context.Context, house integration.Household) {
	for _, r := range house.Radarrs {
		items, err := r.Client.List(ctx)
		if err != nil {
			e.log.Warn("radarr sync failed", "service", r.Service.ID, "name", r.Service.Name, "error", err)
			continue
		}
		if err := e.store.ReplaceArrIndex(ctx, r.Service.ID, toArrEntries(items)); err != nil {
			e.log.Warn("radarr index write failed", "service", r.Service.ID, "error", err)
			continue
		}
		e.mark(&e.state.ArrAt)
	}
	for _, s := range house.Sonarrs {
		items, err := s.Client.List(ctx)
		if err != nil {
			e.log.Warn("sonarr sync failed", "service", s.Service.ID, "name", s.Service.Name, "error", err)
			continue
		}
		if err := e.store.ReplaceArrIndex(ctx, s.Service.ID, toArrEntries(items)); err != nil {
			e.log.Warn("sonarr index write failed", "service", s.Service.ID, "error", err)
			continue
		}
		e.mark(&e.state.ArrAt)
	}
}

func (e *Reconciler) syncRequests(ctx context.Context, house integration.Household) {
	for _, o := range house.Overseerrs {
		requests, err := o.Client.List(ctx)
		if err != nil {
			e.log.Warn("overseerr sync failed", "service", o.Service.ID, "name", o.Service.Name, "error", err)
			continue
		}
		entries := make([]store.RequestEntry, 0, len(requests))
		for _, r := range requests {
			entries = append(entries, store.RequestEntry{
				RequestID: r.ID, TMDBID: int64(r.TMDBID), MediaType: r.Type, Status: r.Status,
			})
		}
		if err := e.store.ReplaceRequests(ctx, o.Service.ID, entries); err != nil {
			e.log.Warn("request index write failed", "service", o.Service.ID, "error", err)
		}
	}
}

func (e *Reconciler) syncLibrary(ctx context.Context, house integration.Household) {
	for _, lib := range house.Libraries {
		e.syncLibraryService(ctx, lib)
	}
}

func (e *Reconciler) syncLibraryService(ctx context.Context, lib integration.Library) {
	e.mu.Lock()
	last := e.state.Library[lib.Service.ID]
	e.mu.Unlock()

	full := time.Since(last.FullSweepAt) > fullSweepInterval
	since := last.SyncedAt
	if full {
		since = time.Time{}
	}

	sweepStart := time.Now().UTC()
	items, err := lib.Client.Items(ctx, lib.Config.SectionIDs, since)
	if err != nil {
		e.log.Warn("library sync failed", "service", lib.Service.ID, "name", lib.Service.Name, "error", err)
		return
	}

	entries := make([]store.LibraryEntry, 0, len(items))
	for _, it := range items {
		entries = append(entries, store.LibraryEntry{
			ProviderItemID: it.ProviderItemID,
			TMDBID:         int64(it.TMDBID), IMDBID: it.IMDBID, TVDBID: int64(it.TVDBID),
			MediaType: it.Type, Title: it.Title, Year: it.Year, AddedAt: it.AddedAt,
		})
	}
	if err := e.store.UpsertLibrary(ctx, lib.Service.ID, entries); err != nil {
		e.log.Warn("library index write failed", "service", lib.Service.ID, "error", err)
		return
	}

	if full {
		removed, err := e.store.TombstoneLibrary(ctx, lib.Service.ID, sweepStart)
		if err != nil {
			e.log.Warn("library tombstone failed", "service", lib.Service.ID, "error", err)
		} else if removed > 0 {
			e.log.Info("library titles removed", "service", lib.Service.ID, "count", removed)
		}
		last.FullSweepAt = time.Now().UTC()
	}
	last.SyncedAt = sweepStart

	e.mu.Lock()
	e.state.Library[lib.Service.ID] = last
	e.mu.Unlock()
	e.log.Debug("library synced", "service", lib.Service.ID, "titles", len(entries), "full", full)
}

// applyStates recomputes every snagged item's status from the local indexes and
// notifies on the transitions that matter.
func (e *Reconciler) applyStates(ctx context.Context, house integration.Household, index *store.StateIndex,
	settings config.Settings) (int, error) {
	items, err := e.store.SnaggedItems(ctx)
	if err != nil {
		return 0, err
	}
	watched, err := e.store.WatchedItemIDs(ctx)
	if err != nil {
		return 0, err
	}
	admins := e.adminIDs(ctx)

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
			e.notifyAvailable(ctx, house, settings, admins, it)
		}
	}
	return becameAvailable, nil
}

// notifyAvailable carries the original capture context, which is the whole
// point of the nudge: the reminder has to say why this title is on the list.
// The push goes to the capturer's own ntfy, and to an admin's when they have
// none — an unowned push still has to reach somebody.
func (e *Reconciler) notifyAvailable(ctx context.Context, house integration.Household, settings config.Settings,
	admins []int64, it store.Item) {
	target := house.NtfyFor(append([]int64{it.CapturedBy}, admins...)...)
	if target == nil {
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

	msg := integration.Message{
		Title:    "Ready to watch",
		Body:     body,
		Tags:     []string{"clapper"},
		Priority: target.Config.Priority,
	}
	if settings.General.PublicURL != "" {
		msg.ClickURL = settings.General.PublicURL
	}
	if err := target.Client.Send(ctx, msg); err != nil {
		e.log.Warn("availability notification failed", "item", it.ID, "error", err)
		return
	}
	if err := e.store.MarkNotified(ctx, it.ID); err != nil {
		e.log.Warn("could not stamp notification", "item", it.ID, "error", err)
	}
}

// syncCollection applies (snagged ∩ available) − watched to every media server.
// This is the only place Snagarr mutates a library. Collections are personal:
// each server gets the snagged titles it actually holds, and never a title that
// only exists on somebody else's.
func (e *Reconciler) syncCollection(ctx context.Context, house integration.Household) {
	if len(house.Libraries) == 0 {
		return
	}
	items, err := e.store.SnaggedItems(ctx)
	if err != nil {
		e.log.Warn("could not read snagged items", "error", err)
		return
	}
	held, err := e.store.LibraryMembers(ctx)
	if err != nil {
		e.log.Warn("could not read library members", "error", err)
		return
	}

	for _, lib := range house.Libraries {
		if lib.Config.CollectionName == "" {
			continue
		}
		titles := held[lib.Service.ID]
		var members []string
		for _, it := range items {
			if it.Status != store.StatusAvailable {
				continue
			}
			if id, ok := titles[store.TitleKey{TMDBID: it.TMDBID, MediaType: it.MediaType}]; ok {
				members = append(members, id)
			}
		}

		if err := lib.Client.SyncCollection(ctx, lib.Config.CollectionName, members); err != nil {
			e.log.Warn("collection sync failed", "service", lib.Service.ID,
				"collection", lib.Config.CollectionName, "error", err)
			continue
		}
		e.mark(&e.state.CollectionAt)
		e.log.Debug("collection synced", "service", lib.Service.ID,
			"collection", lib.Config.CollectionName, "members", len(members))
	}
}

func (e *Reconciler) refreshEntities(ctx context.Context, settings config.Settings) {
	catalogue := integration.TMDB(settings, e.store.HTTPCache())
	if catalogue == nil {
		return
	}
	stale, err := e.store.StaleSnaggedTitles(ctx, entityTTL)
	if err != nil {
		e.log.Warn("could not list stale metadata", "error", err)
		return
	}
	for _, key := range stale {
		details, err := catalogue.Details(ctx, key.MediaType, int(key.TMDBID))
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
func (e *Reconciler) RefreshItem(ctx context.Context, itemID int64) error {
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
func (e *Reconciler) MarkAvailable(ctx context.Context, tmdbID int64, t store.MediaType) error {
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

	if it.NotifiedAt.IsZero() {
		house, err := e.household(ctx)
		if err != nil {
			return err
		}
		e.notifyAvailable(ctx, house, e.settings.Get(), e.adminIDs(ctx), *it)
	}
	e.Trigger()
	return nil
}

// MarkWatched is the playback webhook path. The item leaves the Snagged
// collection on the next pass, so the collection stays live state.
func (e *Reconciler) MarkWatched(ctx context.Context, tmdbID int64, t store.MediaType, source string) error {
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

// adminIDs is the last resort for a personal service: a household that has to
// act on somebody's behalf falls back to whoever runs the install.
func (e *Reconciler) adminIDs(ctx context.Context) []int64 {
	users, err := e.store.Users(ctx)
	if err != nil {
		e.log.Warn("could not list household members", "error", err)
		return nil
	}
	var ids []int64
	for _, u := range users {
		if u.Role == store.RoleAdmin {
			ids = append(ids, u.ID)
		}
	}
	return ids
}

func (e *Reconciler) mark(field *time.Time) {
	e.mu.Lock()
	defer e.mu.Unlock()
	*field = time.Now().UTC()
}

func (e *Reconciler) persistState(ctx context.Context) {
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

func toArrEntries(items []integration.ArrItem) []store.ArrEntry {
	entries := make([]store.ArrEntry, 0, len(items))
	for _, it := range items {
		entries = append(entries, store.ArrEntry{
			ArrID: it.ID, TMDBID: int64(it.TMDBID), TVDBID: int64(it.TVDBID),
			IMDBID: it.IMDBID, Title: it.Title, Year: it.Year,
			Monitored: it.Monitored, HasFile: it.HasFile, QualityProfileID: it.QualityProfileID,
		})
	}
	return entries
}
