package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/sirrobot01/snagarr/internal/media"
)

// Status is an item's composite state. Everything except needs_review is
// derived by the reconcile loop from the local library and *arr indexes.
type Status string

const (
	StatusNeedsReview Status = "needs_review"
	StatusNew         Status = "new"
	StatusMonitored   Status = "monitored"
	StatusRequested   Status = "requested"
	StatusAvailable   Status = "available"
	StatusWatched     Status = "watched"
)

// Source records which client captured an item, so notifications can say where
// the intention came from.
type Source string

const (
	SourceWeb         Source = "web"
	SourceShortcut    Source = "shortcut"
	SourceTelegram    Source = "telegram"
	SourceBookmarklet Source = "bookmarklet"
	SourceAPI         Source = "api"
	SourceCLI         Source = "cli"
)

// Item is one captured intention. Zero values mean absent: TMDBID 0 while the
// item is unresolved, Year 0 when unknown, and zero times for the stamps that
// have not happened yet.
type Item struct {
	ID          int64
	TMDBID      int64
	MediaType   media.Type
	Title       string
	Year        int
	PosterPath  string
	Status      Status
	RawInput    string
	Source      Source
	SourceURL   string
	Note        string
	CapturedBy  int64
	CapturedAt  time.Time
	ResolvedAt  time.Time
	AvailableAt time.Time
	ArchivedAt  time.Time
	NotifiedAt  time.Time

	CapturedByName string
	CapturedByRole Role

	// Filled from the entity cache by the same query.
	Overview string
	Runtime  int
	Genres   []string
}

func (i *Item) Archived() bool { return !i.ArchivedAt.IsZero() }

// Candidate is a possible match kept against a needs_review item so an
// ambiguous capture is never lost.
type Candidate struct {
	TMDBID     int64
	MediaType  media.Type
	Title      string
	Year       int
	PosterPath string
	Overview   string
	Score      float64
}

// ItemFilter is the query behind GET /items. A zero field means "no filter",
// except Archived, which defaults to hiding archived items.
type ItemFilter struct {
	Status     Status
	MediaType  media.Type
	Query      string
	CapturedBy int64
	Archived   bool
	Limit      int
	Offset     int
}

func (s *Store) CreateItem(ctx context.Context, it *Item) error {
	if it.CapturedAt.IsZero() {
		it.CapturedAt = time.Now().UTC()
	}
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO items (tmdb_id, media_type, title, year, poster_path, status, raw_input,
			source, source_url, note, captured_by, captured_at, resolved_at, available_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		nullInt(it.TMDBID), it.MediaType, it.Title, nullInt(int64(it.Year)), nullStr(it.PosterPath),
		it.Status, it.RawInput, it.Source, nullStr(it.SourceURL), nullStr(it.Note),
		nullInt(it.CapturedBy), it.CapturedAt, nullTime(it.ResolvedAt), nullTime(it.AvailableAt))
	if err != nil {
		return fmt.Errorf("create item: %w", err)
	}
	it.ID, err = res.LastInsertId()
	return err
}

const itemColumns = `i.id, i.tmdb_id, i.media_type, i.title, i.year, i.poster_path, i.status,
	i.raw_input, i.source, i.source_url, i.note, i.captured_by, i.captured_at,
	i.resolved_at, i.available_at, i.archived_at, i.notified_at,
	u.display_name, u.role, e.overview, e.runtime, e.genres`

const itemFrom = ` FROM items i
	LEFT JOIN users u ON u.id = i.captured_by
	LEFT JOIN tmdb_entities e ON e.tmdb_id = i.tmdb_id AND e.media_type = i.media_type`

func scanItem(row interface{ Scan(...any) error }) (*Item, error) {
	var it Item
	var tmdbID, year, capturedBy, runtime sql.NullInt64
	var poster, sourceURL, note, name, role, overview, genres sql.NullString
	var resolved, available, archived, notified sql.NullTime

	if err := row.Scan(&it.ID, &tmdbID, &it.MediaType, &it.Title, &year, &poster, &it.Status,
		&it.RawInput, &it.Source, &sourceURL, &note, &capturedBy, &it.CapturedAt,
		&resolved, &available, &archived, &notified,
		&name, &role, &overview, &runtime, &genres); err != nil {
		return nil, err
	}

	it.TMDBID = tmdbID.Int64
	it.Year = int(year.Int64)
	it.PosterPath = poster.String
	it.SourceURL = sourceURL.String
	it.Note = note.String
	it.CapturedBy = capturedBy.Int64
	it.ResolvedAt = resolved.Time
	it.AvailableAt = available.Time
	it.ArchivedAt = archived.Time
	it.NotifiedAt = notified.Time
	it.CapturedByName = name.String
	it.CapturedByRole = Role(role.String)
	it.Overview = overview.String
	it.Runtime = int(runtime.Int64)
	if genres.String != "" {
		json.Unmarshal([]byte(genres.String), &it.Genres)
	}
	return &it, nil
}

