package integration

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
)

// Radarr and Sonarr share the v3 API shape but not their payloads, so the
// helpers below are common and the two clients live in their own files.

// ErrAlreadyAdded reports that the service already tracks the title. Callers
// normally treat it as success.
var ErrAlreadyAdded = errors.New("arr: already added")

// ArrItem is one Radarr movie or Sonarr series.
type ArrItem struct {
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

// ArrExternalIDs identifies a title well enough for Sonarr, which keys on TVDB
// rather than TMDB.
type ArrExternalIDs struct {
	TMDBID, TVDBID int
	IMDBID, Title  string
	Year           int
}

func newREST(baseURL, apiKey string) client {
	return client{BaseURL: baseURL, Header: http.Header{"X-Api-Key": {apiKey}}}
}

func qualityProfiles(ctx context.Context, rest client) ([]Profile, error) {
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

func rootFolders(ctx context.Context, rest client) ([]RootFolder, error) {
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

// arrStatus names the service and its version. It is the cheapest call that
// proves both the URL and the API key are right: counting the library instead
// means dumping it, which takes minutes on a large one.
func arrStatus(ctx context.Context, rest client) (string, error) {
	var body struct {
		AppName string `json:"appName"`
		Version string `json:"version"`
	}
	if err := rest.Get(ctx, "/api/v3/system/status", nil, &body); err != nil {
		return "", err
	}
	if body.AppName == "" {
		return "OK", nil
	}
	return strings.TrimSpace("OK · " + body.AppName + " " + body.Version), nil
}

// lookup returns the raw payloads Radarr and Sonarr require back when adding.
// Posting a hand-built body with only an id is rejected by both services.
func lookup(ctx context.Context, rest client, path, term string) ([]map[string]any, error) {
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
	if StatusOf(err) != http.StatusBadRequest {
		return false
	}
	var e *HTTPError
	if !errors.As(err, &e) {
		return false
	}
	body := strings.ToLower(e.Body)
	return strings.Contains(body, "already been added") || strings.Contains(body, "already exists")
}
