// Package arr talks to Radarr and Sonarr, which share the v3 API shape but not
// their payloads.
package arr

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/sirrobot01/snagarr/internal/httpx"
)

// ErrAlreadyAdded reports that the service already tracks the title. Callers
// normally treat it as success.
var ErrAlreadyAdded = errors.New("arr: already added")

// Item is one Radarr movie or Sonarr series.
type Item struct {
	ID               int // the Radarr movie id / Sonarr series id
	TMDBID           int
	TVDBID           int
	IMDBID           string
	Title            string
	Year             int
	Monitored        bool
	HasFile          bool // Sonarr: true when at least one episode file exists
	QualityProfileID int
}

// Profile is a quality profile the user can pick as a default.
type Profile struct {
	ID   int
	Name string
}

// RootFolder is a configured library path.
type RootFolder struct {
	Path      string
	FreeSpace int64
}

// AddOptions carries the defaults Snagarr applies when it pushes a title.
type AddOptions struct {
	QualityProfileID int
	RootFolder       string
	Monitor          bool
	SearchOnAdd      bool
	SeasonFolder     bool // Sonarr only
}

// ExternalIDs identifies a title well enough for Sonarr, which keys on TVDB
// rather than TMDB.
type ExternalIDs struct {
	TMDBID, TVDBID int
	IMDBID, Title  string
	Year           int
}

func newREST(baseURL, apiKey string) httpx.Client {
	return httpx.Client{BaseURL: baseURL, Header: http.Header{"X-Api-Key": {apiKey}}}
}

func qualityProfiles(ctx context.Context, rest httpx.Client) ([]Profile, error) {
	var body []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	if err := rest.Get(ctx, "/api/v3/qualityprofile", nil, &body); err != nil {
		return nil, err
	}
	out := make([]Profile, 0, len(body))
	for _, p := range body {
		out = append(out, Profile{ID: p.ID, Name: p.Name})
	}
	return out, nil
}

func rootFolders(ctx context.Context, rest httpx.Client) ([]RootFolder, error) {
	var body []struct {
		Path      string `json:"path"`
		FreeSpace int64  `json:"freeSpace"`
	}
	if err := rest.Get(ctx, "/api/v3/rootfolder", nil, &body); err != nil {
		return nil, err
	}
	out := make([]RootFolder, 0, len(body))
	for _, f := range body {
		out = append(out, RootFolder{Path: f.Path, FreeSpace: f.FreeSpace})
	}
	return out, nil
}

// lookup returns the raw payloads Radarr and Sonarr require back when adding.
// Posting a hand-built body with only an id is rejected by both services.
func lookup(ctx context.Context, rest httpx.Client, path, term string) ([]map[string]any, error) {
	var found []map[string]any
	if err := rest.Get(ctx, path, url.Values{"term": {term}}, &found); err != nil {
		return nil, err
	}
	return found, nil
}

// alreadyAdded recognises the 400 both services return for a duplicate. Radarr
// says "This movie has already been added", Sonarr "This series has already been
// added"; older builds phrase it as "already exists".
func alreadyAdded(err error) bool {
	if httpx.StatusOf(err) != http.StatusBadRequest {
		return false
	}
	var e *httpx.Error
	if !errors.As(err, &e) {
		return false
	}
	body := strings.ToLower(e.Body)
	return strings.Contains(body, "already been added") || strings.Contains(body, "already exists")
}
