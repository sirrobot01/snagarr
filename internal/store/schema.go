package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// migrations are applied in order and tracked with PRAGMA user_version.
// Never edit an applied migration; append a new one instead.
var migrations = []string{
	`
CREATE TABLE users (
    id               INTEGER PRIMARY KEY,
    display_name     TEXT    NOT NULL,
    role             TEXT    NOT NULL,
    telegram_user_id INTEGER UNIQUE,
    created_at       TIMESTAMP NOT NULL
);

CREATE TABLE tokens (
    id           INTEGER PRIMARY KEY,
    user_id      INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name         TEXT    NOT NULL,
    token_hash   TEXT    NOT NULL UNIQUE,
    prefix       TEXT    NOT NULL,
    created_at   TIMESTAMP NOT NULL,
    last_used_at TIMESTAMP,
    revoked      INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX tokens_user ON tokens(user_id);

CREATE TABLE items (
    id           INTEGER PRIMARY KEY,
    tmdb_id      INTEGER,
    media_type   TEXT    NOT NULL DEFAULT '',
    title        TEXT    NOT NULL,
    year         INTEGER,
    poster_path  TEXT,
    status       TEXT    NOT NULL,
    raw_input    TEXT    NOT NULL,
    source       TEXT    NOT NULL,
    source_url   TEXT,
    note         TEXT,
    captured_by  INTEGER REFERENCES users(id) ON DELETE SET NULL,
    captured_at  TIMESTAMP NOT NULL,
    resolved_at  TIMESTAMP,
    available_at TIMESTAMP,
    archived_at  TIMESTAMP,
    notified_at  TIMESTAMP
);
CREATE UNIQUE INDEX items_tmdb ON items(tmdb_id, media_type) WHERE tmdb_id IS NOT NULL;
CREATE INDEX items_status ON items(status);

CREATE TABLE item_watches (
    id         INTEGER PRIMARY KEY,
    item_id    INTEGER NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    user_id    INTEGER REFERENCES users(id) ON DELETE SET NULL,
    watched_at TIMESTAMP NOT NULL,
    source     TEXT NOT NULL
);
CREATE INDEX item_watches_item ON item_watches(item_id);

CREATE TABLE candidates (
    id          INTEGER PRIMARY KEY,
    item_id     INTEGER NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    tmdb_id     INTEGER NOT NULL,
    media_type  TEXT    NOT NULL,
    title       TEXT    NOT NULL,
    year        INTEGER,
    poster_path TEXT,
    overview    TEXT,
    score       REAL    NOT NULL
);
CREATE INDEX candidates_item ON candidates(item_id);

CREATE TABLE library_index (
    id               INTEGER PRIMARY KEY,
    provider         TEXT NOT NULL,
    provider_item_id TEXT NOT NULL,
    tmdb_id          INTEGER,
    imdb_id          TEXT,
    tvdb_id          INTEGER,
    media_type       TEXT NOT NULL,
    title            TEXT NOT NULL,
    year             INTEGER,
    added_at         TIMESTAMP,
    last_seen_at     TIMESTAMP NOT NULL
);
CREATE UNIQUE INDEX library_index_item ON library_index(provider, provider_item_id);
CREATE INDEX library_index_tmdb ON library_index(tmdb_id, media_type);

CREATE VIRTUAL TABLE library_fts USING fts5(title, content='library_index', content_rowid='id');
CREATE TRIGGER library_index_ai AFTER INSERT ON library_index BEGIN
    INSERT INTO library_fts(rowid, title) VALUES (new.id, new.title);
END;
CREATE TRIGGER library_index_ad AFTER DELETE ON library_index BEGIN
    INSERT INTO library_fts(library_fts, rowid, title) VALUES ('delete', old.id, old.title);
END;
CREATE TRIGGER library_index_au AFTER UPDATE ON library_index BEGIN
    INSERT INTO library_fts(library_fts, rowid, title) VALUES ('delete', old.id, old.title);
    INSERT INTO library_fts(rowid, title) VALUES (new.id, new.title);
END;

CREATE TABLE arr_index (
    source             TEXT NOT NULL,
    arr_id             INTEGER NOT NULL,
    tmdb_id            INTEGER,
    tvdb_id            INTEGER,
    imdb_id            TEXT,
    title              TEXT NOT NULL,
    year               INTEGER,
    monitored          INTEGER NOT NULL,
    has_file           INTEGER NOT NULL,
    quality_profile_id INTEGER,
    synced_at          TIMESTAMP NOT NULL,
    PRIMARY KEY (source, arr_id)
);
CREATE INDEX arr_index_tmdb ON arr_index(tmdb_id);
CREATE INDEX arr_index_tvdb ON arr_index(tvdb_id);

CREATE TABLE request_index (
    request_id INTEGER PRIMARY KEY,
    tmdb_id    INTEGER NOT NULL,
    media_type TEXT NOT NULL,
    status     TEXT NOT NULL,
    synced_at  TIMESTAMP NOT NULL
);
CREATE INDEX request_index_tmdb ON request_index(tmdb_id, media_type);

CREATE TABLE tmdb_entities (
    tmdb_id       INTEGER NOT NULL,
    media_type    TEXT NOT NULL,
    title         TEXT NOT NULL,
    year          INTEGER,
    poster_path   TEXT,
    backdrop_path TEXT,
    overview      TEXT,
    genres        TEXT,
    runtime       INTEGER,
    popularity    REAL,
    fetched_at    TIMESTAMP NOT NULL,
    PRIMARY KEY (tmdb_id, media_type)
);

CREATE TABLE settings (
    key        TEXT PRIMARY KEY,
    value      BLOB NOT NULL,
    updated_at TIMESTAMP NOT NULL
);

CREATE TABLE http_cache (
    key        TEXT PRIMARY KEY,
    body       BLOB NOT NULL,
    expires_at TIMESTAMP NOT NULL
);
CREATE INDEX http_cache_expiry ON http_cache(expires_at);
`,

	// Integrations move from one global settings blob to one row per member per
	// service.
	`
CREATE TABLE services (
    id         INTEGER PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind       TEXT NOT NULL,
    name       TEXT NOT NULL,
    config     BLOB NOT NULL,
    enabled    INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);
CREATE UNIQUE INDEX services_user_kind_name ON services(user_id, kind, name);
CREATE INDEX services_user ON services(user_id);

-- The three index tables now belong to a service. Their rows are derived state
-- with no owner to attribute them to, so they are dropped rather than migrated:
-- the next reconcile pass rebuilds every one of them from the services.
DROP TRIGGER library_index_ai;
DROP TRIGGER library_index_ad;
DROP TRIGGER library_index_au;
DROP TABLE library_fts;
DROP TABLE library_index;

CREATE TABLE library_index (
    id               INTEGER PRIMARY KEY,
    service_id       INTEGER NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    provider_item_id TEXT NOT NULL,
    tmdb_id          INTEGER,
    imdb_id          TEXT,
    tvdb_id          INTEGER,
    media_type       TEXT NOT NULL,
    title            TEXT NOT NULL,
    year             INTEGER,
    added_at         TIMESTAMP,
    last_seen_at     TIMESTAMP NOT NULL
);
CREATE UNIQUE INDEX library_index_item ON library_index(service_id, provider_item_id);
CREATE INDEX library_index_tmdb ON library_index(tmdb_id, media_type);

CREATE VIRTUAL TABLE library_fts USING fts5(title, content='library_index', content_rowid='id');
CREATE TRIGGER library_index_ai AFTER INSERT ON library_index BEGIN
    INSERT INTO library_fts(rowid, title) VALUES (new.id, new.title);
END;
CREATE TRIGGER library_index_ad AFTER DELETE ON library_index BEGIN
    INSERT INTO library_fts(library_fts, rowid, title) VALUES ('delete', old.id, old.title);
END;
CREATE TRIGGER library_index_au AFTER UPDATE ON library_index BEGIN
    INSERT INTO library_fts(library_fts, rowid, title) VALUES ('delete', old.id, old.title);
    INSERT INTO library_fts(rowid, title) VALUES (new.id, new.title);
END;

DROP TABLE arr_index;
CREATE TABLE arr_index (
    service_id         INTEGER NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    arr_id             INTEGER NOT NULL,
    tmdb_id            INTEGER,
    tvdb_id            INTEGER,
    imdb_id            TEXT,
    title              TEXT NOT NULL,
    year               INTEGER,
    monitored          INTEGER NOT NULL,
    has_file           INTEGER NOT NULL,
    quality_profile_id INTEGER,
    synced_at          TIMESTAMP NOT NULL,
    PRIMARY KEY (service_id, arr_id)
);
CREATE INDEX arr_index_tmdb ON arr_index(tmdb_id);
CREATE INDEX arr_index_tvdb ON arr_index(tvdb_id);

DROP TABLE request_index;
CREATE TABLE request_index (
    service_id INTEGER NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    request_id INTEGER NOT NULL,
    tmdb_id    INTEGER NOT NULL,
    media_type TEXT NOT NULL,
    status     TEXT NOT NULL,
    synced_at  TIMESTAMP NOT NULL,
    PRIMARY KEY (service_id, request_id)
);
CREATE INDEX request_index_tmdb ON request_index(tmdb_id, media_type);
`,
}

