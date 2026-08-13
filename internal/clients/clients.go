// Package clients builds the external service clients from the current
// settings. Settings change while Snagarr runs, so callers build a fresh set
// per operation rather than holding one for the process lifetime.
package clients

import (
	"context"
	"time"

	"github.com/sirrobot01/snagarr/internal/arr"
	"github.com/sirrobot01/snagarr/internal/config"
	"github.com/sirrobot01/snagarr/internal/library"
	"github.com/sirrobot01/snagarr/internal/notify"
	"github.com/sirrobot01/snagarr/internal/overseerr"
	"github.com/sirrobot01/snagarr/internal/tmdb"
)

// LibraryProvider is the media server seen from Snagarr's side. Plex, Emby and
// Jellyfin are interchangeable behind it, and a new provider only has to
// satisfy these four methods.
type LibraryProvider interface {
	Ping(ctx context.Context) (string, error)
	Sections(ctx context.Context) ([]library.Section, error)
	Items(ctx context.Context, sectionIDs []string, since time.Time) ([]library.Item, error)
	SyncCollection(ctx context.Context, name string, itemIDs []string) error
}

// Set holds one client per configured service. Unconfigured services are nil,
// so every caller must check before use.
type Set struct {
	TMDB      *tmdb.Client
	Library   LibraryProvider
	Radarr    *arr.Radarr
	Sonarr    *arr.Sonarr
	Overseerr *overseerr.Client
	Ntfy      *notify.Ntfy
}

func Build(s config.Settings, cache tmdb.Cache) Set {
	var set Set

	if s.TMDB.Configured() {
		set.TMDB = tmdb.New(s.TMDB.APIKey, cache)
	}
	if s.Library.Configured() {
		switch s.Library.Provider {
		case "plex":
			set.Library = library.NewPlex(s.Library.URL, s.Library.Token)
		case "emby":
			set.Library = library.NewEmby(s.Library.URL, s.Library.Token, false)
		case "jellyfin":
			set.Library = library.NewEmby(s.Library.URL, s.Library.Token, true)
		}
	}
	if s.Radarr.Configured() {
		set.Radarr = arr.NewRadarr(s.Radarr.URL, s.Radarr.APIKey)
	}
	if s.Sonarr.Configured() {
		set.Sonarr = arr.NewSonarr(s.Sonarr.URL, s.Sonarr.APIKey)
	}
	if s.Overseerr.Configured() {
		set.Overseerr = overseerr.New(s.Overseerr.URL, s.Overseerr.APIKey)
	}
	if s.Ntfy.Configured() {
		set.Ntfy = notify.NewNtfy(s.Ntfy.URL, s.Ntfy.Topic, s.Ntfy.Token)
	}
	return set
}
