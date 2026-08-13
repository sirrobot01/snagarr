package tmdb

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sirrobot01/snagarr/internal/media"
)

const searchFixture = `{"results":[
	{"id":1233413,"media_type":"movie","title":"Sinners","release_date":"2025-04-16","poster_path":"/a.jpg","overview":"Twins","popularity":412.5},
	{"id":95396,"media_type":"tv","name":"Severance","first_air_date":"2022-02-17","poster_path":"/b.jpg","overview":"Work","popularity":88.1},
	{"id":500,"media_type":"person","name":"Tom Cruise"},
	{"id":777,"media_type":"movie","title":"Unreleased","release_date":""}
]}`

func TestSearchMulti(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/search/multi" {
			t.Errorf("path = %q, want /search/multi", got)
		}
		if got := r.URL.Query().Get("api_key"); got != "key" {
			t.Errorf("api_key = %q, want key", got)
		}
		w.Write([]byte(searchFixture))
	}))
	defer srv.Close()

	c := New("key", nil)
	c.rest.BaseURL = srv.URL

	results, err := c.SearchMulti(context.Background(), "sinners")
	if err != nil {
		t.Fatalf("SearchMulti: %v", err)
	}
	want := []Result{
		{TMDBID: 1233413, Type: media.Movie, Title: "Sinners", Year: 2025, PosterPath: "/a.jpg", Overview: "Twins", Popularity: 412.5},
		{TMDBID: 95396, Type: media.TV, Title: "Severance", Year: 2022, PosterPath: "/b.jpg", Overview: "Work", Popularity: 88.1},
		{TMDBID: 777, Type: media.Movie, Title: "Unreleased"},
	}
	if len(results) != len(want) {
		t.Fatalf("got %d results, want %d: %+v", len(results), len(want), results)
	}
	for i, w := range want {
		if results[i] != w {
			t.Errorf("result %d = %+v, want %+v", i, results[i], w)
		}
	}
}

type memCache struct {
	entries map[string][]byte
}

func (m *memCache) Get(_ context.Context, key string) ([]byte, bool) {
	body, ok := m.entries[key]
	return body, ok
}

func (m *memCache) Set(_ context.Context, key string, body []byte, _ time.Duration) error {
	m.entries[key] = body
	return nil
}

func TestSearchMultiUsesCache(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Write([]byte(searchFixture))
	}))
	defer srv.Close()

	cache := &memCache{entries: map[string][]byte{}}
	c := New("key", cache)
	c.rest.BaseURL = srv.URL

	for i := 0; i < 2; i++ {
		if _, err := c.SearchMulti(context.Background(), "sinners"); err != nil {
			t.Fatalf("SearchMulti %d: %v", i, err)
		}
	}
	if calls != 1 {
		t.Errorf("http calls = %d, want 1", calls)
	}
	for key := range cache.entries {
		if strings.Contains(key, "api_key") {
			t.Errorf("cache key %q leaks the api key", key)
		}
	}
}
