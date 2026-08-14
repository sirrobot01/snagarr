package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/sirrobot01/snagarr/internal/integration"
	"github.com/sirrobot01/snagarr/internal/store"
)

type captureRequest struct {
	Query     string          `json:"query"`
	URL       string          `json:"url"`
	TMDBID    int64           `json:"tmdb_id"`
	MediaType store.MediaType `json:"media_type"`
	Source    store.Source    `json:"source"`
	Note      string          `json:"note"`
	SourceURL string          `json:"source_url"`
}

// capture is the endpoint the whole product hangs off. It must never lose an
// input: anything it cannot identify is stored as needs_review with the raw
// text intact.
func (s *Server) capture(w http.ResponseWriter, r *http.Request) {
	var req captureRequest
	if !decode(w, r, &req) {
		return
	}
	if req.Source == "" {
		req.Source = store.SourceAPI
	}

	// Picking a search result already carries the exact identity, so that path
	// resolves inline and the client gets a finished item back.
	if req.TMDBID != 0 {
		s.captureKnownTitle(w, r, req)
		return
	}

	raw := req.Query
	if raw == "" {
		raw = req.URL
	}
	if raw == "" {
		writeError(w, http.StatusBadRequest, codeBadRequest, "send one of query, url or tmdb_id")
		return
	}

	it := &store.Item{
		Title: raw, RawInput: raw, Status: store.StatusNeedsReview,
		Source: req.Source, SourceURL: req.SourceURL, Note: req.Note,
		CapturedBy: userFrom(r).ID,
	}
	if err := s.store.CreateItem(r.Context(), it); err != nil {
		s.writeStoreError(w, err, "item")
		return
	}

	// Read it back so the response carries the capturer's name, which only the
	// join supplies — and before resolution starts, so a duplicate capture the
	// resolver deletes cannot 404 its own 202.
	saved, err := s.store.Item(r.Context(), it.ID)
	if err != nil {
		s.writeStoreError(w, err, "item")
		return
	}

	// Resolution outlives the request: the capture is already safe on disk.
	// Without a TMDB key there is nothing to resolve against, and the item is
	// already parked in needs_review, so it waits for the key instead.
	if catalogue := s.tmdb(); catalogue != nil {
		go s.resolveInBackground(context.WithoutCancel(r.Context()), catalogue, it.ID)
	}
	writeJSON(w, http.StatusAccepted, newItemDTO(*saved))
}

func (s *Server) captureKnownTitle(w http.ResponseWriter, r *http.Request, req captureRequest) {
	ctx := r.Context()
	if !req.MediaType.Valid() {
		writeError(w, http.StatusBadRequest, codeBadRequest, "media_type must be movie or tv")
		return
	}

	if existing, err := s.store.ItemByTMDB(ctx, req.TMDBID, req.MediaType); err == nil {
		writeJSON(w, http.StatusOK, newItemDTO(*existing))
		return
	} else if !errors.Is(err, store.ErrNotFound) {
		s.writeStoreError(w, err, "item")
		return
	}

	catalogue := s.tmdb()
	if catalogue == nil {
		writeError(w, http.StatusServiceUnavailable, codeNotConfigured, "TMDB is not configured")
		return
	}
	details, err := catalogue.Details(ctx, req.MediaType, int(req.TMDBID))
	if err != nil {
		writeError(w, http.StatusBadGateway, codeUpstreamError, "TMDB lookup failed: %v", err)
		return
	}

	raw := req.Query
	if raw == "" {
		raw = details.Title
	}
	it := &store.Item{
		TMDBID: int64(details.TMDBID), MediaType: details.Type, Title: details.Title,
		Year: details.Year, PosterPath: details.PosterPath, Status: store.StatusNew,
		RawInput: raw, Source: req.Source, SourceURL: req.SourceURL, Note: req.Note,
		CapturedBy: userFrom(r).ID, ResolvedAt: time.Now().UTC(),
	}
	if err := s.store.CreateItem(ctx, it); err != nil {
		// A concurrent capture of the same title won the unique index; answer
		// with the item it created, the same as the idempotency check above.
		if errors.Is(err, store.ErrConflict) {
			if existing, lookupErr := s.store.ItemByTMDB(ctx, req.TMDBID, req.MediaType); lookupErr == nil {
				writeJSON(w, http.StatusOK, newItemDTO(*existing))
				return
			}
		}
		s.writeStoreError(w, err, "item")
		return
	}

	s.cacheEntity(ctx, details)
	if err := s.reconciler.RefreshItem(ctx, it.ID); err != nil {
		s.log.Warn("could not compute state for new item", "item", it.ID, "error", err)
	}
	// Inline, so the item this returns already carries the state the send left
	// it in. The state is what the client draws its badge from.
	s.autoSend(ctx, it.ID)

	saved, err := s.store.Item(ctx, it.ID)
	if err != nil {
		s.writeStoreError(w, err, "item")
		return
	}
	writeJSON(w, http.StatusCreated, newItemDTO(*saved))
}

