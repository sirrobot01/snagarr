package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
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

// DefaultTMDBKey is the shared catalogue key release builds carry, the way
// Overseerr embeds one: TMDB's API is free for non-commercial use and limits
// by IP rather than by key, so one key serves every install. It is empty in a
// plain source build and injected at release time with
//
//	-ldflags "-X github.com/sirrobot01/snagarr/internal/config.DefaultTMDBKey=…"
//
// A key the operator enters, or SNAGARR_TMDB_API_KEY, always wins over it.
var DefaultTMDBKey string

// TMDBSettings is the one catalogue key for the whole install. Every other
// integration belongs to a member and lives in the services table.
type TMDBSettings struct {
	APIKey string `json:"api_key"`
}

// Key resolves what to call TMDB with: the operator's own key when they set
// one, otherwise the embedded default.
func (s TMDBSettings) Key() string {
	if s.APIKey != "" {
		return s.APIKey
	}
	return DefaultTMDBKey
}

// GeneralSettings are the install-wide knobs.
type GeneralSettings struct {
	ReconcileInterval Duration `json:"reconcile_interval"`
	PublicURL         string   `json:"public_url"`

	// AutoSend hands a resolved capture to the capturer's own Radarr or Sonarr
	// without waiting for the Send button. It is on by default: snagging a
	// title nobody owns yet almost always means "get this".
	AutoSend bool `json:"auto_send"`
}

// Settings is what stays global once every integration belongs to a member:
// the catalogue key and the install-wide knobs.
type Settings struct {
	TMDB    TMDBSettings    `json:"tmdb"`
	General GeneralSettings `json:"general"`
}

// Configured reports whether TMDB can be searched.
func (s TMDBSettings) Configured() bool { return s.Key() != "" }

func defaults() Settings {
	return Settings{General: GeneralSettings{
		ReconcileInterval: Duration(15 * time.Minute),
		AutoSend:          true,
	}}
}

// secretFields maps the dotted path of every secret to a pointer into s, so
// masking, restoring and env overlays all work from one list.
func (s *Settings) secretFields() map[string]*string {
	return map[string]*string{"tmdb.api_key": &s.TMDB.APIKey}
}

const maskRunes = "••••"

// Mask hides a secret, keeping its last four characters so an operator can tell
// which key is stored.
func Mask(secret string) string {
	if secret == "" {
		return ""
	}
	if len(secret) > 4 {
		return maskRunes + secret[len(secret)-4:]
	}
	return maskRunes
}

// Masked returns a copy safe to send to a client.
func (s Settings) Masked() Settings {
	masked := s
	for _, field := range masked.secretFields() {
		*field = Mask(*field)
	}
	return masked
}

// IsMasked reports whether a client sent back a value it never actually saw.
func IsMasked(value string) bool { return strings.HasPrefix(value, maskRunes) }

// Manager owns the live settings: it loads them from the database, applies
// environment overrides on top, and persists updates from the settings UI.
type Manager struct {
	store *store.Store

	mu             sync.RWMutex
	current        Settings
	locked         map[string]bool
	lockedServices map[int64]bool
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

// LockedServices reports the services the environment owns, by ID. They are
// rewritten on every start, so the UI renders them read-only rather than
// letting an operator edit a value the next restart will overwrite.
func (m *Manager) LockedServices() map[int64]bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lockedServices
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

// overlayEnv lets a Docker-first operator configure the install without opening
// the UI. Environment values win over anything stored, and are reported back so
// the UI can show them as locked. The per-member services are seeded separately
// by SeedServices; only the global settings pass through here.
func overlayEnv(s *Settings) map[string]bool {
	locked := map[string]bool{}

	strs := map[string]*string{
		"SNAGARR_TMDB_API_KEY": &s.TMDB.APIKey,
		"SNAGARR_PUBLIC_URL":   &s.General.PublicURL,
	}
	paths := map[string]string{
		"SNAGARR_TMDB_API_KEY": "tmdb.api_key",
		"SNAGARR_PUBLIC_URL":   "general.public_url",
	}
	for name, field := range strs {
		if v := os.Getenv(name); v != "" {
			*field = v
			locked[paths[name]] = true
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
