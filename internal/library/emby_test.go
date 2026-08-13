package library

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sirrobot01/snagarr/internal/media"
)

const embyItemsFixture = `{"TotalRecordCount":2,"Items":[
	{"Id":"a1","Name":"Sinners","Type":"Movie","ProductionYear":2025,"DateCreated":"2025-04-20T10:00:00.0000000Z",
	 "ProviderIds":{"Tmdb":"1233413","Imdb":"tt31193180"}},
	{"Id":"b2","Name":"Severance","Type":"Series","ProductionYear":2022,"DateCreated":"2022-02-18T08:30:00Z",
	 "ProviderIds":{"TMDB":"95396","IMDB":"tt11280740","tvdb":"371980"}}
]}`

func TestEmbyItemsMapsProviderIDs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("Recursive") != "true" || q.Get("IncludeItemTypes") != "Movie,Series" {
			t.Errorf("query = %v, want a recursive movie/series sweep", q)
		}
		if q.Get("Limit") != "500" {
			t.Errorf("Limit = %q, want 500", q.Get("Limit"))
		}
		if q.Get("MinDateLastSaved") != "2025-01-01T00:00:00Z" {
			t.Errorf("MinDateLastSaved = %q", q.Get("MinDateLastSaved"))
		}
		if got := r.Header.Get("X-Emby-Token"); got != "key" {
			t.Errorf("token header = %q, want key", got)
		}
		w.Write([]byte(embyItemsFixture))
	}))
	defer srv.Close()

	since := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	items, err := NewEmby(srv.URL, "key", true).Items(context.Background(), nil, since)
	if err != nil {
		t.Fatalf("Items: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2: %+v", len(items), items)
	}
	first := items[0]
	if first.ProviderItemID != "a1" || first.TMDBID != 1233413 || first.IMDBID != "tt31193180" {
		t.Errorf("item 0 = %+v", first)
	}
	if first.Type != media.Movie || first.Year != 2025 || first.AddedAt.IsZero() {
		t.Errorf("item 0 = %+v", first)
	}
	second := items[1]
	if second.TMDBID != 95396 || second.IMDBID != "tt11280740" || second.TVDBID != 371980 {
		t.Errorf("item 1 = %+v, want case-insensitive provider ids", second)
	}
	if second.Type != media.TV {
		t.Errorf("item 1 type = %q, want tv", second.Type)
	}
}

func TestEmbySyncCollection(t *testing.T) {
	var added, removed string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		switch {
		case r.URL.Path == "/Items" && q.Get("IncludeItemTypes") == "BoxSet":
			w.Write([]byte(`{"Items":[{"Id":"col1","Name":"Snagged"}]}`))
		case r.URL.Path == "/Items" && q.Get("ParentId") == "col1":
			w.Write([]byte(`{"Items":[{"Id":"a","Name":"A","Type":"Movie"},{"Id":"b","Name":"B","Type":"Movie"}]}`))
		case r.Method == http.MethodPost && r.URL.Path == "/Collections/col1/Items":
			added = q.Get("Ids")
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodDelete && r.URL.Path == "/Collections/col1/Items":
			removed = q.Get("Ids")
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.RequestURI())
		}
	}))
	defer srv.Close()

	err := NewEmby(srv.URL, "key", false).SyncCollection(context.Background(), "Snagged", []string{"b", "c"})
	if err != nil {
		t.Fatalf("SyncCollection: %v", err)
	}
	if added != "c" {
		t.Errorf("added = %q, want c", added)
	}
	if removed != "a" {
		t.Errorf("removed = %q, want a", removed)
	}
}
