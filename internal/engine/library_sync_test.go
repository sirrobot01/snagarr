package engine

import (
	"context"
	"testing"
	"time"

	"github.com/sirrobot01/snagarr/internal/config"
	"github.com/sirrobot01/snagarr/internal/integration"
	"github.com/sirrobot01/snagarr/internal/store"
)

type fakeLibrary struct {
	items     []integration.LibraryItem
	lastSince time.Time
}

func (f *fakeLibrary) Ping(context.Context) (string, error)                    { return "", nil }
func (f *fakeLibrary) Sections(context.Context) ([]integration.Section, error) { return nil, nil }
func (f *fakeLibrary) SyncCollection(context.Context, string, []integration.CollectionMember) error {
	return nil
}

func (f *fakeLibrary) Items(_ context.Context, _ []string, since time.Time) ([]integration.LibraryItem, error) {
	f.lastSince = since
	return f.items, nil
}

// The first pass sweeps in full, later passes ride the watermark, and a full
// sweep tombstones the titles the server no longer returns.
func TestSyncLibraryServiceSweepsAndTombstones(t *testing.T) {
	ctx := context.Background()
	e, db, owner := newTestReconciler(t)

	svc := &store.Service{UserID: owner, Kind: store.KindEmby, Name: "emby", Enabled: true,
		Config: []byte(`{"url":"http://localhost","api_key":"k"}`)}
	if err := db.CreateService(ctx, svc); err != nil {
		t.Fatalf("create service: %v", err)
	}

	title := func(id string, tmdb int) integration.LibraryItem {
		return integration.LibraryItem{ProviderItemID: id, TMDBID: tmdb,
			Type: store.Movie, Title: "t" + id}
	}
	fake := &fakeLibrary{items: []integration.LibraryItem{title("a", 1), title("b", 2)}}
	lib := integration.Library{Service: *svc, Config: config.LibraryConfig{}, Client: fake}

	e.syncLibraryService(ctx, lib)
	if !fake.lastSince.IsZero() {
		t.Errorf("first pass since = %v, want zero (full sweep)", fake.lastSince)
	}
	members, err := db.LibraryMembers(ctx)
	if err != nil {
		t.Fatalf("library members: %v", err)
	}
	if len(members[svc.ID]) != 2 {
		t.Fatalf("after first sweep members = %d, want 2", len(members[svc.ID]))
	}
	if e.state.Library[svc.ID].FullSweepAt.IsZero() {
		t.Fatal("full sweep did not stamp FullSweepAt")
	}

	// An incremental pass carries the watermark.
	e.syncLibraryService(ctx, lib)
	if fake.lastSince.IsZero() {
		t.Error("incremental pass since = zero, want the previous sweep start")
	}

	// The next full sweep no longer sees title b, so it must go.
	last := e.state.Library[svc.ID]
	last.FullSweepAt = time.Now().UTC().Add(-fullSweepInterval - time.Hour)
	e.state.Library[svc.ID] = last
	fake.items = fake.items[:1]

	e.syncLibraryService(ctx, lib)
	if members, err = db.LibraryMembers(ctx); err != nil {
		t.Fatalf("library members: %v", err)
	}
	got := members[svc.ID]
	if len(got) != 1 {
		t.Fatalf("after tombstone members = %d, want 1", len(got))
	}
	if _, ok := got[store.TitleKey{TMDBID: 1, MediaType: store.Movie}]; !ok {
		t.Error("surviving member is not title a")
	}
}
