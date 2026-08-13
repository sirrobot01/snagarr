package engine

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"time"

	"github.com/sirrobot01/snagarr/internal/integration"
	"github.com/sirrobot01/snagarr/internal/store"
)

// maxCandidates is what the Needs Review card shows: three posters fit the
// grid, and a fourth guess is rarely the right one.
const maxCandidates = 3

// ErrNoTMDB means the operator has not configured a TMDB key yet. The capture
// is already saved by the time this can happen, so nothing is lost.
var ErrNoTMDB = errors.New("tmdb is not configured")

type Resolver struct {
	store *store.Store
	http  *http.Client
	log   *slog.Logger
}

func NewResolver(s *store.Store, log *slog.Logger) *Resolver {
	return &Resolver{
		store: s,
		http:  &http.Client{Timeout: 15 * time.Second},
		log:   log,
	}
}

// Resolve identifies a captured item and writes the outcome. It never returns
// an item to an invalid state: either the item resolves, or it lands in
// needs_review with whatever candidates were found.
func (r *Resolver) Resolve(ctx context.Context, client *integration.TMDBClient, itemID int64) error {
	it, err := r.store.Item(ctx, itemID)
	if err != nil {
		return err
	}
	if client == nil {
		return ErrNoTMDB
	}

	query, ref := it.RawInput, Ref{}
	if IsURL(it.RawInput) {
		ref = r.dereference(ctx, it.RawInput)
		if ref.Title != "" {
			query = ref.Title
		}
	}

	// A direct ID skips search and scoring entirely.
	if ref.TMDBID != 0 {
		return r.resolveByID(ctx, client, it, ref.TMDBID, ref.Type)
	}
	if ref.IMDBID != "" {
		found, err := client.FindByIMDB(ctx, ref.IMDBID)
		if err == nil {
			return r.resolveByID(ctx, client, it, int64(found.TMDBID), found.Type)
		}
		r.log.Debug("imdb lookup failed, falling back to search", "item", it.ID, "error", err)
	}

	results, err := client.SearchMulti(ctx, query)
	if err != nil {
		return fmt.Errorf("search %q: %w", query, err)
	}

	candidates := rank(query, results)
	if len(candidates) == 0 {
		return r.park(ctx, it, nil)
	}

	scores := make([]float64, len(candidates))
	for i, c := range candidates {
		scores[i] = c.Score
	}
	best, confident := Decide(scores)
	if !confident {
		return r.park(ctx, it, candidates)
	}
	return r.commit(ctx, client, it, candidates[best])
}

// dereference turns a captured link into whatever it identifies. A failed
// scrape is not an error: the raw input is still searchable text.
func (r *Resolver) dereference(ctx context.Context, raw string) Ref {
	if ref, ok := ParseURL(raw); ok {
		return ref
	}
	ref, err := Scrape(ctx, r.http, raw)
	if err != nil {
		r.log.Debug("scrape failed", "url", raw, "error", err)
		return Ref{}
	}
	return ref
}

func (r *Resolver) resolveByID(ctx context.Context, client *integration.TMDBClient, it *store.Item, tmdbID int64, t store.MediaType) error {
	if !t.Valid() {
		t = store.Movie
	}
	details, err := client.Details(ctx, t, int(tmdbID))
	if err != nil {
		return fmt.Errorf("tmdb details %s/%d: %w", t, tmdbID, err)
	}
	return r.commit(ctx, client, it, store.Candidate{
		TMDBID: int64(details.TMDBID), MediaType: details.Type, Title: details.Title,
		Year: details.Year, PosterPath: details.PosterPath, Overview: details.Overview, Score: 1,
	})
}

// commit resolves the item, but yields to an existing item for the same title
// so the list never grows a duplicate.
func (r *Resolver) commit(ctx context.Context, client *integration.TMDBClient, it *store.Item, c store.Candidate) error {
	existing, err := r.store.ItemByTMDB(ctx, c.TMDBID, c.MediaType)
	if err == nil && existing.ID != it.ID {
		r.log.Info("capture matched an existing item", "item", it.ID, "existing", existing.ID, "title", c.Title)
		return r.store.DeleteItem(ctx, it.ID)
	}
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return err
	}

	if err := r.store.Resolve(ctx, it.ID, c); err != nil {
		return err
	}
	if err := r.store.SetCandidates(ctx, it.ID, nil); err != nil {
		return err
	}
	r.cacheEntity(ctx, client, c.TMDBID, c.MediaType)
	return nil
}

// park keeps an unresolved capture with its best guesses, which is what makes
// "no capture is ever lost" true.
func (r *Resolver) park(ctx context.Context, it *store.Item, candidates []store.Candidate) error {
	if err := r.store.SetCandidates(ctx, it.ID, candidates); err != nil {
		return err
	}
	if it.Status != store.StatusNeedsReview {
		return r.store.SetStatus(ctx, it.ID, store.StatusNeedsReview, time.Time{})
	}
	return nil
}

// cacheEntity fills the entity cache so the item renders with a poster and an
// overview even when TMDB is unreachable later.
func (r *Resolver) cacheEntity(ctx context.Context, client *integration.TMDBClient, tmdbID int64, t store.MediaType) {
	details, err := client.Details(ctx, t, int(tmdbID))
	if err != nil {
		r.log.Debug("could not cache entity", "tmdb_id", tmdbID, "error", err)
		return
	}
	if err := r.store.PutEntity(ctx, store.Entity{
		TMDBID: int64(details.TMDBID), MediaType: details.Type, Title: details.Title,
		Year: details.Year, PosterPath: details.PosterPath, BackdropPath: details.BackdropPath,
		Overview: details.Overview, Genres: details.Genres, Runtime: details.Runtime,
		Popularity: details.Popularity,
	}); err != nil {
		r.log.Warn("could not cache entity", "tmdb_id", tmdbID, "error", err)
	}
}

// rank scores every search result against the query and returns the strongest
// few, best first.
func rank(query string, results []integration.Result) []store.Candidate {
	candidates := make([]store.Candidate, 0, len(results))
	for _, res := range results {
		candidates = append(candidates, store.Candidate{
			TMDBID: int64(res.TMDBID), MediaType: res.Type, Title: res.Title, Year: res.Year,
			PosterPath: res.PosterPath, Overview: res.Overview,
			Score: Score(query, res.Title, res.Year, res.Popularity),
		})
	}
	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].Score > candidates[j].Score })
	if len(candidates) > maxCandidates {
		candidates = candidates[:maxCandidates]
	}
	return candidates
}
