package config

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/sirrobot01/snagarr/internal/store"
)

// The documents below are what a service record holds in its config blob, one
// shape per kind. They are the contract between the settings UI, the database
// and the client constructors, so they live in one place.

// LibraryConfig configures a Plex, Emby or Jellyfin server. Which of the three
// it is comes from the service kind, not from the document.
type LibraryConfig struct {
	URL            string   `json:"url"`
	Token          string   `json:"token"`
	SectionIDs     []string `json:"section_ids"`
	CollectionName string   `json:"collection_name"`
}

// ArrConfig configures a Radarr or a Sonarr, which share every field except
// SeasonFolder.
type ArrConfig struct {
	URL              string `json:"url"`
	APIKey           string `json:"api_key"`
	QualityProfileID int    `json:"quality_profile_id"`
	RootFolder       string `json:"root_folder"`
	SeasonFolder     bool   `json:"season_folder"`
	SearchOnAdd      bool   `json:"search_on_add"`
}

// OverseerrConfig configures one Overseerr.
type OverseerrConfig struct {
	URL    string `json:"url"`
	APIKey string `json:"api_key"`
}

// NtfyConfig configures one ntfy topic.
type NtfyConfig struct {
	URL      string `json:"url"`
	Topic    string `json:"topic"`
	Token    string `json:"token"`
	Priority int    `json:"priority"`
}

// Configured reports whether the media server can be reached.
func (c LibraryConfig) Configured() bool { return c.URL != "" && c.Token != "" }

// Configured reports whether the *arr can be reached.
func (c ArrConfig) Configured() bool { return c.URL != "" && c.APIKey != "" }

// Configured reports whether Overseerr can be reached.
func (c OverseerrConfig) Configured() bool { return c.URL != "" && c.APIKey != "" }

// Configured reports whether there is a topic to publish to. An ntfy server
// needs no credential, so the topic is the whole requirement.
func (c NtfyConfig) Configured() bool { return c.Topic != "" }

// ServiceSecrets names the config fields that hold a credential. The API masks
// them on the way out and restores them when a client echoes a mask back. One
// list covers every kind: no config uses either name for anything else.
var ServiceSecrets = []string{"api_key", "token"}

// DefaultServiceConfig is what a new service of this kind starts from, so no
// client has to know that the collection is called Snagged or that ntfy.sh is
// the default host.
func DefaultServiceConfig(kind store.ServiceKind) []byte {
	var cfg any
	switch {
	case kind.Library():
		cfg = LibraryConfig{CollectionName: "Snagged"}
	case kind == store.KindRadarr:
		cfg = ArrConfig{SearchOnAdd: true}
	case kind == store.KindSonarr:
		cfg = ArrConfig{SearchOnAdd: true, SeasonFolder: true}
	case kind == store.KindNtfy:
		cfg = NtfyConfig{URL: "https://ntfy.sh", Priority: 3}
	default:
		cfg = OverseerrConfig{}
	}
	// The values are static, so encoding them cannot fail.
	raw, _ := json.Marshal(cfg)
	return raw
}

// seededName is the name every environment-configured service gets, matching
// the name the API gives a service created without one.
const seededName = "Default"

// SeedServices gives the first admin the services the environment describes, so
// a Docker-first operator never has to open the UI. Environment values win over
// what is stored, exactly as they did when integrations lived in the settings
// blob, and the services they own are reported by LockedServices so the UI can
// render them read-only.
func (m *Manager) SeedServices(ctx context.Context, s *store.Store) error {
	users, err := s.Users(ctx)
	if err != nil {
		return err
	}
	admin := -1
	for i, u := range users {
		if u.Role == store.RoleAdmin {
			admin = i
			break
		}
	}
	// Nobody to own them yet. The first run creates the admin, then seeds.
	if admin < 0 {
		return nil
	}

	owned, err := s.UserServices(ctx, users[admin].ID)
	if err != nil {
		return err
	}
	locked := map[int64]bool{}
	for _, kind := range envKinds() {
		current := findService(owned, kind, seededName)
		base := DefaultServiceConfig(kind)
		if current != nil {
			base = current.Config
		}
		config, set, err := seedConfig(kind, base)
		if err != nil {
			return fmt.Errorf("seed %s service: %w", kind, err)
		}
		if !set {
			continue
		}

		if current == nil {
			svc := &store.Service{
				UserID: users[admin].ID, Kind: kind, Name: seededName,
				Config: config, Enabled: true,
			}
			if err := s.CreateService(ctx, svc); err != nil {
				return err
			}
			locked[svc.ID] = true
			continue
		}
		current.Config = config
		if err := s.UpdateService(ctx, current); err != nil {
			return err
		}
		locked[current.ID] = true
	}

	m.mu.Lock()
	m.lockedServices = locked
	m.mu.Unlock()
	return nil
}

