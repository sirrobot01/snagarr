// Package integration is everything that talks to a service outside Snagarr —
// TMDB, Radarr, Sonarr, Plex, Emby, Jellyfin, Overseerr and ntfy — together with
// the code that builds those clients from the household's service records.
// Services change while Snagarr runs, so callers build a fresh set per operation
// rather than holding one for the process lifetime.
package integration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/sirrobot01/snagarr/internal/config"
	"github.com/sirrobot01/snagarr/internal/store"
)

// LibraryProvider is the media server seen from Snagarr's side. Plex, Emby and
// Jellyfin are interchangeable behind it, and a new provider only has to
// satisfy these four methods.
type LibraryProvider interface {
	Ping(ctx context.Context) (string, error)
	Sections(ctx context.Context) ([]Section, error)
	Items(ctx context.Context, sectionIDs []string, since time.Time) ([]LibraryItem, error)
	SyncCollection(ctx context.Context, name string, members []CollectionMember) error
}

// ErrNotConfigured reports a service whose config is missing what the client
// needs to reach it. It is a state, not a fault: half-filled cards are normal
// while an operator is still typing.
var ErrNotConfigured = errors.New("service is not configured")

// Library is one member's media server: the record, its decoded config and the
// client. The four types below follow the same shape, one per kind.
type Library struct {
	Service store.Service
	Config  config.LibraryConfig
	Client  LibraryProvider
}

// Radarr is one member's Radarr.
type Radarr struct {
	Service store.Service
	Config  config.ArrConfig
	Client  *RadarrClient
}

// Sonarr is one member's Sonarr.
type Sonarr struct {
	Service store.Service
	Config  config.ArrConfig
	Client  *SonarrClient
}

// Overseerr is one member's Overseerr.
type Overseerr struct {
	Service store.Service
	Config  config.OverseerrConfig
	Client  *OverseerrClient
}

// Ntfy is one member's ntfy topic.
type Ntfy struct {
	Service store.Service
	Config  config.NtfyConfig
	Client  *NtfyClient
}

// BuildLibrary builds the client for a plex, emby or jellyfin service.
func BuildLibrary(svc store.Service) (Library, error) {
	var cfg config.LibraryConfig
	if err := decodeConfig(svc, &cfg); err != nil {
		return Library{}, err
	}
	if !cfg.Configured() {
		return Library{}, ErrNotConfigured
	}
	var client LibraryProvider
	switch svc.Kind {
	case store.KindPlex:
		client = NewPlex(cfg.URL, cfg.Token)
	case store.KindEmby:
		client = NewEmby(cfg.URL, cfg.Token, false)
	case store.KindJellyfin:
		client = NewEmby(cfg.URL, cfg.Token, true)
	default:
		return Library{}, fmt.Errorf("%q is not a media server", svc.Kind)
	}
	return Library{Service: svc, Config: cfg, Client: client}, nil
}

// BuildRadarr builds the client for a radarr service.
func BuildRadarr(svc store.Service) (Radarr, error) {
	var cfg config.ArrConfig
	if err := decodeConfig(svc, &cfg); err != nil {
		return Radarr{}, err
	}
	if !cfg.Configured() {
		return Radarr{}, ErrNotConfigured
	}
	return Radarr{Service: svc, Config: cfg, Client: NewRadarr(cfg.URL, cfg.APIKey)}, nil
}

// BuildSonarr builds the client for a sonarr service.
func BuildSonarr(svc store.Service) (Sonarr, error) {
	var cfg config.ArrConfig
	if err := decodeConfig(svc, &cfg); err != nil {
		return Sonarr{}, err
	}
	if !cfg.Configured() {
		return Sonarr{}, ErrNotConfigured
	}
	return Sonarr{Service: svc, Config: cfg, Client: NewSonarr(cfg.URL, cfg.APIKey)}, nil
}

// BuildOverseerr builds the client for an overseerr service.
func BuildOverseerr(svc store.Service) (Overseerr, error) {
	var cfg config.OverseerrConfig
	if err := decodeConfig(svc, &cfg); err != nil {
		return Overseerr{}, err
	}
	if !cfg.Configured() {
		return Overseerr{}, ErrNotConfigured
	}
	return Overseerr{Service: svc, Config: cfg, Client: NewOverseerr(cfg.URL, cfg.APIKey)}, nil
}