func (s *Store) Item(ctx context.Context, id int64) (*Item, error) {
	it, err := scanItem(s.db.QueryRowContext(ctx, `SELECT `+itemColumns+itemFrom+` WHERE i.id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get item: %w", err)
	}
	return it, nil
}

// ItemByTMDB backs capture idempotency: snagging a title that is already on the
// list returns the existing item instead of creating a duplicate.
func (s *Store) ItemByTMDB(ctx context.Context, tmdbID int64, t media.Type) (*Item, error) {
	it, err := scanItem(s.db.QueryRowContext(ctx,
		`SELECT `+itemColumns+itemFrom+` WHERE i.tmdb_id = ? AND i.media_type = ?`, tmdbID, t))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get item by tmdb id: %w", err)
	}
	return it, nil
}

func (s *Store) Items(ctx context.Context, f ItemFilter) ([]Item, int, error) {
	var where []string
	var args []any

	if f.Archived {
		where = append(where, "i.archived_at IS NOT NULL")
	} else {
		where = append(where, "i.archived_at IS NULL")
	}
	if f.Status != "" {
		where = append(where, "i.status = ?")
		args = append(args, f.Status)
	}
	if f.MediaType != "" {
		where = append(where, "i.media_type = ?")
		args = append(args, f.MediaType)
	}
	if f.CapturedBy != 0 {
		where = append(where, "i.captured_by = ?")
		args = append(args, f.CapturedBy)
	}
	if f.Query != "" {
		where = append(where, "(i.title LIKE ? OR i.raw_input LIKE ?)")
		like := "%" + f.Query + "%"
		args = append(args, like, like)
	}
	clause := " WHERE " + strings.Join(where, " AND ")

	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM items i`+clause, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count items: %w", err)
	}

	if f.Limit <= 0 || f.Limit > 500 {
		f.Limit = 100
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+itemColumns+itemFrom+clause+` ORDER BY i.captured_at DESC, i.id DESC LIMIT ? OFFSET ?`,
		append(args, f.Limit, f.Offset)...)
	if err != nil {
		return nil, 0, fmt.Errorf("list items: %w", err)
	}
	defer rows.Close()

	items := []Item{}
	for rows.Next() {
		it, err := scanItem(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("list items: %w", err)
		}
		items = append(items, *it)
	}
	return items, total, rows.Err()
}

// SnaggedItems returns every unarchived item that has resolved to a TMDB ID.
// The reconcile loop walks this set to recompute state.
func (s *Store) SnaggedItems(ctx context.Context) ([]Item, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+itemColumns+itemFrom+` WHERE i.tmdb_id IS NOT NULL AND i.archived_at IS NULL`)
	if err != nil {
		return nil, fmt.Errorf("list snagged items: %w", err)
	}
	defer rows.Close()

	var items []Item
	for rows.Next() {
		it, err := scanItem(rows)
		if err != nil {
			return nil, fmt.Errorf("list snagged items: %w", err)
		}
		items = append(items, *it)
	}
	return items, rows.Err()
}

// Resolve points an item at a TMDB entry, which is also how a needs_review item
// leaves the queue.
func (s *Store) Resolve(ctx context.Context, id int64, c Candidate) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE items SET tmdb_id = ?, media_type = ?, title = ?, year = ?, poster_path = ?,
			status = CASE WHEN status = ? THEN ? ELSE status END, resolved_at = ?
		 WHERE id = ?`,
		c.TMDBID, c.MediaType, c.Title, nullInt(int64(c.Year)), nullStr(c.PosterPath),
		StatusNeedsReview, StatusNew, time.Now().UTC(), id)
	if err != nil {
		return fmt.Errorf("resolve item: %w", err)
	}
	return affected(res)
}

// SetStatus is the reconcile loop's only write against an item.
func (s *Store) SetStatus(ctx context.Context, id int64, status Status, availableAt time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE items SET status = ?, available_at = COALESCE(available_at, ?) WHERE id = ?`,
		status, nullTime(availableAt), id)
	if err != nil {
		return fmt.Errorf("set item status: %w", err)
	}
	return nil
}

