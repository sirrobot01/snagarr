package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/sirrobot01/snagarr/internal/media"
)

// Entity is the cached TMDB metadata for one title. Snagged and library titles
// are always cached, so the list keeps working when TMDB is unreachable.
type Entity struct {
	TMDBID       int64
	MediaType    media.Type
	Title        string
	Year         int
	PosterPath   string
	BackdropPath string
	Overview     string
	Genres       []string
	Runtime      int
	Popularity   float64
	FetchedAt    time.Time
}

func (s *Store) PutEntity(ctx context.Context, e Entity) error {
	genres, err := json.Marshal(e.Genres)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO tmdb_entities (tmdb_id, media_type, title, year, poster_path, backdrop_path,
			overview, genres, runtime, popularity, fetched_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT (tmdb_id, media_type) DO UPDATE SET
			title = excluded.title, year = excluded.year, poster_path = excluded.poster_path,
			backdrop_path = excluded.backdrop_path, overview = excluded.overview,
			genres = excluded.genres, runtime = excluded.runtime,
			popularity = excluded.popularity, fetched_at = excluded.fetched_at`,
		e.TMDBID, e.MediaType, e.Title, nullInt(int64(e.Year)), nullStr(e.PosterPath),
		nullStr(e.BackdropPath), nullStr(e.Overview), string(genres),
		nullInt(int64(e.Runtime)), e.Popularity, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("cache entity: %w", err)
	}
	return nil
}

func (s *Store) Entity(ctx context.Context, tmdbID int64, t media.Type) (*Entity, error) {
	var e Entity
	var year, runtime sql.NullInt64
	var poster, backdrop, overview, genres sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT tmdb_id, media_type, title, year, poster_path, backdrop_path, overview,
			genres, runtime, popularity, fetched_at
		 FROM tmdb_entities WHERE tmdb_id = ? AND media_type = ?`, tmdbID, t).
		Scan(&e.TMDBID, &e.MediaType, &e.Title, &year, &poster, &backdrop, &overview,
			&genres, &runtime, &e.Popularity, &e.FetchedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get entity: %w", err)
	}
	e.Year, e.Runtime = int(year.Int64), int(runtime.Int64)
	e.PosterPath, e.BackdropPath, e.Overview = poster.String, backdrop.String, overview.String
	if genres.String != "" {
		json.Unmarshal([]byte(genres.String), &e.Genres)
	}
	return &e, nil
}

// StaleSnaggedTitles lists snagged titles whose cached metadata has aged past
// ttl, so the reconcile loop can refresh them.
func (s *Store) StaleSnaggedTitles(ctx context.Context, ttl time.Duration) ([]TitleKey, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT i.tmdb_id, i.media_type FROM items i
		 LEFT JOIN tmdb_entities e ON e.tmdb_id = i.tmdb_id AND e.media_type = i.media_type
		 WHERE i.tmdb_id IS NOT NULL AND (e.fetched_at IS NULL OR e.fetched_at < ?)`,
		time.Now().UTC().Add(-ttl))
	if err != nil {
		return nil, fmt.Errorf("list stale entities: %w", err)
	}
	defer rows.Close()

	var keys []TitleKey
	for rows.Next() {
		var k TitleKey
		if err := rows.Scan(&k.TMDBID, &k.MediaType); err != nil {
			return nil, fmt.Errorf("list stale entities: %w", err)
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

// HTTPCache is the response cache the TMDB client writes through. It is a
// separate type so Store does not grow bare Get and Set methods.
type HTTPCache struct{ store *Store }

func (s *Store) HTTPCache() *HTTPCache { return &HTTPCache{store: s} }

func (c *HTTPCache) Get(ctx context.Context, key string) ([]byte, bool) {
	var body []byte
	err := c.store.db.QueryRowContext(ctx,
		`SELECT body FROM http_cache WHERE key = ? AND expires_at > ?`, key, time.Now().UTC()).Scan(&body)
	if err != nil {
		return nil, false
	}
	return body, true
}

func (c *HTTPCache) Set(ctx context.Context, key string, body []byte, ttl time.Duration) error {
	_, err := c.store.db.ExecContext(ctx,
		`INSERT INTO http_cache (key, body, expires_at) VALUES (?, ?, ?)
		 ON CONFLICT (key) DO UPDATE SET body = excluded.body, expires_at = excluded.expires_at`,
		key, body, time.Now().UTC().Add(ttl))
	return err
}

func (s *Store) PurgeExpiredCache(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM http_cache WHERE expires_at < ?`, time.Now().UTC())
	return err
}