func (s *Server) resolveInBackground(ctx context.Context, client *integration.TMDBClient, itemID int64) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	if err := s.resolver.Resolve(ctx, client, itemID); err != nil {
		s.log.Warn("capture could not be resolved", "item", itemID, "error", err)
		return
	}
	if err := s.reconciler.RefreshItem(ctx, itemID); err != nil {
		s.log.Warn("could not compute state after resolve", "item", itemID, "error", err)
	}
	s.autoSend(ctx, itemID)
}

func (s *Server) listItems(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := store.ItemFilter{
		Status:    store.Status(q.Get("status")),
		MediaType: store.MediaType(q.Get("type")),
		Query:     q.Get("q"),
		Archived:  q.Get("archived") == "true",
	}
	filter.CapturedBy, _ = strconv.ParseInt(q.Get("captured_by"), 10, 64)
	filter.Limit, _ = strconv.Atoi(q.Get("limit"))
	filter.Offset, _ = strconv.Atoi(q.Get("offset"))

	items, total, err := s.store.Items(r.Context(), filter)
	if err != nil {
		s.writeStoreError(w, err, "items")
		return
	}
	dtos := make([]itemDTO, 0, len(items))
	for _, it := range items {
		dtos = append(dtos, newItemDTO(it))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": dtos, "total": total})
}

func (s *Server) getItem(w http.ResponseWriter, r *http.Request) {
	it, ok := s.loadItem(w, r)
	if !ok {
		return
	}
	dto := newItemDTO(*it)
	if it.Status == store.StatusNeedsReview {
		candidates, err := s.store.Candidates(r.Context(), it.ID)
		if err != nil {
			s.writeStoreError(w, err, "candidates")
			return
		}
		for _, c := range candidates {
			dto.Candidates = append(dto.Candidates, newCandidateDTO(c))
		}
	}
	writeJSON(w, http.StatusOK, dto)
}

func (s *Server) resolveItem(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TMDBID    int64           `json:"tmdb_id"`
		MediaType store.MediaType `json:"media_type"`
	}
	if !decode(w, r, &req) {
		return
	}
	it, ok := s.loadItem(w, r)
	if !ok {
		return
	}
	if !s.mayModify(w, r, it) {
		return
	}
	if !req.MediaType.Valid() {
		writeError(w, http.StatusBadRequest, codeBadRequest, "media_type must be movie or tv")
		return
	}

	catalogue := s.tmdb()
	if catalogue == nil {
		writeError(w, http.StatusServiceUnavailable, codeNotConfigured, "TMDB is not configured")
		return
	}
	details, err := catalogue.Details(r.Context(), req.MediaType, int(req.TMDBID))
	if err != nil {
		writeError(w, http.StatusBadGateway, codeUpstreamError, "TMDB lookup failed: %v", err)
		return
	}

	if err := s.store.Resolve(r.Context(), it.ID, store.Candidate{
		TMDBID: int64(details.TMDBID), MediaType: details.Type, Title: details.Title,
		Year: details.Year, PosterPath: details.PosterPath,
	}); err != nil {
		s.writeStoreError(w, err, "item")
		return
	}
	if err := s.store.SetCandidates(r.Context(), it.ID, nil); err != nil {
		s.log.Warn("could not clear candidates", "item", it.ID, "error", err)
	}
	s.cacheEntity(r.Context(), details)
	if err := s.reconciler.RefreshItem(r.Context(), it.ID); err != nil {
		s.log.Warn("could not compute state after resolve", "item", it.ID, "error", err)
	}
	s.autoSend(r.Context(), it.ID)

	s.respondWithItem(w, r, it.ID)
}

