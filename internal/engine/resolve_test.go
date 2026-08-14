package engine

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/sirrobot01/snagarr/internal/store"
)

// A capture that resolves to a title already on the list must yield to the
// existing item, including when the duplicate is only caught by the unique
// index because two resolutions raced.
func TestCommitYieldsToExistingItem(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(filepath.Join(t.TempDir(), "snagarr.db"), make([]byte, 32))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	r := NewResolver(db, slog.New(slog.NewTextHandler(io.Discard, nil)))

	winner := &store.Item{Title: "Sinners", RawInput: "sinners", Status: store.StatusNew,
		Source: store.SourceWeb, TMDBID: 1233413, MediaType: store.Movie}
	if err := db.CreateItem(ctx, winner); err != nil {
		t.Fatalf("seed winner: %v", err)
	}
	loser := &store.Item{Title: "sinners movie", RawInput: "sinners movie",
		Status: store.StatusNeedsReview, Source: store.SourceWeb}
	if err := db.CreateItem(ctx, loser); err != nil {
		t.Fatalf("seed loser: %v", err)
	}

	c := store.Candidate{TMDBID: 1233413, MediaType: store.Movie, Title: "Sinners", Score: 1}
	if err := r.commit(ctx, nil, loser, c); err != nil {
		t.Fatalf("commit: %v", err)
	}

	if _, err := db.Item(ctx, loser.ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("loser lookup error = %v, want ErrNotFound", err)
	}
	kept, err := db.Item(ctx, winner.ID)
	if err != nil {
		t.Fatalf("winner lookup: %v", err)
	}
	if kept.TMDBID != 1233413 {
		t.Errorf("winner tmdb id = %d, want 1233413", kept.TMDBID)
	}
}