// migrationSteps hold the half of a migration SQL cannot do. Splitting the
// settings blob needs the encryption key, which only the store has.
var migrationSteps = map[int]func(*Store, *sql.Tx) error{
	2: (*Store).splitSettingsIntoServices,
}

// settingsKey duplicates the key config.Manager persists under. The store
// cannot import config — config is built on top of it.
const settingsKey = "settings"

// serviceSections maps each per-user section of the old settings document to
// the kind it becomes. The library section names its own kind in "provider".
var serviceSections = map[string]ServiceKind{
	"radarr":    KindRadarr,
	"sonarr":    KindSonarr,
	"overseerr": KindOverseerr,
	"ntfy":      KindNtfy,
}

// splitSettingsIntoServices moves the configured integrations out of the global
// settings blob and onto the first admin, who is the operator that configured
// them. An install with nothing configured migrates to nothing.
func (s *Store) splitSettingsIntoServices(tx *sql.Tx) error {
	var sealed []byte
	err := tx.QueryRow(`SELECT value FROM settings WHERE key = ?`, settingsKey).Scan(&sealed)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read settings: %w", err)
	}
	plain, err := s.decrypt(sealed)
	if err != nil {
		return fmt.Errorf("decrypt settings: %w", err)
	}

	// The document is walked as raw JSON rather than as config.Settings, whose
	// shape has already moved on.
	var document map[string]json.RawMessage
	if err := json.Unmarshal(plain, &document); err != nil {
		return fmt.Errorf("decode settings: %w", err)
	}

	var adminID int64
	err = tx.QueryRow(`SELECT id FROM users WHERE role = ? ORDER BY id LIMIT 1`, RoleAdmin).Scan(&adminID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("find first admin: %w", err)
	}

	for section, kind := range serviceSections {
		config, ok := document[section]
		if !ok {
			continue
		}
		delete(document, section)
		if !configuredSection(kind, config) {
			continue
		}
		if err := s.insertMigratedService(tx, adminID, kind, config); err != nil {
			return err
		}
	}
	if config, ok := document["library"]; ok {
		delete(document, "library")
		kind, stripped := splitLibrarySection(config)
		if kind.Valid() && configuredSection(kind, stripped) {
			if err := s.insertMigratedService(tx, adminID, kind, stripped); err != nil {
				return err
			}
		}
	}

	rewritten, err := json.Marshal(document)
	if err != nil {
		return fmt.Errorf("encode settings: %w", err)
	}
	resealed, err := s.encrypt(rewritten)
	if err != nil {
		return fmt.Errorf("encrypt settings: %w", err)
	}
	if _, err := tx.Exec(`UPDATE settings SET value = ?, updated_at = ? WHERE key = ?`,
		resealed, time.Now().UTC(), settingsKey); err != nil {
		return fmt.Errorf("rewrite settings: %w", err)
	}
	return nil
}