// sendItem pushes a title to a service. The action is personal: it uses the
// caller's own service, then the capturer's, then an admin's.
func (s *Server) sendItem(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Target string `json:"target"`
	}
	if !decode(w, r, &req) {
		return
	}
	it, ok := s.loadItem(w, r)
	if !ok {
		return
	}
	if it.TMDBID == 0 {
		writeError(w, http.StatusUnprocessableEntity, codeUnresolvable, "resolve this item before sending it")
		return
	}

	ctx := r.Context()
	house, err := s.household(ctx)
	if err != nil {
		s.writeStoreError(w, err, "services")
		return
	}
	owners := s.serviceOwners(ctx, userFrom(r), it.CapturedBy)

	status, err := s.send(ctx, house, *it, req.Target, owners)
	if errors.Is(err, errNoTarget) {
		writeError(w, http.StatusServiceUnavailable, codeNotConfigured, "nobody has a %s configured", req.Target)
		return
	}
	if errors.Is(err, errUnknownTarget) {
		writeError(w, http.StatusBadRequest, codeBadRequest, "target must be radarr, sonarr or overseerr")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadGateway, codeUpstreamError, "%s rejected the request: %v", req.Target, err)
		return
	}
	if err := s.store.SetStatus(ctx, it.ID, status, time.Time{}); err != nil {
		s.writeStoreError(w, err, "item")
		return
	}
	s.reconciler.Trigger()

	s.respondWithItem(w, r, it.ID)
}

var (
	errNoTarget      = errors.New("nobody has that service configured")
	errUnknownTarget = errors.New("unknown send target")
)

// send hands one title to one service and reports the state that leaves the
// item in. It is shared by the Send button and by automatic sending, which have
// to reach the same service with the same options.
func (s *Server) send(ctx context.Context, house integration.Household, it store.Item,
	target string, owners []int64) (store.Status, error) {
	var err error
	status := store.StatusMonitored

	switch target {
	case "radarr":
		radarr := house.RadarrFor(owners...)
		if radarr == nil {
			return "", errNoTarget
		}
		_, err = radarr.Client.Add(ctx, int(it.TMDBID), integration.AddOptions{
			QualityProfileID: radarr.Config.QualityProfileID,
			RootFolder:       radarr.Config.RootFolder,
			Monitor:          true,
			SearchOnAdd:      radarr.Config.SearchOnAdd,
		})

	case "sonarr":
		sonarr := house.SonarrFor(owners...)
		if sonarr == nil {
			return "", errNoTarget
		}
		ids := integration.ArrExternalIDs{TMDBID: int(it.TMDBID), Title: it.Title, Year: it.Year}
		// Sonarr keys on TVDB, so the TMDB ID has to be translated first.
		if catalogue := s.tmdb(); catalogue != nil {
			if external, idErr := catalogue.ExternalIDs(ctx, it.MediaType, int(it.TMDBID)); idErr == nil {
				ids.TVDBID, ids.IMDBID = external.TVDBID, external.IMDBID
			}
		}
		_, err = sonarr.Client.Add(ctx, ids, integration.AddOptions{
			QualityProfileID: sonarr.Config.QualityProfileID,
			RootFolder:       sonarr.Config.RootFolder,
			Monitor:          true,
			SearchOnAdd:      sonarr.Config.SearchOnAdd,
			SeasonFolder:     sonarr.Config.SeasonFolder,
		})

	case "overseerr":
		overseerr := house.OverseerrFor(owners...)
		if overseerr == nil {
			return "", errNoTarget
		}
		_, err = overseerr.Client.Create(ctx, int(it.TMDBID), it.MediaType)
		status = store.StatusRequested

	default:
		return "", errUnknownTarget
	}

	// A title the service already tracks is the outcome the user wanted.
	if err != nil && !errors.Is(err, integration.ErrAlreadyAdded) {
		return "", err
	}
	return status, nil
}