// BuildNtfy builds the client for an ntfy service.
func BuildNtfy(svc store.Service) (Ntfy, error) {
	var cfg config.NtfyConfig
	if err := decodeConfig(svc, &cfg); err != nil {
		return Ntfy{}, err
	}
	if !cfg.Configured() {
		return Ntfy{}, ErrNotConfigured
	}
	return Ntfy{Service: svc, Config: cfg, Client: NewNtfy(cfg.URL, cfg.Topic, cfg.Token)}, nil
}

func decodeConfig(svc store.Service, out any) error {
	if err := json.Unmarshal(svc.Config, out); err != nil {
		return fmt.Errorf("decode %s config: %w", svc.Kind, err)
	}
	return nil
}

// Household is every enabled service in the install, built and grouped by kind.
// State is the union of what all of them report, so the reconcile loop walks
// the whole household on every pass.
type Household struct {
	Libraries  []Library
	Radarrs    []Radarr
	Sonarrs    []Sonarr
	Overseerrs []Overseerr
	Ntfys      []Ntfy
}

// BuildHousehold groups services into ready clients. A service that cannot be
// built is left out and reported, so one unreadable record never hides the rest
// of the household; a merely unconfigured one is left out silently.
func BuildHousehold(services []store.Service) (Household, error) {
	var h Household
	var errs []error

	for _, svc := range services {
		var err error
		switch {
		case svc.Kind.Library():
			var built Library
			if built, err = BuildLibrary(svc); err == nil {
				h.Libraries = append(h.Libraries, built)
			}
		case svc.Kind == store.KindRadarr:
			var built Radarr
			if built, err = BuildRadarr(svc); err == nil {
				h.Radarrs = append(h.Radarrs, built)
			}
		case svc.Kind == store.KindSonarr:
			var built Sonarr
			if built, err = BuildSonarr(svc); err == nil {
				h.Sonarrs = append(h.Sonarrs, built)
			}
		case svc.Kind == store.KindOverseerr:
			var built Overseerr
			if built, err = BuildOverseerr(svc); err == nil {
				h.Overseerrs = append(h.Overseerrs, built)
			}
		case svc.Kind == store.KindNtfy:
			var built Ntfy
			if built, err = BuildNtfy(svc); err == nil {
				h.Ntfys = append(h.Ntfys, built)
			}
		default:
			err = fmt.Errorf("unknown kind %q", svc.Kind)
		}
		if err != nil && !errors.Is(err, ErrNotConfigured) {
			errs = append(errs, fmt.Errorf("service %d: %w", svc.ID, err))
		}
	}
	return h, errors.Join(errs...)
}

// RadarrFor returns the first Radarr owned by one of owners, in the order
// given. Ordering the owners is how a personal action falls back: the caller's
// own service first, then the capturer's, then an admin's.
func (h Household) RadarrFor(owners ...int64) *Radarr {
	for _, owner := range owners {
		for i := range h.Radarrs {
			if h.Radarrs[i].Service.UserID == owner {
				return &h.Radarrs[i]
			}
		}
	}
	return nil
}

// SonarrFor returns the first Sonarr owned by one of owners, in the order given.
func (h Household) SonarrFor(owners ...int64) *Sonarr {
	for _, owner := range owners {
		for i := range h.Sonarrs {
			if h.Sonarrs[i].Service.UserID == owner {
				return &h.Sonarrs[i]
			}
		}
	}
	return nil
}

// OverseerrFor returns the first Overseerr owned by one of owners, in the order
// given.
func (h Household) OverseerrFor(owners ...int64) *Overseerr {
	for _, owner := range owners {
		for i := range h.Overseerrs {
			if h.Overseerrs[i].Service.UserID == owner {
				return &h.Overseerrs[i]
			}
		}
	}
	return nil
}

// NtfyFor returns the first ntfy owned by one of owners, in the order given.
func (h Household) NtfyFor(owners ...int64) *Ntfy {
	for _, owner := range owners {
		for i := range h.Ntfys {
			if h.Ntfys[i].Service.UserID == owner {
				return &h.Ntfys[i]
			}
		}
	}
	return nil
}

// TMDB builds the catalogue client. TMDB stays global — one key per install —
// so it comes from settings rather than from a service record, and is nil when
// no key is set.
func TMDB(s config.Settings, cache Cache) *TMDBClient {
	if !s.TMDB.Configured() {
		return nil
	}
	return NewTMDB(s.TMDB.Key(), cache)
}
