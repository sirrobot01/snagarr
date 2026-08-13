package api

import (
	"time"

	"github.com/sirrobot01/snagarr/internal/store"
)

type userRef struct {
	ID          int64      `json:"id"`
	DisplayName string     `json:"display_name"`
	Role        store.Role `json:"role"`
}

type candidateDTO struct {
	TMDBID     int64           `json:"tmdb_id"`
	MediaType  store.MediaType `json:"media_type"`
	Title      string          `json:"title"`
	Year       *int            `json:"year"`
	PosterPath *string         `json:"poster_path"`
	Overview   *string         `json:"overview"`
	Score      float64         `json:"score"`
}

type itemDTO struct {
	ID          int64           `json:"id"`
	TMDBID      *int64          `json:"tmdb_id"`
	MediaType   store.MediaType `json:"media_type"`
	Title       string          `json:"title"`
	Year        *int            `json:"year"`
	PosterPath  *string         `json:"poster_path"`
	Status      store.Status    `json:"status"`
	Archived    bool            `json:"archived"`
	RawInput    string          `json:"raw_input"`
	Source      store.Source    `json:"source"`
	SourceURL   *string         `json:"source_url"`
	Note        *string         `json:"note"`
	CapturedBy  *userRef        `json:"captured_by"`
	CapturedAt  time.Time       `json:"captured_at"`
	ResolvedAt  *time.Time      `json:"resolved_at"`
	AvailableAt *time.Time      `json:"available_at"`
	Overview    *string         `json:"overview"`
	Runtime     *int            `json:"runtime"`
	Genres      []string        `json:"genres"`
	Candidates  []candidateDTO  `json:"candidates"`
}

func newItemDTO(it store.Item) itemDTO {
	dto := itemDTO{
		ID: it.ID, TMDBID: nullableInt(it.TMDBID), MediaType: it.MediaType, Title: it.Title,
		Year: nullableCount(it.Year), PosterPath: nullableStr(it.PosterPath),
		Status: it.Status, Archived: it.Archived(), RawInput: it.RawInput, Source: it.Source,
		SourceURL: nullableStr(it.SourceURL), Note: nullableStr(it.Note),
		CapturedAt: it.CapturedAt, ResolvedAt: nullableTime(it.ResolvedAt),
		AvailableAt: nullableTime(it.AvailableAt), Overview: nullableStr(it.Overview),
		Runtime: nullableCount(it.Runtime), Genres: it.Genres,
	}
	if it.CapturedBy != 0 {
		dto.CapturedBy = &userRef{ID: it.CapturedBy, DisplayName: it.CapturedByName, Role: it.CapturedByRole}
	}
	return dto
}

func newCandidateDTO(c store.Candidate) candidateDTO {
	return candidateDTO{
		TMDBID: c.TMDBID, MediaType: c.MediaType, Title: c.Title,
		Year: nullableCount(c.Year), PosterPath: nullableStr(c.PosterPath),
		Overview: nullableStr(c.Overview), Score: c.Score,
	}
}

// searchResult carries the composite state so the UI can badge a row before the
// user adds it.
type searchResult struct {
	TMDBID    int64           `json:"tmdb_id"`
	MediaType store.MediaType `json:"media_type"`
	Title     string          `json:"title"`
	Year      *int            `json:"year"`
	Poster    *string         `json:"poster_path"`
	Overview  *string         `json:"overview"`
	State     store.Status    `json:"state"`
	ItemID    *int64          `json:"item_id"`
	From      string          `json:"from"`
}

// The API distinguishes "absent" from "zero", so absent fields travel as null.
func nullableInt(v int64) *int64 {
	if v == 0 {
		return nil
	}
	return &v
}

func nullableCount(v int) *int {
	if v == 0 {
		return nil
	}
	return &v
}

func nullableStr(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

func nullableTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}
