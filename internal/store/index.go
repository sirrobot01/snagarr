package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/sirrobot01/snagarr/internal/media"
)

// TitleKey identifies a title across every index. TMDB is the primary key
// everywhere; rows that arrive with only a TVDB or IMDB ID are matched during
// the sync that writes them, never at read time.
type TitleKey struct {
	TMDBID    int64
	MediaType media.Type
}

// LibraryEntry mirrors one title held by the media server.
type LibraryEntry struct {
	Provider       string
	ProviderItemID string
	TMDBID         int64
	IMDBID         string
	TVDBID         int64
	MediaType      media.Type
	Title          string
	Year           int
	AddedAt        time.Time
}

// ArrEntry mirrors one title monitored by Radarr or Sonarr.
type ArrEntry struct {
	Source           string
	ArrID            int
	TMDBID           int64
	TVDBID           int64
	IMDBID           string
	Title            string
	Year             int
	Monitored        bool
	HasFile          bool
	QualityProfileID int
}

// RequestEntry mirrors one Overseerr request.
type RequestEntry struct {
	RequestID int
	TMDBID    int64
	MediaType media.Type
	Status    string
}

// UpsertLibrary writes a batch of library titles and refreshes their
// last_seen_at, which is what TombstoneLibrary later uses to find deletions.
func (s *Store) UpsertLibrary(ctx context.Context, entries []LibraryEntry) error {
	if len(entries) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("sync library index: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op once Commit has run

	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO library_index
			(provider, provider_item_id, tmdb_id, imdb_id, tvdb_id, media_type, title, year, added_at, last_seen_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT (provider, provider_item_id) DO UPDATE SET
			tmdb_id = excluded.tmdb_id, imdb_id = excluded.imdb_id, tvdb_id = excluded.tvdb_id,
			media_type = excluded.media_type, title = excluded.title, year = excluded.year,
			last_seen_at = excluded.last_seen_at`)
	if err != nil {
		return fmt.Errorf("sync library index: %w", err)
	}
	defer stmt.Close()

	now := time.Now().UTC()
	for _, e := range entries {
		if _, err := stmt.ExecContext(ctx, e.Provider, e.ProviderItemID, nullInt(e.TMDBID),
			nullStr(e.IMDBID), nullInt(e.TVDBID), e.MediaType, e.Title,
			nullInt(int64(e.Year)), nullTime(e.AddedAt), now); err != nil {
			return fmt.Errorf("sync library index: %w", err)
		}
	}
	return tx.Commit()
}

// TombstoneLibrary drops titles the last full sweep did not see. Only a full
// sweep may call this; an incremental sync would delete the whole library.
func (s *Store) TombstoneLibrary(ctx context.Context, provider string, sweepStart time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM library_index WHERE provider = ? AND last_seen_at < ?`, provider, sweepStart.UTC())
	if err != nil {
		return 0, fmt.Errorf("tombstone library index: %w", err)
	}
	return res.RowsAffected()
}

// ReplaceArrIndex swaps the whole mirror for one service. Radarr and Sonarr
// each return their full list in a single call, so a replace is simpler and
// cheaper than diffing.
func (s *Store) ReplaceArrIndex(ctx context.Context, source string, entries []ArrEntry) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("sync arr index: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op once Commit has run

	if _, err := tx.ExecContext(ctx, `DELETE FROM arr_index WHERE source = ?`, source); err != nil {
		return fmt.Errorf("sync arr index: %w", err)
	}
	now := time.Now().UTC()
	for _, e := range entries {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO arr_index (source, arr_id, tmdb_id, tvdb_id, imdb_id, title, year,
				monitored, has_file, quality_profile_id, synced_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			source, e.ArrID, nullInt(e.TMDBID), nullInt(e.TVDBID), nullStr(e.IMDBID),
			e.Title, nullInt(int64(e.Year)), e.Monitored, e.HasFile,
			nullInt(int64(e.QualityProfileID)), now); err != nil {
			return fmt.Errorf("sync arr index: %w", err)
		}
	}
	return tx.Commit()
}

func (s *Store) ReplaceRequests(ctx context.Context, entries []RequestEntry) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("sync request index: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op once Commit has run

	if _, err := tx.ExecContext(ctx, `DELETE FROM request_index`); err != nil {
		return fmt.Errorf("sync request index: %w", err)
	}
	now := time.Now().UTC()
	for _, e := range entries {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO request_index (request_id, tmdb_id, media_type, status, synced_at)
			 VALUES (?, ?, ?, ?, ?)`,
			e.RequestID, e.TMDBID, e.MediaType, e.Status, now); err != nil {
			return fmt.Errorf("sync request index: %w", err)
		}
	}
	return tx.Commit()
}

// StateIndex is the whole local picture in memory. The reconcile loop loads it
// once and answers every item's state as set arithmetic, so recomputing state
// never touches an external API.
type StateIndex struct {
	Library  map[TitleKey]string
	Arr      map[TitleKey]ArrEntry
	Requests map[TitleKey]string
}

func (s *Store) LoadStateIndex(ctx context.Context) (*StateIndex, error) {
	idx := &StateIndex{
		Library:  map[TitleKey]string{},
		Arr:      map[TitleKey]ArrEntry{},
		Requests: map[TitleKey]string{},
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT tmdb_id, media_type, provider_item_id FROM library_index WHERE tmdb_id IS NOT NULL`)
	if err != nil {
		return nil, fmt.Errorf("load state index: %w", err)
	}
	for rows.Next() {
		var k TitleKey
		var providerItemID string
		if err := rows.Scan(&k.TMDBID, &k.MediaType, &providerItemID); err != nil {
			rows.Close()
			return nil, fmt.Errorf("load state index: %w", err)
		}
		idx.Library[k] = providerItemID
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	rows, err = s.db.QueryContext(ctx,
		`SELECT source, arr_id, tmdb_id, tvdb_id, monitored, has_file, quality_profile_id
		 FROM arr_index WHERE tmdb_id IS NOT NULL`)
	if err != nil {
		return nil, fmt.Errorf("load state index: %w", err)
	}
	for rows.Next() {
		var e ArrEntry
		var tvdb, profile sql.NullInt64
		if err := rows.Scan(&e.Source, &e.ArrID, &e.TMDBID, &tvdb, &e.Monitored, &e.HasFile, &profile); err != nil {
			rows.Close()
			return nil, fmt.Errorf("load state index: %w", err)
		}
		e.TVDBID = tvdb.Int64
		e.QualityProfileID = int(profile.Int64)
		t := media.Movie
		if e.Source == "sonarr" {
			t = media.TV
		}
		idx.Arr[TitleKey{TMDBID: e.TMDBID, MediaType: t}] = e
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	rows, err = s.db.QueryContext(ctx, `SELECT tmdb_id, media_type, status FROM request_index`)
	if err != nil {
		return nil, fmt.Errorf("load state index: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var k TitleKey
		var status string
		if err := rows.Scan(&k.TMDBID, &k.MediaType, &status); err != nil {
			return nil, fmt.Errorf("load state index: %w", err)
		}
		idx.Requests[k] = status
	}
	return idx, rows.Err()
}

// State answers one title's composite state from the local indexes alone.
func (idx *StateIndex) State(k TitleKey) Status {
	if _, ok := idx.Library[k]; ok {
		return StatusAvailable
	}
	if e, ok := idx.Arr[k]; ok {
		if e.HasFile {
			return StatusAvailable
		}
		if e.Monitored {
			return StatusMonitored
		}
	}
	if status, ok := idx.Requests[k]; ok {
		if status == "available" {
			return StatusAvailable
		}
		return StatusRequested
	}
	return StatusNew
}

// SearchLibrary matches the user's own library first, so they discover they
// already own something before snagging a duplicate.
func (s *Store) SearchLibrary(ctx context.Context, query string, limit int) ([]LibraryEntry, error) {
	match := ftsQuery(query)
	if match == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 10
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT l.provider, l.provider_item_id, l.tmdb_id, l.imdb_id, l.tvdb_id,
			l.media_type, l.title, l.year
		 FROM library_fts
		 JOIN library_index l ON l.id = library_fts.rowid
		 WHERE library_fts MATCH ? AND l.tmdb_id IS NOT NULL
		 ORDER BY bm25(library_fts) LIMIT ?`, match, limit)
	if err != nil {
		return nil, fmt.Errorf("search library: %w", err)
	}
	defer rows.Close()

	var entries []LibraryEntry
	for rows.Next() {
		var e LibraryEntry
		var tmdb, tvdb, year sql.NullInt64
		var imdb sql.NullString
		if err := rows.Scan(&e.Provider, &e.ProviderItemID, &tmdb, &imdb, &tvdb,
			&e.MediaType, &e.Title, &year); err != nil {
			return nil, fmt.Errorf("search library: %w", err)
		}
		e.TMDBID, e.TVDBID, e.Year, e.IMDBID = tmdb.Int64, tvdb.Int64, int(year.Int64), imdb.String
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (s *Store) LibraryCount(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM library_index`).Scan(&n)
	return n, err
}

// ftsQuery turns free text into a safe FTS5 prefix query. FTS5 treats most
// punctuation as syntax, so every token is quoted and the last one gets a
// prefix star to support search-as-you-type.
func ftsQuery(query string) string {
	fields := strings.Fields(query)
	terms := make([]string, 0, len(fields))
	for _, f := range fields {
		f = strings.Map(func(r rune) rune {
			if r == '"' || r == '*' || r == '(' || r == ')' || r == ':' || r == '^' {
				return -1
			}
			return r
		}, f)
		if f != "" {
			terms = append(terms, `"`+f+`"`)
		}
	}
	if len(terms) == 0 {
		return ""
	}
	terms[len(terms)-1] += "*"
	return strings.Join(terms, " ")
}