func (s *Store) insertMigratedService(tx *sql.Tx, userID int64, kind ServiceKind, config []byte) error {
	sealed, err := s.encrypt(config)
	if err != nil {
		return fmt.Errorf("encrypt %s config: %w", kind, err)
	}
	now := time.Now().UTC()
	if _, err := tx.Exec(
		`INSERT INTO services (user_id, kind, name, config, enabled, created_at, updated_at)
		 VALUES (?, ?, ?, ?, 1, ?, ?)`,
		userID, kind, "Default", sealed, now, now); err != nil {
		return fmt.Errorf("migrate %s settings: %w", kind, err)
	}
	return nil
}

// splitLibrarySection reads the provider out of the old library section and
// returns the config without it: the kind is the service row's own column now.
func splitLibrarySection(config []byte) (ServiceKind, []byte) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(config, &fields); err != nil {
		return "", nil
	}
	var provider string
	if raw, ok := fields["provider"]; ok {
		_ = json.Unmarshal(raw, &provider)
	}
	delete(fields, "provider")
	stripped, err := json.Marshal(fields)
	if err != nil {
		return "", nil
	}
	return ServiceKind(provider), stripped
}

// configuredSection keeps half-filled sections out of the services table: a
// service row exists only where the operator supplied enough to connect.
func configuredSection(kind ServiceKind, config []byte) bool {
	var fields struct {
		URL    string `json:"url"`
		Token  string `json:"token"`
		APIKey string `json:"api_key"`
		Topic  string `json:"topic"`
	}
	if err := json.Unmarshal(config, &fields); err != nil {
		return false
	}
	switch {
	case kind.Library():
		return fields.URL != "" && fields.Token != ""
	case kind == KindNtfy:
		return fields.Topic != ""
	default:
		return fields.URL != "" && fields.APIKey != ""
	}
}
