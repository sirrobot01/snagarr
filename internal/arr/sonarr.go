package arr

import (
	"context"
	"fmt"
	"strconv"

	"github.com/sirrobot01/snagarr/internal/httpx"
)

// Sonarr is a Sonarr v3 client.
type Sonarr struct {
	rest httpx.Client
}

// NewSonarr returns a client for a Sonarr instance. baseURL may include a
// reverse-proxy subpath.
func NewSonarr(baseURL, apiKey string) *Sonarr {
	return &Sonarr{rest: newREST(baseURL, apiKey)}
}

// List returns every series Sonarr tracks.
func (s *Sonarr) List(ctx context.Context) ([]Item, error) {
	var body []sonarrSeries
	if err := s.rest.Get(ctx, "/api/v3/series", nil, &body); err != nil {
		return nil, fmt.Errorf("sonarr list: %w", err)
	}
	out := make([]Item, 0, len(body))
	for _, series := range body {
		out = append(out, series.item())
	}
	return out, nil
}

// Add pushes a series to Sonarr. Sonarr keys on TVDB, so ids should carry a TVDB
// id where possible. It returns ErrAlreadyAdded when Sonarr already tracks the
// series.
func (s *Sonarr) Add(ctx context.Context, ids ExternalIDs, opts AddOptions) (*Item, error) {
	payload, err := s.lookupSeries(ctx, ids)
	if err != nil {
		return nil, err
	}
	payload["qualityProfileId"] = opts.QualityProfileID
	payload["rootFolderPath"] = opts.RootFolder
	payload["monitored"] = opts.Monitor
	payload["seasonFolder"] = opts.SeasonFolder
	// A monitored series still lands with every season unmonitored unless
	// addOptions.monitor says otherwise.
	monitor := "none"
	if opts.Monitor {
		monitor = "all"
	}
	payload["addOptions"] = map[string]any{
		"monitor":                  monitor,
		"searchForMissingEpisodes": opts.SearchOnAdd,
	}

	var added sonarrSeries
	if err := s.rest.Post(ctx, "/api/v3/series", payload, &added); err != nil {
		if alreadyAdded(err) {
			return nil, ErrAlreadyAdded
		}
		return nil, fmt.Errorf("sonarr add %q: %w", ids.Title, err)
	}
	item := added.item()
	return &item, nil
}

// QualityProfiles lists the profiles configured in Sonarr.
func (s *Sonarr) QualityProfiles(ctx context.Context) ([]Profile, error) {
	profiles, err := qualityProfiles(ctx, s.rest)
	if err != nil {
		return nil, fmt.Errorf("sonarr quality profiles: %w", err)
	}
	return profiles, nil
}

// RootFolders lists the library paths configured in Sonarr.
func (s *Sonarr) RootFolders(ctx context.Context) ([]RootFolder, error) {
	folders, err := rootFolders(ctx, s.rest)
	if err != nil {
		return nil, fmt.Errorf("sonarr root folders: %w", err)
	}
	return folders, nil
}

// Ping reports connectivity and how many series Sonarr tracks.
func (s *Sonarr) Ping(ctx context.Context) (string, error) {
	items, err := s.List(ctx)
	if err != nil {
		return "", fmt.Errorf("sonarr ping: %w", err)
	}
	return "OK · " + strconv.Itoa(len(items)) + " series", nil
}

// lookupSeries tries TVDB, then IMDB, then the plain title, and prefers a result
// whose year matches.
func (s *Sonarr) lookupSeries(ctx context.Context, ids ExternalIDs) (map[string]any, error) {
	var terms []string
	if ids.TVDBID > 0 {
		terms = append(terms, "tvdb:"+strconv.Itoa(ids.TVDBID))
	}
	if ids.IMDBID != "" {
		terms = append(terms, "imdb:"+ids.IMDBID)
	}
	if ids.Title != "" {
		terms = append(terms, ids.Title)
	}
	if len(terms) == 0 {
		return nil, fmt.Errorf("sonarr lookup: no tvdb id, imdb id or title given")
	}

	var lastErr error
	for _, term := range terms {
		found, err := lookup(ctx, s.rest, "/api/v3/series/lookup", term)
		if err != nil {
			lastErr = fmt.Errorf("sonarr lookup %q: %w", term, err)
			continue
		}
		if len(found) == 0 {
			continue
		}
		if ids.Year > 0 {
			for _, candidate := range found {
				if seriesYear(candidate) == ids.Year {
					return candidate, nil
				}
			}
		}
		return found[0], nil
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("sonarr lookup %q: no result", terms[len(terms)-1])
}

func seriesYear(payload map[string]any) int {
	// JSON numbers arrive as float64 through map[string]any.
	f, ok := payload["year"].(float64)
	if !ok {
		return 0
	}
	return int(f)
}

type sonarrSeries struct {
	ID               int    `json:"id"`
	TvdbID           int    `json:"tvdbId"`
	TmdbID           int    `json:"tmdbId"`
	ImdbID           string `json:"imdbId"`
	Title            string `json:"title"`
	Year             int    `json:"year"`
	Monitored        bool   `json:"monitored"`
	QualityProfileID int    `json:"qualityProfileId"`
	Statistics       struct {
		EpisodeFileCount int `json:"episodeFileCount"`
	} `json:"statistics"`
}

func (s sonarrSeries) item() Item {
	return Item{
		ID:               s.ID,
		TMDBID:           s.TmdbID,
		TVDBID:           s.TvdbID,
		IMDBID:           s.ImdbID,
		Title:            s.Title,
		Year:             s.Year,
		Monitored:        s.Monitored,
		HasFile:          s.Statistics.EpisodeFileCount > 0,
		QualityProfileID: s.QualityProfileID,
	}
}
