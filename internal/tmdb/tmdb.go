// Package tmdb is the TMDB v3 client used for search, resolution and metadata.
package tmdb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/sirrobot01/snagarr/internal/httpx"
	"github.com/sirrobot01/snagarr/internal/media"
)

const (
	baseURL    = "https://api.themoviedb.org/3"
	searchTTL  = 24 * time.Hour
	detailsTTL = 7 * 24 * time.Hour
)

// ErrNotFound is returned when TMDB has no match for a lookup.
var ErrNotFound = errors.New("tmdb: not found")

// Cache stores raw TMDB responses. It is satisfied by internal/store; a nil
// Cache disables caching.
type Cache interface {
	Get(ctx context.Context, key string) ([]byte, bool)
	Set(ctx context.Context, key string, body []byte, ttl time.Duration) error
}

// Client calls the TMDB v3 API.
type Client struct {
	rest   httpx.Client
	apiKey string
	cache  Cache
}

// Result is a search hit or the summary half of a details response.
type Result struct {
	TMDBID     int
	Type       media.Type
	Title      string
	Year       int // 0 when unknown
	PosterPath string
	Overview   string
	Popularity float64
}

// Details is the full metadata record cached per title.
type Details struct {
	Result
	BackdropPath string
	Runtime      int
	Genres       []string
	IMDBID       string
}

// ExternalIDs holds the non-TMDB identifiers used to match *arr and library items.
type ExternalIDs struct {
	IMDBID string
	TVDBID int
}

// New returns a TMDB client. cache may be nil.
func New(apiKey string, cache Cache) *Client {
	return &Client{rest: httpx.Client{BaseURL: baseURL}, apiKey: apiKey, cache: cache}
}

// SearchMulti searches movies and TV in one call. Person results are dropped.
func (c *Client) SearchMulti(ctx context.Context, query string) ([]Result, error) {
	var body struct {
		Results []entity `json:"results"`
	}
	if err := c.get(ctx, "/search/multi", url.Values{"query": {query}}, searchTTL, &body); err != nil {
		return nil, fmt.Errorf("tmdb search %q: %w", query, err)
	}
	out := make([]Result, 0, len(body.Results))
	for _, e := range body.Results {
		t, ok := typeOf(e.MediaType)
		if !ok {
			continue
		}
		out = append(out, e.result(t))
	}
	return out, nil
}

// Details fetches the full record for a movie or TV id.
func (c *Client) Details(ctx context.Context, t media.Type, id int) (*Details, error) {
	if !t.Valid() {
		return nil, fmt.Errorf("tmdb details: unknown media type %q", t)
	}
	var e entity
	// TV details carry no imdb_id, so the external ids are appended to the same
	// request instead of costing a second round trip.
	q := url.Values{"append_to_response": {"external_ids"}}
	if err := c.get(ctx, "/"+string(t)+"/"+strconv.Itoa(id), q, detailsTTL, &e); err != nil {
		return nil, fmt.Errorf("tmdb details %s/%d: %w", t, id, err)
	}
	d := Details{
		Result:       e.result(t),
		BackdropPath: e.BackdropPath,
		Runtime:      e.Runtime,
		IMDBID:       e.IMDBID,
	}
	if d.Runtime == 0 && len(e.EpisodeRunTime) > 0 {
		d.Runtime = e.EpisodeRunTime[0]
	}
	if d.IMDBID == "" {
		d.IMDBID = e.ExternalIDs.IMDBID
	}
	for _, g := range e.Genres {
		d.Genres = append(d.Genres, g.Name)
	}
	return &d, nil
}

// ExternalIDs returns the IMDB and TVDB ids for a movie or TV id.
func (c *Client) ExternalIDs(ctx context.Context, t media.Type, id int) (ExternalIDs, error) {
	if !t.Valid() {
		return ExternalIDs{}, fmt.Errorf("tmdb external ids: unknown media type %q", t)
	}
	var body struct {
		IMDBID string `json:"imdb_id"`
		TVDBID int    `json:"tvdb_id"`
	}
	path := "/" + string(t) + "/" + strconv.Itoa(id) + "/external_ids"
	if err := c.get(ctx, path, nil, detailsTTL, &body); err != nil {
		return ExternalIDs{}, fmt.Errorf("tmdb external ids %s/%d: %w", t, id, err)
	}
	return ExternalIDs{IMDBID: body.IMDBID, TVDBID: body.TVDBID}, nil
}

