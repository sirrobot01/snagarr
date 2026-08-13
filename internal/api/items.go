package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/sirrobot01/snagarr/internal/arr"
	"github.com/sirrobot01/snagarr/internal/clients"
	"github.com/sirrobot01/snagarr/internal/media"
	"github.com/sirrobot01/snagarr/internal/store"
	"github.com/sirrobot01/snagarr/internal/tmdb"
)

type captureRequest struct {
	Query     string       `json:"query"`
	URL       string       `json:"url"`
	TMDBID    int64        `json:"tmdb_id"`
	MediaType media.Type   `json:"media_type"`
	Source    store.Source `json:"source"`
	Note      string       `json:"note"`
	SourceURL string       `json:"source_url"`
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

	// Resolution outlives the request: the capture is already safe on disk.
	// Without a TMDB key there is nothing to resolve against, and the item is
	// already parked in needs_review, so it waits for the key instead.
	if tmdbClient := s.clients().TMDB; tmdbClient != nil {
		go s.resolveInBackground(context.WithoutCancel(r.Context()), tmdbClient, it.ID)
	}

	// Read it back so the response carries the capturer's name, which only the
	// join supplies.
	saved, err := s.store.Item(r.Context(), it.ID)
	if err != nil {
		s.writeStoreError(w, err, "item")
		return
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

	set := s.clients()
	if set.TMDB == nil {
		writeError(w, http.StatusServiceUnavailable, codeNotConfigured, "TMDB is not configured")
		return
	}
	details, err := set.TMDB.Details(ctx, req.MediaType, int(req.TMDBID))
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
		s.writeStoreError(w, err, "item")
		return
	}

	s.store.PutEntity(ctx, store.Entity{
		TMDBID: int64(details.TMDBID), MediaType: details.Type, Title: details.Title,
		Year: details.Year, PosterPath: details.PosterPath, BackdropPath: details.BackdropPath,
		Overview: details.Overview, Genres: details.Genres, Runtime: details.Runtime,
		Popularity: details.Popularity,
	})
	if err := s.engine.RefreshItem(ctx, it.ID); err != nil {
		s.log.Warn("could not compute state for new item", "item", it.ID, "error", err)
	}

	saved, err := s.store.Item(ctx, it.ID)
	if err != nil {
		s.writeStoreError(w, err, "item")
		return
	}
	writeJSON(w, http.StatusCreated, newItemDTO(*saved))
}

func (s *Server) resolveInBackground(ctx context.Context, client *tmdb.Client, itemID int64) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	if err := s.resolver.Resolve(ctx, client, itemID); err != nil {
		s.log.Warn("capture could not be resolved", "item", itemID, "error", err)
		return
	}
	if err := s.engine.RefreshItem(ctx, itemID); err != nil {
		s.log.Warn("could not compute state after resolve", "item", itemID, "error", err)
	}
}

func (s *Server) listItems(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := store.ItemFilter{
		Status:    store.Status(q.Get("status")),
		MediaType: media.Type(q.Get("type")),
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
		TMDBID    int64      `json:"tmdb_id"`
		MediaType media.Type `json:"media_type"`
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

	set := s.clients()
	if set.TMDB == nil {
		writeError(w, http.StatusServiceUnavailable, codeNotConfigured, "TMDB is not configured")
		return
	}
	details, err := set.TMDB.Details(r.Context(), req.MediaType, int(req.TMDBID))
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
	s.store.SetCandidates(r.Context(), it.ID, nil)
	s.store.PutEntity(r.Context(), store.Entity{
		TMDBID: int64(details.TMDBID), MediaType: details.Type, Title: details.Title,
		Year: details.Year, PosterPath: details.PosterPath, BackdropPath: details.BackdropPath,
		Overview: details.Overview, Genres: details.Genres, Runtime: details.Runtime,
		Popularity: details.Popularity,
	})
	s.engine.RefreshItem(r.Context(), it.ID)

	s.respondWithItem(w, r, it.ID)
}

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

	settings := s.settings.Get()
	set := s.clients()
	var err error
	var status store.Status

	switch req.Target {
	case "radarr":
		if set.Radarr == nil {
			writeError(w, http.StatusServiceUnavailable, codeNotConfigured, "Radarr is not configured")
			return
		}
		_, err = set.Radarr.Add(r.Context(), int(it.TMDBID), arr.AddOptions{
			QualityProfileID: settings.Radarr.QualityProfileID,
			RootFolder:       settings.Radarr.RootFolder,
			Monitor:          true,
			SearchOnAdd:      settings.Radarr.SearchOnAdd,
		})
		status = store.StatusMonitored

	case "sonarr":
		if set.Sonarr == nil {
			writeError(w, http.StatusServiceUnavailable, codeNotConfigured, "Sonarr is not configured")
			return
		}
		ids := arr.ExternalIDs{TMDBID: int(it.TMDBID), Title: it.Title, Year: it.Year}
		// Sonarr keys on TVDB, so the TMDB ID has to be translated first.
		if set.TMDB != nil {
			if external, idErr := set.TMDB.ExternalIDs(r.Context(), it.MediaType, int(it.TMDBID)); idErr == nil {
				ids.TVDBID, ids.IMDBID = external.TVDBID, external.IMDBID
			}
		}
		_, err = set.Sonarr.Add(r.Context(), ids, arr.AddOptions{
			QualityProfileID: settings.Sonarr.QualityProfileID,
			RootFolder:       settings.Sonarr.RootFolder,
			Monitor:          true,
			SearchOnAdd:      settings.Sonarr.SearchOnAdd,
			SeasonFolder:     settings.Sonarr.SeasonFolder,
		})
		status = store.StatusMonitored

	case "overseerr":
		if set.Overseerr == nil {
			writeError(w, http.StatusServiceUnavailable, codeNotConfigured, "Overseerr is not configured")
			return
		}
		_, err = set.Overseerr.Create(r.Context(), int(it.TMDBID), it.MediaType)
		status = store.StatusRequested

	default:
		writeError(w, http.StatusBadRequest, codeBadRequest, "target must be radarr, sonarr or overseerr")
		return
	}

	// A title the service already tracks is the outcome the user wanted.
	if err != nil && !errors.Is(err, arr.ErrAlreadyAdded) {
		writeError(w, http.StatusBadGateway, codeUpstreamError, "%s rejected the request: %v", req.Target, err)
		return
	}
	if err := s.store.SetStatus(r.Context(), it.ID, status, time.Time{}); err != nil {
		s.writeStoreError(w, err, "item")
		return
	}
	s.engine.Trigger()

	s.respondWithItem(w, r, it.ID)
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

func (s *Server) clients() clients.Set {
	return clients.Build(s.settings.Get(), s.store.HTTPCache())
}