func (s *Store) MarkNotified(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE items SET notified_at = ? WHERE id = ?`, time.Now().UTC(), id)
	return err
}

func (s *Store) SetArchived(ctx context.Context, id int64, archived bool) error {
	var at any
	if archived {
		at = time.Now().UTC()
	}
	res, err := s.db.ExecContext(ctx, `UPDATE items SET archived_at = ? WHERE id = ?`, at, id)
	if err != nil {
		return fmt.Errorf("archive item: %w", err)
	}
	return affected(res)
}

func (s *Store) DeleteItem(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM items WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete item: %w", err)
	}
	return affected(res)
}

// SetCandidates replaces the candidate set of a needs_review item.
func (s *Store) SetCandidates(ctx context.Context, itemID int64, candidates []Candidate) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("set candidates: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM candidates WHERE item_id = ?`, itemID); err != nil {
		return fmt.Errorf("set candidates: %w", err)
	}
	for _, c := range candidates {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO candidates (item_id, tmdb_id, media_type, title, year, poster_path, overview, score)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			itemID, c.TMDBID, c.MediaType, c.Title, nullInt(int64(c.Year)),
			nullStr(c.PosterPath), nullStr(c.Overview), c.Score); err != nil {
			return fmt.Errorf("set candidates: %w", err)
		}
	}
	return tx.Commit()
}

func (s *Store) Candidates(ctx context.Context, itemID int64) ([]Candidate, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT tmdb_id, media_type, title, year, poster_path, overview, score
		 FROM candidates WHERE item_id = ? ORDER BY score DESC`, itemID)
	if err != nil {
		return nil, fmt.Errorf("list candidates: %w", err)
	}
	defer rows.Close()

	var candidates []Candidate
	for rows.Next() {
		var c Candidate
		var year sql.NullInt64
		var poster, overview sql.NullString
		if err := rows.Scan(&c.TMDBID, &c.MediaType, &c.Title, &year, &poster, &overview, &c.Score); err != nil {
			return nil, fmt.Errorf("list candidates: %w", err)
		}
		c.Year = int(year.Int64)
		c.PosterPath = poster.String
		c.Overview = overview.String
		candidates = append(candidates, c)
	}
	return candidates, rows.Err()
}

// MarkWatched records a playback event. MVP semantics are "watched by anyone
// removes it from the Snagged collection"; the per-user rows are already here
// for when that changes.
func (s *Store) MarkWatched(ctx context.Context, itemID, userID int64, source string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO item_watches (item_id, user_id, watched_at, source) VALUES (?, ?, ?, ?)`,
		itemID, nullInt(userID), time.Now().UTC(), source)
	if err != nil {
		return fmt.Errorf("mark watched: %w", err)
	}
	return nil
}

func (s *Store) WatchedItemIDs(ctx context.Context) (map[int64]bool, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT item_id FROM item_watches`)
	if err != nil {
		return nil, fmt.Errorf("list watched items: %w", err)
	}
	defer rows.Close()

	watched := map[int64]bool{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		watched[id] = true
	}
	return watched, rows.Err()
}

// Counts drives the filter chips and the status endpoint.
type Counts struct {
	Total       int `json:"total"`
	Ready       int `json:"ready"`
	Pending     int `json:"pending"`
	NeedsReview int `json:"needs_review"`
	Archived    int `json:"archived"`
}

func (s *Store) Counts(ctx context.Context) (Counts, error) {
	var c Counts
	err := s.db.QueryRowContext(ctx, `SELECT
		COUNT(*) FILTER (WHERE archived_at IS NULL),
		COUNT(*) FILTER (WHERE archived_at IS NULL AND status IN (?, ?)),
		COUNT(*) FILTER (WHERE archived_at IS NULL AND status IN (?, ?, ?)),
		COUNT(*) FILTER (WHERE archived_at IS NULL AND status = ?),
		COUNT(*) FILTER (WHERE archived_at IS NOT NULL)
		FROM items`,
		StatusAvailable, StatusWatched,
		StatusNew, StatusMonitored, StatusRequested,
		StatusNeedsReview).
		Scan(&c.Total, &c.Ready, &c.Pending, &c.NeedsReview, &c.Archived)
	if err != nil {
		return c, fmt.Errorf("count items: %w", err)
	}
	return c, nil
}