// FindByIMDB resolves an IMDB id to a movie or TV result. It returns ErrNotFound
// when TMDB knows no title with that id.
func (c *Client) FindByIMDB(ctx context.Context, imdbID string) (*Result, error) {
	var body struct {
		Movies []entity `json:"movie_results"`
		TV     []entity `json:"tv_results"`
	}
	q := url.Values{"external_source": {"imdb_id"}}
	if err := c.get(ctx, "/find/"+imdbID, q, detailsTTL, &body); err != nil {
		return nil, fmt.Errorf("tmdb find %s: %w", imdbID, err)
	}
	if len(body.Movies) > 0 {
		r := body.Movies[0].result(media.Movie)
		return &r, nil
	}
	if len(body.TV) > 0 {
		r := body.TV[0].result(media.TV)
		return &r, nil
	}
	return nil, fmt.Errorf("tmdb find %s: %w", imdbID, ErrNotFound)
}

// Ping checks the API key against /configuration.
func (c *Client) Ping(ctx context.Context) (string, error) {
	if err := c.get(ctx, "/configuration", nil, 0, nil); err != nil {
		return "", fmt.Errorf("tmdb ping: %w", err)
	}
	return "OK", nil
}

// get serves out of the cache when ttl is positive and a fresh entry exists,
// otherwise it calls TMDB and stores the raw response. Cache failures are
// ignored: they must never fail the call.
func (c *Client) get(ctx context.Context, path string, query url.Values, ttl time.Duration, out any) error {
	key := path
	if len(query) > 0 {
		key += "?" + query.Encode()
	}
	cacheable := c.cache != nil && ttl > 0 && out != nil
	if cacheable {
		if body, ok := c.cache.Get(ctx, key); ok {
			if err := json.Unmarshal(body, out); err == nil {
				return nil
			}
		}
	}

	// The api key stays out of the cache key so that keys hold no secret.
	q := url.Values{"api_key": {c.apiKey}}
	for k, v := range query {
		q[k] = v
	}
	var raw json.RawMessage
	if err := c.rest.Get(ctx, path, q, &raw); err != nil {
		return err
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	if cacheable {
		_ = c.cache.Set(ctx, key, raw, ttl)
	}
	return nil
}

// entity covers every TMDB shape we read: movies use title/release_date, TV uses
// name/first_air_date, and details add the rest.
type entity struct {
	ID             int     `json:"id"`
	MediaType      string  `json:"media_type"`
	Title          string  `json:"title"`
	Name           string  `json:"name"`
	ReleaseDate    string  `json:"release_date"`
	FirstAirDate   string  `json:"first_air_date"`
	PosterPath     string  `json:"poster_path"`
	BackdropPath   string  `json:"backdrop_path"`
	Overview       string  `json:"overview"`
	Popularity     float64 `json:"popularity"`
	Runtime        int     `json:"runtime"`
	EpisodeRunTime []int   `json:"episode_run_time"`
	IMDBID         string  `json:"imdb_id"`
	Genres         []struct {
		Name string `json:"name"`
	} `json:"genres"`
	ExternalIDs struct {
		IMDBID string `json:"imdb_id"`
		TVDBID int    `json:"tvdb_id"`
	} `json:"external_ids"`
}

func (e entity) result(t media.Type) Result {
	r := Result{
		TMDBID:     e.ID,
		Type:       t,
		Title:      e.Title,
		Year:       parseYear(e.ReleaseDate),
		PosterPath: e.PosterPath,
		Overview:   e.Overview,
		Popularity: e.Popularity,
	}
	if t == media.TV {
		r.Title = e.Name
		r.Year = parseYear(e.FirstAirDate)
	}
	return r
}

func typeOf(mediaType string) (media.Type, bool) {
	switch mediaType {
	case "movie":
		return media.Movie, true
	case "tv":
		return media.TV, true
	default:
		return "", false
	}
}

// parseYear reads the year from a TMDB date, which is "2025-04-18" or empty.
func parseYear(date string) int {
	if len(date) < 4 {
		return 0
	}
	year, err := strconv.Atoi(date[:4])
	if err != nil {
		return 0
	}
	return year
}
