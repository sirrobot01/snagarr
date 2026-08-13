package config

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sirrobot01/snagarr/internal/store"
)

// Duration is a time.Duration that reads and writes as a string, so settings
// stay legible in JSON and in the UI.
type Duration time.Duration

func (d Duration) MarshalJSON() ([]byte, error) { return json.Marshal(time.Duration(d).String()) }

func (d *Duration) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", s, err)
	}
	*d = Duration(parsed)
	return nil
}

type TMDBSettings struct {
	APIKey string `json:"api_key"`
}

type LibrarySettings struct {
	Provider       string   `json:"provider"` // plex | emby | jellyfin
	URL            string   `json:"url"`
	Token          string   `json:"token"`
	SectionIDs     []string `json:"section_ids"`
	CollectionName string   `json:"collection_name"`
}

type ArrSettings struct {
	URL              string `json:"url"`
	APIKey           string `json:"api_key"`
	QualityProfileID int    `json:"quality_profile_id"`
	RootFolder       string `json:"root_folder"`
	SeasonFolder     bool   `json:"season_folder"`
	SearchOnAdd      bool   `json:"search_on_add"`
}

type OverseerrSettings struct {
	URL    string `json:"url"`
	APIKey string `json:"api_key"`
	Prefer bool   `json:"prefer"`
}

type NtfySettings struct {
	URL      string `json:"url"`
	Topic    string `json:"topic"`
	Token    string `json:"token"`
	Priority int    `json:"priority"`
}

type TelegramSettings struct {
	BotToken string `json:"bot_token"`
}

type GeneralSettings struct {
	ReconcileInterval Duration `json:"reconcile_interval"`
	StaleDays         int      `json:"stale_days"`
	PublicURL         string   `json:"public_url"`
	ImageBase         string   `json:"image_base"`
	WebhookSecret     string   `json:"webhook_secret"`
}

type Settings struct {
	TMDB      TMDBSettings      `json:"tmdb"`
	Library   LibrarySettings   `json:"library"`
	Radarr    ArrSettings       `json:"radarr"`
	Sonarr    ArrSettings       `json:"sonarr"`
	Overseerr OverseerrSettings `json:"overseerr"`
	Ntfy      NtfySettings      `json:"ntfy"`
	Telegram  TelegramSettings  `json:"telegram"`
	General   GeneralSettings   `json:"general"`
}

func (s TMDBSettings) Configured() bool { return s.APIKey != "" }
func (s LibrarySettings) Configured() bool {
	return s.Provider != "" && s.URL != "" && s.Token != ""
}
func (s ArrSettings) Configured() bool       { return s.URL != "" && s.APIKey != "" }
func (s OverseerrSettings) Configured() bool { return s.URL != "" && s.APIKey != "" }
func (s NtfySettings) Configured() bool      { return s.Topic != "" }
func (s TelegramSettings) Configured() bool  { return s.BotToken != "" }

func defaults() Settings {
	return Settings{
		Library: LibrarySettings{CollectionName: "Snagged"},
		Radarr:  ArrSettings{SearchOnAdd: true},
		Sonarr:  ArrSettings{SearchOnAdd: true, SeasonFolder: true},
		Ntfy:    NtfySettings{URL: "https://ntfy.sh", Priority: 3},
		General: GeneralSettings{
			ReconcileInterval: Duration(15 * time.Minute),
			StaleDays:         90,
			ImageBase:         "https://image.tmdb.org/t/p",
		},
	}
}

// secretFields maps the dotted path of every secret to a pointer into s, so
// masking, restoring and env overlays all work from one list.
func (s *Settings) secretFields() map[string]*string {
	return map[string]*string{
		"tmdb.api_key":           &s.TMDB.APIKey,
		"library.token":          &s.Library.Token,
		"radarr.api_key":         &s.Radarr.APIKey,
		"sonarr.api_key":         &s.Sonarr.APIKey,
		"overseerr.api_key":      &s.Overseerr.APIKey,
		"ntfy.token":             &s.Ntfy.Token,
		"telegram.bot_token":     &s.Telegram.BotToken,
		"general.webhook_secret": &s.General.WebhookSecret,
	}
}

const maskRunes = "••••"

// Masked returns a copy safe to send to a client. Secrets keep their last four
// characters so an operator can tell which key is stored.
func (s Settings) Masked() Settings {
	masked := s
	for _, field := range masked.secretFields() {
		if *field == "" {
			continue
		}
		if len(*field) > 4 {
			*field = maskRunes + (*field)[len(*field)-4:]
		} else {
			*field = maskRunes
		}
	}
	return masked
}

// IsMasked reports whether a client sent back a value it never actually saw.
func IsMasked(value string) bool { return strings.HasPrefix(value, maskRunes) }

// Manager owns the live settings: it loads them from the database, applies
// environment overrides on top, and persists updates from the settings UI.
type Manager struct {
	store *store.Store

	mu      sync.RWMutex
	current Settings
	locked  map[string]bool
}

const settingsKey = "settings"

func NewManager(ctx context.Context, s *store.Store) (*Manager, error) {
	m := &Manager{store: s, current: defaults()}

	raw, err := s.Setting(ctx, settingsKey)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}
	if err == nil {
		if err := json.Unmarshal(raw, &m.current); err != nil {
			return nil, fmt.Errorf("decode settings: %w", err)
		}
	}
	if m.current.General.WebhookSecret == "" {
		m.current.General.WebhookSecret = randomSecret()
		if err := m.persist(ctx); err != nil {
			return nil, err
		}
	}
	m.locked = overlayEnv(&m.current)
	return m, nil
}

func (m *Manager) Get() Settings {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.current
}

