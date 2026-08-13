package integration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sirrobot01/snagarr/internal/store"
)

// Plex always sends a plain "guid" string and adds the "Guid" array only when
// includeGuids is set. Both decode into the same field.
const plexItemsFixture = `{"MediaContainer":{"size":4,"Metadata":[
	{"ratingKey":"101","type":"movie","title":"Sinners","year":2025,"addedAt":1700000000,
	 "guid":"plex://movie/5d776","Guid":[{"id":"tmdb://1233413"},{"id":"imdb://tt31193180"},{"id":"tvdb://456"}]},
	{"ratingKey":"202","type":"show","title":"Severance","year":2022,"addedAt":1600000000,
	 "Guid":[{"id":"tvdb://371980/1/2"},{"id":"plex://show/5d9c0"}],"guid":"plex://show/5d9c0"},
	{"ratingKey":"404","type":"movie","title":"Heat","year":1995,"addedAt":1500000000,
	 "guid":"com.plexapp.agents.imdb://tt0113277?lang=en"},
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
	want := []LibraryItem{
		{
			ProviderItemID: "101", SectionID: "1", TMDBID: 1233413, IMDBID: "tt31193180", TVDBID: 456,
			Type: store.Movie, Title: "Sinners", Year: 2025, AddedAt: time.Unix(1700000000, 0).UTC(),
		},
		{
			ProviderItemID: "202", SectionID: "1", TVDBID: 371980,
			Type: store.TV, Title: "Severance", Year: 2022, AddedAt: time.Unix(1600000000, 0).UTC(),
		},
		{
			ProviderItemID: "404", SectionID: "1", IMDBID: "tt0113277",
			Type: store.Movie, Title: "Heat", Year: 1995, AddedAt: time.Unix(1500000000, 0).UTC(),
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

// A Plex collection belongs to one section and takes only that section's type,
// so a mixed list has to become one collection per section. Sending a show to
// the movie collection is what produced a bare HTML 400.
func TestPlexSyncCollectionSplitsBySection(t *testing.T) {
	added := map[string]string{}
	var created []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/library/sections":
			w.Write([]byte(`{"MediaContainer":{"Directory":[
				{"key":"1","title":"Movies","type":"movie"},
				{"key":"2","title":"Shows","type":"show"}]}}`))
		case r.URL.Path == "/identity":
			w.Write([]byte(`{"MediaContainer":{"machineIdentifier":"abc123"}}`))

		// Only the movie section already holds a Snagged collection.
		case r.URL.Path == "/library/sections/1/collections":
			w.Write([]byte(`{"MediaContainer":{"Metadata":[{"ratingKey":"900","title":"Snagged"}]}}`))
		case r.URL.Path == "/library/sections/2/collections":
			if len(created) == 0 {
				w.Write([]byte(`{"MediaContainer":{"Metadata":[]}}`))
				return
			}
			w.Write([]byte(`{"MediaContainer":{"Metadata":[{"ratingKey":"901","title":"Snagged"}]}}`))

		case r.URL.Path == "/library/collections/900/children":
			w.Write([]byte(`{"MediaContainer":{"Metadata":[]}}`))
		// Creating seeded this one, so it already holds the show.
		case r.URL.Path == "/library/collections/901/children":
			w.Write([]byte(`{"MediaContainer":{"Metadata":[{"ratingKey":"202"}]}}`))

		case r.Method == http.MethodPost && r.URL.Path == "/library/collections":
			q := r.URL.Query()
			// The type has to match the section, or Plex rejects the members.
			if q.Get("sectionId") == "2" && q.Get("type") != strconv.Itoa(plexShowType) {
				t.Errorf("show collection created with type %q, want %d", q.Get("type"), plexShowType)
			}
			created = append(created, q.Get("sectionId"))
			w.Write([]byte(`{}`))

		case r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/items"):
			key := strings.Split(r.URL.Path, "/")[3]
			added[key] = r.URL.Query().Get("uri")
			w.Write([]byte(`{}`))

		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	members := []CollectionMember{
		{ID: "101", SectionID: "1"},
		{ID: "102", SectionID: "1"},
		{ID: "202", SectionID: "2"},
	}
	if err := NewPlex(srv.URL, "tok").SyncCollection(context.Background(), "Snagged", members); err != nil {
		t.Fatalf("SyncCollection: %v", err)
	}

	// The movie keys go to the movie collection and nothing else does.
	if got := added["900"]; !strings.HasSuffix(got, "/101,102") {
		t.Errorf("movie collection took %q, want it to end in /101,102", got)
	}
	if strings.Contains(added["900"], "202") {
		t.Errorf("movie collection took the show key: %q", added["900"])
	}
	// The show section had no collection, so one was created for it.
	if len(created) != 1 || created[0] != "2" {
		t.Errorf("created = %v, want one collection in section 2", created)
	}
}

// A section whose titles have all gone still has to be visited, or the last one
// out never leaves the collection.
func TestPlexSyncCollectionRemovesFromEmptiedSection(t *testing.T) {
	var removed []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/library/sections":
			w.Write([]byte(`{"MediaContainer":{"Directory":[{"key":"1","title":"Movies","type":"movie"}]}}`))
		case r.URL.Path == "/identity":
			w.Write([]byte(`{"MediaContainer":{"machineIdentifier":"abc123"}}`))
		case r.URL.Path == "/library/sections/1/collections":
			w.Write([]byte(`{"MediaContainer":{"Metadata":[{"ratingKey":"900","title":"Snagged"}]}}`))
		case r.URL.Path == "/library/collections/900/children":
			w.Write([]byte(`{"MediaContainer":{"Metadata":[{"ratingKey":"101"}]}}`))
		case r.Method == http.MethodDelete:
			removed = append(removed, strings.TrimPrefix(r.URL.Path, "/library/collections/900/children/"))
			w.Write([]byte(`{}`))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	if err := NewPlex(srv.URL, "tok").SyncCollection(context.Background(), "Snagged", nil); err != nil {
		t.Fatalf("SyncCollection: %v", err)
	}
	if len(removed) != 1 || removed[0] != "101" {
		t.Errorf("removed = %v, want [101]", removed)
	}
}
