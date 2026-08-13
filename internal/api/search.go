package api

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/sirrobot01/snagarr/internal/store"
)

const defaultSearchLimit = 20

// search merges the local library index with live TMDB results. Library matches
// rank first so the user discovers they already own something before snagging a
// duplicate, and every row carries its composite state for the badge.
func (s *Server) search(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		writeJSON(w, http.StatusOK, map[string]any{"results": []searchResult{}})
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 50 {
		limit = defaultSearchLimit
	}

	ctx := r.Context()
	index, err := s.store.LoadStateIndex(ctx)
	if err != nil {
		s.writeStoreError(w, err, "state index")
		return
	}
	snagged, err := s.snaggedByTitle(ctx)
	if err != nil {
		s.writeStoreError(w, err, "items")
		return
	}

	results := make([]searchResult, 0, limit)
	seen := map[store.TitleKey]bool{}

	entries, err := s.store.SearchLibrary(ctx, query, limit)
	if err != nil {
		s.log.Warn("library search failed", "error", err)
	}
	for _, e := range entries {
		key := store.TitleKey{TMDBID: e.TMDBID, MediaType: e.MediaType}
		if seen[key] {
			continue
		}
		seen[key] = true
		results = append(results, searchResult{
			TMDBID: e.TMDBID, MediaType: e.MediaType, Title: e.Title,
			Year: nullableCount(e.Year), Poster: s.cachedPoster(ctx, key),
			State: store.StatusAvailable, ItemID: nullableInt(snagged[key]), From: "library",
		})
	}

	// TMDB being unreachable degrades search rather than breaking it: the
	// library tier has already answered.
	if set := s.clients(); set.TMDB != nil {
		found, err := set.TMDB.SearchMulti(ctx, query)
		if err != nil {
			s.log.Warn("tmdb search failed", "query", query, "error", err)
		}
		for _, res := range found {
			key := store.TitleKey{TMDBID: int64(res.TMDBID), MediaType: res.Type}
			if seen[key] || len(results) >= limit {
				continue
			}
			seen[key] = true
			results = append(results, searchResult{
				TMDBID: key.TMDBID, MediaType: res.Type, Title: res.Title,
				Year: nullableCount(res.Year), Poster: nullableStr(res.PosterPath),
				Overview: nullableStr(res.Overview), State: index.State(key),
				ItemID: nullableInt(snagged[key]), From: "tmdb",
			})
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}

// snaggedByTitle lets a search row point at the item it already created, so the
// UI can show "snagged" instead of offering to snag it again.
func (s *Server) snaggedByTitle(ctx context.Context) (map[store.TitleKey]int64, error) {
	items, err := s.store.SnaggedItems(ctx)
	if err != nil {
		return nil, err
	}
	byTitle := make(map[store.TitleKey]int64, len(items))
	for _, it := range items {
		byTitle[store.TitleKey{TMDBID: it.TMDBID, MediaType: it.MediaType}] = it.ID
	}
	return byTitle, nil
}

// cachedPoster fills in artwork for library rows, which carry no poster of
// their own until TMDB metadata has been fetched once.
func (s *Server) cachedPoster(ctx context.Context, key store.TitleKey) *string {
	entity, err := s.store.Entity(ctx, key.TMDBID, key.MediaType)
	if err != nil {
		return nil
	}
	return nullableStr(entity.PosterPath)
}
