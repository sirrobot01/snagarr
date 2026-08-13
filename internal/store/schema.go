package store

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
}