// Locked reports the settings pinned by environment variables. The UI renders
// them read-only rather than letting an operator edit a value that will be
// overwritten on the next restart.
func (m *Manager) Locked() map[string]bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.locked
}

// Apply merges a partial settings document over the current values. Unmarshal
// does the merge: any field the client omits keeps what it already had.
func (m *Manager) Apply(ctx context.Context, patch []byte) (Settings, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	updated := m.current
	before := m.current
	if err := json.Unmarshal(patch, &updated); err != nil {
		return Settings{}, fmt.Errorf("decode settings: %w", err)
	}

	// A client that echoes a masked secret back means "leave it alone".
	old := before.secretFields()
	for path, field := range updated.secretFields() {
		if IsMasked(*field) {
			*field = *old[path]
		}
	}

	m.current = updated
	if err := m.persist(ctx); err != nil {
		m.current = before
		return Settings{}, err
	}
	m.locked = overlayEnv(&m.current)
	return m.current, nil
}

func (m *Manager) persist(ctx context.Context) error {
	raw, err := json.Marshal(m.current)
	if err != nil {
		return fmt.Errorf("encode settings: %w", err)
	}
	return m.store.SetSetting(ctx, settingsKey, raw)
}

// overlayEnv lets a Docker-first operator configure everything without opening
// the UI. Environment values win over anything stored, and are reported back so
// the UI can show them as locked.
func overlayEnv(s *Settings) map[string]bool {
	locked := map[string]bool{}

	strs := map[string]*string{
		"SNAGARR_TMDB_API_KEY":       &s.TMDB.APIKey,
		"SNAGARR_LIBRARY_PROVIDER":   &s.Library.Provider,
		"SNAGARR_LIBRARY_URL":        &s.Library.URL,
		"SNAGARR_LIBRARY_TOKEN":      &s.Library.Token,
		"SNAGARR_LIBRARY_COLLECTION": &s.Library.CollectionName,
		"SNAGARR_RADARR_URL":         &s.Radarr.URL,
		"SNAGARR_RADARR_API_KEY":     &s.Radarr.APIKey,
		"SNAGARR_RADARR_ROOT_FOLDER": &s.Radarr.RootFolder,
		"SNAGARR_SONARR_URL":         &s.Sonarr.URL,
		"SNAGARR_SONARR_API_KEY":     &s.Sonarr.APIKey,
		"SNAGARR_SONARR_ROOT_FOLDER": &s.Sonarr.RootFolder,
		"SNAGARR_OVERSEERR_URL":      &s.Overseerr.URL,
		"SNAGARR_OVERSEERR_API_KEY":  &s.Overseerr.APIKey,
		"SNAGARR_NTFY_URL":           &s.Ntfy.URL,
		"SNAGARR_NTFY_TOPIC":         &s.Ntfy.Topic,
		"SNAGARR_NTFY_TOKEN":         &s.Ntfy.Token,
		"SNAGARR_TELEGRAM_BOT_TOKEN": &s.Telegram.BotToken,
		"SNAGARR_PUBLIC_URL":         &s.General.PublicURL,
	}
	paths := map[string]string{
		"SNAGARR_TMDB_API_KEY":       "tmdb.api_key",
		"SNAGARR_LIBRARY_PROVIDER":   "library.provider",
		"SNAGARR_LIBRARY_URL":        "library.url",
		"SNAGARR_LIBRARY_TOKEN":      "library.token",
		"SNAGARR_LIBRARY_COLLECTION": "library.collection_name",
		"SNAGARR_RADARR_URL":         "radarr.url",
		"SNAGARR_RADARR_API_KEY":     "radarr.api_key",
		"SNAGARR_RADARR_ROOT_FOLDER": "radarr.root_folder",
		"SNAGARR_SONARR_URL":         "sonarr.url",
		"SNAGARR_SONARR_API_KEY":     "sonarr.api_key",
		"SNAGARR_SONARR_ROOT_FOLDER": "sonarr.root_folder",
		"SNAGARR_OVERSEERR_URL":      "overseerr.url",
		"SNAGARR_OVERSEERR_API_KEY":  "overseerr.api_key",
		"SNAGARR_NTFY_URL":           "ntfy.url",
		"SNAGARR_NTFY_TOPIC":         "ntfy.topic",
		"SNAGARR_NTFY_TOKEN":         "ntfy.token",
		"SNAGARR_TELEGRAM_BOT_TOKEN": "telegram.bot_token",
		"SNAGARR_PUBLIC_URL":         "general.public_url",
	}
	for name, field := range strs {
		if v := os.Getenv(name); v != "" {
			*field = v
			locked[paths[name]] = true
		}
	}

	ints := map[string]*int{
		"SNAGARR_RADARR_QUALITY_PROFILE_ID": &s.Radarr.QualityProfileID,
		"SNAGARR_SONARR_QUALITY_PROFILE_ID": &s.Sonarr.QualityProfileID,
	}
	intPaths := map[string]string{
		"SNAGARR_RADARR_QUALITY_PROFILE_ID": "radarr.quality_profile_id",
		"SNAGARR_SONARR_QUALITY_PROFILE_ID": "sonarr.quality_profile_id",
	}
	for name, field := range ints {
		if v, err := strconv.Atoi(os.Getenv(name)); err == nil {
			*field = v
			locked[intPaths[name]] = true
		}
	}

	if v := os.Getenv("SNAGARR_RECONCILE_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			s.General.ReconcileInterval = Duration(d)
			locked["general.reconcile_interval"] = true
		}
	}
	return locked
}

func randomSecret() string {
	buf := make([]byte, 16)
	rand.Read(buf)
	return hex.EncodeToString(buf)
}
