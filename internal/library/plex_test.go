package library

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/sirrobot01/snagarr/internal/media"
)

const plexItemsFixture = `{"MediaContainer":{"size":3,"Metadata":[
	{"ratingKey":"101","type":"movie","title":"Sinners","year":2025,"addedAt":1700000000,
	 "Guid":[{"id":"tmdb://1233413"},{"id":"imdb://tt31193180"},{"id":"tvdb://456"}]},
	{"ratingKey":"202","type":"show","title":"Severance","year":2022,"addedAt":1600000000,
	 "Guid":[{"id":"tvdb://371980/1/2"},{"id":"plex://show/5d9c0"}]},
	{"ratingKey":"303","type":"artist","title":"Radiohead"}
]}}`

func TestPlexItemsParsesGuids(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/library/sections/1/all" {
			t.Errorf("path = %q, want /library/sections/1/all", got)
		}
		if got := r.URL.Query().Get("includeGuids"); got != "1" {
			t.Errorf("includeGuids = %q, want 1", got)
		}
		if got := r.URL.Query().Get("addedAt>="); got != "1700000000" {
			t.Errorf("addedAt>= = %q, want 1700000000", got)
		}
		if got := r.Header.Get("X-Plex-Token"); got != "tok" {
			t.Errorf("token header = %q, want tok", got)
		}
		w.Write([]byte(plexItemsFixture))
	}))
	defer srv.Close()

	since := time.Unix(1700000000, 0)
	items, err := NewPlex(srv.URL, "tok").Items(context.Background(), []string{"1"}, since)
	if err != nil {
		t.Fatalf("Items: %v", err)
	}
	want := []Item{
		{
			ProviderItemID: "101", TMDBID: 1233413, IMDBID: "tt31193180", TVDBID: 456,
			Type: media.Movie, Title: "Sinners", Year: 2025, AddedAt: time.Unix(1700000000, 0).UTC(),
		},
		{
			ProviderItemID: "202", TVDBID: 371980,
			Type: media.TV, Title: "Severance", Year: 2022, AddedAt: time.Unix(1600000000, 0).UTC(),
		},
	}
	if len(items) != len(want) {
		t.Fatalf("got %d items, want %d: %+v", len(items), len(want), items)
	}
	for i, w := range want {
		if items[i] != w {
			t.Errorf("item %d = %+v, want %+v", i, items[i], w)
		}
	}
}

func TestDiffMembers(t *testing.T) {
	cases := []struct {
		name             string
		current, desired []string
		wantAdd          []string
		wantRemove       []string
	}{
		{name: "empty"},
		{name: "create", desired: []string{"a", "b"}, wantAdd: []string{"a", "b"}},
		{name: "drain", current: []string{"a", "b"}, wantRemove: []string{"a", "b"}},
		{
			name:       "mixed",
			current:    []string{"a", "b", "c"},
			desired:    []string{"b", "c", "d"},
			wantAdd:    []string{"d"},
			wantRemove: []string{"a"},
		},
		{name: "duplicates in desired", current: []string{"a"}, desired: []string{"a", "b", "b"}, wantAdd: []string{"b"}},
		{name: "unchanged", current: []string{"a"}, desired: []string{"a"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			add, remove := diffMembers(tc.current, tc.desired)
			if !slices.Equal(add, tc.wantAdd) {
				t.Errorf("add = %v, want %v", add, tc.wantAdd)
			}
			if !slices.Equal(remove, tc.wantRemove) {
				t.Errorf("remove = %v, want %v", remove, tc.wantRemove)
			}
		})
	}
}