func findService(services []store.Service, kind store.ServiceKind, name string) *store.Service {
	for i := range services {
		if services[i].Kind == kind && services[i].Name == name {
			return &services[i]
		}
	}
	return nil
}

// envKinds lists the kinds the environment can describe. The media server names
// itself in SNAGARR_LIBRARY_PROVIDER; the rest are one kind each.
func envKinds() []store.ServiceKind {
	kinds := []store.ServiceKind{store.KindRadarr, store.KindSonarr, store.KindOverseerr, store.KindNtfy}
	if provider := store.ServiceKind(os.Getenv("SNAGARR_LIBRARY_PROVIDER")); provider.Library() {
		kinds = append(kinds, provider)
	}
	return kinds
}

// seedConfig merges the environment over a kind's current config. It reports
// false when the environment says nothing about this kind, which leaves any
// service the operator made in the UI alone.
func seedConfig(kind store.ServiceKind, base []byte) ([]byte, bool, error) {
	var cfg any
	var set bool

	switch {
	case kind.Library():
		var c LibraryConfig
		if err := json.Unmarshal(base, &c); err != nil {
			return nil, false, err
		}
		set = overlay(map[string]*string{
			"SNAGARR_LIBRARY_URL":        &c.URL,
			"SNAGARR_LIBRARY_TOKEN":      &c.Token,
			"SNAGARR_LIBRARY_COLLECTION": &c.CollectionName,
		}, nil)
		cfg = c

	case kind == store.KindRadarr, kind == store.KindSonarr:
		var c ArrConfig
		if err := json.Unmarshal(base, &c); err != nil {
			return nil, false, err
		}
		prefix := "SNAGARR_RADARR_"
		if kind == store.KindSonarr {
			prefix = "SNAGARR_SONARR_"
		}
		set = overlay(map[string]*string{
			prefix + "URL":         &c.URL,
			prefix + "API_KEY":     &c.APIKey,
			prefix + "ROOT_FOLDER": &c.RootFolder,
		}, map[string]*int{
			prefix + "QUALITY_PROFILE_ID": &c.QualityProfileID,
		})
		cfg = c

	case kind == store.KindOverseerr:
		var c OverseerrConfig
		if err := json.Unmarshal(base, &c); err != nil {
			return nil, false, err
		}
		set = overlay(map[string]*string{
			"SNAGARR_OVERSEERR_URL":     &c.URL,
			"SNAGARR_OVERSEERR_API_KEY": &c.APIKey,
		}, nil)
		cfg = c

	case kind == store.KindNtfy:
		var c NtfyConfig
		if err := json.Unmarshal(base, &c); err != nil {
			return nil, false, err
		}
		set = overlay(map[string]*string{
			"SNAGARR_NTFY_URL":   &c.URL,
			"SNAGARR_NTFY_TOPIC": &c.Topic,
			"SNAGARR_NTFY_TOKEN": &c.Token,
		}, nil)
		cfg = c

	default:
		return nil, false, fmt.Errorf("unknown kind %q", kind)
	}

	if !set {
		return nil, false, nil
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		return nil, false, err
	}
	return raw, true, nil
}

// overlay copies every environment value that is set over its field, and
// reports whether any of them was.
func overlay(strs map[string]*string, ints map[string]*int) bool {
	set := false
	for name, field := range strs {
		if v := os.Getenv(name); v != "" {
			*field = v
			set = true
		}
	}
	for name, field := range ints {
		if v, err := strconv.Atoi(os.Getenv(name)); err == nil {
			*field = v
			set = true
		}
	}
	return set
}
