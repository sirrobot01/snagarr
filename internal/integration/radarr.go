package integration

import (
	"context"
	"fmt"
	"strconv"
)

// RadarrClient is a Radarr v3 client.
type RadarrClient struct {
	rest client
}

// NewRadarr returns a client for a Radarr instance. baseURL may include a
// reverse-proxy subpath.
func NewRadarr(baseURL, apiKey string) *RadarrClient {
	return &RadarrClient{rest: newREST(baseURL, apiKey)}
}

// List returns every movie Radarr tracks.
func (r *RadarrClient) List(ctx context.Context) ([]ArrItem, error) {
	var body []radarrMovie
	if err := r.rest.Get(ctx, "/api/v3/movie", nil, &body); err != nil {
		return nil, fmt.Errorf("radarr list: %w", err)
	}
	out := make([]ArrItem, 0, len(body))
	for _, m := range body {
		out = append(out, m.item())
	}
	return out, nil
}

// Add pushes a TMDB id to Radarr. It returns ErrAlreadyAdded when Radarr already
// tracks the movie.
func (r *RadarrClient) Add(ctx context.Context, tmdbID int, opts AddOptions) (*ArrItem, error) {
	term := "tmdb:" + strconv.Itoa(tmdbID)
	found, err := lookup(ctx, r.rest, "/api/v3/movie/lookup", term)
	if err != nil {
		return nil, fmt.Errorf("radarr lookup %s: %w", term, err)
	}
	if len(found) == 0 {
		return nil, fmt.Errorf("radarr lookup %s: no result", term)
	}

	payload := found[0]
	payload["qualityProfileId"] = opts.QualityProfileID
	payload["rootFolderPath"] = opts.RootFolder
	payload["monitored"] = opts.Monitor
	payload["addOptions"] = map[string]any{"searchForMovie": opts.SearchOnAdd}

	var added radarrMovie
	if err := r.rest.Post(ctx, "/api/v3/movie", payload, &added); err != nil {
		if alreadyAdded(err) {
			return nil, ErrAlreadyAdded
		}
		return nil, fmt.Errorf("radarr add %d: %w", tmdbID, err)
	}
	item := added.item()
	return &item, nil
}

// QualityProfiles lists the profiles configured in Radarr.
func (r *RadarrClient) QualityProfiles(ctx context.Context) ([]Profile, error) {
	profiles, err := qualityProfiles(ctx, r.rest)
	if err != nil {
		return nil, fmt.Errorf("radarr quality profiles: %w", err)
	}
	return profiles, nil
}

// RootFolders lists the library paths configured in Radarr.
func (r *RadarrClient) RootFolders(ctx context.Context) ([]RootFolder, error) {
	folders, err := rootFolders(ctx, r.rest)
	if err != nil {
		return nil, fmt.Errorf("radarr root folders: %w", err)
	}
	return folders, nil
}

// Ping reports connectivity and how many movies Radarr monitors.
func (r *RadarrClient) Ping(ctx context.Context) (string, error) {
	items, err := r.List(ctx)
	if err != nil {
		return "", fmt.Errorf("radarr ping: %w", err)
	}
	monitored := 0
	for _, m := range items {
		if m.Monitored {
			monitored++
		}
	}
	return "OK · " + strconv.Itoa(monitored) + " monitored", nil
}

type radarrMovie struct {
	ID               int    `json:"id"`
	TmdbID           int    `json:"tmdbId"`
	ImdbID           string `json:"imdbId"`
	Title            string `json:"title"`
	Year             int    `json:"year"`
	Monitored        bool   `json:"monitored"`
	HasFile          bool   `json:"hasFile"`
	QualityProfileID int    `json:"qualityProfileId"`
}

func (m radarrMovie) item() ArrItem {
	return ArrItem{
		ID:               m.ID,
		TMDBID:           m.TmdbID,
		IMDBID:           m.ImdbID,
		Title:            m.Title,
		Year:             m.Year,
		Monitored:        m.Monitored,
		HasFile:          m.HasFile,
		QualityProfileID: m.QualityProfileID,
	}
}