// autoSend is "add to Radarr or Sonarr by default": a title that resolves to
// something nobody owns yet goes straight to the capturer's own download
// manager. It spends nobody else's service, and it never fails a capture — an
// unreachable Radarr leaves the item exactly where the Send button can retry.
func (s *Server) autoSend(ctx context.Context, itemID int64) {
	if !s.settings.Get().General.AutoSend {
		return
	}
	it, err := s.store.Item(ctx, itemID)
	if err != nil || it.TMDBID == 0 {
		return
	}
	// Anything already in the library, monitored or requested needs nothing.
	if it.Status != store.StatusNew || it.Archived() {
		return
	}

	target := "radarr"
	if it.MediaType == store.TV {
		target = "sonarr"
	}
	house, err := s.household(ctx)
	if err != nil {
		s.log.Warn("automatic send could not read services", "item", itemID, "error", err)
		return
	}

	status, err := s.send(ctx, house, *it, target, []int64{it.CapturedBy})
	if errors.Is(err, errNoTarget) {
		return
	}
	if err != nil {
		s.log.Warn("automatic send failed", "item", itemID, "target", target, "error", err)
		return
	}
	if err := s.store.SetStatus(ctx, it.ID, status, time.Time{}); err != nil {
		s.log.Warn("automatic send could not record the state", "item", itemID, "error", err)
		return
	}
	s.log.Info("sent automatically", "item", itemID, "title", it.Title, "target", target)
	s.reconciler.Trigger()
}

func (s *Server) archiveItem(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Archived bool `json:"archived"`
	}
	if !decode(w, r, &req) {
		return
	}
	it, ok := s.loadItem(w, r)
	if !ok {
		return
	}
	if !s.mayModify(w, r, it) {
		return
	}
	if err := s.store.SetArchived(r.Context(), it.ID, req.Archived); err != nil {
		s.writeStoreError(w, err, "item")
		return
	}
	s.respondWithItem(w, r, it.ID)
}

func (s *Server) deleteItem(w http.ResponseWriter, r *http.Request) {
	it, ok := s.loadItem(w, r)
	if !ok {
		return
	}
	if !s.mayModify(w, r, it) {
		return
	}
	if err := s.store.DeleteItem(r.Context(), it.ID); err != nil {
		s.writeStoreError(w, err, "item")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) loadItem(w http.ResponseWriter, r *http.Request) (*store.Item, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, codeBadRequest, "item id must be a number")
		return nil, false
	}
	it, err := s.store.Item(r.Context(), id)
	if err != nil {
		s.writeStoreError(w, err, "item")
		return nil, false
	}
	return it, true
}

// mayModify lets a member act on their own capture — which is what makes undo
// work for everyone — while other people's items stay admin-only.
func (s *Server) mayModify(w http.ResponseWriter, r *http.Request, it *store.Item) bool {
	u := userFrom(r)
	if u.Role == store.RoleAdmin || it.CapturedBy == u.ID {
		return true
	}
	writeError(w, http.StatusForbidden, codeForbidden, "only an admin can change another member's item")
	return false
}

func (s *Server) respondWithItem(w http.ResponseWriter, r *http.Request, id int64) {
	it, err := s.store.Item(r.Context(), id)
	if err != nil {
		s.writeStoreError(w, err, "item")
		return
	}
	writeJSON(w, http.StatusOK, newItemDTO(*it))
}

// cacheEntity stores TMDB metadata so the item keeps its poster and overview
// when TMDB is later unreachable. A failure here costs artwork, never data.
func (s *Server) cacheEntity(ctx context.Context, d *integration.Details) {
	if err := s.store.PutEntity(ctx, store.Entity{
		TMDBID: int64(d.TMDBID), MediaType: d.Type, Title: d.Title,
		Year: d.Year, PosterPath: d.PosterPath, BackdropPath: d.BackdropPath,
		Overview: d.Overview, Genres: d.Genres, Runtime: d.Runtime,
		Popularity: d.Popularity,
	}); err != nil {
		s.log.Warn("could not cache metadata", "tmdb_id", d.TMDBID, "error", err)
	}
}

// tmdb builds the catalogue client. TMDB stays global — one key per install —
// and is nil when no key is set.
func (s *Server) tmdb() *integration.TMDBClient {
	return integration.TMDB(s.settings.Get(), s.store.HTTPCache())
}
